import type { Metadata } from "next";
import Link from "next/link";
import { Suspense } from "react";
import { Container, PageHeader } from "@/components/layout/page-header";
import { Syllabus, SyllabusWithStatus } from "@/components/learning/syllabus";
import { totalPlannedLessons } from "@/lib/curriculum";

export const metadata: Metadata = {
  title: "Curriculum",
  description: "Hands-on Kubernetes courses, from your first Pod to recovering a broken application.",
};

export default function LearnPage() {
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader
        title="Curriculum"
        description={
          <>
            Six courses and {totalPlannedLessons} lessons, white belt to black. Each lesson has a lab with its own
            simulated cluster. Lessons open as they&apos;re written and reviewed; new terms are explained in the{" "}
            <Link href="/learn/glossary" className="text-link underline underline-offset-4">
              glossary
            </Link>
            .
          </>
        }
      />
      <Suspense fallback={<Syllabus />}>
        <SyllabusWithStatus />
      </Suspense>
    </Container>
  );
}
