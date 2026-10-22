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
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-xl border border-dashed p-8 text-center",
        className,
      )}
    >
      <span className="grid size-10 place-items-center rounded-full bg-muted text-muted-foreground">
        <Icon aria-hidden className="size-5" />
      </span>
      <Heading className="text-base font-semibold">{title}</Heading>
      {description && <div className="max-w-md text-sm text-muted-foreground">{description}</div>}
      {action && <div className="mt-1">{action}</div>}
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
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-xl border border-danger/40 bg-danger/5 p-8 text-center",
        className,
      )}
    >
      <span className="grid size-10 place-items-center rounded-full bg-danger/10 text-danger">
        <Icon aria-hidden className="size-5" />
      </span>
      <Heading className="text-base font-semibold">{title}</Heading>
      {description && <div className="max-w-md text-sm text-muted-foreground">{description}</div>}
      {action && <div className="mt-1">{action}</div>}
    </div>
  );
}
