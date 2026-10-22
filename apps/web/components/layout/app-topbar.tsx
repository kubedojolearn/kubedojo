"use client";

import Link from "next/link";
import { Bell, LogIn, Menu, Search, User } from "lucide-react";
import { useState } from "react";
import { Logo } from "@/components/brand/logo";
import { AppNav } from "@/components/layout/app-nav";
import { ThemeToggle } from "@/components/theme/theme-toggle";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Kbd } from "@/components/ui/kbd";
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { UnavailableButton } from "@/components/feedback/unavailable-button";

/** Top bar of the signed-in app. Search and notifications are visibly disabled until they exist. */
export function AppTopbar() {
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

      <UnavailableButton
        variant="outline"
        reason="Search arrives with the course catalog."
        className="ml-auto hidden w-64 justify-start gap-2 text-muted-foreground sm:inline-flex lg:ml-0"
      >
        <Search />
        <span>Search lessons…</span>
        <Kbd className="ml-auto">⌘K</Kbd>
      </UnavailableButton>

      <div className="ml-auto flex items-center gap-1">
        <UnavailableButton
          variant="ghost"
          size="icon"
          reason="Notifications are coming soon."
          aria-label="Notifications"
        >
          <Bell />
        </UnavailableButton>
        <ThemeToggle />
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Account menu">
              <User />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel>Not signed in</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/login">
                <LogIn /> Sign in
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </div>
  );
}
