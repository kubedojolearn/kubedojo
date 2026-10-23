import { NextResponse } from "next/server";
import { auth } from "@/lib/auth/config";

/**
 * Optimistic route protection: signed-out visitors are sent to /login before protected pages
 * render. The Go API is the real enforcement point; this only improves the experience.
 */
const PROTECTED = [/^\/dashboard(\/|$)/, /^\/settings(\/|$)/, /^\/lab(\/|$)/, /^\/admin(\/|$)/, /^\/onboarding(\/|$)/];

export default auth((request) => {
  const { pathname, search } = request.nextUrl;
  if (!request.auth?.subject && PROTECTED.some((re) => re.test(pathname))) {
    const login = new URL("/login", request.nextUrl.origin);
    login.searchParams.set("callbackUrl", `${pathname}${search}`);
    return NextResponse.redirect(login);
  }
  return NextResponse.next();
});

export const config = {
  matcher: ["/dashboard/:path*", "/settings/:path*", "/lab/:path*", "/admin/:path*", "/onboarding/:path*"],
};
