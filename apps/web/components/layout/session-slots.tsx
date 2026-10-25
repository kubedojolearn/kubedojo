import Link from "next/link";
import { MobileNav } from "@/components/layout/mobile-nav";
import { ReducedMotion } from "@/components/layout/reduced-motion";
import { UserMenu } from "@/components/layout/user-menu";
import { Button } from "@/components/ui/button";
import { getCurrentUser } from "@/lib/auth/current-user";

/*
 * Pieces of the chrome that depend on the signed-in user. Each reads request data, so callers
 * render them inside <Suspense> (Cache Components).
 */

/** Holds the account controls' space while the session loads, so the header doesn't jump. */
export function AccountPlaceholder() {
  return <span aria-hidden className="h-8 w-24 animate-pulse rounded-md bg-card max-sm:w-16" />;
}

export function SignedOutControls() {
  return (
    <div className="flex items-center gap-2">
      <Button asChild variant="ghost" size="sm">
        <Link href="/login">Sign in</Link>
      </Button>
      <Button asChild size="sm" className="max-sm:hidden">
        <Link href="/learn">Start learning</Link>
      </Button>
    </div>
  );
}

/** Sign in / Start learning when signed out; the account menu (with admin pages for admins) when signed in. */
export async function SessionAccountControls() {
  const user = await getCurrentUser();
  if (!user) return <SignedOutControls />;
  return (
    <UserMenu
      user={{ handle: user.handle, displayName: user.displayName, avatarUrl: user.avatarUrl ?? null }}
      isAdmin={user.role === "admin"}
    />
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
