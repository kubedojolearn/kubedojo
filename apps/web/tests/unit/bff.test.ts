// @vitest-environment node
import { generateKeyPairSync } from "node:crypto";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

const session = vi.hoisted(() => ({ current: null as null | { subject?: string } }));
vi.mock("@/lib/auth/config", () => ({ auth: async () => session.current }));

const { forwardToApi, isAllowedPath } = await import("@/lib/api/bff");

const ORIGIN = "http://localhost:8000";
let upstream: ReturnType<typeof vi.fn>;

beforeAll(() => {
  const { privateKey } = generateKeyPairSync("ed25519");
  process.env.BFF_SIGNING_PRIVATE_KEY = privateKey.export({ format: "der", type: "pkcs8" }).toString("base64");
  process.env.BFF_SIGNING_KID = "unit-1";
  process.env.WEB_ORIGIN = ORIGIN;
  process.env.API_INTERNAL_URL = "http://api:8080";
});

beforeEach(() => {
  session.current = { subject: "github:42" };
  upstream = vi.fn(async () =>
    Response.json(
      { data: { ok: true }, meta: { requestId: "r" } },
      { headers: { "x-request-id": "r", "set-cookie": "evil=1", "x-internal": "1" } },
    ),
  );
  vi.stubGlobal("fetch", upstream);
});
afterEach(() => vi.unstubAllGlobals());

function req(
  method: string,
  path: string,
  init: { origin?: string | null; body?: string; headers?: Record<string, string> } = {},
) {
  const headers = new Headers(init.headers);
  if (init.origin !== null) headers.set("origin", init.origin ?? ORIGIN);
  if (init.body) headers.set("content-type", "application/json");
  headers.set("cookie", "authjs.session-token=secret-cookie");
  return new Request(`${ORIGIN}/api/bff/${path}`, { method, headers, body: init.body });
}

const segments = (p: string) => p.split("/");

describe("BFF proxy", () => {
  it("forwards with a fresh assertion and no cookies", async () => {
    const res = await forwardToApi(req("GET", "v1/users/me?x=1"), segments("v1/users/me"));
    expect(res.status).toBe(200);
    const [url, init] = upstream.mock.calls[0]!;
    expect(url).toBe("http://api:8080/api/v1/users/me?x=1");
    const h = init.headers as Headers;
    expect(h.get("authorization")).toMatch(/^Bearer [\w-]+\.[\w-]+\.[\w-]+$/);
    expect(h.get("cookie")).toBeNull();
    expect(h.get("x-request-id")).toMatch(/^[0-9a-f-]{36}$/);
  });

  it("returns only safe response headers", async () => {
    const res = await forwardToApi(req("GET", "v1/users/me"), segments("v1/users/me"));
    expect(res.headers.get("set-cookie")).toBeNull();
    expect(res.headers.get("x-internal")).toBeNull();
    expect(res.headers.get("cache-control")).toBe("no-store");
  });

  it("requires a session for private routes", async () => {
    session.current = null;
    const res = await forwardToApi(req("GET", "v1/users/me"), segments("v1/users/me"));
    expect(res.status).toBe(401);
    expect((await res.json()).error.code).toBe("UNAUTHENTICATED");
    expect(upstream).not.toHaveBeenCalled();
  });

  it("serves public catalog reads without a session and without an assertion", async () => {
    session.current = null;
    const res = await forwardToApi(req("GET", "v1/catalog/courses"), segments("v1/catalog/courses"));
    expect(res.status).toBe(200);
    expect((upstream.mock.calls[0]![1].headers as Headers).get("authorization")).toBeNull();
  });

  it.each([
    ["foreign origin", "https://evil.example"],
    ["missing origin and referer", null],
  ])("rejects mutations with %s (CSRF)", async (_name, origin) => {
    const res = await forwardToApi(req("PATCH", "v1/users/me", { origin, body: "{}" }), segments("v1/users/me"));
    expect(res.status).toBe(403);
    expect(upstream).not.toHaveBeenCalled();
  });

  it("accepts same-origin mutations via Referer when Origin is absent", async () => {
    const res = await forwardToApi(
      req("PATCH", "v1/users/me", { origin: null, body: "{}", headers: { referer: `${ORIGIN}/settings` } }),
      segments("v1/users/me"),
    );
    expect(res.status).toBe(200);
  });

  it("rejects non-JSON mutation bodies", async () => {
    const r = new Request(`${ORIGIN}/api/bff/v1/users/me`, {
      method: "PATCH",
      headers: { origin: ORIGIN, "content-type": "application/x-www-form-urlencoded" },
      body: "role=admin",
    });
    expect((await forwardToApi(r, segments("v1/users/me"))).status).toBe(415);
  });

  it("rejects oversized bodies", async () => {
    const res = await forwardToApi(
      req("PATCH", "v1/users/me", { body: JSON.stringify({ x: "a".repeat(300_000) }) }),
      segments("v1/users/me"),
    );
    expect(res.status).toBe(413);
  });

  it.each([
    "v1/auth/provision",
    "v1/ws/labs/x",
    "v1/healthz",
    "v2/users/me",
    "v1/users/me/../../auth/provision",
    "v1/admin/users",
  ])("does not expose %s", async (path) => {
    const res = await forwardToApi(req("POST", path, { body: "{}" }), segments(path));
    expect(res.status).toBe(404);
    expect(upstream).not.toHaveBeenCalled();
  });

  it("maps upstream failures to a 502 envelope without details", async () => {
    upstream.mockRejectedValueOnce(new Error("connect ECONNREFUSED 10.0.0.5:8080"));
    const res = await forwardToApi(req("GET", "v1/users/me"), segments("v1/users/me"));
    expect(res.status).toBe(502);
    expect(JSON.stringify(await res.json())).not.toContain("10.0.0.5");
  });

  it("passes rate-limit hints through", async () => {
    upstream.mockResolvedValueOnce(
      Response.json(
        { error: { code: "RATE_LIMITED", message: "x", details: [] }, meta: { requestId: "r" } },
        { status: 429, headers: { "retry-after": "12" } },
      ),
    );
    const res = await forwardToApi(req("GET", "v1/users/me"), segments("v1/users/me"));
    expect(res.status).toBe(429);
    expect(res.headers.get("retry-after")).toBe("12");
  });

  it("allowlists lab routes by UUID only", () => {
    expect(isAllowedPath("labs/0b6f4c1e-8f53-4f3c-9a51-2d7e3c1a9b10/commands")).toBe(true);
    expect(isAllowedPath("labs/not-a-uuid/commands")).toBe(false);
    expect(isAllowedPath("labs/0b6f4c1e-8f53-4f3c-9a51-2d7e3c1a9b10/ws")).toBe(false);
  });
});
