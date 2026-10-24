import Link from "next/link";
import { AppNav } from "@/components/layout/app-nav";
import { MobileNav } from "@/components/layout/mobile-nav";
import { ReducedMotion } from "@/components/layout/reduced-motion";
import { UserMenu } from "@/components/layout/user-menu";
import { Button } from "@/components/ui/button";
import { getCurrentUser } from "@/lib/auth/current-user";

/*
 * Pieces of the chrome that depend on the signed-in user. Each reads request data, so callers
 * render them inside <Suspense> with the signed-out variant as the fallback (Cache Components).
 */

export async function SessionUserMenu() {
  const user = await getCurrentUser();
  return (
    <UserMenu
      user={user && { handle: user.handle, displayName: user.displayName, avatarUrl: user.avatarUrl ?? null }}
    />
  );
}

/** The Admin group appears only for admins. The API enforces the role regardless. */
export async function SessionAppNav({ onNavigate }: { onNavigate?: () => void }) {
  const user = await getCurrentUser();
  return <AppNav showAdmin={user?.role === "admin"} onNavigate={onNavigate} />;
}

export async function SessionHeaderAction() {
  const user = await getCurrentUser();
  return (
    <Button asChild size="sm" className="hidden sm:inline-flex">
      {user ? <Link href="/dashboard">Dashboard</Link> : <Link href="/login">Sign in</Link>}
    </Button>
  );
}

export async function SessionMobileNav() {
  const user = await getCurrentUser();
  return <MobileNav signedIn={Boolean(user)} />;
}

/** Applies the account's reduced-motion preference (the OS preference is honored in CSS). */
export async function SessionPreferences() {
  const user = await getCurrentUser();
  return <ReducedMotion enabled={Boolean(user?.settings.reducedMotion)} />;
}
