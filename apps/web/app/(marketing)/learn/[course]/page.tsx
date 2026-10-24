import type { Metadata } from "next";
import Link from "next/link";
import { notFound, unstable_rethrow } from "next/navigation";
import { Suspense } from "react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { Container } from "@/components/layout/page-header";
import { Markdown } from "@/components/learning/markdown";
import { Belt, beltName } from "@/components/brand/belt";
import { Badge } from "@/components/ui/badge";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { getCourse, isSlug, NotPublishedError } from "@/lib/catalog";
import { courseOrder } from "@/lib/curriculum";

type Props = PageProps<"/learn/[course]">;

async function load(slug: string) {
  if (!isSlug(slug)) notFound();
  try {
    return await getCourse(slug);
  } catch (err) {
    if (err instanceof NotPublishedError) notFound();
    throw err;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { course } = await params;
  try {
    const c = await load(course);
    return { title: c.title, description: c.summary };
  } catch (err) {
    unstable_rethrow(err);
    return { title: "Course" };
  }
}

const goalLabel: Record<string, string> = { state: "Build it", answer: "Investigate", observation: "Explore" };

async function CourseView({ params }: Props) {
  const { course: slug } = await params;
  const course = await load(slug);
  const planned = course.plannedLessonCount - course.lessonCount;
  const order = courseOrder(course.slug) ?? course.displayOrder;
  return (
    <>
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/learn">Curriculum</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{course.title}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>
      <header className="mt-6 space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <Belt order={order} className="h-4 w-10" />
          <span className="font-mono text-xs text-muted-foreground">
            {beltName(order).toLowerCase()} belt · course {order} · {course.difficulty}
          </span>
        </div>
        <h1 className="text-h2 font-semibold tracking-tight text-balance sm:text-h1">{course.title}</h1>
        <p className="text-sm text-muted-foreground tabular-nums">
          {course.lessonCount} of {course.plannedLessonCount} lessons open · about {course.estimatedMinutes} minutes
        </p>
        <Markdown className="text-muted-foreground">{course.description}</Markdown>
        {course.prerequisites.length > 0 && (
          <p className="text-sm">
            Recommended first:{" "}
            {course.prerequisites.map((p, i) => (
              <span key={p}>
                {i > 0 && ", "}
                <Link href={`/learn/${p}`} className="text-link underline underline-offset-4">
                  {p.replace(/-/g, " ")}
                </Link>
              </span>
            ))}
          </p>
        )}
      </header>
      <section aria-labelledby="lessons-heading" className="mt-10">
        <h2 id="lessons-heading" className="text-xl font-bold">
          Lessons
        </h2>
        <ol className="mt-4 divide-y rounded-lg border bg-card">
          {course.lessons.map((l, i) => (
            <li key={l.slug} className="relative p-4 transition-colors focus-within:bg-accent/40 hover:bg-accent/40">
              <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
                <span className="font-mono text-sm text-muted-foreground">{i + 1}.</span>
                <Link
                  href={`/learn/${course.slug}/${l.slug}`}
                  className="font-semibold after:absolute after:inset-0 focus-visible:outline-none"
                >
                  {l.title}
                </Link>
                <span className="text-xs text-muted-foreground">{l.estimatedMinutes} min</span>
                {l.goalTypes.map((g) => (
                  <Badge key={g} variant="outline">
                    {goalLabel[g] ?? g}
                  </Badge>
                ))}
              </div>
              <p className="mt-1 pl-7 text-sm text-muted-foreground">{l.summary}</p>
            </li>
          ))}
        </ol>
        {planned > 0 && (
          <p className="mt-4 text-sm text-muted-foreground">
            {planned} more {planned === 1 ? "lesson is" : "lessons are"} being written for this course.
          </p>
        )}
      </section>
    </>
  );
}

export default function CoursePage(props: Props) {
  return (
    <Container className="max-w-4xl py-12 sm:py-16">
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading course" />
            <CardListSkeleton count={2} />
          </>
        }
      >
        <CourseView {...props} />
      </Suspense>
    </Container>
  );
}
