import type { Metadata } from "next";
import { Trophy } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";

export const metadata: Metadata = { title: "Leaderboard" };

export default function LeaderboardPage() {
  return (
    <>
      <PageHeader title="Leaderboard" description="Learners ranked by XP. Only people who opt in are listed." />
      <EmptyState
        icon={Trophy}
        title="No one on the leaderboard yet"
        description="The leaderboard is opt-in. Once sign-in opens, you can choose to appear here in your settings."
      />
    </>
  );
}
