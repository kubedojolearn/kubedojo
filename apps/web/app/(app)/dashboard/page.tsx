import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { ArrowRight, BookOpen, Flame, PlayCircle, Sparkles, Trophy } from "lucide-react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { EmptyState, ErrorState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress as Bar } from "@/components/ui/progress";
import { getCurrentUser } from "@/lib/auth/current-user";
import { listCourses } from "@/lib/catalog";
import { getAchievements, getProgress, getRecentActivity, type ActivityItem } from "@/lib/learner";

export const metadata: Metadata = { title: "Dashboard" };

const activityLabel: Record<ActivityItem["type"], string> = {
  "lesson-opened": "Opened",
  "lab-started": "Started the lab for",
  "lesson-completed": "Completed",
  "achievement-earned": "Earned an achievement",
  "hint-revealed": "Revealed a hint in",
};

const timeFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  hour: "2-digit",
  minute: "2-digit",
});

async function Dashboard() {
  const me = await getCurrentUser();
  if (!me) redirect("/login?callbackUrl=/dashboard");
  if (!me.onboardingCompletedAt) redirect("/onboarding");

  const [progress, courses, achievements, activity] = await Promise.allSettled([
    getProgress(),
    listCourses(),
    getAchievements(),
    getRecentActivity(5),
  ]);
  const titles = new Map(courses.status === "fulfilled" ? courses.value.map((c) => [c.slug, c.title]) : []);

  if (progress.status === "rejected") {
    return (
      <ErrorState
        title="Couldn't load your progress"
        description="Something went wrong on our side. Your progress is safe; try again in a moment."
        action={
          <Button asChild variant="outline">
            <Link href="/dashboard">Try again</Link>
          </Button>
        }
      />
    );
  }
  const p = progress.value;
  const next = p.continueLesson ?? p.recommendedLesson;
  const earned = achievements.status === "fulfilled" ? achievements.value.filter((a) => a.awardedAt) : [];

  return (
    <div className="grid gap-6 lg:grid-cols-3">
      <section aria-labelledby="continue-heading" className="lg:col-span-2">
        <h2 id="continue-heading" className="sr-only">
          {p.continueLesson ? "Continue learning" : "Start learning"}
        </h2>
        {next ? (
          <Card className="h-full">
            <CardHeader>
              <p className="text-sm text-muted-foreground">
                {p.continueLesson ? "Continue where you left off" : "Recommended next"}
              </p>
              <CardTitle className="text-xl">
                <h3>{next.title}</h3>
              </CardTitle>
              <p className="text-sm text-muted-foreground">{titles.get(next.courseSlug) ?? next.courseSlug}</p>
            </CardHeader>
            <CardContent>
              <Button asChild>
                <Link href={`/learn/${next.courseSlug}/${next.slug}`}>
                  {p.continueLesson ? "Continue" : "Start lesson"} <ArrowRight data-icon="inline-end" />
                </Link>
              </Button>
            </CardContent>
          </Card>
        ) : (
          <EmptyState
            icon={PlayCircle}
            title="You've finished every published lesson"
            description="New lessons are added regularly. Check the changelog to see what's coming."
            action={
              <Button asChild variant="outline">
                <Link href="/changelog">See what&apos;s new</Link>
              </Button>
            }
            headingLevel={3}
            className="h-full"
          />
        )}
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
          <CardContent>
            <p className="text-3xl font-semibold">
              {p.totals.currentStreak}{" "}
              <span className="text-base font-normal text-muted-foreground">
                {p.totals.currentStreak === 1 ? "day" : "days"}
              </span>
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              {p.totals.currentStreak === 0
                ? "Complete a lesson today to start a streak."
                : `Longest: ${p.totals.longestStreak} days.`}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Sparkles aria-hidden className="size-4 text-highlight" /> Experience
            </CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-3xl font-semibold">
              {p.totals.totalXp} <span className="text-base font-normal text-muted-foreground">XP</span>
            </p>
            <p className="mt-1 text-sm text-muted-foreground">
              {p.totals.completedLessons} {p.totals.completedLessons === 1 ? "lesson" : "lessons"} completed
            </p>
          </CardContent>
        </Card>
      </section>

      <section aria-labelledby="courses-heading" className="lg:col-span-2">
        <h2 id="courses-heading" className="mb-3 flex items-center gap-2 text-lg font-semibold">
          <BookOpen aria-hidden className="size-4" /> Course progress
        </h2>
        <ul className="grid gap-3">
          {p.courses.map((c) => {
            const pct = c.totalLessons ? Math.round((c.completedLessons / c.totalLessons) * 100) : 0;
            const title = titles.get(c.courseSlug) ?? c.courseSlug;
            return (
              <li key={c.courseSlug} className="rounded-xl border bg-card p-4">
                <div className="flex items-baseline justify-between gap-3">
                  <Link href={`/learn/${c.courseSlug}`} className="font-medium hover:underline">
                    {title}
                  </Link>
                  <span className="text-sm text-muted-foreground">
                    {c.completedLessons} of {c.totalLessons} lessons
                  </span>
                </div>
                <Bar value={pct} className="mt-3" aria-label={`${title}: ${pct}% complete`} />
              </li>
            );
          })}
        </ul>
      </section>

      <section aria-labelledby="activity-heading">
        <h2 id="activity-heading" className="mb-3 text-lg font-semibold">
          Recent activity
        </h2>
        {activity.status === "rejected" ? (
          <ErrorState title="Couldn't load activity" headingLevel={3} className="p-4" />
        ) : activity.value.length === 0 ? (
          <EmptyState
            title="No activity yet"
            description="Lessons you open and complete show up here."
            headingLevel={3}
            className="p-6"
          />
        ) : (
          <ol className="grid gap-2 text-sm">
            {activity.value.map((a, i) => (
              <li key={i} className="flex flex-col rounded-lg border p-3">
                <span>
                  {activityLabel[a.type]}{" "}
                  {a.lessonId && (
                    <Link href={`/learn/${a.lessonId}`} className="text-link underline underline-offset-4">
                      {a.lessonId.split("/")[1]?.replace(/-/g, " ")}
                    </Link>
                  )}
                </span>
                <time dateTime={a.occurredAt} className="text-xs text-muted-foreground">
                  {timeFormat.format(new Date(a.occurredAt))}
                </time>
              </li>
            ))}
          </ol>
        )}
      </section>

      <section aria-labelledby="achievements-heading" className="lg:col-span-3">
        <h2 id="achievements-heading" className="mb-3 text-lg font-semibold">
          Recent achievements
        </h2>
        {earned.length === 0 ? (
          <EmptyState
            icon={Trophy}
            title="No achievements yet"
            description="Achievements mark milestones like your first completed lesson or a seven-day streak."
            headingLevel={3}
          />
        ) : (
          <ul className="flex flex-wrap gap-3">
            {earned.map((a) => (
              <li key={a.key} className="rounded-lg border bg-card px-4 py-3 text-sm">
                <p className="font-medium">{a.title}</p>
                <p className="text-muted-foreground">{a.description}</p>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

async function Greeting() {
  const me = await getCurrentUser();
  return me ? <span>Welcome back, {me.displayName || me.handle}.</span> : null;
}

export default function DashboardPage() {
  return (
    <>
      <PageHeader
        title="Dashboard"
        description={
          <Suspense fallback={<span>Your learning at a glance.</span>}>
            <Greeting />
          </Suspense>
        }
      />
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading your dashboard" />
            <CardListSkeleton count={3} />
          </>
        }
      >
        <Dashboard />
      </Suspense>
    </>
  );
}
