import type { LucideIcon } from "lucide-react";
import { AlertTriangle, Inbox } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

interface StateProps {
  title: string;
  description?: ReactNode;
  icon?: LucideIcon;
  action?: ReactNode;
  className?: string;
  /** Heading level for the title, so states nest correctly in the page outline. */
  headingLevel?: 2 | 3;
}

/** Nothing to show yet. Explains why and, when possible, offers the next step. */
export function EmptyState({
  title,
  description,
  icon: Icon = Inbox,
  action,
  className,
  headingLevel = 2,
}: StateProps) {
  const Heading = `h${headingLevel}` as const;
  return (
    <div className={cn("flex gap-4 rounded-lg border bg-card/60 p-5 sm:p-6", className)}>
      <Icon aria-hidden className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
      <div className="grid gap-1.5">
        <Heading className="text-base font-semibold">{title}</Heading>
        {description && <div className="max-w-prose text-sm text-muted-foreground">{description}</div>}
        {action && <div className="mt-2">{action}</div>}
      </div>
    </div>
  );
}

/** Something went wrong. Announced to assistive technology via role="alert". */
export function ErrorState({
  title,
  description,
  icon: Icon = AlertTriangle,
  action,
  className,
  headingLevel = 2,
}: StateProps) {
  const Heading = `h${headingLevel}` as const;
  return (
    <div
      role="alert"
      className={cn("flex gap-4 rounded-lg border border-danger/40 border-l-4 bg-danger/5 p-5 sm:p-6", className)}
    >
      <Icon aria-hidden className="mt-0.5 size-5 shrink-0 text-danger" />
      <div className="grid gap-1.5">
        <Heading className="text-base font-semibold">{title}</Heading>
        {description && <div className="max-w-prose text-sm text-muted-foreground">{description}</div>}
        {action && <div className="mt-2">{action}</div>}
      </div>
    </div>
  );
}
