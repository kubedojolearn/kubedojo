import type { Metadata } from "next";
import { UserRound } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";

export const metadata: Metadata = { title: "Profile" };

export default function ProfilePage() {
  return (
    <>
      <PageHeader title="Profile" />
      <EmptyState
        icon={UserRound}
        title="Profiles aren't available yet"
        description="Profiles are private by default. Learners who make theirs public will show their handle, completed lessons, XP and achievements here."
      />
    </>
  );
}
