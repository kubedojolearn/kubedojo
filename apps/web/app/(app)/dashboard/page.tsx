import type { Metadata } from "next";
import Link from "next/link";
import { Flame, PlayCircle, Sparkles, Trophy } from "lucide-react";
import { EmptyState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export const metadata: Metadata = { title: "Dashboard" };

export default function DashboardPage() {
  return (
    <>
      <PageHeader title="Dashboard" description="Your learning at a glance." />
      <div className="grid gap-6 lg:grid-cols-3">
        <section aria-labelledby="continue-heading" className="lg:col-span-2">
          <h2 id="continue-heading" className="sr-only">
            Continue learning
          </h2>
          <EmptyState
            icon={PlayCircle}
            title="No lessons started yet"
            description="Start your first lesson and it will appear here so you can pick up where you left off."
            action={
              <Button asChild>
                <Link href="/learn">Browse the curriculum</Link>
              </Button>
            }
            headingLevel={3}
            className="h-full"
          />
        </section>
        <section aria-labelledby="stats-heading" className="grid gap-6">
          <h2 id="stats-heading" className="sr-only">
            Your stats
          </h2>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Flame aria-hidden className="size-4 text-warning" /> Streak
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              Complete a lesson on consecutive days to build a streak.
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Sparkles aria-hidden className="size-4 text-highlight" /> Experience
              </CardTitle>
            </CardHeader>
            <CardContent className="text-sm text-muted-foreground">
              Earn XP by completing lessons. Fewer hints and first-try passes earn more.
            </CardContent>
          </Card>
        </section>
        <section aria-labelledby="achievements-heading" className="lg:col-span-3">
          <h2 id="achievements-heading" className="mb-3 text-lg font-semibold">
            Recent achievements
          </h2>
          <EmptyState
            icon={Trophy}
            title="No achievements yet"
            description="Achievements mark milestones like your first completed course or a seven-day streak."
            headingLevel={3}
          />
        </section>
      </div>
    </>
  );
}
