import { unstable_rethrow } from "next/navigation";
import { SiteSearch, type SearchLesson } from "@/components/layout/site-search";
import { getCourse, listCourses } from "@/lib/catalog";

/** Loads the published lessons for search. If the catalog is down, search still covers terms and pages. */
export async function SiteSearchWithIndex({ className }: { className?: string }) {
  let lessons: SearchLesson[] | null = null;
  try {
    const courses = await listCourses();
    const details = await Promise.all(courses.filter((c) => c.lessonCount > 0).map((c) => getCourse(c.slug)));
    lessons = details.flatMap((course) =>
      course.lessons.map((l) => ({
        href: `/learn/${course.slug}/${l.slug}`,
        title: l.title,
        course: course.title,
        summary: l.summary,
      })),
    );
  } catch (err) {
    unstable_rethrow(err);
  }
  return <SiteSearch lessons={lessons} className={className} />;
}
