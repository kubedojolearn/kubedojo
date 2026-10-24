import Link from "next/link";
import { Suspense } from "react";
import { SessionHeaderAction, SessionMobileNav } from "@/components/layout/session-slots";
import { GithubIcon } from "@/components/brand/github-icon";
import { Logo } from "@/components/brand/logo";
import { NavLink } from "@/components/layout/nav-link";
import { MobileNav } from "@/components/layout/mobile-nav";
import { Container } from "@/components/layout/page-header";
import { ThemeToggle } from "@/components/theme/theme-toggle";
import { Button } from "@/components/ui/button";
import { marketingNav, site } from "@/lib/site";

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-40 border-b bg-background/90 backdrop-blur supports-[backdrop-filter]:bg-background/75">
      <Container className="flex h-16 items-center gap-4">
        <Link href="/" aria-label="KubeDojo home" className="rounded-md">
          <Logo />
        </Link>
        <nav aria-label="Main" className="ml-6 hidden md:block">
          <ul className="flex items-center gap-1">
            {marketingNav.map((item) => (
              <li key={item.href}>
                <NavLink
                  href={item.href}
                  className="relative rounded-md px-3 py-2 text-sm text-muted-foreground transition-colors hover:text-foreground"
                  activeClassName="text-foreground font-medium after:absolute after:inset-x-3 after:-bottom-[13px] after:h-0.5 after:bg-highlight"
                >
                  {item.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
        <div className="ml-auto flex items-center gap-1">
          <Button variant="ghost" size="icon" asChild className="hidden sm:inline-flex">
            <a href={site.repoUrl} aria-label="KubeDojo on GitHub" rel="noreferrer">
              <GithubIcon />
            </a>
          </Button>
          <ThemeToggle />
          <Suspense
            fallback={
              <Button asChild size="sm" className="hidden sm:inline-flex">
                <Link href="/login">Sign in</Link>
              </Button>
            }
          >
            <SessionHeaderAction />
          </Suspense>
          <Suspense fallback={<MobileNav signedIn={false} />}>
            <SessionMobileNav />
          </Suspense>
        </div>
      </Container>
    </header>
  );
}
