import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { LoadingStatus } from "@/components/feedback/loading";
import { OnboardingForm } from "@/components/onboarding/onboarding-form";
import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { getCurrentUser } from "@/lib/auth/current-user";

export const metadata: Metadata = { title: "Welcome", robots: { index: false } };

async function Loader() {
  const me = await getCurrentUser();
  if (!me) redirect("/login?callbackUrl=/onboarding");
  if (me.onboardingCompletedAt) redirect("/dashboard");
  return <OnboardingForm handle={me.handle} next="/dashboard" />;
}

export default function OnboardingPage() {
  return (
    <Card className="w-full max-w-lg">
      <CardHeader>
        <h1 className="text-xl font-semibold">Welcome to KubeDojo</h1>
        <CardDescription>Three quick questions so we can suggest where to start. All optional.</CardDescription>
      </CardHeader>
      <CardContent>
        <Suspense
          fallback={
            <>
              <LoadingStatus label="Loading" />
              <Skeleton className="h-64 w-full" aria-hidden />
            </>
          }
        >
          <Loader />
        </Suspense>
      </CardContent>
    </Card>
  );
}
