import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Belt, beltName } from "@/components/brand/belt";
import { listCourses } from "@/lib/catalog";
import { plannedCourses, type PlannedCourse } from "@/lib/curriculum";

type Status = { published: number; planned: number };

/** The six courses as belts, in order. With `status`, each row says how many lessons are open. */
export function BeltLadder({ courses, status }: { courses: PlannedCourse[]; status?: Map<string, Status> }) {
  return (
    <ol className="border-t">
      {courses.map((course, i) => {
        const s = status?.get(course.slug);
        const open = Boolean(s && s.published > 0);
        const body = (
          <>
            <span className="flex items-center gap-3 sm:w-36 sm:shrink-0">
              <Belt order={i + 1} />
              <span className="font-mono text-caption text-muted-foreground">{beltName(i + 1).toLowerCase()} belt</span>
            </span>
            <span className="min-w-0 flex-1">
              <span className="flex flex-wrap items-baseline gap-x-2">
                <span className={open ? "text-body font-semibold group-hover:underline" : "text-body font-semibold"}>
                  {course.title}
                </span>
                <span className="font-mono text-caption text-muted-foreground tabular-nums">
                  {s
                    ? open
                      ? `${s.published} of ${s.planned} open`
                      : "being written"
                    : `${course.lessons.length} lessons`}
                </span>
              </span>
              <span className="mt-1 block text-small text-foreground-secondary">{course.summary}</span>
            </span>
          </>
        );
        return (
          <li key={course.slug} className="border-b">
            {open ? (
              <Link
                href={`/learn/${course.slug}`}
                className="group -mx-2 flex flex-col gap-2 rounded-md px-2 py-5 transition-colors duration-150 ease-out hover:bg-hover sm:flex-row sm:items-start sm:gap-6"
              >
                {body}
              </Link>
            ) : (
              <div className="flex flex-col gap-2 py-5 sm:flex-row sm:items-start sm:gap-6">{body}</div>
            )}
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
