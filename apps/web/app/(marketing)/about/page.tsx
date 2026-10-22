import type { Metadata } from "next";
import { Container, PageHeader } from "@/components/layout/page-header";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "About",
  description: "Why KubeDojo exists, how it teaches, and the principles behind it.",
};

const principles = [
  {
    title: "Learn by doing",
    body: "Reading about Kubernetes teaches vocabulary. Changing a cluster and seeing what happens teaches Kubernetes. Every lesson ends in hands-on practice.",
  },
  {
    title: "Understand, don't memorize",
    body: "Lessons focus on the ideas that make Kubernetes predictable: desired state, controllers, labels and selectors. Commands follow from those.",
  },
  {
    title: "Honest simulation",
    body: "The cluster is simulated and always labelled as such. Unsupported commands say so; lessons explain where real clusters differ.",
  },
  {
    title: "Fair grading",
    body: "Work is checked by looking at the cluster you produced, so every correct approach passes, not just the one the author had in mind.",
  },
  {
    title: "Accessible to everyone",
    body: "Keyboard and screen-reader support, WCAG 2.2 AA contrast, and a text equivalent for every visual like the cluster map.",
  },
  {
    title: "Open by default",
    body: "Code, lessons, design decisions and the threat model are public under the Apache 2.0 license.",
  },
];

export default function AboutPage() {
  return (
    <Container className="py-12 sm:py-16">
      <PageHeader
        title="About KubeDojo"
        description="An independent, open-source place to learn Kubernetes by practising it."
      />
      <div className="grid gap-12 lg:grid-cols-[1fr_1.2fr]">
        <div className="space-y-4 text-pretty text-muted-foreground">
          <p>
            Most people learning Kubernetes are stuck between two options. Docs and videos explain the words but not the
            feel of a cluster reacting to you. Real clusters give you that feel, but cost setup time and money, fail in
            confusing ways, and never tell you whether you did the exercise right.
          </p>
          <p>
            KubeDojo sits in between. You get a cluster that reacts the way Kubernetes does to the commands you type,
            shows you why through a live map of its objects, and tells you when you&apos;ve reached the goal. There is
            nothing to install and nothing to pay.
          </p>
          <p>
            The project is in early development and is being built in the open. You can follow progress in the{" "}
            <a href={site.repoUrl} className="text-link underline underline-offset-4" rel="noreferrer">
              GitHub repository
            </a>
            .
          </p>
        </div>
        <section aria-labelledby="principles-heading">
          <h2 id="principles-heading" className="text-xl font-semibold">
            Principles
          </h2>
          <dl className="mt-6 grid gap-6 sm:grid-cols-2">
            {principles.map((p) => (
              <div key={p.title}>
                <dt className="font-medium">{p.title}</dt>
                <dd className="mt-1 text-sm text-muted-foreground">{p.body}</dd>
              </div>
            ))}
          </dl>
        </section>
      </div>
      <p className="mt-16 border-t pt-6 text-sm text-muted-foreground">
        KubeDojo is an independent project and is not affiliated with, endorsed by or sponsored by the Cloud Native
        Computing Foundation or The Linux Foundation. Kubernetes is a registered trademark of The Linux Foundation.
      </p>
    </Container>
  );
}
