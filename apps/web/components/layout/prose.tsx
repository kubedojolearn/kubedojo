import type { ReactNode } from "react";

/** Readable long-form text (privacy, terms). Styles headings, lists and links without a plugin. */
export function Prose({ children }: { children: ReactNode }) {
  return (
    <div className="max-w-3xl space-y-4 text-pretty text-muted-foreground [&_a]:text-link [&_a]:underline [&_a]:underline-offset-4 [&_h2]:mt-10 [&_h2]:text-xl [&_h2]:font-semibold [&_h2]:text-foreground [&_h3]:mt-6 [&_h3]:font-semibold [&_h3]:text-foreground [&_li]:ml-5 [&_ol]:list-decimal [&_ol]:space-y-1 [&_strong]:text-foreground [&_table]:w-full [&_table]:text-sm [&_td]:border-t [&_td]:py-2 [&_td]:pr-4 [&_td]:align-top [&_th]:pb-2 [&_th]:pr-4 [&_th]:text-left [&_th]:text-foreground [&_ul]:list-disc [&_ul]:space-y-1">
      {children}
    </div>
  );
}
