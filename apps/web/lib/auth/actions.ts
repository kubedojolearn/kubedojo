"use server";

import { AuthError } from "next-auth";
import { redirect } from "next/navigation";
import { safeCallbackPath, signIn, signOut, TEST_PROVIDER_ID } from "@/lib/auth/config";
import { githubAuthConfigured, serverEnv, testAuthEnabled } from "@/lib/server-env";

function destination(formData: FormData): string {
  return safeCallbackPath(String(formData.get("callbackUrl") ?? ""), serverEnv.webOrigin);
}

/** Starts the GitHub OAuth flow. */
export async function signInWithGitHub(formData: FormData) {
  if (!githubAuthConfigured()) {
    redirect(`/login?error=GitHubNotConfigured&callbackUrl=${encodeURIComponent(destination(formData))}`);
  }
  await signIn("github", { redirectTo: destination(formData) });
}

/** Test-only sign-in (AUTH_TEST_MODE=1, never in production). */
export async function signInAsTestUser(formData: FormData) {
  if (!testAuthEnabled()) throw new Error("Test sign-in is disabled.");
  try {
    await signIn(TEST_PROVIDER_ID, {
      githubId: String(formData.get("githubId") ?? ""),
      login: String(formData.get("login") ?? ""),
      redirectTo: destination(formData),
    });
  } catch (err) {
    if (err instanceof AuthError) redirect("/login?error=TestSignInFailed");
    throw err;
  }
}

export async function signOutAction() {
  await signOut({ redirectTo: "/" });
}
