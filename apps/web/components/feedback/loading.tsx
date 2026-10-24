import { Skeleton } from "@/components/ui/skeleton";

/** Non-blocking placeholder for a list of cards. Hidden from screen readers; pair with LoadingStatus. */
export function CardListSkeleton({ count = 3 }: { count?: number }) {
  return (
    <div aria-hidden className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="space-y-3 rounded-xl border p-5">
          <Skeleton className="h-5 w-2/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
          <Skeleton className="mt-4 h-8 w-24" />
        </div>
      ))}
    </div>
  );
}

/** Announces loading politely to screen readers while skeletons are shown. */
export function LoadingStatus({ label = "Loading" }: { label?: string }) {
  return (
    <p role="status" aria-live="polite" className="sr-only">
      {label}…
    </p>
  );
}
