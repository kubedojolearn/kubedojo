import "server-only";
import NextAuth, { type NextAuthConfig } from "next-auth";
import Credentials from "next-auth/providers/credentials";
import Asgardeo from "next-auth/providers/asgardeo";
import { asgardeoSubject, provisionProfile, testUserId, type AsgardeoClaims } from "@/lib/auth/identity";
import { provisionAccount } from "@/lib/auth/provision";
import { testAuthEnabled } from "@/lib/server-env";

declare module "next-auth" {
  interface Session {
    /** Immutable identity, "asgardeo:<user id>". Public information; not a credential. */
    subject?: string;
  }
}

/** The encrypted session token carries only the immutable subject. */
type TokenWithSubject = { subject?: string };

/** Test sign-in, available only when AUTH_TEST_MODE=1 outside production (see server-env.ts). */
const TEST_PROVIDER_ID = "test-login";

function providers(): NextAuthConfig["providers"] {
  const list: NextAuthConfig["providers"] = [
    // Asgardeo brokers Google, GitHub and email/password sign-in (ADR-0008). Credentials and
    // issuer come from AUTH_ASGARDEO_ID, AUTH_ASGARDEO_SECRET and AUTH_ASGARDEO_ISSUER.
    Asgardeo({
      authorization: {
        params: {
          // Only the profile: we never request or store email addresses (requirements PR1).
          scope: "openid profile",
          // Always authenticate at Asgardeo, so signing out of KubeDojo on a shared computer
          // isn't undone by Asgardeo's own session on the next click.
          prompt: "login",
        },
      },
      // Drop everything but the user ID, name and picture, so no email reaches the session.
      profile: (p) => ({ id: String(p.sub ?? ""), name: typeof p.name === "string" ? p.name : null, image: null }),
    }),
  ];
  if (testAuthEnabled()) {
    list.push(
      Credentials({
        id: TEST_PROVIDER_ID,
        name: "Test account",
        credentials: { testId: {}, login: {} },
        async authorize(credentials) {
          const testId = String(credentials?.testId ?? "");
          const login = String(credentials?.login ?? "");
          if (!/^[1-9][0-9]{0,19}$/.test(testId) || !/^[A-Za-z0-9-]{1,39}$/.test(login)) return null;
          return { id: await testUserId(testId), name: login };
        },
      }),
    );
  }
  return list;
}

/** Only same-origin relative paths are allowed as post-sign-in destinations (threat T20). */
export function safeCallbackPath(url: string | null | undefined, baseUrl: string): string {
  if (!url) return "/dashboard";
  try {
    const base = new URL(baseUrl);
    const target = new URL(url, base);
    if (target.origin !== base.origin) return "/dashboard";
    const path = `${target.pathname}${target.search}${target.hash}`;
    return path.startsWith("//") ? "/dashboard" : path;
  } catch {
    return "/dashboard";
  }
}

/** The assertion subject for a completed sign-in, or null if the provider gave an unusable ID. */
function subjectFor(account: { provider: string; providerAccountId: string }): string | null {
  if (account.provider !== "asgardeo" && account.provider !== TEST_PROVIDER_ID) return null;
  return asgardeoSubject(account.providerAccountId);
}

export const authConfig: NextAuthConfig = {
  providers: providers(),
  session: { strategy: "jwt", maxAge: 30 * 24 * 60 * 60 },
  pages: { signIn: "/login", error: "/login" },
  trustHost: true,
  callbacks: {
    /** Provision the KubeDojo account before any session exists (ADR-0003). */
    async signIn({ account, profile, user }) {
      if (!account) return false;
      const subject = subjectFor(account);
      // Asgardeo must send its user ID as the subject (docs/operations/auth.md).
      if (!subject) return "/login?error=Configuration";
      const claims: AsgardeoClaims =
        account.provider === TEST_PROVIDER_ID ? { preferred_username: user.name } : (profile ?? {});
      const ok = await provisionAccount(subject, provisionProfile(claims));
      return ok ? true : "/login?error=ProvisioningFailed";
    },
    jwt({ token, account }) {
      if (account) (token as TokenWithSubject).subject = subjectFor(account) ?? undefined;
      return token;
    },
    session({ session, token }) {
      // Expose the identity only; profile and role are always read from the API.
      return { expires: session.expires, subject: (token as TokenWithSubject).subject } as typeof session;
    },
    redirect({ url, baseUrl }) {
      return new URL(safeCallbackPath(url, baseUrl), baseUrl).toString();
    },
  },
};

export const { handlers, auth, signIn, signOut } = NextAuth(authConfig);
export { TEST_PROVIDER_ID };
