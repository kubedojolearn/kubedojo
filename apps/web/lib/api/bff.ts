import "server-only";
import { AUDIENCE_API, mintAssertion } from "@/lib/auth/assertion";
import { auth } from "@/lib/auth/config";
import { serverEnv } from "@/lib/server-env";

/**
 * Backend-for-frontend proxy: browser → /api/bff/v1/<path> → Go /api/v1/<path>.
 * - Requires an Auth.js session; mints a ≤60 s assertion per request (never sent to the browser).
 * - Mutations must come from WEB_ORIGIN with a JSON body (CSRF, threat T7).
 * - Only allowlisted API paths are reachable; provisioning and WebSockets are not.
 * - Forwards a minimal header set; cookies never reach the API.
 */

const MAX_BODY_BYTES = 256 * 1024;
const MUTATING = new Set(["POST", "PUT", "PATCH", "DELETE"]);

/** Go paths (after /api/v1/) the browser may reach through the BFF. */
const ALLOWED_PATHS: RegExp[] = [
  /^users\/me(\/(progress|achievements|activity|export|deletion))?$/,
  /^profiles\/[A-Za-z0-9-]{1,39}$/,
  /^leaderboard$/,
  /^catalog\/courses(\/[a-z0-9-]+(\/lessons\/[a-z0-9-]+)?)?$/,
  /^labs(\/[0-9a-f-]{36}(\/(resume|reset|finish|snapshot|events|commands|files|answers|validate|ws-ticket|hints\/\d))?)?$/,
  /^admin\/(content(\/(validate|publish))?|audit)$/,
];

const FORWARD_REQUEST_HEADERS = ["content-type", "idempotency-key", "accept"];
const FORWARD_RESPONSE_HEADERS = ["content-type", "retry-after", "x-request-id"];

function envelope(status: number, code: string, message: string, requestId: string): Response {
  return Response.json(
    { error: { code, message, details: [] }, meta: { requestId } },
    { status, headers: { "Cache-Control": "no-store", "X-Request-ID": requestId } },
  );
}

/** True when the request comes from our own origin (Origin header, or Referer as fallback). */
export function isSameOrigin(request: Request, webOrigin: string): boolean {
  const origin = request.headers.get("origin");
  if (origin) return origin === webOrigin;
  const referer = request.headers.get("referer");
  if (!referer) return false;
  try {
    return new URL(referer).origin === webOrigin;
  } catch {
    return false;
  }
}

export function isAllowedPath(path: string): boolean {
  return ALLOWED_PATHS.some((re) => re.test(path));
}

export async function forwardToApi(request: Request, segments: string[]): Promise<Response> {
  const requestId = crypto.randomUUID();
  const [version, ...rest] = segments;
  const path = rest.join("/");
  if (version !== "v1" || !isAllowedPath(path)) return envelope(404, "NOT_FOUND", "Route not found.", requestId);

  const method = request.method.toUpperCase();
  if (MUTATING.has(method)) {
    if (!isSameOrigin(request, serverEnv.webOrigin)) {
      return envelope(403, "FORBIDDEN", "Cross-origin requests are not allowed.", requestId);
    }
    const ct = request.headers.get("content-type");
    if (request.body && ct && !ct.toLowerCase().startsWith("application/json")) {
      return envelope(415, "VALIDATION_FAILED", "Content-Type must be application/json.", requestId);
    }
  }

  const session = await auth();
  const isPublic = method === "GET" && /^(catalog\/|leaderboard$|profiles\/)/.test(path);
  if (!session?.subject && !isPublic) return envelope(401, "UNAUTHENTICATED", "Authentication required.", requestId);

  let body: ArrayBuffer | undefined;
  if (MUTATING.has(method)) {
    body = await request.arrayBuffer();
    if (body.byteLength > MAX_BODY_BYTES)
      return envelope(413, "VALIDATION_FAILED", "Request body is too large.", requestId);
  }

  const headers = new Headers({ "X-Request-ID": requestId });
  for (const name of FORWARD_REQUEST_HEADERS) {
    const value = request.headers.get(name);
    if (value) headers.set(name, value);
  }
  if (session?.subject) headers.set("Authorization", `Bearer ${await mintAssertion(session.subject, AUDIENCE_API)}`);

  const search = new URL(request.url).search;
  let upstream: Response;
  try {
    upstream = await fetch(`${serverEnv.apiInternalUrl}/api/v1/${path}${search}`, {
      method,
      headers,
      body: body && body.byteLength > 0 ? body : undefined,
      cache: "no-store",
      redirect: "manual",
      signal: AbortSignal.timeout(10_000),
    });
  } catch (err) {
    console.error("bff upstream failure", { requestId, path, error: err instanceof Error ? err.message : String(err) });
    return envelope(502, "INTERNAL", "The service is temporarily unavailable.", requestId);
  }

  const out = new Headers({ "Cache-Control": "no-store" });
  for (const name of FORWARD_RESPONSE_HEADERS) {
    const value = upstream.headers.get(name);
    if (value) out.set(name, value);
  }
  return new Response(upstream.body, { status: upstream.status, headers: out });
}
