import "server-only";

function getWebOrigin(): string {
  const configured = process.env.WEB_ORIGIN;
  if (configured) return configured.replace(/\/+$/, "");

  // Vercel assigns a unique HTTPS hostname to each preview deployment. This keeps
  // origin checks, callback paths, and generated metadata aligned with that host.
  if (process.env.VERCEL_URL) return `https://${process.env.VERCEL_URL.replace(/\/+$/, "")}`;
  return "http://localhost:8000";
}

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
    return getWebOrigin();
  },
  get signingKey() {
    return required("BFF_SIGNING_PRIVATE_KEY");
  },
  get signingKid() {
    return required("BFF_SIGNING_KID");
  },
  get asgardeoClientId() {
    return required("AUTH_ASGARDEO_ID");
  },
  get asgardeoClientSecret() {
    return required("AUTH_ASGARDEO_SECRET");
  },
  get asgardeoIssuer() {
    return required("AUTH_ASGARDEO_ISSUER");
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

/**
 * Sign-in needs an Asgardeo application (docs/operations/auth.md). Without it the sign-in
 * buttons are disabled instead of failing after the click.
 */
export function asgardeoConfigured(): boolean {
  return Boolean(process.env.AUTH_ASGARDEO_ID && process.env.AUTH_ASGARDEO_SECRET && process.env.AUTH_ASGARDEO_ISSUER);
}

/** The sign-in options on /login. Google and GitHub are Asgardeo connections; email is Asgardeo's own login. */
export type SignInMethod = "google" | "github" | "email";

/**
 * Asgardeo's `fidp` value for each connection: the connection's name in the Asgardeo console.
 * It sends the learner straight to Google or GitHub instead of Asgardeo's login page.
 */
export function asgardeoConnection(method: SignInMethod): string | undefined {
  if (method === "google") return process.env.AUTH_ASGARDEO_GOOGLE_CONNECTION || "Google";
  if (method === "github") return process.env.AUTH_ASGARDEO_GITHUB_CONNECTION || "GitHub";
  return undefined;
}
