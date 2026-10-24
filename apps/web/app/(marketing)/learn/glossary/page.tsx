import type { Metadata } from "next";
import { ExternalLink } from "lucide-react";
import { Container, PageHeader } from "@/components/layout/page-header";
import { glossary } from "@/lib/glossary.generated";

export const metadata: Metadata = {
  title: "Glossary",
  description: "Plain-language definitions of the Kubernetes terms used in KubeDojo lessons.",
};

export default function GlossaryPage() {
  return (
    <Container className="max-w-3xl py-12 sm:py-16">
      <PageHeader title="Glossary" description="Plain-language definitions of the terms used in lessons." />
      <dl className="divide-y rounded-xl border">
        {glossary.map((t) => (
          <div key={t.id} id={t.id} className="scroll-mt-24 p-4">
            <dt className="font-semibold">{t.term}</dt>
            <dd className="mt-1 text-muted-foreground">
              {t.definition}
              {t.docsUrl && (
                <>
                  {" "}
                  <a
                    href={t.docsUrl}
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 text-link underline underline-offset-4"
                  >
                    Kubernetes docs <ExternalLink aria-hidden className="size-3" />
                  </a>
                </>
              )}
            </dd>
          </div>
        ))}
      </dl>
    </Container>
  );
}
