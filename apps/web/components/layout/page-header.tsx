import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function Container({ className, children }: { className?: string; children: ReactNode }) {
  return <div className={cn("mx-auto w-full max-w-[1280px] px-4 sm:px-6", className)}>{children}</div>;
}

/** Page title block. Every page has exactly one h1, rendered here. */
export function PageHeader({
  title,
  description,
  eyebrow,
  actions,
  className,
}: {
  title: string;
  description?: ReactNode;
  eyebrow?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return (
    <header className={cn("flex flex-col gap-4 pb-10 sm:flex-row sm:items-end sm:justify-between", className)}>
      <div>
        {eyebrow && <div className="mb-3">{eyebrow}</div>}
        <h1 className="text-h2 font-semibold tracking-tight text-balance text-foreground sm:text-h1">{title}</h1>
        {description && (
          <div className="mt-3 max-w-2xl text-body-lg text-pretty text-foreground-secondary">{description}</div>
        )}
      </div>
      {actions && <div className="flex shrink-0 gap-2">{actions}</div>}
    </header>
  );
}
