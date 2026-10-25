import Link from "next/link";
import { Suspense } from "react";
import { GithubIcon } from "@/components/brand/github-icon";
import { Logo } from "@/components/brand/logo";
import { MobileNav } from "@/components/layout/mobile-nav";
import { NavLink } from "@/components/layout/nav-link";
import { SiteSearchWithIndex } from "@/components/layout/search-index";
import { AccountPlaceholder, SessionAccountControls, SessionMobileNav } from "@/components/layout/session-slots";
import { SiteSearch } from "@/components/layout/site-search";
import { ThemeToggle } from "@/components/theme/theme-toggle";
import { Button } from "@/components/ui/button";
import { mainNav, site } from "@/lib/site";

const linkClass =
  "rounded-md px-3 py-1.5 text-small whitespace-nowrap text-foreground-secondary transition-colors hover:bg-hover hover:text-foreground";

/** The one header on every page (marketing and signed-in alike), laid out like GitDojo's. */
export function SiteHeader() {
  return (
    <header className="sticky top-0 z-40 border-b border-border-subtle bg-background/90 backdrop-blur-sm">
      <div className="mx-auto flex h-14 max-w-[1280px] items-center justify-between gap-4 px-4 sm:px-6">
        <div className="flex min-w-0 items-center gap-6">
          <Link href="/" aria-label="KubeDojo home" className="shrink-0 rounded-md">
            <Logo />
          </Link>
          <nav aria-label="Main" className="hidden xl:block">
            <ul className="flex items-center gap-1">
              {mainNav.map((item) => (
                <li key={item.href}>
                  {item.href.startsWith("http") ? (
                    <a href={item.href} rel="noreferrer" className={linkClass}>
                      {item.label}
                    </a>
                  ) : (
                    <NavLink href={item.href} className={linkClass} activeClassName="bg-hover text-foreground">
                      {item.label}
                    </NavLink>
                  )}
                </li>
              ))}
            </ul>
          </nav>
        </div>
        <div className="flex items-center gap-2">
          <Suspense fallback={<SiteSearch lessons={null} />}>
            <SiteSearchWithIndex />
          </Suspense>
          <ThemeToggle />
          <Button asChild variant="outline" size="sm" className="max-sm:hidden max-lg:w-8 max-lg:px-0">
            <a href={site.repoUrl} rel="noreferrer" aria-label="Star KubeDojo on GitHub">
              <GithubIcon />
              <span className="hidden lg:inline">Star on GitHub</span>
            </a>
          </Button>
          <Suspense fallback={<AccountPlaceholder />}>
            <SessionAccountControls />
          </Suspense>
          <Suspense fallback={<MobileNav signedIn={false} />}>
            <SessionMobileNav />
          </Suspense>
        </div>
      </div>
    </header>
  );
}
