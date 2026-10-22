import type { Metadata } from "next";
import { Lightbulb, ListChecks, Network, RotateCcw, SquareTerminal, FileCode } from "lucide-react";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { UnavailableButton } from "@/components/feedback/unavailable-button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export const metadata: Metadata = { title: "Lab" };

const panels = [
  { value: "terminal", label: "Terminal", icon: SquareTerminal, body: "Run kubectl against your simulated cluster." },
  {
    value: "editor",
    label: "YAML editor",
    icon: FileCode,
    body: "Edit manifests and apply them with kubectl apply -f.",
  },
  {
    value: "map",
    label: "Cluster map",
    icon: Network,
    body: "See Deployments, ReplicaSets, Pods and Services and how they connect, with an equivalent list view.",
  },
];

/** Layout preview of the lab. Each panel describes what it will do; none is interactive yet. */
export default function LabPage() {
  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-semibold tracking-tight">Lab</h1>
        <InDevelopmentBadge label="Layout preview" />
        <div className="ml-auto flex flex-wrap gap-2">
          <UnavailableButton variant="outline" reason="Hints arrive with grading.">
            <Lightbulb /> Hint
          </UnavailableButton>
          <UnavailableButton variant="outline" reason="Resetting arrives with persistent labs.">
            <RotateCcw /> Reset
          </UnavailableButton>
          <UnavailableButton reason="Checking your work arrives with grading.">
            <ListChecks /> Check my work
          </UnavailableButton>
        </div>
      </header>
      <div className="grid min-h-[60svh] gap-4 lg:grid-cols-[minmax(18rem,2fr)_3fr]">
        <section aria-labelledby="lesson-pane-heading" className="min-w-0 space-y-3 rounded-xl border bg-card p-5">
          <h2 id="lesson-pane-heading" className="font-semibold">
            Lesson and goals
          </h2>
          <p className="text-sm text-muted-foreground">
            The lesson text, the goals you need to meet and any questions to answer appear here.
          </p>
          <div aria-hidden className="space-y-2 pt-2">
            <Skeleton className="h-4 w-11/12" />
            <Skeleton className="h-4 w-4/5" />
            <Skeleton className="h-4 w-3/5" />
          </div>
        </section>
        <Tabs defaultValue="terminal" className="min-w-0 rounded-xl border bg-card p-3">
          <TabsList aria-label="Lab panels" className="h-auto max-w-full flex-wrap">
            {panels.map((p) => (
              <TabsTrigger key={p.value} value={p.value}>
                <p.icon aria-hidden /> {p.label}
              </TabsTrigger>
            ))}
          </TabsList>
          {panels.map((p) => (
            <TabsContent key={p.value} value={p.value} className="mt-3">
              <div className="grid min-h-80 place-items-center rounded-lg border border-dashed bg-terminal p-6 text-center text-sm text-terminal-foreground">
                <p className="max-w-sm">{p.body}</p>
              </div>
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </div>
  );
}
