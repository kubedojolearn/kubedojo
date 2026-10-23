import "server-only";
import NextAuth, { type NextAuthConfig } from "next-auth";
import Credentials from "next-auth/providers/credentials";
import GitHub from "next-auth/providers/github";
import { provisionAccount } from "@/lib/auth/provision";
import { testAuthEnabled } from "@/lib/server-env";

declare module "next-auth" {
  interface Session {
    /** Immutable identity, "github:<numeric id>". Public information; not a credential. */
    subject?: string;
  }
}

/** The encrypted session token carries only the immutable subject. */
type TokenWithSubject = { subject?: string };

/** Test sign-in, available only when AUTH_TEST_MODE=1 outside production (see server-env.ts). */
const TEST_PROVIDER_ID = "test-login";

function providers(): NextAuthConfig["providers"] {
  const list: NextAuthConfig["providers"] = [
    // Only the public profile: we never request or store email addresses (requirements PR1).
    GitHub({ authorization: { params: { scope: "read:user" } } }),
  ];
  if (testAuthEnabled()) {
    list.push(
      Credentials({
        id: TEST_PROVIDER_ID,
        name: "Test account",
        credentials: { githubId: {}, login: {} },
        authorize(credentials) {
          const githubId = String(credentials?.githubId ?? "");
          const login = String(credentials?.login ?? "");
          if (!/^[1-9][0-9]{0,19}$/.test(githubId) || !/^[A-Za-z0-9-]{1,39}$/.test(login)) return null;
          return { id: githubId, name: login };
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

export const authConfig: NextAuthConfig = {
  providers: providers(),
  session: { strategy: "jwt", maxAge: 30 * 24 * 60 * 60 },
  pages: { signIn: "/login", error: "/login" },
  trustHost: true,
  callbacks: {
    /** Provision the KubeDojo account before any session exists (ADR-0003). */
    async signIn({ account, profile, user }) {
      if (!account) return false;
      const githubId = account.providerAccountId;
      if (!/^[1-9][0-9]{0,19}$/.test(githubId)) return false;
      const gh = profile as { login?: string; name?: string | null; avatar_url?: string } | undefined;
      const login = gh?.login ?? user.name ?? "";
      const ok = await provisionAccount(githubId, {
        login,
        name: gh?.name ?? user.name ?? login,
        avatar_url: gh?.avatar_url ?? "",
      });
      return ok ? true : "/login?error=ProvisioningFailed";
    },
    jwt({ token, account }) {
      if (account) (token as TokenWithSubject).subject = `github:${account.providerAccountId}`;
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
