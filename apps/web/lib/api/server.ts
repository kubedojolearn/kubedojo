import "server-only";
import { createApiClient, type ApiClient } from "@kubedojo/api-client";
import { AUDIENCE_API, mintAssertion } from "@/lib/auth/assertion";
import { serverEnv } from "@/lib/server-env";

/**
 * A typed Go API client for one request. When `subject` is set, every call carries a freshly
 * minted ≤60 s assertion. Server-only: the assertion must never reach the browser.
 */
export function apiClient(subject?: string, requestId?: string): ApiClient {
  const client = createApiClient({ baseUrl: serverEnv.apiInternalUrl, cache: "no-store" });
  client.use({
    async onRequest({ request }) {
      if (subject) request.headers.set("Authorization", `Bearer ${await mintAssertion(subject, AUDIENCE_API)}`);
      request.headers.set("X-Request-ID", requestId ?? crypto.randomUUID());
      return request;
    },
  });
  return client;
}
