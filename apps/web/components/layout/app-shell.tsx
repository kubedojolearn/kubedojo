import Link from "next/link";
import { Suspense, type ReactNode } from "react";
import { Logo } from "@/components/brand/logo";
import { AppNav } from "@/components/layout/app-nav";
import { AppTopbar } from "@/components/layout/app-topbar";
import { OfflineBanner } from "@/components/layout/offline-banner";
import { SessionAppNav, SessionPreferences, SessionUserMenu } from "@/components/layout/session-slots";
import { UserMenu } from "@/components/layout/user-menu";
import { InDevelopmentBadge } from "@/components/feedback/in-development";

/** Layout for signed-in pages: sidebar on large screens, drawer on small ones. */
export function AppShell({ children }: { children: ReactNode }) {
  return (
    <div className="flex min-h-svh">
      <aside className="sticky top-0 hidden h-svh w-64 shrink-0 flex-col gap-6 border-r bg-sidebar p-4 lg:flex">
        <Link href="/" aria-label="KubeDojo home" className="self-start rounded-md px-2 py-1">
          <Logo />
        </Link>
        <Suspense fallback={<AppNav />}>
          <SessionAppNav />
        </Suspense>
      </aside>
      <div className="flex min-w-0 flex-1 flex-col">
        <AppTopbar
          userMenu={
            <Suspense fallback={<UserMenu user={null} />}>
              <SessionUserMenu />
            </Suspense>
          }
        />
        <OfflineBanner />
        <Suspense fallback={null}>
          <SessionPreferences />
        </Suspense>
        <div
          role="note"
          className="flex flex-wrap items-center gap-2 border-b bg-sim-badge/40 px-4 py-2 text-sm lg:px-6"
        >
          <InDevelopmentBadge label="Preview" />
          <span>
            The signed-in app is under construction. Screens show their layout and empty states; nothing is connected to
            an account yet.
          </span>
        </div>
        <main id="main" tabIndex={-1} className="flex-1 px-4 py-8 outline-none lg:px-8">
          {children}
        </main>
      </div>
    </div>
  );
}
