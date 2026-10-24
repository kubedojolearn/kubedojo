import type { Metadata } from "next";
import type { ReactNode } from "react";
import Link from "next/link";
import { Container, PageHeader } from "@/components/layout/page-header";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { site } from "@/lib/site";

export const metadata: Metadata = {
  title: "FAQ",
  description: "Answers to common questions about KubeDojo, the simulated cluster, grading and your data.",
};

const faqs: Array<{ q: string; a: ReactNode }> = [
  {
    q: "Is KubeDojo free?",
    a: "Yes. KubeDojo is free to use and open source under the Apache License 2.0. There are no paid plans.",
  },
  {
    q: "Do I need to install anything?",
    a: "No. Everything runs in your browser: the lessons, the terminal, the YAML editor and the cluster map.",
  },
  {
    q: "Is it a real Kubernetes cluster?",
    a: (
      <>
        No. It&apos;s a simulator that implements a documented subset of Kubernetes {site.kubernetesVersion}. No
        containers run and there is no real networking. That keeps labs instant, free and safe. The simulator tells you
        when something isn&apos;t supported, and lessons explain where real clusters behave differently.{" "}
        <a href={site.compatibilityUrl} className="text-link underline underline-offset-4" rel="noreferrer">
          See exactly what is supported
        </a>
        .
      </>
    ),
  },
  {
    q: "Which kubectl commands can I use?",
    a: "The commands you need for the lessons: get, describe, logs, events, explain, create, apply, delete, run, expose, scale, set image, rollout, label and annotate, among others. Shell features like pipes, redirects and other programs are not available.",
  },
  {
    q: "How is my work graded?",
    a: "KubeDojo checks the state of your simulated cluster, not the exact command you typed, so any correct approach passes. Some lessons also ask you a question about what you found, such as which node a Pod is running on.",
  },
  {
    q: "Will this prepare me for the CKA, CKAD or CKS exams?",
    a: "It covers many fundamentals those exams test, but it isn't an exam course and makes no certification promises. Exams use real clusters and cover topics the simulator doesn't.",
  },
  {
    q: "Can I use it on a phone or tablet?",
    a: "The lessons and site work on any screen size. Labs work on tablets and phones too, with the terminal, editor and cluster map as tabs, but a keyboard and a larger screen make practice much more comfortable.",
  },
  {
    q: "What data do you collect?",
    a: (
      <>
        Only what&apos;s needed to save your progress: your GitHub account ID, username, display name and avatar. We
        don&apos;t request your email. Read the{" "}
        <Link href="/privacy" className="text-link underline underline-offset-4">
          privacy policy
        </Link>
        .
      </>
    ),
  },
  {
    q: "How can I help?",
    a: (
      <>
        Report bugs or simulator differences, suggest lessons, or contribute code and content on{" "}
        <a href={site.repoUrl} className="text-link underline underline-offset-4" rel="noreferrer">
          GitHub
        </a>
        .
      </>
    ),
  },
  {
    q: "Is KubeDojo affiliated with the CNCF or the Kubernetes project?",
    a: "No. KubeDojo is an independent project. Kubernetes is a registered trademark of The Linux Foundation.",
  },
];

export default function FaqPage() {
  return (
    <Container className="max-w-3xl py-12 sm:py-16">
      <PageHeader title="Frequently asked questions" />
      <h2 className="sr-only">Questions</h2>
      <Accordion type="multiple" className="rounded-xl border px-4">
        {faqs.map((item, i) => (
          <AccordionItem key={item.q} value={`item-${i}`}>
            <AccordionTrigger className="text-base">{item.q}</AccordionTrigger>
            <AccordionContent className="text-muted-foreground">{item.a}</AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </Container>
  );
}
