"use client";

import Link from "next/link";
import { Menu } from "lucide-react";
import { useState } from "react";
import { Logo } from "@/components/brand/logo";
import { NavLink } from "@/components/layout/nav-link";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { marketingNav, site } from "@/lib/site";

/** Drawer navigation for small screens. Focus is trapped while open and returns to the trigger. */
export function MobileNav({ signedIn }: { signedIn: boolean }) {
  const [open, setOpen] = useState(false);
  const close = () => setOpen(false);
  return (
    <Sheet open={open} onOpenChange={setOpen}>
      <SheetTrigger asChild>
        <Button variant="ghost" size="icon" className="md:hidden" aria-label="Open menu">
          <Menu />
        </Button>
      </SheetTrigger>
      <SheetContent side="right" className="w-72">
        <SheetHeader>
          <SheetTitle>
            <Logo />
          </SheetTitle>
          <SheetDescription className="sr-only">Site navigation</SheetDescription>
        </SheetHeader>
        <nav aria-label="Mobile" className="px-4">
          <ul className="grid gap-1">
            <li>
              <NavLink
                href="/"
                exact
                onClick={close}
                className="block rounded-md px-3 py-2 text-sm hover:bg-muted"
                activeClassName="bg-muted font-medium"
              >
                Home
              </NavLink>
            </li>
            {marketingNav.map((item) => (
              <li key={item.href}>
                <NavLink
                  href={item.href}
                  onClick={close}
                  className="block rounded-md px-3 py-2 text-sm hover:bg-muted"
                  activeClassName="bg-muted font-medium"
                >
                  {item.label}
                </NavLink>
              </li>
            ))}
            <li>
              <a href={site.repoUrl} rel="noreferrer" className="block rounded-md px-3 py-2 text-sm hover:bg-muted">
                Source code on GitHub
              </a>
            </li>
          </ul>
          <Button asChild className="mt-4 w-full">
            {signedIn ? (
              <Link href="/dashboard" onClick={close}>
                Dashboard
              </Link>
            ) : (
              <Link href="/login" onClick={close}>
                Sign in
              </Link>
            )}
          </Button>
        </nav>
      </SheetContent>
    </Sheet>
  );
}
