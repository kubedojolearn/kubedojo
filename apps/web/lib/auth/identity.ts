import type { ProvisionProfile } from "@/lib/auth/assertion";

/**
 * Identity rules for Asgardeo sign-in (ADR-0008). Asgardeo brokers Google, GitHub and
 * email/password sign-in; KubeDojo only ever sees the Asgardeo user. Pure functions, so they
 * can be unit tested without Auth.js.
 */

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const HANDLE = /^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$/;

/**
 * The assertion subject for an Asgardeo user ID: "asgardeo:<uuid>". Returns null for anything
 * else, e.g. when the Asgardeo app's subject attribute is set to the username or email.
 */
export function asgardeoSubject(userId: unknown): string | null {
  if (typeof userId !== "string") return null;
  const id = userId.toLowerCase();
  return UUID.test(id) ? `asgardeo:${id}` : null;
}

/** The OIDC claims KubeDojo reads from Asgardeo's ID token (scope "openid profile"). */
export interface AsgardeoClaims {
  sub?: unknown;
  preferred_username?: unknown;
  username?: unknown;
  nickname?: unknown;
  name?: unknown;
  given_name?: unknown;
  family_name?: unknown;
  picture?: unknown;
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/** Lowercase, ASCII-only, hyphen-separated, at most 39 characters; "" when nothing is left. */
function slug(value: string): string {
  return value
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .slice(0, 39)
    .replace(/^-+|-+$/g, "");
}

/**
 * The profile sent to the API when an account is provisioned. `login` seeds the handle, so it
 * must look like one; Asgardeo usernames are often email addresses, which KubeDojo never stores
 * (requirements PR1), so any value containing "@" is ignored.
 */
export function provisionProfile(claims: AsgardeoClaims): ProvisionProfile {
  const usable = (v: unknown) => {
    const s = text(v);
    return s && !s.includes("@") ? s : "";
  };
  const givenAndFamily = [usable(claims.given_name), usable(claims.family_name)].filter(Boolean).join(" ");
  const name = usable(claims.name) || givenAndFamily || usable(claims.nickname);
  const candidates = [claims.preferred_username, claims.username, claims.nickname].map(usable);
  const login =
    candidates.find((c) => HANDLE.test(c)) ?? (candidates.map(slug).find(Boolean) || slug(name) || "learner");
  return { login, name: name || login, avatar_url: text(claims.picture) };
}

/**
 * Test sign-in (AUTH_TEST_MODE=1) maps a short test ID such as "1001" to a stable UUID that
 * looks like an Asgardeo user ID, so the same ID always signs back into the same account.
 */
export async function testUserId(testId: string): Promise<string> {
  const digest = new Uint8Array(
    await crypto.subtle.digest("SHA-1", new TextEncoder().encode(`kubedojo-test-user:${testId}`)),
  ).slice(0, 16);
  digest[6] = (digest[6]! & 0x0f) | 0x50; // version 5 (name-based, SHA-1)
  digest[8] = (digest[8]! & 0x3f) | 0x80; // RFC 4122 variant
  const hex = Array.from(digest, (b) => b.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}
