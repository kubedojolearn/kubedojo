import type { Metadata } from "next";
import { PageHeader } from "@/components/layout/page-header";
import { Badge } from "@/components/ui/badge";
import { plannedCourses, totalPlannedLessons } from "@/lib/curriculum";

export const metadata: Metadata = { title: "Learning paths" };

export default function PathsPage() {
  return (
    <>
      <PageHeader title="Learning paths" description="A path is an ordered route through the courses toward a goal." />
      <section aria-labelledby="foundations-heading" className="rounded-xl border bg-card p-6">
        <div className="flex flex-wrap items-center gap-2">
          <h2 id="foundations-heading" className="text-lg font-semibold">
            Kubernetes foundations
          </h2>
          <Badge variant="outline">{totalPlannedLessons} lessons</Badge>
        </div>
        <p className="mt-1 text-sm text-muted-foreground">
          All six courses in the recommended order. Start here if Kubernetes is new to you.
        </p>
        <ol className="mt-6 grid gap-3">
          {plannedCourses.map((course, i) => (
            <li key={course.slug} className="flex items-start gap-4 rounded-lg border p-4">
              <span className="grid size-8 shrink-0 place-items-center rounded-full bg-muted font-mono text-sm">
                {i + 1}
              </span>
              <div>
                <p className="font-medium">{course.title}</p>
                <p className="text-sm text-muted-foreground">
                  {course.lessons.length} lessons · <span className="capitalize">{course.difficulty}</span>
                </p>
              </div>
            </li>
          ))}
        </ol>
      </section>
    </>
  );
}
