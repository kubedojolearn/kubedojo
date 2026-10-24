"use client";

import Link from "next/link";
import { ChevronDown, FileStack, LayoutDashboard, LogOut, ScrollText, Settings, Shield, UserRound } from "lucide-react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { signOutAction } from "@/lib/auth/actions";
import { adminNav } from "@/lib/site";

export interface MenuUser {
  handle: string;
  displayName: string;
  avatarUrl: string | null;
}

const adminIcons = [Shield, FileStack, ScrollText];

function UserAvatar({ user }: { user: MenuUser }) {
  const initials = (user.displayName || user.handle).slice(0, 2).toUpperCase();
  return (
    <Avatar className="size-7">
      {user.avatarUrl && <AvatarImage src={user.avatarUrl} alt="" />}
      <AvatarFallback className="bg-primary text-micro font-semibold text-primary-foreground">
        {initials}
      </AvatarFallback>
    </Avatar>
  );
}

/** The signed-in account menu: avatar and handle in the header, pages and sign-out inside. */
export function UserMenu({ user, isAdmin = false }: { user: MenuUser; isAdmin?: boolean }) {
  return (
    // Non-modal (the WAI-ARIA menu button pattern): the page isn't hidden from assistive tech.
    <DropdownMenu modal={false}>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label={`Account menu for ${user.handle}`}
          className="flex h-9 min-w-9 items-center gap-2 rounded-md px-1 text-small text-foreground-secondary transition-colors outline-none hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/25 data-[state=open]:bg-hover sm:pr-2"
        >
          <UserAvatar user={user} />
          <span className="max-w-32 truncate max-sm:hidden">{user.handle}</span>
          <ChevronDown aria-hidden className="size-4 max-sm:hidden" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <DropdownMenuLabel className="flex items-center gap-3">
          <UserAvatar user={user} />
          <span className="grid min-w-0">
            <span className="truncate text-small font-medium text-foreground">{user.displayName || user.handle}</span>
            <span className="truncate text-caption font-normal text-muted-foreground">@{user.handle}</span>
          </span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link href="/dashboard">
            <LayoutDashboard /> Dashboard
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={`/profile/${user.handle}`}>
            <UserRound /> Profile
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href="/settings">
            <Settings /> Settings
          </Link>
        </DropdownMenuItem>
        {isAdmin && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel className="text-caption font-normal text-muted-foreground">Admin</DropdownMenuLabel>
            {adminNav.map((item, i) => {
              const Icon = adminIcons[i] ?? Shield;
              return (
                <DropdownMenuItem key={item.href} asChild>
                  <Link href={item.href}>
                    <Icon /> {item.label}
                  </Link>
                </DropdownMenuItem>
              );
            })}
          </>
        )}
        <DropdownMenuSeparator />
        <form action={signOutAction}>
          <DropdownMenuItem asChild>
            <button type="submit" className="w-full">
              <LogOut /> Sign out
            </button>
          </DropdownMenuItem>
        </form>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
