import "server-only";

/**
 * Server-only configuration. Secrets live here and are never read from client components;
 * scripts/check-client-bundle.mjs fails the build if they appear in browser bundles.
 */
function required(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set (see .env.example)`);
  return value;
}

export const serverEnv = {
  get apiInternalUrl() {
    return (process.env.API_INTERNAL_URL ?? "http://localhost:8080").replace(/\/+$/, "");
  },
  get webOrigin() {
    return (process.env.WEB_ORIGIN ?? "http://localhost:8000").replace(/\/+$/, "");
  },
  get signingKey() {
    return required("BFF_SIGNING_PRIVATE_KEY");
  },
  get signingKid() {
    return required("BFF_SIGNING_KID");
  },
  get isProduction() {
    return process.env.APP_ENV === "production";
  },
};

/**
 * The test sign-in provider exists only for automated tests. It is enabled by AUTH_TEST_MODE=1
 * and refuses to start when APP_ENV=production.
 */
export function testAuthEnabled(): boolean {
  if (process.env.AUTH_TEST_MODE !== "1") return false;
  if (process.env.APP_ENV === "production") {
    throw new Error("AUTH_TEST_MODE must never be enabled when APP_ENV=production");
  }
  return true;
}
