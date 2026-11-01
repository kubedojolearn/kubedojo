import type { Metadata } from "next";
import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { Mail } from "lucide-react";
import { GithubIcon } from "@/components/brand/github-icon";
import { GoogleIcon } from "@/components/brand/google-icon";
import { LogoMark } from "@/components/brand/logo";
import { ErrorState } from "@/components/feedback/states";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { signInAsTestUser, signInWithAsgardeo } from "@/lib/auth/actions";
import { asgardeoConfigured, serverEnv, testAuthEnabled, type SignInMethod } from "@/lib/server-env";

export const metadata: Metadata = { title: "Sign in", robots: { index: false } };

/** Auth.js error codes mapped to plain-language explanations. */
const errorMessages: Record<string, string> = {
  AccessDenied: "Sign-in was cancelled or access was denied. Try again when you're ready.",
  OAuthCallbackError: "The sign-in didn't complete. Please try again.",
  OAuthSignin: "We couldn't start signing in. Please try again.",
  Configuration: "Sign-in isn't configured correctly on this server. Please try again later.",
  ProvisioningFailed: "You signed in, but we couldn't set up your KubeDojo account. Please try again.",
  SignInNotConfigured: "Sign-in isn't set up on this server yet.",
  TestSignInFailed: "Test sign-in failed. Check the test ID and username.",
  SessionRequired: "Please sign in to continue.",
};

const signInMethods: { method: SignInMethod; label: string; icon: ReactNode }[] = [
  { method: "google", label: "Continue with Google", icon: <GoogleIcon /> },
  { method: "github", label: "Continue with GitHub", icon: <GithubIcon /> },
  { method: "email", label: "Continue with email", icon: <Mail aria-hidden /> },
];

async function LoginForm({ searchParams }: { searchParams: PageProps<"/login">["searchParams"] }) {
  const params = await searchParams;
  const callbackUrl = typeof params.callbackUrl === "string" ? params.callbackUrl : "/dashboard";
  const error = typeof params.error === "string" ? params.error : undefined;
  const testMode = testAuthEnabled();
  const ready = asgardeoConfigured();
  return (
    <>
      {error && (
        <ErrorState
          title="Couldn't sign you in"
          description={errorMessages[error] ?? "Something went wrong while signing in. Please try again."}
          headingLevel={2}
          className="p-4"
        />
      )}
      <div className="grid gap-2">
        {signInMethods.map((m) => (
          <form key={m.method} action={signInWithAsgardeo}>
            <input type="hidden" name="callbackUrl" value={callbackUrl} />
            <input type="hidden" name="method" value={m.method} />
            <Button
              type="submit"
              size="lg"
              variant={m.method === "email" ? "outline" : "default"}
              className="w-full"
              disabled={!ready}
              aria-describedby={ready ? undefined : "sign-in-not-configured"}
            >
              {m.icon} {m.label}
            </Button>
          </form>
        ))}
        {!ready && (
          <p id="sign-in-not-configured" className="text-caption text-muted-foreground">
            {serverEnv.isProduction
              ? "Sign-in isn't available right now."
              : "Sign-in isn't set up: add AUTH_ASGARDEO_ID, AUTH_ASGARDEO_SECRET and AUTH_ASGARDEO_ISSUER to .env (see docs/operations/auth.md)."}
          </p>
        )}
      </div>
      {testMode && (
        <form
          action={signInAsTestUser}
          className="grid gap-3 rounded-lg border border-dashed p-4"
          aria-label="Test sign-in"
        >
          <p className="text-sm font-medium">Test sign-in (development only)</p>
          <input type="hidden" name="callbackUrl" value={callbackUrl} />
          <div className="grid gap-1.5">
            <Label htmlFor="test-id">Test ID</Label>
            <Input
              id="test-id"
              name="testId"
              inputMode="numeric"
              required
              pattern="[1-9][0-9]{0,19}"
              title="Digits only, for example 1001"
              placeholder="1001"
              aria-describedby="test-id-hint"
            />
            <p id="test-id-hint" className="text-caption text-muted-foreground">
              Any number. The same number signs you back into the same account.
            </p>
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="test-login">Username</Label>
            <Input
              id="test-login"
              name="login"
              required
              pattern="[A-Za-z0-9-]{1,39}"
              title="Letters, numbers and hyphens, for example test-user"
              placeholder="test-user"
              aria-describedby="test-login-hint"
            />
            <p id="test-login-hint" className="text-caption text-muted-foreground">
              Letters, numbers and hyphens. It becomes your handle on a new account.
            </p>
          </div>
          <Button type="submit" variant="outline">
            Sign in as test user
          </Button>
        </form>
      )}
    </>
  );
}

export default function LoginPage({ searchParams }: PageProps<"/login">) {
  return (
    <div className="grid gap-6">
      <header>
        <LogoMark className="size-9" />
        <h1 className="mt-5 text-h2 font-bold tracking-tight">Sign in</h1>
        <p className="mt-2 text-body text-foreground-secondary">
          Save your progress, pick labs up where you left them, and earn XP. Lessons are free to read without an
          account.
        </p>
      </header>
      <div className="grid gap-4">
        <Suspense
          fallback={
            <div className="grid gap-2">
              {signInMethods.map((m) => (
                <Button
                  key={m.method}
                  size="lg"
                  variant={m.method === "email" ? "outline" : "default"}
                  className="w-full"
                  disabled
                >
                  {m.icon} {m.label}
                </Button>
              ))}
            </div>
          }
        >
          <LoginForm searchParams={searchParams} />
        </Suspense>
      </div>
      <p className="border-t border-border-subtle pt-4 text-caption text-muted-foreground">
        Sign-in is handled by Asgardeo. We only receive your account ID, username, name and picture, never your email.
        See the{" "}
        <Link href="/privacy" className="text-link underline underline-offset-4">
          privacy policy
        </Link>
        .
      </p>
      <Link href="/learn" className="text-small text-foreground-secondary hover:text-foreground hover:underline">
        Continue learning without an account
      </Link>
    </div>
  );
}
