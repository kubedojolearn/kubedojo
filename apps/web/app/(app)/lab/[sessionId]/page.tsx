import type { Metadata } from "next";
import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { Suspense } from "react";
import { ArrowLeft, Clock, History, Network, SquareTerminal } from "lucide-react";
import { CardListSkeleton, LoadingStatus } from "@/components/feedback/loading";
import { EmptyState } from "@/components/feedback/states";
import { LabControls } from "@/components/lab/lab-controls";
import { ResourceList } from "@/components/lab/resource-list";
import { Terminal } from "@/components/lab/terminal";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { getCurrentUser } from "@/lib/auth/current-user";
import { getCommandHistory, getLab, getRecentEvents, getSnapshot, isLabId, type Lab } from "@/lib/labs";

export const metadata: Metadata = { title: "Lab" };

type Props = PageProps<"/lab/[sessionId]">;

const stateLabel: Record<Lab["state"], string> = {
  active: "Active",
  completed: "Completed",
  abandoned: "Ended",
  expired: "Expired",
};

// Simulated time is shown in UTC: it is the lab's logical clock, not the learner's wall clock.
const simTime = new Intl.DateTimeFormat("en-GB", {
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  timeZone: "UTC",
});

async function LabView({ params }: Props) {
  const { sessionId } = await params;
  if (!isLabId(sessionId)) notFound();
  const me = await getCurrentUser();
  if (!me) redirect(`/login?callbackUrl=/lab/${sessionId}`);

  // Ownership first: someone else's lab is simply not found.
  const lab = await getLab(sessionId);
  if (!lab) notFound();
  const [snapshot, events, history] = await Promise.all([
    getSnapshot(sessionId),
    getRecentEvents(sessionId, 30),
    getCommandHistory(sessionId),
  ]);
  if (!snapshot) notFound();
  const lessonHref = `/learn/${lab.courseSlug}/${lab.lessonSlug}`;
  const active = lab.state === "active";

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center gap-3">
        <div className="min-w-0 space-y-1">
          <Link href={lessonHref} className="inline-flex items-center gap-1 text-sm text-link hover:underline">
            <ArrowLeft aria-hidden className="size-4" /> Back to the lesson
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight text-balance">{lab.title}</h1>
          <p className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <Badge variant={active ? "default" : "secondary"}>{stateLabel[lab.state]}</Badge>
            <span className="inline-flex items-center gap-1">
              <Clock aria-hidden className="size-3.5" />
              Simulated clock {simTime.format(new Date(snapshot.simTime))} UTC
            </span>
            <span>Version {lab.version}</span>
          </p>
        </div>
        {active && (
          <div className="ml-auto">
            <LabControls labId={lab.id} />
          </div>
        )}
      </header>

      {!active && (
        <div role="status" className="rounded-xl border bg-muted/40 p-4 text-sm">
          This lab is read-only because it{" "}
          {lab.state === "expired" ? "expired after 14 days without activity" : "has ended"}.{" "}
          <Link href={lessonHref} className="text-link underline underline-offset-4">
            Start the lesson again
          </Link>{" "}
          for a fresh cluster.
        </div>
      )}

      <Tabs defaultValue="terminal" className="min-w-0 rounded-xl border bg-card p-3">
        <TabsList aria-label="Lab panels" className="h-auto max-w-full flex-wrap">
          <TabsTrigger value="terminal">
            <SquareTerminal aria-hidden /> Terminal
          </TabsTrigger>
          <TabsTrigger value="cluster">
            <Network aria-hidden /> Cluster
          </TabsTrigger>
          <TabsTrigger value="events">
            <History aria-hidden /> Events
          </TabsTrigger>
        </TabsList>
        <TabsContent value="cluster" className="mt-3">
          <p className="mb-4 text-sm text-muted-foreground">
            Every object in your simulated cluster ({snapshot.resources.length}). Nothing here runs on a real cluster.
          </p>
          <ResourceList resources={snapshot.resources} />
        </TabsContent>
        <TabsContent value="events" className="mt-3">
          {events.length === 0 ? (
            <EmptyState title="No events yet" description="Controllers record events here as your cluster changes." />
          ) : (
            <ol className="divide-y rounded-lg border text-sm">
              {events.map((e) => (
                <li key={e.seq} className="grid gap-1 px-3 py-2 sm:grid-cols-[6rem_8rem_1fr]">
                  <span className="font-mono text-xs text-muted-foreground">
                    {simTime.format(new Date(e.occurredAt))}
                  </span>
                  <span className={e.type === "Warning" ? "font-medium text-warning" : "font-medium"}>{e.reason}</span>
                  <span className="min-w-0 break-words">
                    <span className="font-mono text-xs text-muted-foreground">
                      {e.involvedObject.kind.toLowerCase()}/{e.involvedObject.name}
                    </span>{" "}
                    {e.message}
                  </span>
                </li>
              ))}
            </ol>
          )}
        </TabsContent>
        <TabsContent value="terminal" className="mt-3">
          <Terminal
            labId={lab.id}
            active={active}
            initial={history.map((c) => ({
              key: c.requestId,
              input: c.input,
              stdout: c.stdout,
              stderr: c.stderr,
              exitCode: c.exitCode,
            }))}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function LabPage(props: Props) {
  return (
    <Suspense
      fallback={
        <>
          <LoadingStatus label="Loading your lab" />
          <CardListSkeleton count={2} />
        </>
      }
    >
      <LabView {...props} />
    </Suspense>
  );
}
