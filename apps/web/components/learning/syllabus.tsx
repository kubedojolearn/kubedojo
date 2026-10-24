import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Belt, beltName } from "@/components/brand/belt";
import { getCourse, listCourses } from "@/lib/catalog";
import { plannedCourses } from "@/lib/curriculum";

/** Published lesson slugs per course slug. */
type Published = Map<string, Set<string>>;

/**
 * The whole planned curriculum, one belt per course. Lessons that are published link to the
 * lesson; the rest are listed so learners can see where a course is going.
 */
export function Syllabus({ published, note }: { published?: Published; note?: string }) {
  return (
    <div className="grid gap-12">
      {note && (
        <p role="status" className="rounded-md border bg-card px-4 py-3 text-sm text-muted-foreground">
          {note}
        </p>
      )}
      {plannedCourses.map((course, i) => {
        const open = published?.get(course.slug);
        const openCount = open?.size ?? 0;
        return (
          <section
            key={course.slug}
            aria-labelledby={`course-${course.slug}`}
            className="grid gap-6 md:grid-cols-[14rem_1fr]"
          >
            <div>
              <div className="flex items-center gap-3">
                <Belt order={i + 1} className="h-4 w-10" />
                <span className="font-mono text-xs text-muted-foreground">
                  {beltName(i + 1).toLowerCase()} belt · course {i + 1}
                </span>
              </div>
              <h2 id={`course-${course.slug}`} className="mt-3 text-xl font-bold">
                {openCount > 0 ? (
                  <Link href={`/learn/${course.slug}`} className="underline-offset-4 hover:underline">
                    {course.title}
                  </Link>
                ) : (
                  course.title
                )}
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">{course.summary}</p>
              {published && (
                <p className="mt-3 text-xs text-muted-foreground tabular-nums">
                  {openCount > 0 ? `${openCount} of ${course.lessons.length} lessons open` : "Being written"}
                </p>
              )}
            </div>
            <ol className="divide-y rounded-lg border bg-card">
              {course.lessons.map((lesson, li) => {
                const isOpen = open?.has(lesson.slug);
                return (
                  <li key={lesson.slug} className="relative flex items-baseline gap-4 px-4 py-3 text-sm">
                    <span className="w-5 shrink-0 text-right font-mono text-xs text-muted-foreground">{li + 1}</span>
                    {isOpen ? (
                      <Link
                        href={`/learn/${course.slug}/${lesson.slug}`}
                        className="font-medium after:absolute after:inset-0 hover:underline focus-visible:outline-none"
                      >
                        {lesson.title}
                      </Link>
                    ) : (
                      <span className="text-muted-foreground">{lesson.title}</span>
                    )}
                    {published && (
                      <span
                        className={
                          isOpen
                            ? "ml-auto shrink-0 font-mono text-[11px] text-success"
                            : "ml-auto shrink-0 font-mono text-[11px] text-muted-foreground"
                        }
                      >
                        {isOpen ? "open" : "soon"}
                      </span>
                    )}
                  </li>
                );
              })}
            </ol>
          </section>
        );
      })}
    </div>
  );
}

/** The syllabus with what's published right now. Without the API it still shows the plan. */
export async function SyllabusWithStatus() {
  const published = await loadPublished();
  return published ? (
    <Syllabus published={published} />
  ) : (
    <Syllabus note="We couldn't check which lessons are open right now. Here's the full plan; try again shortly." />
  );
}

async function loadPublished(): Promise<Published | null> {
  try {
    const courses = await listCourses();
    const details = await Promise.all(courses.filter((c) => c.lessonCount > 0).map((c) => getCourse(c.slug)));
    return new Map(details.map((d) => [d.slug, new Set(d.lessons.map((l) => l.slug))]));
  } catch (err) {
    unstable_rethrow(err);
    return null;
  }
}
