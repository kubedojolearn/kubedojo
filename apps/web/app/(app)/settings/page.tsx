import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { PageHeader } from "@/components/layout/page-header";
import { SettingsForm } from "@/components/settings/settings-form";
import { YourData } from "@/components/settings/your-data";
import { getCurrentUser } from "@/lib/auth/current-user";

export const metadata: Metadata = { title: "Settings" };

async function SettingsLoader() {
  const me = await getCurrentUser();
  if (!me) redirect("/login?callbackUrl=/settings");
  return (
    <>
      <SettingsForm me={me} />
      <YourData handle={me.handle} />
    </>
  );
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
    </>
  );
}
