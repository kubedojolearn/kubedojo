import Link from "next/link";
import {
  ArrowRight,
  GitFork,
  LayoutDashboard,
  LogIn,
  Route,
  ShieldCheck,
  Sparkles,
  Trophy,
  UserRoundX,
} from "lucide-react";
import { Suspense } from "react";
import { GithubIcon } from "@/components/brand/github-icon";
import { LogoMark } from "@/components/brand/logo";
import { BeltLadder, BeltLadderStatus } from "@/components/marketing/belt-ladder";
import { CountUp, DrawLine, Reveal, RevealGroup, RevealItem, Spotlight, Tilt } from "@/components/marketing/motion";
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
const changeCommands = ["apply -f", "create", "run", "delete", "expose", "scale", "set image", "rollout undo", "label"];

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
    body: "The check looks at the state you produced, not the command you typed, so any correct approach passes. Pass every goal to complete the lab and earn XP.",
  },
];

function Chip({ children }: { children: string }) {
  return (
    <code className="inline-block rounded-sm border border-border-strong bg-elevated px-2 py-0.5 font-mono text-caption text-foreground-secondary transition-[border-color,color,transform] duration-150 ease-out hover:-translate-y-px hover:border-highlight/60 hover:text-foreground">
      {children}
    </code>
  );
}

function ChipList({ items }: { items: string[] }) {
  return (
    <RevealGroup stagger={0.035} className="flex flex-wrap gap-1.5">
      {items.map((c) => (
        <RevealItem as="span" key={c}>
          <Chip>{c}</Chip>
        </RevealItem>
      ))}
    </RevealGroup>
  );
}

const stats = [
  { value: totalPlannedLessons, label: "hands-on lessons" },
  { value: plannedCourses.length, label: "belts to earn" },
  { value: supportedKinds.length, label: "resource kinds simulated" },
  { value: 0, label: "things to install" },
];

