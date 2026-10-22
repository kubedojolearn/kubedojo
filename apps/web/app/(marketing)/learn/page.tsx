import type { Metadata } from "next";
import { Info } from "lucide-react";
import { Container, PageHeader } from "@/components/layout/page-header";
import { CourseCard } from "@/components/marketing/course-card";
import { plannedCourses, totalPlannedLessons } from "@/lib/curriculum";

export const metadata: Metadata = {
  title: "Curriculum",
  description: `Six Kubernetes courses and ${totalPlannedLessons} hands-on lessons, from your first Pod to recovering a broken application.`,
};

export default function LearnPage() {
  const beginner = plannedCourses.filter((c) => c.difficulty === "beginner").length;
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader
        title="Curriculum"
        description={`Six courses and ${totalPlannedLessons} lessons. Start with the ${beginner} beginner courses in order, then pick any intermediate course.`}
      />
      <div role="note" className="mb-10 flex gap-3 rounded-xl border bg-card p-4 text-sm">
        <Info aria-hidden className="mt-0.5 size-4 shrink-0 text-highlight" />
        <p>
          Lessons open as they are published. This page shows the full plan; lesson pages, progress tracking and labs
          arrive as each lesson is finished.
        </p>
      </div>
      <ol className="grid gap-6 md:grid-cols-2">
        {plannedCourses.map((course, i) => (
          <li key={course.slug}>
            <CourseCard course={course} index={i} showLessons />
          </li>
        ))}
      </ol>
    </Container>
  );
}
