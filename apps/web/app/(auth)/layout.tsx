import Link from "next/link";
import { Logo, LogoLockup } from "@/components/brand/logo";
import { site } from "@/lib/site";

/** Sign-in and onboarding: the full logo on a navy panel beside the form (stacked on small screens). */
export default function AuthLayout({ children }: LayoutProps<"/">) {
  return (
    <div className="grid flex-1 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
      <aside className="hidden flex-col justify-between bg-ink p-10 text-[#E8EEF8] lg:flex">
        <Link href="/" aria-label="KubeDojo home" className="self-start rounded-md">
          <span className="font-heading text-lg font-bold">
            Kube<span className="text-[#22E2F9]">Dojo</span>
          </span>
        </Link>
        <div className="grid justify-items-center gap-8">
          <LogoLockup className="w-72" />
          <p className="max-w-sm text-center text-[#97A7C0]">{site.tagline}</p>
        </div>
        <p className="text-xs text-[#97A7C0]">Free and open source. Nothing to install.</p>
      </aside>
      <main
        id="main"
        tabIndex={-1}
        className="flex flex-col items-center justify-center gap-8 px-4 py-16 outline-none sm:px-8"
      >
        <Link href="/" aria-label="KubeDojo home" className="rounded-md lg:hidden">
          <Logo />
        </Link>
        {children}
      </main>
    </div>
  );
}
