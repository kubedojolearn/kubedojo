import Link from "next/link";
import { Logo } from "@/components/brand/logo";
import { site } from "@/lib/site";

/** Sign-in and onboarding: the form on a card, and a small kubectl session beside it (large screens). */
export default function AuthLayout({ children }: LayoutProps<"/">) {
  return (
    <div className="grid flex-1 lg:grid-cols-2">
      <div className="flex flex-col px-4 py-6 sm:px-8">
        <Link href="/" aria-label="KubeDojo home" className="self-start rounded-md">
          <Logo />
        </Link>
        <main id="main" tabIndex={-1} className="flex flex-1 items-center justify-center py-12 outline-none">
          <div className="w-full max-w-md rounded-xl border bg-card p-6 shadow-2xl shadow-shade/30 sm:p-8">
            {children}
          </div>
        </main>
      </div>
      <aside
        aria-hidden
        className="relative hidden items-center justify-center overflow-hidden border-l border-border-subtle bg-surface lg:flex"
      >
        <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_at_center,var(--glow),transparent_65%)]" />
        <div className="relative grid w-full max-w-lg gap-6 px-8">
          <div className="overflow-hidden rounded-lg border bg-terminal font-mono text-[12.5px] leading-6 whitespace-pre text-terminal-foreground">
            <div className="flex gap-1.5 border-b border-white/10 px-3 py-2.5">
              <span className="size-2.5 rounded-full bg-[#f06a78]/80" />
              <span className="size-2.5 rounded-full bg-[#f5c451]/80" />
              <span className="size-2.5 rounded-full bg-[#4fd18b]/80" />
            </div>
            <div className="p-4">
              <p>
                <span className="text-terminal-prompt">$</span> kubectl scale deployment web --replicas=2
              </p>
              <p className="text-terminal-muted">deployment.apps/web scaled</p>
              <p>
                <span className="text-terminal-prompt">$</span> kubectl get pods
              </p>
              <p className="text-terminal-muted">NAME READY STATUS</p>
              <p className="text-terminal-muted">
                web-7c5ddbdf54-x8f2k 1/1 <span className="text-[#4fd18b]">Running</span>
              </p>
              <p className="text-terminal-muted">
                web-7c5ddbdf54-q4vzn 1/1 <span className="text-[#4fd18b]">Running</span>
              </p>
            </div>
          </div>
          <p className="text-center text-small text-muted-foreground">{site.tagline}</p>
        </div>
      </aside>
    </div>
  );
}
