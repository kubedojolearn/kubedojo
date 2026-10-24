import { cn } from "@/lib/utils";

/**
 * The KubeDojo mark (torii gate, three cubes, a tied belt) on its navy tile, from the master
 * artwork in brand/. Regenerate the file with `pnpm --filter @kubedojo/web brand:assets`.
 */
export function LogoMark({ className, title }: { className?: string; title?: string }) {
  return (
    // A plain <img>: the file is already sized for 1–3x displays, so it skips the image optimizer.
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src="/brand/kubedojo-mark.png"
      width={192}
      height={192}
      alt={title ?? ""}
      aria-hidden={title ? undefined : true}
      decoding="async"
      className={cn("size-8 shrink-0 rounded-md", className)}
    />
  );
}

/** Mark plus wordmark. The text is real text, so it is read by screen readers and scales with zoom. */
export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2.5", className)}>
      <LogoMark />
      <span className="font-heading text-lg font-bold tracking-tight">
        Kube<span className="text-highlight">Dojo</span>
      </span>
    </span>
  );
}

/** The full logo artwork (mark above wordmark) for places with room for it. */
export function LogoLockup({ className }: { className?: string }) {
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src="/brand/kubedojo-logo.webp"
      width={640}
      height={640}
      alt="KubeDojo"
      loading="lazy"
      decoding="async"
      className={cn("h-auto w-40 rounded-2xl shadow-2xl ring-1 ring-white/10", className)}
    />
  );
}
