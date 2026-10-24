import Link from "next/link";
import { Logo } from "@/components/brand/logo";
import { Container } from "@/components/layout/page-header";
import { footerNav, site } from "@/lib/site";

function FooterLink({ href, label }: { href: string; label: string }) {
  const external = href.startsWith("http");
  const className = "text-sm text-muted-foreground transition-colors hover:text-foreground";
  return external ? (
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
    <footer className="mt-16 border-t bg-card/40">
      <Container className="grid gap-10 py-12 md:grid-cols-[2fr_repeat(3,1fr)]">
        <div className="space-y-3">
          <Logo />
          <p className="max-w-xs text-sm text-muted-foreground">{site.tagline}</p>
        </div>
        {footerNav.map((group) => (
          <nav key={group.title} aria-label={group.title}>
            <h2 className="mb-3 text-sm font-semibold">{group.title}</h2>
            <ul className="grid gap-2">
              {group.items.map((item) => (
                <li key={item.href}>
                  <FooterLink {...item} />
                </li>
              ))}
            </ul>
          </nav>
        ))}
      </Container>
      <Container className="flex flex-col gap-2 border-t py-6 text-xs text-muted-foreground sm:flex-row sm:justify-between">
        <p>Open source under the Apache License 2.0.</p>
        <p>
          KubeDojo is an independent project, not affiliated with the CNCF or The Linux Foundation. Kubernetes is a
          registered trademark of The Linux Foundation.
        </p>
      </Container>
    </footer>
  );
}
