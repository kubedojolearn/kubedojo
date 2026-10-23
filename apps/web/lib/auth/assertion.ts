import "server-only";
import { importPKCS8, SignJWT, type CryptoKey } from "jose";
import { serverEnv } from "@/lib/server-env";

/** Internal assertion contract with the Go API (ADR-0003, docs/api/conventions.md). */
export const ASSERTION_ISSUER = "kubedojo-web";
export const AUDIENCE_API = "kubedojo-api";
export const AUDIENCE_PROVISION = "kubedojo-api-provision";
export const ASSERTION_LIFETIME_SECONDS = 60;

export interface ProvisionProfile {
  login: string;
  name: string;
  avatar_url: string;
}

let cachedKey: { source: string; key: Promise<CryptoKey> } | undefined;

function signingKey(): Promise<CryptoKey> {
  const source = serverEnv.signingKey;
  if (cachedKey?.source !== source) {
    const body =
      source
        .replace(/\s+/g, "")
        .match(/.{1,64}/g)
        ?.join("\n") ?? "";
    const pem = `-----BEGIN PRIVATE KEY-----\n${body}\n-----END PRIVATE KEY-----`;
    cachedKey = { source, key: importPKCS8(pem, "EdDSA") };
  }
  return cachedKey.key;
}

/**
 * Signs a short-lived Ed25519 assertion. `subject` must be "github:<numeric id>". Provisioning
 * assertions carry profile claims; request assertions must not. Never send the result to a browser.
 */
export async function mintAssertion(
  subject: string,
  audience: typeof AUDIENCE_API | typeof AUDIENCE_PROVISION,
  profile?: ProvisionProfile,
): Promise<string> {
  if (!/^github:[1-9][0-9]{0,19}$/.test(subject)) throw new Error("invalid assertion subject");
  if ((audience === AUDIENCE_PROVISION) !== Boolean(profile)) {
    throw new Error("profile claims belong on provisioning assertions only");
  }
  const now = Math.floor(Date.now() / 1000);
  return new SignJWT(profile ? { ...profile } : {})
    .setProtectedHeader({ alg: "EdDSA", kid: serverEnv.signingKid, typ: "JWT" })
    .setIssuer(ASSERTION_ISSUER)
    .setAudience(audience)
    .setSubject(subject)
    .setIssuedAt(now)
    .setExpirationTime(now + ASSERTION_LIFETIME_SECONDS)
    .setJti(crypto.randomUUID())
    .sign(await signingKey());
}
