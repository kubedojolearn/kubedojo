import type { Metadata } from "next";
import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Suspense } from "react";
import { Belt, beltName } from "@/components/brand/belt";
import { PageHeader } from "@/components/layout/page-header";
import { Progress as Bar } from "@/components/ui/progress";
import { auth } from "@/lib/auth/config";
import { listCourses } from "@/lib/catalog";
import { plannedCourses, totalPlannedLessons } from "@/lib/curriculum";
import { getProgress } from "@/lib/learner";

export const metadata: Metadata = { title: "Learning path" };

type CourseStatus = { open: number; done: number };

function Path({ status, here }: { status?: Map<string, CourseStatus>; here?: string }) {
  return (
    <ol className="relative grid gap-4 before:absolute before:top-4 before:bottom-4 before:left-[1.4rem] before:w-px before:bg-border">
      {plannedCourses.map((course, i) => {
        const s = status?.get(course.slug);
        const isHere = here === course.slug;
        const pct = s && s.open > 0 ? Math.round((s.done / s.open) * 100) : 0;
        return (
          <li
            key={course.slug}
            aria-current={isHere ? "step" : undefined}
            className="relative grid grid-cols-[2.8rem_1fr] items-start gap-4"
          >
            <span className="relative z-10 grid size-11 place-items-center rounded-full border bg-background">
              <Belt order={i + 1} className="w-7" />
            </span>
            <div
              className={isHere ? "rounded-lg border border-highlight/60 bg-card p-5" : "rounded-lg border bg-card p-5"}
            >
              <p className="font-mono text-xs text-muted-foreground">
                {beltName(i + 1).toLowerCase()} belt · {course.lessons.length} lessons
                {isHere && <span className="ml-2 text-highlight">you are here</span>}
              </p>
              <h2 className="mt-1 text-lg font-bold">
                {s && s.open > 0 ? (
                  <Link href={`/learn/${course.slug}`} className="underline-offset-4 hover:underline">
                    {course.title}
                  </Link>
                ) : (
                  course.title
                )}
              </h2>
              <p className="mt-1 text-sm text-muted-foreground">{course.summary}</p>
              {s &&
                (s.open > 0 ? (
                  <div className="mt-4 grid max-w-sm gap-1.5">
                    <Bar value={pct} aria-label={`${course.title}: ${pct}% of open lessons complete`} />
                    <span className="text-xs text-muted-foreground tabular-nums">
                      {s.done} of {s.open} open lessons done
                    </span>
                  </div>
                ) : (
                  <p className="mt-3 text-xs text-muted-foreground">Lessons for this belt are being written.</p>
                ))}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

/** The path with what's open and, when signed in, the learner's own progress. */
async function PathWithStatus() {
  const loaded = await loadStatus();
  return loaded ? <Path status={loaded.status} here={loaded.here} /> : <Path />;
}

async function loadStatus(): Promise<{ status: Map<string, CourseStatus>; here?: string } | null> {
  try {
    const [courses, session] = await Promise.all([listCourses(), auth()]);
    const progress = session?.subject ? await getProgress() : null;
    const status = new Map<string, CourseStatus>(
      courses.map((c) => [
        c.slug,
        { open: c.lessonCount, done: progress?.courses.find((p) => p.courseSlug === c.slug)?.completedLessons ?? 0 },
      ]),
    );
    const here = progress
      ? plannedCourses.find((c) => {
          const s = status.get(c.slug);
          return s && s.open > 0 && s.done < s.open;
        })?.slug
      : undefined;
    return { status, here };
  } catch (err) {
    unstable_rethrow(err);
    return null;
  }
}

export default function PathsPage() {
  return (
    <>
      <PageHeader
        title="Learning path"
        description={`Kubernetes foundations: all six courses, white belt to black, ${totalPlannedLessons} lessons. Start here if Kubernetes is new to you.`}
      />
      <section aria-label="Kubernetes foundations" className="max-w-3xl">
        <Suspense fallback={<Path />}>
          <PathWithStatus />
        </Suspense>
      </section>
    </>
  );
}
