import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Belt, beltName } from "@/components/brand/belt";
import { listCourses } from "@/lib/catalog";
import { plannedCourses, type PlannedCourse } from "@/lib/curriculum";

type Status = { published: number; planned: number };

/** The six courses as belts, in order. With `status`, each row says how many lessons are open. */
export function BeltLadder({ courses, status }: { courses: PlannedCourse[]; status?: Map<string, Status> }) {
  return (
    <ol className="mt-10 divide-y rounded-lg border bg-background">
      {courses.map((course, i) => {
        const s = status?.get(course.slug);
        const open = Boolean(s && s.published > 0);
        return (
          <li
            key={course.slug}
            className="grid gap-x-6 gap-y-2 p-5 sm:grid-cols-[9rem_1fr_auto] sm:items-center sm:px-6"
          >
            <div className="flex items-center gap-3">
              <Belt order={i + 1} />
              <span className="font-mono text-xs text-muted-foreground">{beltName(i + 1).toLowerCase()} belt</span>
            </div>
            <div>
              <h3 className="font-semibold">
                {open ? (
                  <Link href={`/learn/${course.slug}`} className="underline-offset-4 hover:underline">
                    {course.title}
                  </Link>
                ) : (
                  course.title
                )}
              </h3>
              <p className="mt-1 text-sm text-muted-foreground">{course.summary}</p>
            </div>
            <p className="text-sm text-muted-foreground tabular-nums sm:text-right">
              {s ? (
                open ? (
                  <span className="text-foreground">
                    {s.published} of {s.planned} open
                  </span>
                ) : (
                  "Being written"
                )
              ) : (
                `${course.lessons.length} lessons`
              )}
            </p>
          </li>
        );
      })}
    </ol>
  );
}

/** The ladder with live counts from the catalog. Falls back to the plain ladder if the API is down. */
export async function BeltLadderStatus() {
  return <BeltLadder courses={plannedCourses} status={await loadStatus()} />;
}

async function loadStatus(): Promise<Map<string, Status> | undefined> {
  try {
    const courses = await listCourses();
    return new Map(courses.map((c) => [c.slug, { published: c.lessonCount, planned: c.plannedLessonCount }]));
  } catch (err) {
    unstable_rethrow(err);
    return undefined;
  }
}
