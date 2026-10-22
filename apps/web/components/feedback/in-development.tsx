import { Construction } from "lucide-react";
import { cn } from "@/lib/utils";

/** Marks UI that is visible but not functional yet, so nothing implies it works. */
export function InDevelopmentBadge({ className, label = "In development" }: { className?: string; label?: string }) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full bg-sim-badge px-2 py-0.5 text-xs font-medium text-sim-badge-foreground",
        className,
      )}
    >
      <Construction aria-hidden className="size-3" />
      {label}
    </span>
  );
}
