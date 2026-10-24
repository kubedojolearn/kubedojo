"use client";

import Link from "next/link";
import { Menu } from "lucide-react";
import { useState, type ReactNode } from "react";
import { Logo } from "@/components/brand/logo";
import { AppNav } from "@/components/layout/app-nav";
import { ThemeToggle } from "@/components/theme/theme-toggle";
import { Button } from "@/components/ui/button";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";

/** Top bar of the signed-in app: navigation drawer (small screens), search, theme and account. */
export function AppTopbar({ userMenu, search }: { userMenu: ReactNode; search: ReactNode }) {
  const [navOpen, setNavOpen] = useState(false);
  return (
    <div className="sticky top-0 z-30 flex h-14 items-center gap-2 border-b bg-background/85 px-4 backdrop-blur lg:px-6">
      <Sheet open={navOpen} onOpenChange={setNavOpen}>
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" className="lg:hidden" aria-label="Open navigation">
            <Menu />
          </Button>
        </SheetTrigger>
        <SheetContent side="left" className="w-72">
          <SheetHeader>
            <SheetTitle>
              <Logo />
            </SheetTitle>
            <SheetDescription className="sr-only">App navigation</SheetDescription>
          </SheetHeader>
          <div className="px-2">
            <AppNav onNavigate={() => setNavOpen(false)} />
          </div>
        </SheetContent>
      </Sheet>
      <Link href="/" aria-label="KubeDojo home" className="rounded-md lg:hidden">
        <Logo className="[&>span:last-child]:hidden sm:[&>span:last-child]:inline" />
      </Link>

      <div className="ml-auto hidden sm:block lg:ml-0">{search}</div>

      <div className="ml-auto flex items-center gap-1">
        <ThemeToggle />
        {userMenu}
      </div>
    </div>
  );
}
