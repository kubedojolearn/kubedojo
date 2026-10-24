import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { PageHeader } from "@/components/layout/page-header";
import { SettingsForm } from "@/components/settings/settings-form";
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { getCurrentUser } from "@/lib/auth/current-user";

export const metadata: Metadata = { title: "Settings" };

async function SettingsLoader() {
  const me = await getCurrentUser();
  if (!me) redirect("/login?callbackUrl=/settings");
  return <SettingsForm me={me} />;
}

export default function SettingsPage() {
  return (
    <>
      <PageHeader title="Settings" />
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading your settings" />
            <CardListSkeleton count={2} />
          </>
        }
      >
        <SettingsLoader />
      </Suspense>
      <Card className="mt-6 max-w-3xl">
        <CardHeader>
          <CardTitle className="flex flex-wrap items-center gap-2">
            <h2>Your data</h2> <InDevelopmentBadge label="Coming soon" />
          </CardTitle>
          <CardDescription>Export everything KubeDojo stores about you, or delete your account.</CardDescription>
        </CardHeader>
      </Card>
    </>
  );
}
