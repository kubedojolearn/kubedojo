import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Suspense } from "react";
import { GithubIcon } from "@/components/brand/github-icon";
import { Container } from "@/components/layout/page-header";
import { BeltLadder, BeltLadderStatus } from "@/components/marketing/belt-ladder";
import { TerminalPreview } from "@/components/marketing/terminal-preview";
import { Button } from "@/components/ui/button";
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

export default function HomePage() {
  return (
    <>
      <section className="border-b">
        <Container className="grid items-center gap-12 py-14 sm:py-20 lg:grid-cols-[1fr_1.1fr] lg:gap-16">
          <div>
            <p className="font-mono text-xs text-muted-foreground">Free · open source · nothing to install</p>
            <h1 className="mt-4 text-[2.6rem] leading-[1.02] font-bold text-balance sm:text-6xl">
              Master Kubernetes, one command at a time.
            </h1>
            <p className="mt-6 max-w-lg text-lg text-pretty text-muted-foreground">
              Short lessons, then your own simulated cluster to practise on. Type real kubectl, read real output, and
              work out what&apos;s wrong the way you would at work, without a cloud bill or a broken laptop.
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <Button asChild size="lg">
                <Link href="/learn">
                  Browse the curriculum <ArrowRight data-icon="inline-end" />
                </Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <Link href="/login">Sign in with GitHub</Link>
              </Button>
            </div>
          </div>
          <TerminalPreview />
        </Container>
      </section>

      <section aria-labelledby="lesson-heading" className="py-16 sm:py-20">
        <Container className="grid gap-10 lg:grid-cols-[16rem_1fr]">
          <div>
            <h2 id="lesson-heading" className="text-2xl font-bold sm:text-3xl">
              How a lesson goes
            </h2>
            <p className="mt-3 text-muted-foreground">Ten to twenty minutes, in this order, every time.</p>
          </div>
          <ol className="grid gap-px overflow-hidden rounded-lg border bg-border md:grid-cols-3">
            <li className="bg-background p-6">
              <p className="font-mono text-xs text-muted-foreground">read</p>
              <h3 className="mt-2 text-lg font-semibold">One idea, explained plainly</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                What a ReplicaSet is for. Why a Service needs a selector. A few paragraphs, a table, a term you can
                hover.
              </p>
            </li>
            <li className="bg-background p-6">
              <p className="font-mono text-xs text-muted-foreground">practise</p>
              <h3 className="mt-2 text-lg font-semibold">Your own cluster, in the browser</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                Each lab starts a fresh simulated cluster set up for the lesson. Poke at it with kubectl; reset it
                whenever you like.
              </p>
            </li>
            <li className="bg-background p-6">
              <p className="font-mono text-xs text-muted-foreground">prove it</p>
              <h3 className="mt-2 text-lg font-semibold">Goals checked against the cluster</h3>
              <p className="mt-2 text-sm text-muted-foreground">
                The check looks at the state you produced, not the command you typed, so any correct approach passes.{" "}
                <span className="text-foreground">Automatic checking ships with the next release.</span>
              </p>
            </li>
          </ol>
        </Container>
      </section>

      <section aria-labelledby="curriculum-heading" className="border-y bg-card/50 py-16 sm:py-20">
        <Container>
          <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h2 id="curriculum-heading" className="text-2xl font-bold sm:text-3xl">
                Six belts, {totalPlannedLessons} lessons
              </h2>
              <p className="mt-3 max-w-2xl text-muted-foreground">
                Start at the white belt with what a cluster is made of. Finish at black, bringing a broken shop back
                online. Lessons open as they&apos;re written and reviewed.
              </p>
            </div>
            <Button asChild variant="outline">
              <Link href="/learn">
                Open lessons <ArrowRight data-icon="inline-end" />
              </Link>
            </Button>
          </div>
          <Suspense fallback={<BeltLadder courses={plannedCourses} />}>
            <BeltLadderStatus />
          </Suspense>
        </Container>
      </section>

      <section aria-labelledby="sim-heading" className="py-16 sm:py-20">
        <Container className="grid gap-10 lg:grid-cols-[16rem_1fr]">
          <div>
            <h2 id="sim-heading" className="text-2xl font-bold sm:text-3xl">
              What the simulator does
            </h2>
            <p className="mt-3 text-muted-foreground">
              A documented subset of Kubernetes {site.kubernetesVersion}. When you step outside it, it says so instead
              of pretending.
            </p>
            <a
              href={site.compatibilityUrl}
              rel="noreferrer"
              className="mt-4 inline-flex items-center gap-1 text-sm text-link underline-offset-4 hover:underline"
            >
              Full compatibility notes <ArrowRight aria-hidden className="size-3.5" />
            </a>
          </div>
          <dl className="grid gap-x-10 gap-y-8 sm:grid-cols-2">
            <div>
              <dt className="text-sm font-semibold">Resources</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {supportedKinds.map((k) => (
                  <code key={k} className="rounded-[3px] border bg-card px-1.5 py-0.5 font-mono text-xs">
                    {k}
                  </code>
                ))}
              </dd>
              <dd className="mt-3 text-sm text-muted-foreground">
                With the controllers that connect them: rollouts, scheduling, image pulls, crash loops, endpoints and
                garbage collection.
              </dd>
            </div>
            <div>
              <dt className="text-sm font-semibold">kubectl, today</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {readCommands.map((c) => (
                  <code key={c} className="rounded-[3px] border bg-card px-1.5 py-0.5 font-mono text-xs">
                    {c}
                  </code>
                ))}
              </dd>
              <dd className="mt-3 text-sm text-muted-foreground">
                Output matches kubectl&apos;s tables, YAML, JSON and error messages.
              </dd>
            </div>
            <div>
              <dt className="text-sm font-semibold">kubectl, next</dt>
              <dd className="mt-3 flex flex-wrap gap-1.5">
                {comingCommands.map((c) => (
                  <code
                    key={c}
                    className="rounded-[3px] border border-dashed px-1.5 py-0.5 font-mono text-xs text-muted-foreground"
                  >
                    {c}
                  </code>
                ))}
              </dd>
              <dd className="mt-3 text-sm text-muted-foreground">Changing the cluster, including YAML apply.</dd>
            </div>
            <div>
              <dt className="text-sm font-semibold">Never</dt>
              <dd className="mt-3 text-sm text-muted-foreground">
                A shell, real containers, or anything you type leaving the simulator. Pipes and{" "}
                <code className="font-mono text-xs">$(…)</code> are refused, not filtered.
              </dd>
            </div>
          </dl>
        </Container>
      </section>

      <section aria-labelledby="open-heading" className="border-t">
        <Container className="flex flex-col gap-6 py-12 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 id="open-heading" className="text-xl font-bold">
              Built in the open
            </h2>
            <p className="mt-2 max-w-xl text-sm text-muted-foreground">
              The code, the lessons and every design decision are on GitHub under Apache 2.0. Issues and lesson
              corrections are welcome.
            </p>
          </div>
          <Button asChild variant="outline">
            <a href={site.repoUrl} rel="noreferrer">
              <GithubIcon /> kubedojolearn/kubedojo
            </a>
          </Button>
        </Container>
      </section>
    </>
  );
}
