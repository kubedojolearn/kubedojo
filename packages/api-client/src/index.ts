/**
 * Typed client for the KubeDojo Go API. Types are generated from docs/api/openapi.yaml
 * (`pnpm generate`); never edit src/schema.d.ts by hand. Server-side use only: requests carry
 * the BFF's internal assertion, which must never reach the browser (ADR-0003).
 */
import createClient, { type ClientOptions } from "openapi-fetch";
import type { components, operations, paths } from "./schema";

export type { components, operations, paths };
export type Schemas = components["schemas"];
export type ErrorCode = Schemas["ErrorCode"];
export type ErrorResponse = Schemas["ErrorResponse"];

export interface ApiClientOptions extends Omit<ClientOptions, "baseUrl"> {
  /** API_INTERNAL_URL, e.g. http://api:8080 */
  baseUrl: string;
}

/** Creates a typed client. Callers add the Authorization header per request (it expires in ≤60 s). */
export function createApiClient({ baseUrl, ...options }: ApiClientOptions) {
  return createClient<paths>({ baseUrl: baseUrl.replace(/\/+$/, ""), ...options });
}

export type ApiClient = ReturnType<typeof createApiClient>;

/** Narrows an unknown error body to the API's error envelope. */
export function isErrorResponse(body: unknown): body is ErrorResponse {
  if (typeof body !== "object" || body === null || !("error" in body)) return false;
  const error = (body as { error: unknown }).error;
  return typeof error === "object" && error !== null && typeof (error as { code?: unknown }).code === "string";
}
