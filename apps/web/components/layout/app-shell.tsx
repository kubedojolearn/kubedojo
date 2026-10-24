import { Suspense, type ReactNode } from "react";
import { OfflineBanner } from "@/components/layout/offline-banner";
import { SessionPreferences } from "@/components/layout/session-slots";
import { SiteFooter } from "@/components/layout/site-footer";
import { SiteHeader } from "@/components/layout/site-header";

/** Layout for signed-in pages: the same header and footer as the rest of the site. */
export function AppShell({ children }: { children: ReactNode }) {
  return (
    <>
      <SiteHeader />
      <OfflineBanner />
      <Suspense fallback={null}>
        <SessionPreferences />
      </Suspense>
      <main
        id="main"
        tabIndex={-1}
        className="mx-auto w-full max-w-[1280px] flex-1 px-4 py-12 outline-none sm:px-6 sm:py-16"
      >
        {children}
      </main>
      <SiteFooter />
    </>
  );
}
