import { cn } from "@/lib/utils";

/** The KubeDojo mark: an isometric cube wearing a belt. Provisional until a final logo is approved. */
export function LogoMark({ className, title }: { className?: string; title?: string }) {
  return (
    <svg
      viewBox="0 0 64 64"
      className={cn("size-8 shrink-0", className)}
      role={title ? "img" : undefined}
      aria-hidden={title ? undefined : true}
      aria-label={title}
    >
      <rect x="1" y="1" width="62" height="62" rx="14" fill="#09111F" stroke="#22314D" strokeWidth="2" />
      <path d="M32 11 L51 21.5 L32 32 L13 21.5 Z" fill="#7AA2F5" />
      <path d="M13 21.5 L32 32 L32 54 L13 43.5 Z" fill="#2453B8" />
      <path d="M51 21.5 L51 43.5 L32 54 L32 32 Z" fill="#326CE5" />
      <path
        d="M13 34.5 L32 45 L51 34.5"
        fill="none"
        stroke="#22D3EE"
        strokeWidth="4.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path d="M32 45 L28.5 53 M32 45 L35.5 52.5" fill="none" stroke="#22D3EE" strokeWidth="3" strokeLinecap="round" />
    </svg>
  );
}

/** Mark plus wordmark. The text is real text, so it is read by screen readers and scales with zoom. */
export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <LogoMark />
      <span className="text-lg font-semibold tracking-tight">
        Kube<span className="text-highlight">Dojo</span>
      </span>
    </span>
  );
}
