import type { Metadata } from "next";
import {
  Box,
  Copy,
  FileCog,
  FolderTree,
  KeyRound,
  Layers,
  Network,
  Server,
  Waypoints,
  type LucideIcon,
} from "lucide-react";
import { Logo, LogoMark } from "@/components/brand/logo";
import { ConfirmDemo, ToastDemo } from "@/components/design-system/interactive-demos";
import { InDevelopmentBadge } from "@/components/feedback/in-development";
import { CardListSkeleton } from "@/components/feedback/loading";
import { EmptyState, ErrorState } from "@/components/feedback/states";
import { Container, PageHeader } from "@/components/layout/page-header";
import { SiteHeader } from "@/components/layout/site-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Kbd } from "@/components/ui/kbd";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { contrastRatio, graphNodes, terminal, themes, type ThemeColors } from "@/lib/design-tokens";

export const metadata: Metadata = {
  title: "Design system",
  description: "KubeDojo design tokens and components.",
  robots: { index: false },
};

const swatches: Array<{ bg: keyof ThemeColors; fg: keyof ThemeColors; label: string }> = [
  { bg: "background", fg: "foreground", label: "Background" },
  { bg: "card", fg: "card-foreground", label: "Surface" },
  { bg: "primary", fg: "primary-foreground", label: "Primary" },
  { bg: "secondary", fg: "secondary-foreground", label: "Secondary" },
  { bg: "muted", fg: "muted-foreground", label: "Muted" },
  { bg: "sim-badge", fg: "sim-badge-foreground", label: "Simulation badge" },
];
const textTokens: Array<keyof ThemeColors> = ["link", "highlight", "success", "warning", "danger"];
const nodeIcons: Record<string, LucideIcon> = {
  FolderTree,
  Server,
  Layers,
  Copy,
  Box,
  Network,
  Waypoints,
  FileCog,
  KeyRound,
};

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section aria-labelledby={id} className="space-y-4 border-t py-10">
      <h2 id={id} className="text-xl font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

