import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

/**
 * One section of a marketing page. `band` sets it on the surface color, edge to edge, so the page
 * alternates between open and banded sections instead of stacking identical blocks.
 */
export function Section({
  id,
  labelledBy,
  band = false,
  children,
  className,
}: {
  id?: string;
  labelledBy: string;
  band?: boolean;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section
      id={id}
      aria-labelledby={labelledBy}
      className={cn("scroll-mt-20", band && "border-y border-border-subtle bg-surface")}
    >
      <div className={cn("mx-auto max-w-[1280px] px-4 py-16 sm:px-6 sm:py-24", className)}>{children}</div>
    </section>
  );
}

/** Section label: a numbered, monospace eyebrow, like a step in a script. */
export function Eyebrow({ index, children }: { index: string; children: ReactNode }) {
  return (
    <p className="flex items-center gap-2 font-mono text-caption text-muted-foreground">
      <span className="text-highlight">{index}</span>
      <span aria-hidden className="h-px w-6 bg-border-strong" />
      {children}
    </p>
  );
}

export function SectionIntro({
  id,
  index,
  eyebrow,
  title,
  children,
  className,
}: {
  id: string;
  index: string;
  eyebrow: string;
  title: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("max-w-2xl", className)}>
      <Eyebrow index={index}>{eyebrow}</Eyebrow>
      <h2
        id={id}
        className="mt-4 text-[26px] leading-[34px] font-[650] tracking-[-0.02em] text-balance text-foreground sm:text-h2"
      >
        {title}
      </h2>
      {children ? <p className="mt-4 text-body-lg text-pretty text-foreground-secondary">{children}</p> : null}
    </div>
  );
}
