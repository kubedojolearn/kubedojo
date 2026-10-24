/** Browser-side calls to the BFF (/api/bff/v1/*). Never import server-only modules here. */

export type BffResult<T> =
  { ok: true; status: number; data: T } | { ok: false; status: number; code: string; message: string };

/**
 * Sends a JSON request through the BFF and unwraps the API envelope. Network failures and
 * unexpected bodies become `{ ok: false }` with a message fit to show the learner.
 */
export async function bff<T>(
  path: string,
  init: { method?: string; body?: unknown; idempotencyKey?: string } = {},
): Promise<BffResult<T>> {
  const headers: Record<string, string> = { Accept: "application/json" };
  if (init.body !== undefined) headers["Content-Type"] = "application/json";
  if (init.idempotencyKey) headers["Idempotency-Key"] = init.idempotencyKey;
  let res: Response;
  try {
    res = await fetch(`/api/bff/v1/${path}`, {
      method: init.method ?? "GET",
      headers,
      body: init.body === undefined ? undefined : JSON.stringify(init.body),
    });
  } catch {
    return { ok: false, status: 0, code: "NETWORK", message: "Check your connection and try again." };
  }
  const payload = (await res.json().catch(() => null)) as {
    data?: T;
    error?: { code?: string; message?: string };
  } | null;
  if (res.ok && payload && "data" in payload) return { ok: true, status: res.status, data: payload.data as T };
  // 202/204 carry no body.
  if (res.ok && (res.status === 202 || res.status === 204))
    return { ok: true, status: res.status, data: undefined as T };
  return {
    ok: false,
    status: res.status,
    code: payload?.error?.code ?? "INTERNAL",
    message: payload?.error?.message ?? "Something went wrong. Please try again.",
  };
}
