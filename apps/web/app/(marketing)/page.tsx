import Link from "next/link";
import { ArrowRight, GitFork, LayoutDashboard, Route, ShieldCheck, Trophy, UserRoundX } from "lucide-react";
import { Suspense } from "react";
import { GithubIcon } from "@/components/brand/github-icon";
import { LogoMark } from "@/components/brand/logo";
import { BeltLadder, BeltLadderStatus } from "@/components/marketing/belt-ladder";
import { Eyebrow, Section, SectionIntro } from "@/components/marketing/section";
import { TerminalPreview } from "@/components/marketing/terminal-preview";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { plannedCourses, totalPlannedLessons } from "@/lib/curriculum";
import { site } from "@/lib/site";

const supportedKinds = [
  "Pod",
  "Deployment",
  "ReplicaSet",
  "Service",
  "EndpointSlice",
  "ConfigMap",
  "Secret",
  "Node",
  "Namespace",
];
const readCommands = [
  "get",
  "describe",
  "logs",
  "events",
  "explain",
  "rollout status",
  "rollout history",
  "api-resources",
];
const comingCommands = ["apply", "create", "scale", "edit", "delete", "label"];

const trust = [
  { icon: GitFork, label: "Open source" },
  { icon: ShieldCheck, label: "A simulated cluster: nothing can break" },
  { icon: UserRoundX, label: "No account needed to read" },
];

const steps = [
  {
    verb: "read",
    title: "One idea, explained plainly",
    body: "What a ReplicaSet is for. Why a Service needs a selector. A few paragraphs, a table, a term you can hover.",
  },
  {
    verb: "practise",
    title: "Your own cluster, in the browser",
    body: "Each lab starts a fresh simulated cluster set up for the lesson. Poke at it with kubectl; reset it whenever you like.",
  },
  {
    verb: "prove it",
    title: "Goals checked against the cluster",
    body: "The check looks at the state you produced, not the command you typed, so any correct approach passes. Automatic checking ships with the next release.",
  },
];

function Chip({ children, planned = false }: { children: string; planned?: boolean }) {
  return (
    <code
      className={cn(
        "rounded-sm border px-2 py-0.5 font-mono text-caption",
        planned
          ? "border-dashed border-border-strong text-muted-foreground"
          : "border-border-strong bg-elevated text-foreground-secondary",
      )}
    >
      {children}
    </code>
  );
}

