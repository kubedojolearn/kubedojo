import Link from "next/link";
import { GitFork } from "lucide-react";
import { Logo } from "@/components/brand/logo";
import { footerNav, site } from "@/lib/site";

function FooterLink({ href, label }: { href: string; label: string }) {
  const className = "text-muted-foreground hover:text-foreground hover:underline";
  return href.startsWith("http") ? (
    <a href={href} rel="noreferrer" className={className}>
      {label}
    </a>
  ) : (
    <Link href={href} className={className}>
      {label}
    </Link>
  );
}

export function SiteFooter() {
  return (
    <footer className="border-t border-border-subtle">
      <div className="mx-auto grid max-w-[1280px] grid-cols-2 gap-x-6 gap-y-10 px-4 py-12 sm:grid-cols-3 sm:px-6 md:grid-cols-[2fr_repeat(3,1fr)]">
        <div className="col-span-2 sm:col-span-3 md:col-span-1">
          <Logo />
          <p className="mt-3 max-w-xs text-small text-muted-foreground">{site.tagline}</p>
          <p className="mt-3 flex items-center gap-1.5 text-caption text-muted-foreground">
            <GitFork className="size-3.5" aria-hidden /> Open source, Apache 2.0 licensed
          </p>
          <p className="mt-3 max-w-xs text-caption text-muted-foreground">
            An independent project, not affiliated with the CNCF or The Linux Foundation. Kubernetes is a registered
            trademark of The Linux Foundation.
          </p>
        </div>
        {footerNav.map((group) => (
          <nav key={group.title} aria-label={group.title}>
            <h2 className="text-small font-semibold text-foreground">{group.title}</h2>
            <ul className="mt-3 space-y-2 text-small">
              {group.items.map((item) => (
                <li key={item.href}>
                  <FooterLink {...item} />
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </div>
    </footer>
  );
}
