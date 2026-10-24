import Link from "next/link";
import { ArrowRight, BookOpen, CircleCheck, Eye, GraduationCap, ShieldCheck, SquareTerminal } from "lucide-react";
import { GithubIcon } from "@/components/brand/github-icon";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { Container } from "@/components/layout/page-header";
import { CourseCard } from "@/components/marketing/course-card";
import { TerminalPreview } from "@/components/marketing/terminal-preview";
import { Button } from "@/components/ui/button";
import { plannedCourses, totalPlannedLessons } from "@/lib/curriculum";
import { site } from "@/lib/site";

const steps = [
  {
    icon: BookOpen,
    title: "Learn the idea",
    body: "Each lesson starts with a short, plain-language explanation of one concept: what a Deployment is for, why a Service needs a selector, what the scheduler is doing.",
  },
  {
    icon: SquareTerminal,
    title: "Try it in a cluster",
    body: "Run real kubectl commands against your own simulated cluster. Edit YAML, apply it, and watch Pods, ReplicaSets and Services change on the cluster map.",
  },
  {
    icon: CircleCheck,
    title: "Check your work",
    body: "KubeDojo checks the state you produced, not the exact command you typed, so any correct approach passes. You get specific feedback on what's still missing.",
  },
];

export default function HomePage() {
  return (
    <>
      <section className="relative overflow-hidden border-b">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_top_right,color-mix(in_oklab,var(--primary)_22%,transparent),transparent_60%)]"
        />
        <Container className="relative grid items-center gap-12 py-16 sm:py-24 lg:grid-cols-[1.05fr_1fr]">
          <div className="space-y-6">
            <div className="flex flex-wrap items-center gap-2">
              <InDevelopmentBadge />
              <span className="text-sm text-muted-foreground">Free and open source</span>
            </div>
            <h1 className="text-4xl font-semibold tracking-tight text-balance sm:text-5xl lg:text-6xl">
              Master Kubernetes, <span className="text-highlight">one command at a time.</span>
            </h1>
            <p className="max-w-xl text-lg text-pretty text-muted-foreground">
              KubeDojo teaches Kubernetes by doing. Run kubectl against a safe simulated cluster, see exactly what your
              commands changed, and get instant feedback on every challenge. Nothing to install.
            </p>
            <div className="flex flex-wrap gap-3">
              <Button asChild size="lg">
                <Link href="/learn">
                  Explore the curriculum <ArrowRight data-icon="inline-end" />
                </Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <a href="#how-it-works">How it works</a>
              </Button>
            </div>
          </div>
          <TerminalPreview />
        </Container>
      </section>

      <section id="how-it-works" aria-labelledby="how-heading" className="scroll-mt-20 py-20">
        <Container>
          <h2 id="how-heading" className="text-2xl font-semibold tracking-tight sm:text-3xl">
            How it works
          </h2>
          <p className="mt-2 max-w-2xl text-muted-foreground">
            Short lessons, hands-on practice and immediate feedback, in that order, every time.
          </p>
          <ol className="mt-10 grid gap-6 md:grid-cols-3">
            {steps.map((step, i) => (
              <li key={step.title} className="rounded-xl border bg-card p-6">
                <div className="flex items-center gap-3">
                  <span className="grid size-10 place-items-center rounded-lg bg-primary/15 text-link">
                    <step.icon aria-hidden className="size-5" />
                  </span>
                  <span className="font-mono text-sm text-muted-foreground">Step {i + 1}</span>
                </div>
                <h3 className="mt-4 text-lg font-semibold">{step.title}</h3>
                <p className="mt-2 text-sm text-muted-foreground">{step.body}</p>
              </li>
            ))}
          </ol>
        </Container>
      </section>

      <section aria-labelledby="curriculum-heading" className="border-y bg-card/40 py-20">
        <Container>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 id="curriculum-heading" className="text-2xl font-semibold tracking-tight sm:text-3xl">
                What you&apos;ll learn
              </h2>
              <p className="mt-2 max-w-2xl text-muted-foreground">
                Six courses and {totalPlannedLessons} hands-on lessons, from your first Pod to recovering a broken
                application.
              </p>
            </div>
            <Button asChild variant="outline">
              <Link href="/learn">
                See every lesson <ArrowRight data-icon="inline-end" />
              </Link>
            </Button>
          </div>
          <ul className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {plannedCourses.map((course, i) => (
              <li key={course.slug}>
                <CourseCard course={course} index={i} />
              </li>
            ))}
          </ul>
        </Container>
      </section>

      <section aria-labelledby="honest-heading" className="py-20">
        <Container className="grid gap-12 lg:grid-cols-2">
          <div>
            <h2 id="honest-heading" className="text-2xl font-semibold tracking-tight sm:text-3xl">
              A simulator, and honest about it
            </h2>
            <p className="mt-4 text-muted-foreground">
              Your cluster is simulated, so it starts instantly, costs nothing and can&apos;t be broken in ways that
              teach you nothing. It implements a documented subset of Kubernetes {site.kubernetesVersion}: Pods,
              Deployments, ReplicaSets, Services, ConfigMaps, Secrets, Nodes and Namespaces, with the controllers that
              connect them.
            </p>
            <p className="mt-4 text-muted-foreground">
              When you step outside that subset, KubeDojo tells you plainly instead of pretending. Lessons point out
              where a real cluster would behave differently.
            </p>
            <Button asChild variant="link" className="mt-2 h-auto px-0 text-left whitespace-normal">
              <a href={site.compatibilityUrl} rel="noreferrer">
                Read exactly what the simulator supports <ArrowRight data-icon="inline-end" />
              </a>
            </Button>
          </div>
          <ul className="grid gap-4 sm:grid-cols-2">
            {[
              {
                icon: Eye,
                title: "See cause and effect",
                body: "A live cluster map shows how one command ripples through controllers.",
              },
              {
                icon: ShieldCheck,
                title: "Safe by design",
                body: "No real containers, no shell. Your terminal only understands kubectl.",
              },
              {
                icon: GraduationCap,
                title: "Graded fairly",
                body: "Any approach that produces the right cluster state passes.",
              },
              {
                icon: GithubIcon,
                title: "Open source",
                body: "Apache 2.0. Read the code, the lessons and the design decisions.",
              },
            ].map((item) => (
              <li key={item.title} className="rounded-xl border p-5">
                <item.icon aria-hidden className="size-5 text-highlight" />
                <h3 className="mt-3 font-semibold">{item.title}</h3>
                <p className="mt-1 text-sm text-muted-foreground">{item.body}</p>
              </li>
            ))}
          </ul>
        </Container>
      </section>

      <section aria-labelledby="cta-heading" className="pb-8">
        <Container>
          <div className="rounded-2xl border bg-card px-6 py-12 text-center sm:px-12">
            <h2 id="cta-heading" className="text-2xl font-semibold tracking-tight sm:text-3xl">
              KubeDojo is being built in the open
            </h2>
            <p className="mx-auto mt-3 max-w-xl text-muted-foreground">
              Lessons open as they&apos;re finished. Follow along, try the curriculum as it lands, or help build it.
            </p>
            <div className="mt-6 flex flex-wrap justify-center gap-3">
              <Button asChild>
                <Link href="/learn">Explore the curriculum</Link>
              </Button>
              <Button asChild variant="outline">
                <a href={site.repoUrl} rel="noreferrer">
                  <GithubIcon /> Star or contribute
                </a>
              </Button>
            </div>
          </div>
        </Container>
      </section>
    </>
  );
}