export default function HomePage() {
  return (
    <>
      <section aria-labelledby="hero-heading" className="relative overflow-hidden">
        {/* A soft glow behind the hero. */}
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 -top-40 h-140 bg-[radial-gradient(ellipse_at_center,var(--glow),transparent_65%)]"
        />
        <div className="relative mx-auto grid max-w-[1280px] gap-12 px-4 pt-14 pb-16 sm:px-6 sm:pt-24 sm:pb-24 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] lg:items-center lg:gap-16">
          <div>
            <h1
              id="hero-heading"
              className="text-h1 font-bold tracking-[-0.03em] text-balance text-foreground sm:text-display"
            >
              Master Kubernetes, <span className="text-highlight">one command at a time.</span>
            </h1>
            <p className="mt-5 max-w-xl text-body-lg text-pretty text-foreground-secondary">
              Short lessons, then your own simulated cluster to practise on. Type real kubectl, read real output, and
              work out what&apos;s wrong the way you would at work, without a cloud bill or a broken laptop.
            </p>
            <div className="mt-8 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <Button asChild size="hero">
                <Link href="/learn">
                  Start learning <ArrowRight />
                </Link>
              </Button>
              <Button asChild size="hero" variant="outline">
                <Link href="/login">
                  <GithubIcon /> Sign in with GitHub
                </Link>
              </Button>
            </div>
            <ul className="mt-8 flex flex-wrap gap-x-5 gap-y-2 text-small text-muted-foreground">
              {trust.map(({ icon: Icon, label }) => (
                <li key={label} className="flex items-center gap-1.5">
                  <Icon aria-hidden className="size-4 text-faint" />
                  {label}
                </li>
              ))}
            </ul>
          </div>
          <div className="min-w-0">
            <TerminalPreview />
          </div>
        </div>
      </section>

      <Section id="how-it-works" labelledBy="lesson-heading">
        <div className="grid gap-x-16 gap-y-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:items-end">
          <SectionIntro
            id="lesson-heading"
            index="01"
            eyebrow="How a lesson goes"
            title="Every command changes the cluster. KubeDojo shows you how."
          />
          <p className="max-w-xl text-body-lg text-pretty text-foreground-secondary lg:pb-1">
            Ten to twenty minutes, in this order, every time: read the idea, try it on a cluster of your own, then prove
            it worked.
          </p>
        </div>
        {/* The steps sit on a line, like a rollout moving through its revisions. */}
        <ol className="relative mt-12 grid gap-10 sm:mt-16 lg:grid-cols-3 lg:gap-8">
          <span
            aria-hidden
            className="absolute top-2 bottom-2 left-[7px] w-px bg-border lg:top-[7px] lg:right-0 lg:bottom-auto lg:left-2 lg:h-px lg:w-auto"
          />
          {steps.map(({ verb, title, body }, i) => (
            <li key={verb} className="relative pl-9 lg:pt-10 lg:pl-0">
              <span
                aria-hidden
                className={cn(
                  "absolute top-1 left-0 size-[15px] rounded-full border-2 lg:top-0",
                  i === steps.length - 1
                    ? "border-highlight bg-highlight ring-4 ring-primary-soft"
                    : "border-highlight bg-background",
                )}
              />
              <p className="font-mono text-small text-highlight">
                <span className="text-muted-foreground">{String(i + 1).padStart(2, "0")} </span>
                {verb}
              </p>
              <h3 className="mt-2 text-h4 font-semibold text-foreground">{title}</h3>
              <p className="mt-2 max-w-sm text-small text-foreground-secondary">{body}</p>
            </li>
          ))}
        </ol>
        <p className="mt-12 flex items-start gap-2 text-small text-muted-foreground sm:items-center">
          <LayoutDashboard aria-hidden className="mt-0.5 size-4 shrink-0 sm:mt-0" />
          <span>
            Your{" "}
            <Link href="/dashboard" className="text-foreground-secondary underline hover:text-foreground">
              dashboard
            </Link>{" "}
            shows where to continue. Sign in with GitHub to save progress and run labs.
          </span>
        </p>
      </Section>

      <Section id="curriculum" labelledBy="curriculum-heading" band>
        <div className="grid gap-10 lg:grid-cols-[minmax(0,4fr)_minmax(0,8fr)] lg:gap-16">
          <div className="lg:sticky lg:top-24 lg:self-start">
            <SectionIntro
              id="curriculum-heading"
              index="02"
              eyebrow="What you can learn"
              title={`Six belts, ${totalPlannedLessons} lessons`}
            >
              Start at the white belt with what a cluster is made of. Finish at black, bringing a broken shop back
              online. Lessons open as they&apos;re written and reviewed.
            </SectionIntro>
            <Button asChild variant="outline" size="lg" className="mt-8">
              <Link href="/learn">
                Browse the curriculum <ArrowRight />
              </Link>
            </Button>
          </div>
          <Suspense fallback={<BeltLadder courses={plannedCourses} />}>
            <BeltLadderStatus />
          </Suspense>
        </div>
      </Section>

      <Section id="simulator" labelledBy="sim-heading">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-16">
          <div>
            <SectionIntro id="sim-heading" index="03" eyebrow="Inside the simulator" title="What the simulator does">
              A documented subset of Kubernetes {site.kubernetesVersion}. When you step outside it, it says so instead
              of pretending.
            </SectionIntro>
            <a
              href={site.compatibilityUrl}
              rel="noreferrer"
              className="mt-6 inline-flex items-center gap-1 text-small text-link underline-offset-4 hover:underline"
            >
              Full compatibility notes <ArrowRight aria-hidden className="size-3.5" />
            </a>
          </div>
          <dl className="divide-y divide-border-subtle border-y border-border-subtle">
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">Resources</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {supportedKinds.map((k) => (
                  <Chip key={k}>{k}</Chip>
                ))}
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">
                With the controllers that connect them: rollouts, scheduling, image pulls, crash loops, endpoints and
                garbage collection.
              </dd>
            </div>
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">kubectl, today</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {readCommands.map((c) => (
                  <Chip key={c}>{c}</Chip>
                ))}
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">
                Output matches kubectl&apos;s tables, YAML, JSON and error messages.
              </dd>
            </div>
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">kubectl, next</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {comingCommands.map((c) => (
                  <Chip key={c} planned>
                    {c}
                  </Chip>
                ))}
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">Changing the cluster, including YAML apply.</dd>
            </div>
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">Never</dt>
              <dd className="mt-2 text-small text-foreground-secondary">
                A shell, real containers, or anything you type leaving the simulator. Pipes and{" "}
                <code className="font-mono text-caption">$(…)</code> are refused, not filtered.
              </dd>
            </div>
          </dl>
        </div>
      </Section>

      <Section id="progress" labelledBy="progress-heading" band>
        <div className="grid gap-10 lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] lg:items-center lg:gap-16">
          <div>
            <Eyebrow index="04">Belts and XP</Eyebrow>
            <h2
              id="progress-heading"
              className="mt-4 text-[32px] leading-[40px] font-bold tracking-[-0.03em] text-foreground sm:text-h1"
            >
              Earn your black belt.
            </h2>
            <p className="mt-4 max-w-xl text-body-lg text-pretty text-foreground-secondary">
              Finishing lessons earns XP and moves you up the belts. The leaderboard is opt-in: you only appear on it if
              you choose to.
            </p>
          </div>
          <div className="grid gap-3">
            <Link
              href="/paths"
              className="group flex items-start gap-4 rounded-lg border bg-card p-5 transition-colors duration-150 ease-out hover:border-border-strong hover:bg-hover"
            >
              <Route aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                <span className="text-body font-semibold text-foreground group-hover:underline">The learning path</span>
                <span className="mt-1 block text-small text-foreground-secondary">
                  The six courses in order, and where you are on them.
                </span>
              </span>
              <ArrowRight aria-hidden className="mt-1 size-4 shrink-0 text-faint group-hover:text-foreground" />
            </Link>
            <Link
              href="/leaderboard"
              className="group flex items-start gap-4 rounded-lg border bg-card p-5 transition-colors duration-150 ease-out hover:border-border-strong hover:bg-hover"
            >
              <Trophy aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
              <span className="min-w-0 flex-1">
                <span className="text-body font-semibold text-foreground group-hover:underline">The leaderboard</span>
                <span className="mt-1 block text-small text-foreground-secondary">
                  Learners ranked by XP, for those who opt in.
                </span>
              </span>
              <ArrowRight aria-hidden className="mt-1 size-4 shrink-0 text-faint group-hover:text-foreground" />
            </Link>
          </div>
        </div>
      </Section>

      <section aria-labelledby="open-heading">
        <div className="mx-auto flex max-w-[1280px] flex-col items-start justify-between gap-6 px-4 py-12 sm:px-6 lg:flex-row lg:items-center">
          <div>
            <h2 id="open-heading" className="text-h3 font-semibold text-foreground">
              Built in the open.
            </h2>
            <p className="mt-2 max-w-xl text-body text-foreground-secondary">
              The code, the lessons and every design decision are on GitHub under Apache 2.0. Issues and lesson
              corrections are welcome.
            </p>
          </div>
          <Button asChild variant="outline" size="lg">
            <a href={site.repoUrl} rel="noreferrer">
              <GithubIcon /> View on GitHub
            </a>
          </Button>
        </div>
      </section>

      <section aria-labelledby="final-heading" className="relative overflow-hidden border-t border-border-subtle">
        <div
          aria-hidden
          className="pointer-events-none absolute inset-x-0 -bottom-40 h-120 bg-[radial-gradient(ellipse_at_center,var(--glow),transparent_65%)]"
        />
        <div className="relative mx-auto max-w-[1280px] px-4 py-20 text-center sm:px-6 sm:py-28">
          <LogoMark className="mx-auto size-12 rounded-lg" />
          <h2
            id="final-heading"
            className="mt-6 text-[32px] leading-[40px] font-bold tracking-[-0.03em] text-balance text-foreground sm:text-h1"
          >
            Your first kubectl command is a few minutes away.
          </h2>
          <p className="mx-auto mt-4 max-w-xl text-body-lg text-pretty text-foreground-secondary">
            No install and no cloud account. Read the lessons straight away; sign in with GitHub when you want a lab of
            your own.
          </p>
          <div className="mx-auto mt-8 flex max-w-sm flex-col gap-3 sm:max-w-none sm:flex-row sm:justify-center">
            <Button asChild size="hero">
              <Link href="/learn">
                Start learning <ArrowRight />
              </Link>
            </Button>
            <Button asChild size="hero" variant="outline">
              <Link href="/learn/glossary">Open the glossary</Link>
            </Button>
          </div>
        </div>
      </section>
    </>
  );
}
