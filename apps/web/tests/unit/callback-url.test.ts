// @vitest-environment node
import { describe, expect, it, vi } from "vitest";

vi.mock("next-auth", () => ({ default: () => ({ handlers: {}, auth: vi.fn(), signIn: vi.fn(), signOut: vi.fn() }) }));
vi.mock("next-auth/providers/github", () => ({ default: () => ({ id: "github" }) }));
vi.mock("next-auth/providers/credentials", () => ({ default: () => ({ id: "test-login" }) }));

const { safeCallbackPath } = await import("@/lib/auth/config");
const base = "http://localhost:8000";

describe("safeCallbackPath (open redirect protection)", () => {
  it.each([
    ["/settings", "/settings"],
    ["/lab/abc?tab=terminal#x", "/lab/abc?tab=terminal#x"],
    [`${base}/dashboard`, "/dashboard"],
    ["https://evil.example/phish", "/dashboard"],
    ["//evil.example/phish", "/dashboard"],
    ["javascript:alert(1)", "/dashboard"],
    ["", "/dashboard"],
    [undefined, "/dashboard"],
  ])("%s → %s", (input, expected) => {
    expect(safeCallbackPath(input, base)).toBe(expected);
  });
});

describe("test sign-in guard", () => {
  it("refuses to run in production", async () => {
    const { testAuthEnabled } = await import("@/lib/server-env");
    process.env.AUTH_TEST_MODE = "1";
    process.env.APP_ENV = "production";
    expect(() => testAuthEnabled()).toThrow();
    process.env.APP_ENV = "development";
    expect(testAuthEnabled()).toBe(true);
    delete process.env.AUTH_TEST_MODE;
    expect(testAuthEnabled()).toBe(false);
  });
});
