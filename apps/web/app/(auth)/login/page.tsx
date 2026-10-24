import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { GithubIcon } from "@/components/brand/github-icon";
import { ErrorState } from "@/components/feedback/states";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { signInAsTestUser, signInWithGitHub } from "@/lib/auth/actions";
import { testAuthEnabled } from "@/lib/server-env";

export const metadata: Metadata = { title: "Sign in", robots: { index: false } };

/** Auth.js error codes mapped to plain-language explanations. */
const errorMessages: Record<string, string> = {
  AccessDenied: "Sign-in was cancelled or GitHub denied access. Try again when you're ready.",
  OAuthCallbackError: "GitHub didn't complete the sign-in. Please try again.",
  OAuthSignin: "We couldn't start signing in with GitHub. Please try again.",
  Configuration: "Sign-in isn't configured correctly on this server. Please try again later.",
  ProvisioningFailed: "You signed in with GitHub, but we couldn't set up your KubeDojo account. Please try again.",
  TestSignInFailed: "Test sign-in failed. Check the GitHub ID and login.",
  SessionRequired: "Please sign in to continue.",
};

async function LoginForm({ searchParams }: { searchParams: PageProps<"/login">["searchParams"] }) {
  const params = await searchParams;
  const callbackUrl = typeof params.callbackUrl === "string" ? params.callbackUrl : "/dashboard";
  const error = typeof params.error === "string" ? params.error : undefined;
  const testMode = testAuthEnabled();
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
      <form action={signInWithGitHub}>
        <input type="hidden" name="callbackUrl" value={callbackUrl} />
        <Button type="submit" size="lg" className="w-full">
          <GithubIcon /> Continue with GitHub
        </Button>
      </form>
      {testMode && (
        <form
          action={signInAsTestUser}
          className="grid gap-3 rounded-lg border border-dashed p-4"
          aria-label="Test sign-in"
        >
          <p className="text-sm font-medium">Test sign-in (development only)</p>
          <input type="hidden" name="callbackUrl" value={callbackUrl} />
          <div className="grid gap-1.5">
            <Label htmlFor="test-github-id">GitHub ID</Label>
            <Input id="test-github-id" name="githubId" inputMode="numeric" required pattern="[1-9][0-9]{0,19}" />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="test-login">Login</Label>
            <Input id="test-login" name="login" required pattern="[A-Za-z0-9-]{1,39}" />
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
    <Card className="w-full max-w-sm">
      <CardHeader className="text-center">
        <h1 className="text-xl font-semibold">Sign in to KubeDojo</h1>
        <CardDescription>Save your progress, resume labs and earn XP.</CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <Suspense
          fallback={
            <Button size="lg" className="w-full" disabled>
              <GithubIcon /> Continue with GitHub
            </Button>
          }
        >
          <LoginForm searchParams={searchParams} />
        </Suspense>
      </CardContent>
      <CardFooter className="justify-center text-center text-xs text-muted-foreground">
        <p>
          We only use your GitHub ID, username, name and avatar, never your email. See the{" "}
          <Link href="/privacy" className="text-link underline underline-offset-4">
            privacy policy
          </Link>
          .
        </p>
      </CardFooter>
    </Card>
  );
}
