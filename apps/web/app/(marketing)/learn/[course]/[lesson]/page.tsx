import type { Metadata } from "next";
import Link from "next/link";
import { notFound, unstable_rethrow } from "next/navigation";
import { Suspense } from "react";
import { ArrowLeft, ArrowRight, Clock, ExternalLink, FlaskConical } from "lucide-react";
import { Belt, beltName } from "@/components/brand/belt";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { LoadingStatus } from "@/components/feedback/loading";
import { StartLabButton } from "@/components/lab/start-lab-button";
import { Container } from "@/components/layout/page-header";
import { LessonOpened } from "@/components/learning/lesson-opened";
import { LessonShortcuts } from "@/components/learning/lesson-shortcuts";
import { Markdown } from "@/components/learning/markdown";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Kbd } from "@/components/ui/kbd";
import { Skeleton } from "@/components/ui/skeleton";
import { auth } from "@/lib/auth/config";
import { getLesson, isSlug, NotPublishedError } from "@/lib/catalog";
import { courseOrder, courseTitle } from "@/lib/curriculum";
import { glossaryById } from "@/lib/glossary.generated";

type Props = PageProps<"/learn/[course]/[lesson]">;

async function load(course: string, lesson: string) {
  if (!isSlug(course) || !isSlug(lesson)) notFound();
  try {
    return await getLesson(course, lesson);
  } catch (err) {
    if (err instanceof NotPublishedError) notFound();
    throw err;
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { course, lesson } = await params;
  try {
    const l = await load(course, lesson);
    return { title: l.title, description: l.summary };
  } catch (err) {
    unstable_rethrow(err);
    return { title: "Lesson" };
  }
}

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

async function LessonView({ params }: Props) {
  const { course, lesson: slug } = await params;
  const [lesson, session] = await Promise.all([load(course, slug), auth()]);
  const published = new Map(lesson.prerequisiteLessons.map((p) => [`${p.courseSlug}/${p.slug}`, p]));
  const terms = lesson.glossary.map((id) => glossaryById.get(id)).filter((t) => t !== undefined);
  const prev = lesson.navigation.previous;
  const next = lesson.navigation.next;
  const href = (r: { courseSlug: string; slug: string }) => `/learn/${r.courseSlug}/${r.slug}`;
  const order = courseOrder(lesson.courseSlug) ?? 1;

  return (
    <article aria-labelledby="lesson-title">
      {session?.subject && <LessonOpened courseSlug={lesson.courseSlug} lessonSlug={lesson.slug} />}
      <LessonShortcuts previous={prev ? href(prev) : undefined} next={next ? href(next) : undefined} />
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href="/learn">Curriculum</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbLink asChild>
              <Link href={`/learn/${lesson.courseSlug}`}>{courseTitle(lesson.courseSlug)}</Link>
            </BreadcrumbLink>
          </BreadcrumbItem>
          <BreadcrumbSeparator />
          <BreadcrumbItem>
            <BreadcrumbPage>{lesson.title}</BreadcrumbPage>
          </BreadcrumbItem>
        </BreadcrumbList>
      </Breadcrumb>

      <header className="mt-6 space-y-3">
        <p className="flex items-center gap-3 font-mono text-xs text-muted-foreground">
          <Belt order={order} />
          {beltName(order).toLowerCase()} belt · lesson {lesson.displayOrder}
        </p>
        <h1 id="lesson-title" className="text-h2 font-semibold tracking-tight text-balance sm:text-h1">
          {lesson.title}
        </h1>
        <p className="max-w-3xl text-lg text-muted-foreground">{lesson.summary}</p>
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <Clock aria-hidden className="size-4" /> {lesson.estimatedMinutes} min
          </span>
          <span className="inline-flex items-center gap-1">
            <FlaskConical aria-hidden className="size-4" /> Simulated lab
          </span>
          <span>Up to {lesson.reward.baseXp + lesson.reward.firstTryBonusXp} XP</span>
        </div>
      </header>

      <div className="mt-10 grid gap-10 lg:grid-cols-[1fr_18rem]">
        <div className="min-w-0">
          <section aria-labelledby="objectives-heading" className="border-l-2 border-highlight py-1 pl-5">
            <h2 id="objectives-heading" className="font-semibold">
              By the end you&apos;ll be able to
            </h2>
            <ul className="mt-3 list-disc space-y-1 pl-6 text-sm">
              {lesson.objectives.map((o) => (
                <li key={o}>{o}</li>
              ))}
            </ul>
            {lesson.prerequisites.length > 0 && (
              <p className="mt-4 text-sm">
                <span className="font-medium">Before this lesson: </span>
                {lesson.prerequisites.map((id, i) => {
                  const ref = published.get(id);
                  return (
                    <span key={id}>
                      {i > 0 && ", "}
                      {ref ? (
                        <Link href={href(ref)} className="text-link underline underline-offset-4">
                          {ref.title}
                        </Link>
                      ) : (
                        <span>
                          {id.split("/")[1]?.replace(/-/g, " ")}{" "}
                          <span className="text-muted-foreground">(coming soon)</span>
                        </span>
                      )}
                    </span>
                  );
                })}
              </p>
            )}
          </section>

          <Markdown className="mt-6">{lesson.body}</Markdown>

          <section aria-labelledby="goals-heading" className="mt-12 rounded-lg border bg-card p-5 sm:p-6">
            <h2 id="goals-heading" className="text-lg font-bold">
              Lab goals
            </h2>
            <p className="mt-1 text-sm text-muted-foreground">
              Your lab has its own simulated cluster. Use the terminal to find each answer; automatic goal checks and
              hints arrive with the next release.
            </p>
            <ol className="mt-3 list-decimal space-y-1 pl-6 text-sm">
              {lesson.goals.map((g) => (
                <li key={g.id}>{g.description}</li>
              ))}
            </ol>
            <div className="mt-5 flex flex-wrap items-center gap-3">
              {session?.subject ? (
                <StartLabButton challengeVersionId={lesson.challengeVersionId} />
              ) : (
                <Button asChild>
                  <Link href={`/login?callbackUrl=${encodeURIComponent(`/learn/${lesson.courseSlug}/${lesson.slug}`)}`}>
                    <FlaskConical /> Sign in to start the lab
                  </Link>
                </Button>
              )}
              <InDevelopmentBadge label="Read-only commands for now" />
            </div>
          </section>

          <nav
            aria-label="Lesson navigation"
            className="mt-10 flex flex-col gap-3 border-t pt-6 sm:flex-row sm:justify-between"
          >
            {prev ? (
              <Link href={href(prev)} className="group inline-flex items-center gap-2 text-sm">
                <ArrowLeft aria-hidden className="size-4" />
                <span>
                  <span className="block text-xs text-muted-foreground">Previous</span>
                  <span className="text-link group-hover:underline">{prev.title}</span>
                </span>
              </Link>
            ) : (
              <span />
            )}
            {next && (
              <Link href={href(next)} className="group inline-flex items-center gap-2 text-right text-sm sm:ml-auto">
                <span>
                  <span className="block text-xs text-muted-foreground">Next</span>
                  <span className="text-link group-hover:underline">{next.title}</span>
                </span>
                <ArrowRight aria-hidden className="size-4" />
              </Link>
            )}
          </nav>
          <p className="mt-3 text-xs text-muted-foreground">
            Tip: press <Kbd>[</Kbd> or <Kbd>]</Kbd> for the previous or next lesson.
          </p>
        </div>

        <aside className="space-y-8 text-sm">
          {terms.length > 0 && (
            <section aria-labelledby="terms-heading">
              <h2 id="terms-heading" className="font-semibold">
                Key terms
              </h2>
              <dl className="mt-3 space-y-3">
                {terms.map((t) => (
                  <div key={t.id}>
                    <dt className="font-medium">{t.term}</dt>
                    <dd className="text-muted-foreground">{t.definition}</dd>
                  </div>
                ))}
              </dl>
              <Link href="/learn/glossary" className="mt-3 inline-block text-link underline underline-offset-4">
                Full glossary
              </Link>
            </section>
          )}
          {lesson.references.length > 0 && (
            <section aria-labelledby="refs-heading">
              <h2 id="refs-heading" className="font-semibold">
                Further reading
              </h2>
              <ul className="mt-3 space-y-2">
                {lesson.references.map((r) => (
                  <li key={r.url}>
                    <a
                      href={r.url}
                      rel="noreferrer"
                      className="inline-flex items-start gap-1 text-link underline underline-offset-4"
                    >
                      {r.title} <ExternalLink aria-hidden className="mt-0.5 size-3 shrink-0" />
                    </a>
                  </li>
                ))}
              </ul>
            </section>
          )}
          <section aria-labelledby="about-heading" className="text-muted-foreground">
            <h2 id="about-heading" className="font-semibold text-foreground">
              About this lesson
            </h2>
            <ul className="mt-3 space-y-1">
              <li>
                Last reviewed{" "}
                <time dateTime={lesson.lastReviewedAt}>
                  {dateFormat.format(new Date(`${lesson.lastReviewedAt}T00:00:00Z`))}
                </time>
              </li>
              <li>
                Simulator: <Badge variant="outline">{lesson.simulatorVersion}</Badge>
              </li>
            </ul>
          </section>
        </aside>
      </div>
    </article>
  );
}

function LessonSkeleton() {
  return (
    <div aria-hidden className="space-y-4">
      <Skeleton className="h-4 w-64" />
      <Skeleton className="h-10 w-3/4" />
      <Skeleton className="h-5 w-1/2" />
      <Skeleton className="mt-8 h-40 w-full" />
    </div>
  );
}

export default function LessonPage(props: Props) {
  return (
    <Container className="py-12 sm:py-16">
      <Suspense
        fallback={
          <>
            <LoadingStatus label="Loading lesson" />
            <LessonSkeleton />
          </>
        }
      >
        <LessonView {...props} />
      </Suspense>
    </Container>
  );
}
