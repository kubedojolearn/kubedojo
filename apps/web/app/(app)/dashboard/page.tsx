import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { ArrowRight, PlayCircle, SquareTerminal, Trophy } from "lucide-react";
import { Belt, beltName } from "@/components/brand/belt";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { EmptyState, ErrorState } from "@/components/feedback/states";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Progress as Bar } from "@/components/ui/progress";
import { getCurrentUser } from "@/lib/auth/current-user";
import { courseOrder, courseTitle, lessonTitle, plannedCourses } from "@/lib/curriculum";
import { listActiveLabs } from "@/lib/labs";
import { getAchievements, getProgress, getRecentActivity, type ActivityItem, type Progress } from "@/lib/learner";

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
  timeZone: "UTC",
});

/**
 * The learner's belt: the last course in order whose published lessons are all complete, plus
 * one. Everyone starts on the white belt.
 */
function currentBelt(p: Progress): number {
  let belt = 1;
  for (const course of plannedCourses) {
    const c = p.courses.find((x) => x.courseSlug === course.slug);
    if (!c || c.totalLessons === 0 || c.completedLessons < c.totalLessons) break;
    belt = Math.min((courseOrder(course.slug) ?? 1) + 1, plannedCourses.length);
  }
  return belt;
}

async function Dashboard() {
  const me = await getCurrentUser();
  if (!me) redirect("/login?callbackUrl=/dashboard");
  if (!me.onboardingCompletedAt) redirect("/onboarding");

  const [progress, labs, achievements, activity] = await Promise.allSettled([
    getProgress(),
    listActiveLabs(3),
    getAchievements(),
    getRecentActivity(6),
  ]);

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
  const activeLabs = labs.status === "fulfilled" ? labs.value : [];
  const belt = currentBelt(p);

  return (
    <div className="grid gap-10">
      <div className="grid gap-6 lg:grid-cols-[1.5fr_1fr]">
        <section aria-labelledby="continue-heading" className="rounded-lg border bg-card p-6">
          <h2 id="continue-heading" className="text-sm text-muted-foreground">
            {p.continueLesson ? "Continue where you left off" : next ? "Recommended next" : "Up next"}
          </h2>
          {next ? (
            <>
              <h3 className="mt-2 text-2xl font-bold text-balance">{next.title}</h3>
              <p className="mt-2 flex items-center gap-2 text-sm text-muted-foreground">
                <Belt order={courseOrder(next.courseSlug) ?? 1} />
                {courseTitle(next.courseSlug)}
              </p>
              <Button asChild className="mt-5">
                <Link href={`/learn/${next.courseSlug}/${next.slug}`}>
                  {p.continueLesson ? "Continue" : "Start lesson"} <ArrowRight data-icon="inline-end" />
                </Link>
              </Button>
            </>
          ) : (
            <EmptyState
              icon={PlayCircle}
              title="You've finished every published lesson"
              description="New lessons open regularly. The changelog says what's coming."
              action={
                <Button asChild variant="outline">
                  <Link href="/changelog">See what&apos;s new</Link>
                </Button>
              }
              headingLevel={3}
              className="mt-3 border-0 bg-transparent p-0 sm:p-0"
            />
          )}

          {activeLabs.length > 0 && (
            <div className="mt-6 border-t pt-5">
              <h3 className="text-sm font-semibold">Labs in progress</h3>
              <ul className="mt-2 grid gap-1">
                {activeLabs.map((lab) => (
                  <li key={lab.id}>
                    <Link
                      href={`/lab/${lab.id}`}
                      className="-mx-2 flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
                    >
                      <SquareTerminal aria-hidden className="size-4 text-highlight" />
                      <span className="font-medium">{lab.title}</span>
                      <span className="ml-auto text-xs text-muted-foreground tabular-nums">
                        {lab.commandSeq} {lab.commandSeq === 1 ? "command" : "commands"}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </section>

        <section aria-labelledby="stats-heading" className="rounded-lg border bg-card p-6">
          <h2 id="stats-heading" className="sr-only">
            Your stats
          </h2>
          <div className="flex items-center gap-3">
            <Belt order={belt} className="h-5 w-12" />
            <div>
              <p className="font-heading text-lg font-bold">{beltName(belt)} belt</p>
              <p className="text-xs text-muted-foreground">
                Finish {courseTitle(plannedCourses[belt - 1]!.slug)} to move up
              </p>
            </div>
          </div>
          <dl className="mt-6 grid grid-cols-3 gap-4 border-t pt-5">
            <div>
              <dt className="text-xs text-muted-foreground">Streak</dt>
              <dd className="mt-1 text-2xl font-bold tabular-nums">
                {p.totals.currentStreak}{" "}
                <span className="text-sm font-normal text-muted-foreground">
                  {p.totals.currentStreak === 1 ? "day" : "days"}
                </span>
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Experience</dt>
              <dd className="mt-1 text-2xl font-bold tabular-nums">
                {p.totals.totalXp} <span className="text-sm font-normal text-muted-foreground">XP</span>
              </dd>
            </div>
            <div>
              <dt className="text-xs text-muted-foreground">Lessons done</dt>
              <dd className="mt-1 text-2xl font-bold tabular-nums">{p.totals.completedLessons}</dd>
            </div>
          </dl>
          <p className="mt-4 text-xs text-muted-foreground">
            {p.totals.currentStreak === 0
              ? "Complete a lesson today to start a streak."
              : `Longest streak: ${p.totals.longestStreak} days.`}
          </p>
        </section>
      </div>

      <section aria-labelledby="courses-heading">
        <h2 id="courses-heading" className="text-lg font-bold">
          Course progress
        </h2>
        <ul className="mt-3 divide-y rounded-lg border bg-card">
          {p.courses.map((c) => {
            const pct = c.totalLessons ? Math.round((c.completedLessons / c.totalLessons) * 100) : 0;
            const title = courseTitle(c.courseSlug);
            return (
              <li key={c.courseSlug} className="grid gap-3 p-4 sm:grid-cols-[1fr_12rem] sm:items-center sm:px-5">
                <div className="flex items-center gap-3">
                  <Belt order={courseOrder(c.courseSlug) ?? 1} />
                  <Link href={`/learn/${c.courseSlug}`} className="font-medium hover:underline">
                    {title}
                  </Link>
                </div>
                <div className="grid gap-1.5">
                  <Bar value={pct} aria-label={`${title}: ${pct}% complete`} />
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {c.completedLessons} of {c.totalLessons} lessons
                  </span>
                </div>
              </li>
            );
          })}
        </ul>
      </section>

      <div className="grid gap-10 lg:grid-cols-2">
        <section aria-labelledby="activity-heading">
          <h2 id="activity-heading" className="text-lg font-bold">
            Recent activity
          </h2>
          {activity.status === "rejected" ? (
            <ErrorState title="Couldn't load activity" headingLevel={3} className="mt-3" />
          ) : activity.value.length === 0 ? (
            <EmptyState
              title="No activity yet"
              description="Lessons you open and labs you start show up here."
              headingLevel={3}
              className="mt-3"
            />
          ) : (
            <ol className="mt-3 divide-y rounded-lg border bg-card text-sm">
              {activity.value.map((a, i) => (
                <li key={i} className="flex items-baseline justify-between gap-4 px-4 py-3">
                  <span>
                    {activityLabel[a.type]}{" "}
                    {a.lessonId && (
                      <Link href={`/learn/${a.lessonId}`} className="text-link underline-offset-4 hover:underline">
                        {lessonTitle(a.lessonId)}
                      </Link>
                    )}
                  </span>
                  <time dateTime={a.occurredAt} className="shrink-0 text-xs text-muted-foreground tabular-nums">
                    {timeFormat.format(new Date(a.occurredAt))}
                  </time>
                </li>
              ))}
            </ol>
          )}
        </section>

        <section aria-labelledby="achievements-heading">
          <h2 id="achievements-heading" className="text-lg font-bold">
            Achievements
          </h2>
          {earned.length === 0 ? (
            <EmptyState
              icon={Trophy}
              title="No achievements yet"
              description="Achievements mark milestones like your first completed lesson or a seven-day streak."
              headingLevel={3}
              className="mt-3"
            />
          ) : (
            <ul className="mt-3 divide-y rounded-lg border bg-card text-sm">
              {earned.map((a) => (
                <li key={a.key} className="px-4 py-3">
                  <p className="font-medium">{a.title}</p>
                  <p className="text-muted-foreground">{a.description}</p>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
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
