// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("next-auth", () => ({ default: () => ({ handlers: {}, auth: vi.fn(), signIn: vi.fn(), signOut: vi.fn() }) }));
vi.mock("next-auth/providers/asgardeo", () => ({ default: (options: object) => ({ id: "asgardeo", ...options }) }));
vi.mock("next-auth/providers/credentials", () => ({
  default: (options: object) => ({ id: "test-login", ...options }),
}));

const asgardeoEnv = ["AUTH_ASGARDEO_ID", "AUTH_ASGARDEO_SECRET", "AUTH_ASGARDEO_ISSUER"] as const;

afterEach(() => {
  delete process.env.AUTH_TEST_MODE;
  delete process.env.APP_ENV;
  for (const key of asgardeoEnv) delete process.env[key];
  vi.resetModules();
});

describe("Auth.js providers", () => {
  it("uses test sign-in without requiring an Asgardeo application", async () => {
    process.env.AUTH_TEST_MODE = "1";

    const { authConfig } = await import("@/lib/auth/config");

    expect(authConfig.providers.map((provider) => (typeof provider === "function" ? undefined : provider.id))).toEqual([
      "test-login",
    ]);
  });

  it("configures Asgardeo discovery from the issuer URL", async () => {
    process.env.AUTH_ASGARDEO_ID = "client-id";
    process.env.AUTH_ASGARDEO_SECRET = "client-secret";
    process.env.AUTH_ASGARDEO_ISSUER = "https://api.asgardeo.io/t/kubedojo/oauth2/token";

    const { authConfig } = await import("@/lib/auth/config");
    const provider = authConfig.providers[0];

    expect(provider).toMatchObject({
      id: "asgardeo",
      clientId: "client-id",
      clientSecret: "client-secret",
      issuer: "https://api.asgardeo.io/t/kubedojo/oauth2/token",
      wellKnown: "https://api.asgardeo.io/t/kubedojo/oauth2/token/.well-known/openid-configuration",
    });
  });
});
