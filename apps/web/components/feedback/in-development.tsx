import { cn } from "@/lib/utils";

/** Marks UI that is visible but not functional yet, so nothing implies it works. */
export function InDevelopmentBadge({ className, label = "In development" }: { className?: string; label?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-[3px] bg-sim-badge px-1.5 py-0.5 font-mono text-[11px] font-medium tracking-wide text-sim-badge-foreground uppercase",
        className,
      )}
    >
      {label}
    </span>
  );
}
