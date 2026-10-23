import type { Metadata } from "next";
import Link from "next/link";
import { unstable_rethrow } from "next/navigation";
import { Suspense } from "react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { ErrorState } from "@/components/feedback/states";
import { Container, PageHeader } from "@/components/layout/page-header";
import { CourseFilter } from "@/components/learning/course-filter";
import { Button } from "@/components/ui/button";
import { listCourses } from "@/lib/catalog";

export const metadata: Metadata = {
  title: "Curriculum",
  description: "Hands-on Kubernetes courses, from your first Pod to recovering a broken application.",
};

async function Catalog() {
  let courses;
  try {
    courses = await listCourses();
  } catch (err) {
    unstable_rethrow(err);
    return (
      <ErrorState
        title="Couldn't load the curriculum"
        description="The course catalog is temporarily unavailable. Please try again in a moment."
        action={
          <Button asChild variant="outline">
            <Link href="/learn">Try again</Link>
          </Button>
        }
      />
    );
  }
  return <CourseFilter courses={courses} />;
}

export default function LearnPage() {
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader
        title="Curriculum"
        description={
          <>
            Short lessons with hands-on labs. Lessons open as they&apos;re published; the{" "}
            <Link href="/paths" className="text-link underline underline-offset-4">
              learning path
            </Link>{" "}
            shows the full plan. New terms are explained in the{" "}
            <Link href="/learn/glossary" className="text-link underline underline-offset-4">
              glossary
            </Link>
            .
          </>
        }
      />
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading courses" />
            <CardListSkeleton count={4} />
          </>
        }
      >
        <Catalog />
      </Suspense>
    </Container>
  );
}
