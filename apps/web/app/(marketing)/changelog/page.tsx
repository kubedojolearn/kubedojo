import type { Metadata } from "next";
import { Container, PageHeader } from "@/components/layout/page-header";
import { changelog } from "@/lib/changelog";

export const metadata: Metadata = {
  title: "Changelog",
  description: "What's new in KubeDojo.",
};

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

export default function ChangelogPage() {
  return (
    <Container className="max-w-3xl py-12 sm:py-16">
      <PageHeader title="Changelog" description="What's new in KubeDojo, newest first." />
      <ol className="relative space-y-10 border-l pl-6">
        {changelog.map((entry) => (
          <li key={`${entry.date}-${entry.title}`} className="relative">
            <span aria-hidden className="absolute top-1.5 -left-[29px] size-2.5 rounded-full bg-highlight" />
            <article aria-labelledby={`${entry.date}-${entry.title}`}>
              <time dateTime={entry.date} className="font-mono text-xs text-muted-foreground">
                {dateFormat.format(new Date(`${entry.date}T00:00:00Z`))}
              </time>
              <h2 id={`${entry.date}-${entry.title}`} className="mt-1 text-lg font-semibold">
                {entry.title}
              </h2>
              <ul className="mt-3 list-disc space-y-1.5 pl-5 text-muted-foreground">
                {entry.items.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            </article>
          </li>
        ))}
      </ol>
    </Container>
  );
}
