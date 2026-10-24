"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Suspense, type ComponentProps } from "react";
import { cn } from "@/lib/utils";

type NavLinkProps = ComponentProps<typeof Link> & { href: string; exact?: boolean; activeClassName?: string };

function ActiveAwareLink({ href, exact = false, className, activeClassName, ...props }: NavLinkProps) {
  const pathname = usePathname();
  const active = exact || href === "/" ? pathname === href : pathname === href || pathname.startsWith(`${href}/`);
  return (
    <Link
      href={href}
      aria-current={active ? "page" : undefined}
      className={cn(className, active && activeClassName)}
      {...props}
    />
  );
}

/**
 * A link that marks itself as the current page (aria-current) when its route is active.
 * On routes with request-time params (e.g. /lab/[sessionId]) the pathname is unknown while
 * prerendering, so the plain link renders first and the active state streams in (Cache Components).
 */
export function NavLink(props: NavLinkProps) {
  const { exact: _exact, activeClassName: _active, ...linkProps } = props;
  return (
    <Suspense fallback={<Link {...linkProps} />}>
      <ActiveAwareLink {...props} />
    </Suspense>
  );
}
