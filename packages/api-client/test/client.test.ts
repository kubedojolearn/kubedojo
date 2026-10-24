import { describe, expect, it } from "vitest";
import { createApiClient, isErrorResponse } from "../src/index";

function fakeFetch(status: number, body: unknown) {
  const calls: Request[] = [];
  const fetch = async (input: Request) => {
    calls.push(input);
    return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
  };
  return { fetch, calls };
}

describe("api client", () => {
  it("calls typed routes against the configured base URL", async () => {
    const { fetch, calls } = fakeFetch(200, {
      data: {
        id: "33333333-3333-4333-8333-333333333333",
        handle: "kube-learner",
        displayName: "Kube Learner",
        avatarUrl: null,
        role: "learner",
        createdAt: "2026-10-01T09:00:00Z",
        settings: {
          theme: "system",
          reducedMotion: false,
          leaderboardOptIn: false,
          profilePublic: false,
          analyticsOptOut: false,
          timeZone: "UTC",
        },
        onboardingCompletedAt: null,
      },
      meta: { requestId: "abc" },
    });
    const client = createApiClient({ baseUrl: "http://api:8080/", fetch });
    const { data, error } = await client.GET("/api/v1/users/me", { headers: { Authorization: "Bearer t" } });
    expect(error).toBeUndefined();
    expect(data?.data.handle).toBe("kube-learner");
    expect(calls[0]?.url).toBe("http://api:8080/api/v1/users/me");
    expect(calls[0]?.headers.get("Authorization")).toBe("Bearer t");
  });

  it("returns error envelopes as typed errors", async () => {
    const { fetch } = fakeFetch(401, {
      error: { code: "UNAUTHENTICATED", message: "Authentication required.", details: [] },
      meta: { requestId: "abc" },
    });
    const client = createApiClient({ baseUrl: "http://api:8080", fetch });
    const { data, error } = await client.GET("/api/v1/users/me");
    expect(data).toBeUndefined();
    expect(isErrorResponse(error)).toBe(true);
    expect(error?.error.code).toBe("UNAUTHENTICATED");
  });

  it("isErrorResponse rejects other shapes", () => {
    expect(isErrorResponse(null)).toBe(false);
    expect(isErrorResponse({ error: "x" })).toBe(false);
    expect(isErrorResponse({ data: {} })).toBe(false);
  });
});
