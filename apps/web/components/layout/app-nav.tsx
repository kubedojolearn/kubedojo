"use client";

import {
  BookOpen,
  FileStack,
  LayoutDashboard,
  Route,
  ScrollText,
  Settings,
  Shield,
  Trophy,
  type LucideIcon,
} from "lucide-react";
import { NavLink } from "@/components/layout/nav-link";
import { appNav } from "@/lib/site";

const icons: Record<string, LucideIcon> = {
  LayoutDashboard,
  BookOpen,
  Route,
  Trophy,
  Settings,
  Shield,
  FileStack,
  ScrollText,
};

/** Sidebar navigation for the signed-in app; also rendered inside the mobile drawer. */
export function AppNav({ onNavigate, showAdmin = false }: { onNavigate?: () => void; showAdmin?: boolean }) {
  return (
    <nav aria-label="App" className="grid gap-6">
      {appNav
        .filter((group) => showAdmin || group.title !== "Admin")
        .map((group, gi) => (
          <div key={gi} className="grid gap-1">
            {group.title && (
              <h2 className="px-3 pb-1 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                {group.title}
              </h2>
            )}
            <ul className="grid gap-0.5">
              {group.items.map((item) => {
                const Icon = icons[item.icon] ?? LayoutDashboard;
                return (
                  <li key={item.href}>
                    <NavLink
                      href={item.href}
                      exact={item.href === "/admin"}
                      onClick={onNavigate}
                      className="flex items-center gap-3 rounded-md px-3 py-2 text-sm text-sidebar-foreground/85 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground"
                      activeClassName="bg-sidebar-accent text-sidebar-accent-foreground font-medium"
                    >
                      <Icon aria-hidden className="size-4" />
                      {item.label}
                    </NavLink>
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
    </nav>
  );
}