export default function HomePage() {
  return (
    <>
      {/* Without JavaScript, show everything the motion components would have revealed. */}
      <noscript>
        <style>{`[data-reveal]{opacity:1!important;transform:none!important;filter:none!important}[data-pending]{visibility:visible!important;opacity:1!important}`}</style>
      </noscript>

      <section aria-labelledby="hero-heading" className="relative overflow-hidden">
        <div aria-hidden className="kd-aurora" />
        <div aria-hidden className="kd-grid" />
        <div className="relative grid w-full gap-12 px-4 pt-14 pb-16 sm:px-6 sm:pt-24 sm:pb-24 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] lg:items-center lg:gap-16 lg:px-10 xl:px-14 2xl:gap-24 2xl:px-20">
          <RevealGroup onMount stagger={0.09}>
            <RevealItem>
              <Link
                href="#simulator"
                className="group inline-flex items-center gap-2 rounded-full border border-border-strong bg-card/70 py-1 pr-3 pl-1.5 text-caption text-foreground-secondary shadow-sm backdrop-blur transition-colors duration-150 hover:border-highlight/60 hover:text-foreground"
              >
                <span className="inline-flex items-center gap-1 rounded-full bg-primary-soft px-2 py-0.5 font-medium text-highlight">
                  <Sparkles aria-hidden className="size-3" />
                  New
                </span>
                Change the cluster with <code className="font-mono">kubectl apply</code>
                <ArrowRight
                  aria-hidden
                  className="size-3 transition-transform duration-150 group-hover:translate-x-0.5"
                />
              </Link>
            </RevealItem>
            <RevealItem>
              <h1
                id="hero-heading"
                className="mt-6 text-h1 font-bold tracking-[-0.03em] text-balance text-foreground sm:text-display xl:text-[64px] xl:leading-[72px] 2xl:text-[72px] 2xl:leading-[80px]"
              >
                Master Kubernetes, <span className="kd-gradient-text">one command at a time.</span>
              </h1>
            </RevealItem>
            <RevealItem>
              <p className="mt-5 max-w-2xl text-body-lg text-pretty text-foreground-secondary 2xl:text-[19px] 2xl:leading-[30px]">
                Short lessons, then your own simulated cluster to practise on. Type real kubectl, read real output, and
                work out what&apos;s wrong the way you would at work, without a cloud bill or a broken laptop.
              </p>
            </RevealItem>
            <RevealItem className="mt-8 flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <Button
                asChild
                size="hero"
                className="kd-shine shadow-lg shadow-primary/25 transition-[background-color,box-shadow,transform] hover:-translate-y-px hover:shadow-xl hover:shadow-primary/30"
              >
                <Link href="/learn">
                  Start learning{" "}
                  <ArrowRight className="transition-transform duration-150 group-hover/button:translate-x-0.5" />
                </Link>
              </Button>
              <Button asChild size="hero" variant="outline">
                <Link href="/login">
                  <LogIn /> Sign in
                </Link>
              </Button>
            </RevealItem>
            <RevealItem>
              <ul className="mt-8 flex flex-wrap gap-x-5 gap-y-2 text-small text-muted-foreground">
                {trust.map(({ icon: Icon, label }) => (
                  <li key={label} className="flex items-center gap-1.5">
                    <Icon aria-hidden className="size-4 text-faint" />
                    {label}
                  </li>
                ))}
              </ul>
            </RevealItem>
          </RevealGroup>
          <Reveal onMount delay={0.25} className="min-w-0">
            <Tilt>
              <TerminalPreview />
            </Tilt>
          </Reveal>
        </div>

        <div className="relative w-full px-4 pb-14 sm:px-6 sm:pb-20 lg:px-10 xl:px-14 2xl:px-20">
          <RevealGroup
            as="dl"
            stagger={0.07}
            className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border bg-border-subtle lg:grid-cols-4"
          >
            {stats.map((s) => (
              <RevealItem key={s.label} className="flex flex-col-reverse gap-1 bg-card/80 px-5 py-4 backdrop-blur">
                <dt className="text-small text-muted-foreground">{s.label}</dt>
                <dd className="text-h3 font-semibold tracking-[-0.02em] text-foreground">
                  <CountUp value={s.value} />
                </dd>
              </RevealItem>
            ))}
          </RevealGroup>
        </div>
      </section>

      <Section id="how-it-works" labelledBy="lesson-heading">
        <Reveal className="grid gap-x-16 gap-y-6 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:items-end">
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
        </Reveal>
        {/* The steps sit on a line, like a rollout moving through its revisions. */}
        <RevealGroup
          as="ol"
          stagger={0.15}
          delay={0.2}
          className="relative mt-12 grid gap-10 sm:mt-16 lg:grid-cols-3 lg:gap-8"
        >
          <DrawLine className="absolute top-2 bottom-2 left-[7px] w-px bg-gradient-to-b from-highlight/70 via-border-strong to-border lg:top-[7px] lg:right-0 lg:bottom-auto lg:left-2 lg:h-px lg:w-auto lg:bg-gradient-to-r" />
          {steps.map(({ verb, title, body }, i) => (
            <RevealItem as="li" key={verb} className="relative pl-9 lg:pt-10 lg:pl-0">
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
            </RevealItem>
          ))}
        </RevealGroup>
        <p className="mt-12 flex items-start gap-2 text-small text-muted-foreground sm:items-center">
          <LayoutDashboard aria-hidden className="mt-0.5 size-4 shrink-0 sm:mt-0" />
          <span>
            Your{" "}
            <Link href="/dashboard" className="text-foreground-secondary underline hover:text-foreground">
              dashboard
            </Link>{" "}
            shows where to continue. Sign in with Google, GitHub or email to save progress and run labs.
          </span>
        </p>
      </Section>

      <Section id="curriculum" labelledBy="curriculum-heading" band>
        <div className="grid gap-10 lg:grid-cols-[minmax(0,4fr)_minmax(0,8fr)] lg:gap-16">
          <Reveal className="lg:sticky lg:top-24 lg:self-start">
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
          </Reveal>
          <Reveal delay={0.1}>
            <Suspense fallback={<BeltLadder courses={plannedCourses} />}>
              <BeltLadderStatus />
            </Suspense>
          </Reveal>
        </div>
      </Section>

      <Section id="simulator" labelledBy="sim-heading">
        <div className="grid gap-12 lg:grid-cols-[minmax(0,5fr)_minmax(0,7fr)] lg:gap-16">
          <Reveal>
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
          </Reveal>
          <dl className="divide-y divide-border-subtle border-y border-border-subtle">
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">Resources</dt>
              <dd className="mt-3">
                <ChipList items={supportedKinds} />
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">
                With the controllers that connect them: rollouts, scheduling, image pulls, crash loops, endpoints and
                garbage collection.
              </dd>
            </div>
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">kubectl: look around</dt>
              <dd className="mt-3">
                <ChipList items={readCommands} />
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">
                Output matches kubectl&apos;s tables, YAML, JSON and error messages.
              </dd>
            </div>
            <div className="py-5">
              <dt className="text-body font-semibold text-foreground">kubectl: change things</dt>
              <dd className="mt-3">
                <ChipList items={changeCommands} />
              </dd>
              <dd className="mt-3 text-small text-foreground-secondary">
                Imperatively, or by writing YAML in the lab&apos;s editor and applying it. Controllers reconcile before
                the command returns.
              </dd>
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
          <Reveal>
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
          </Reveal>
          <RevealGroup stagger={0.12} className="grid gap-3">
            <RevealItem>
              <Spotlight className="rounded-lg">
                <Link
                  href="/paths"
                  className="group flex items-start gap-4 rounded-lg border bg-card p-5 transition-[border-color,background-color,transform,box-shadow] duration-200 ease-out hover:-translate-y-0.5 hover:border-border-strong hover:shadow-lg hover:shadow-shade/10"
                >
                  <Route aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1">
                    <span className="text-body font-semibold text-foreground group-hover:underline">
                      The learning path
                    </span>
                    <span className="mt-1 block text-small text-foreground-secondary">
                      The six courses in order, and where you are on them.
                    </span>
                  </span>
                  <ArrowRight
                    aria-hidden
                    className="mt-1 size-4 shrink-0 text-faint transition-[color,transform] duration-150 group-hover:translate-x-0.5 group-hover:text-foreground"
                  />
                </Link>
              </Spotlight>
            </RevealItem>
            <RevealItem>
              <Spotlight className="rounded-lg">
                <Link
                  href="/leaderboard"
                  className="group flex items-start gap-4 rounded-lg border bg-card p-5 transition-[border-color,background-color,transform,box-shadow] duration-200 ease-out hover:-translate-y-0.5 hover:border-border-strong hover:shadow-lg hover:shadow-shade/10"
                >
                  <Trophy aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
                  <span className="min-w-0 flex-1">
                    <span className="text-body font-semibold text-foreground group-hover:underline">
                      The leaderboard
                    </span>
                    <span className="mt-1 block text-small text-foreground-secondary">
                      Learners ranked by XP, for those who opt in.
                    </span>
                  </span>
                  <ArrowRight
                    aria-hidden
                    className="mt-1 size-4 shrink-0 text-faint transition-[color,transform] duration-150 group-hover:translate-x-0.5 group-hover:text-foreground"
                  />
                </Link>
              </Spotlight>
            </RevealItem>
          </RevealGroup>
        </div>
      </Section>

      <section aria-labelledby="open-heading">
        <Reveal className="flex w-full flex-col items-start justify-between gap-6 px-4 py-12 sm:px-6 lg:flex-row lg:items-center lg:px-10 xl:px-14 2xl:px-20">
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
        </Reveal>
      </section>

      <section aria-labelledby="final-heading" className="relative overflow-hidden border-t border-border-subtle">
        <div aria-hidden className="kd-aurora rotate-180" />
        <RevealGroup
          stagger={0.1}
          className="relative w-full px-4 py-20 text-center sm:px-6 sm:py-28 lg:px-10 xl:px-14 2xl:px-20"
        >
          <RevealItem>
            <LogoMark className="mx-auto size-12 rounded-lg shadow-lg shadow-primary/30" />
          </RevealItem>
          <RevealItem>
            <h2
              id="final-heading"
              className="mt-6 text-[32px] leading-[40px] font-bold tracking-[-0.03em] text-balance text-foreground sm:text-h1"
            >
              Your first kubectl command is a few minutes away.
            </h2>
          </RevealItem>
          <RevealItem>
            <p className="mx-auto mt-4 max-w-xl text-body-lg text-pretty text-foreground-secondary">
              No install and no cloud account. Read the lessons straight away; sign in when you want a lab of your own.
            </p>
          </RevealItem>
          <RevealItem className="mx-auto mt-8 flex max-w-sm flex-col gap-3 sm:max-w-none sm:flex-row sm:justify-center">
            <Button
              asChild
              size="hero"
              className="kd-shine shadow-lg shadow-primary/25 transition-[background-color,box-shadow,transform] hover:-translate-y-px hover:shadow-xl hover:shadow-primary/30"
            >
              <Link href="/learn">
                Start learning{" "}
                <ArrowRight className="transition-transform duration-150 group-hover/button:translate-x-0.5" />
              </Link>
            </Button>
            <Button asChild size="hero" variant="outline">
              <Link href="/learn/glossary">Open the glossary</Link>
            </Button>
          </RevealItem>
        </RevealGroup>
      </section>
    </>
  );
}