function Palette({ theme }: { theme: "dark" | "light" }) {
  const t = themes[theme];
  return (
    <div>
      {/* Colors come from the tokens directly, so each panel renders its own theme regardless of the page theme. */}
      <div
        className="rounded-xl border p-4"
        style={{ background: t.background, color: t.foreground, borderColor: t.border }}
      >
        <p className="mb-3 text-sm font-medium capitalize">{theme} theme</p>
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {swatches.map((s) => (
            <div
              key={s.bg}
              className="rounded-lg border p-3"
              style={{ background: t[s.bg], color: t[s.fg], borderColor: t.border }}
            >
              <p className="text-sm font-medium">{s.label}</p>
              <p className="font-mono text-xs">
                {t[s.bg]} / {t[s.fg]} · {contrastRatio(t[s.fg], t[s.bg]).toFixed(2)}:1
              </p>
            </div>
          ))}
        </div>
        <ul className="mt-4 flex flex-wrap gap-4 text-sm">
          {textTokens.map((name) => (
            <li key={name} style={{ color: t[name] }} className="font-medium">
              {name} {t[name]} · {contrastRatio(t[name], t.background).toFixed(2)}:1
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}

export default function DesignSystemPage() {
  return (
    <>
      <SiteHeader />
      <main id="main" tabIndex={-1} className="flex-1 outline-none">
        <Container className="py-12">
          <PageHeader
            title="Design system"
            description="Tokens and components that every KubeDojo screen is built from. Contrast ratios are computed from the tokens and enforced by unit tests."
          />

          <Section id="brand" title="Brand">
            <div className="flex flex-wrap items-center gap-8">
              <Logo />
              <LogoMark className="size-16" title="KubeDojo mark" />
              <LogoMark className="size-6" />
              <InDevelopmentBadge label="Provisional logo" />
            </div>
          </Section>

          <Section id="color" title="Color">
            <div className="grid gap-4 lg:grid-cols-2">
              <Palette theme="dark" />
              <Palette theme="light" />
            </div>
          </Section>

          <Section id="type" title="Typography">
            <div className="space-y-3">
              <p className="text-5xl font-semibold tracking-tight">Display 48</p>
              <p className="text-3xl font-semibold tracking-tight">Heading 30</p>
              <p className="text-xl font-semibold">Title 20</p>
              <p className="text-base">
                Body 16 — Kubernetes keeps your desired state and observed state in agreement.
              </p>
              <p className="text-sm text-muted-foreground">Secondary 14 — supporting text and descriptions.</p>
              <p className="font-mono text-sm">Mono 14 — kubectl get pods -n kube-system</p>
            </div>
          </Section>

          <Section id="buttons" title="Buttons">
            <div className="flex flex-wrap items-center gap-3">
              <Button>Primary</Button>
              <Button variant="secondary">Secondary</Button>
              <Button variant="outline">Outline</Button>
              <Button variant="ghost">Ghost</Button>
              <Button variant="destructive">Destructive</Button>
              <Button variant="link">Link</Button>
              <Button disabled>Disabled</Button>
            </div>
            <div className="flex flex-wrap items-center gap-3">
              <Button size="sm">Small</Button>
              <Button>Default</Button>
              <Button size="lg">Large</Button>
            </div>
          </Section>

          <Section id="badges" title="Badges and status">
            <div className="flex flex-wrap items-center gap-2">
              <Badge>Default</Badge>
              <Badge variant="secondary">Secondary</Badge>
              <Badge variant="outline">Beginner</Badge>
              <Badge variant="destructive">Failed</Badge>
              <InDevelopmentBadge />
              <span className="text-sm font-medium text-success">● Running</span>
              <span className="text-sm font-medium text-warning">◐ Pending</span>
              <span className="text-sm font-medium text-danger">✕ CrashLoopBackOff</span>
            </div>
            <div className="max-w-sm space-y-2">
              <p className="text-sm">Course progress</p>
              <Progress value={40} aria-label="Course progress" />
            </div>
          </Section>

          <Section id="forms" title="Forms">
            <div className="grid max-w-md gap-2">
              <Label htmlFor="ds-handle">Handle</Label>
              <Input id="ds-handle" placeholder="kube-learner" aria-describedby="ds-handle-help" />
              <p id="ds-handle-help" className="text-sm text-muted-foreground">
                Letters, numbers and dashes. Shown on your public profile.
              </p>
            </div>
          </Section>

          <Section id="cards" title="Cards">
            <div className="grid gap-4 sm:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>Scale a Deployment</CardTitle>
                  <CardDescription>Change how many copies of an app run.</CardDescription>
                </CardHeader>
                <CardContent className="text-sm text-muted-foreground">8 minutes · Beginner</CardContent>
              </Card>
              <CardListSkeleton count={1} />
            </div>
          </Section>

          <Section id="feedback" title="Feedback">
            <div className="grid gap-4 lg:grid-cols-2">
              <EmptyState
                title="No lessons started yet"
                description="Start a lesson to see it here."
                headingLevel={3}
              />
              <ErrorState
                title="Couldn't load your progress"
                description="Check your connection and try again."
                headingLevel={3}
              />
            </div>
            <ToastDemo />
            <ConfirmDemo />
          </Section>

          <Section id="terminal" title="Terminal palette">
            <pre
              className="overflow-x-auto rounded-xl p-4 font-mono text-sm"
              style={{ background: terminal.background, color: terminal.foreground }}
            >
              <span style={{ color: terminal.cyan }}>$ </span>
              {"kubectl get pods\nNAME        READY   STATUS             RESTARTS\nweb-7d4f9   1/1     "}
              <span style={{ color: terminal.green }}>Running</span>
              {"            0\napi-5c8b2   0/1     "}
              <span style={{ color: terminal.red }}>CrashLoopBackOff</span>
              {"   4\njob-2xk7p   0/1     "}
              <span style={{ color: terminal.yellow }}>Pending</span>
              {"            0\n"}
              <span style={{ color: terminal.brightBlack }}>ⓘ Simulated output</span>
            </pre>
          </Section>

          <Section id="graph" title="Cluster map nodes">
            <p className="text-sm text-muted-foreground">
              Each kind has its own icon and shape, so color is never the only way to tell them apart.
            </p>
            <ul className="flex flex-wrap gap-3">
              {Object.entries(graphNodes).map(([kind, style]) => {
                const Icon = nodeIcons[style.icon] ?? Box;
                return (
                  <li
                    key={kind}
                    className="flex items-center gap-2 rounded-lg border bg-card px-3 py-2 text-sm"
                    style={{ borderColor: style.color }}
                  >
                    <Icon aria-hidden className="size-4" style={{ color: style.color }} />
                    {kind}
                    <span className="font-mono text-xs text-muted-foreground">{style.shape}</span>
                  </li>
                );
              })}
            </ul>
          </Section>

          <Section id="keyboard" title="Keyboard">
            <p className="text-sm">
              Press <Kbd>?</Kbd> anywhere to list shortcuts. <Kbd>g</Kbd> then <Kbd>l</Kbd> opens the curriculum.
            </p>
          </Section>
        </Container>
      </main>
    </>
  );
}
