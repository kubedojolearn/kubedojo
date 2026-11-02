import "server-only";
import { AUDIENCE_PROVISION, mintAssertion, type ProvisionProfile } from "@/lib/auth/assertion";
import { serverEnv } from "@/lib/server-env";

/**
 * Creates or refreshes the KubeDojo account for an identity, "asgardeo:<user id>" (ADR-0003,
 * ADR-0008). Called from the Auth.js sign-in callback; sign-in fails if this fails, so no
 * session exists without an account.
 */
export async function provisionAccount(subject: string, profile: ProvisionProfile): Promise<boolean> {
  try {
    const token = await mintAssertion(subject, AUDIENCE_PROVISION, profile);
    const res = await fetch(`${serverEnv.apiInternalUrl}/api/v1/auth/provision`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}`, "X-Request-ID": crypto.randomUUID() },
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });
    if (!res.ok) {
      console.error("provisioning failed", { status: res.status, requestId: res.headers.get("x-request-id") });
      return false;
    }
    return true;
  } catch (err) {
    console.error("provisioning failed", { error: err instanceof Error ? err.message : String(err) });
    return false;
  }
}
