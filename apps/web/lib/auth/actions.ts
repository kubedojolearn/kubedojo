"use server";

import { AuthError } from "next-auth";
import { redirect } from "next/navigation";
import { safeCallbackPath, signIn, signOut, TEST_PROVIDER_ID } from "@/lib/auth/config";
import {
  asgardeoConfigured,
  asgardeoConnection,
  serverEnv,
  testAuthEnabled,
  type SignInMethod,
} from "@/lib/server-env";

const methods: readonly SignInMethod[] = ["google", "github", "email"];

function destination(formData: FormData): string {
  return safeCallbackPath(String(formData.get("callbackUrl") ?? ""), serverEnv.webOrigin);
}

/** Starts Asgardeo sign-in, going straight to Google or GitHub when the learner picked one. */
export async function signInWithAsgardeo(formData: FormData) {
  if (!asgardeoConfigured()) {
    redirect(`/login?error=SignInNotConfigured&callbackUrl=${encodeURIComponent(destination(formData))}`);
  }
  const method = methods.find((m) => m === formData.get("method")) ?? "email";
  const fidp = asgardeoConnection(method);
  await signIn("asgardeo", { redirectTo: destination(formData) }, fidp ? { fidp } : undefined);
}

/** Test-only sign-in (AUTH_TEST_MODE=1, never in production). */
export async function signInAsTestUser(formData: FormData) {
  if (!testAuthEnabled()) throw new Error("Test sign-in is disabled.");
  try {
    await signIn(TEST_PROVIDER_ID, {
      testId: String(formData.get("testId") ?? ""),
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
