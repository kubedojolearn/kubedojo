import { cn } from "@/lib/utils";

/** Belt ranks, one per course in curriculum order: course 1 is the white belt, course 6 the black. */
export const beltNames = ["White", "Yellow", "Orange", "Green", "Blue", "Black"] as const;

/** The belt for a course's 1-based position. Positions past the last belt stay black. */
export function beltName(order: number): string {
  return beltNames[Math.min(Math.max(order, 1), beltNames.length) - 1]!;
}

/**
 * A small tied belt in the course's color. Decorative: always shown next to the course title or
 * the belt name, so color is never the only signal.
 */
export function Belt({ order, className }: { order: number; className?: string }) {
  const n = Math.min(Math.max(order, 1), beltNames.length);
  return (
    <svg
      viewBox="0 0 40 16"
      aria-hidden
      className={cn("h-3.5 w-9 shrink-0 text-foreground", className)}
      style={{ color: `var(--belt-${n})` }}
    >
      <g fill="currentColor" stroke="var(--foreground)" strokeOpacity="0.4" strokeWidth="0.8">
        <rect x="0.5" y="3.5" width="39" height="6" rx="1" />
        <path d="M17 9.5 L13 15.5 L16.5 15.5 L19.5 10.5 Z" />
        <path d="M23 9.5 L27 15.5 L23.5 15.5 L20.5 10.5 Z" />
        <rect x="16" y="1.5" width="8" height="10" rx="1.5" />
      </g>
    </svg>
  );
}
