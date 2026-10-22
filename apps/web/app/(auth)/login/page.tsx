import type { Metadata } from "next";
import Link from "next/link";
import { GithubIcon } from "@/components/brand/github-icon";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { UnavailableButton } from "@/components/feedback/unavailable-button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader } from "@/components/ui/card";

export const metadata: Metadata = {
  title: "Sign in",
  robots: { index: false },
};

export default function LoginPage() {
  return (
    <Card className="w-full max-w-sm">
      <CardHeader className="text-center">
        <div className="flex justify-center">
          <InDevelopmentBadge label="Coming soon" />
        </div>
        <h1 className="text-xl font-semibold">Sign in to KubeDojo</h1>
        <CardDescription>
          Save your progress, resume labs and earn XP. Sign-in isn&apos;t open yet; it&apos;s being built next.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <UnavailableButton reason="Sign-in isn't available yet." size="lg" className="w-full">
          <GithubIcon /> Continue with GitHub
        </UnavailableButton>
      </CardContent>
      <CardFooter className="justify-center text-center text-xs text-muted-foreground">
        <p>
          We only use your GitHub ID, username, name and avatar. See the{" "}
          <Link href="/privacy" className="text-link underline underline-offset-4">
            privacy policy
          </Link>
          .
        </p>
      </CardFooter>
    </Card>
  );
}
