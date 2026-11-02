# ADR-0008: Asgardeo brokers sign-in with Google, GitHub and email

- **Status:** Accepted · **Date:** 2026-10-27
- Amends [ADR-0003](0003-auth-bff.md): the identity provider and the subject format change; sessions, assertions, provisioning and the network shape do not.

## Context

KubeDojo signed people in with GitHub only. Learners asked for Google, and some have neither account and want an email and password. Adding each provider to Auth.js means a client app, callback, profile mapping, error copy and account rules per provider, and email/password would make KubeDojo store password hashes and send verification and reset emails, which we decided against in requirements PR1.

Asgardeo (WSO2's hosted identity service) speaks OpenID Connect, offers Google and GitHub as *connections*, and runs its own email/password login, verification and recovery. Auth.js ships an Asgardeo provider.

## Decision

**One provider.** Auth.js talks only to Asgardeo, as an OIDC "Traditional Web Application" (`AUTH_ASGARDEO_ID`, `AUTH_ASGARDEO_SECRET`, `AUTH_ASGARDEO_ISSUER`). Google and GitHub are Asgardeo connections; email/password is Asgardeo's own login. KubeDojo never talks to Google or GitHub.

**One button per login.** `/login` shows *Continue with Google*, *Continue with GitHub* and *Continue with email*. The first two pass Asgardeo's `fidp` parameter (the connection's name, configurable) so the learner goes straight to Google or GitHub; *email* opens Asgardeo's login page. Every request sends `prompt=login`, so a lingering Asgardeo session on a shared computer does not sign the next person in.

**Subject.** The immutable identity is the Asgardeo user ID, a UUID: `sub = asgardeo:<lowercase uuid>`. The Asgardeo application must use the user ID as its subject attribute, and Google/GitHub sign-ins must be JIT-provisioned as Asgardeo users so they get one. Any other `sub` (a username, an email, a federated provider's own ID) fails sign-in with the *Configuration* error. Go accepts only this format; `oauth_identities.provider = 'asgardeo'`.

**No email.** Scope is `openid profile`. The web app derives the provisioning `login` from `preferred_username`/`username`/`nickname` when it looks like a handle, otherwise from the name, otherwise `learner`; any value containing `@` is ignored. Go also rejects a provisioning `login` containing `@`. Avatars are kept only from `avatars.githubusercontent.com` and `lh3.googleusercontent.com`.

**Account linking stays explicit.** One Asgardeo user is one KubeDojo account. Whether a Google and a GitHub sign-in for the same person become one Asgardeo user is Asgardeo's configuration, and it must not link by unverified email (see Consequences). KubeDojo never matches users by email.

**Existing GitHub accounts.** KubeDojo had no production users. GitHub identities already in a database are kept but can no longer sign in; signing in through Asgardeo creates a new account.

## Consequences

- One client secret instead of one per provider; adding another login (e.g. Microsoft) is console configuration plus a button.
- Asgardeo becomes a processor of sign-in data and holds learners' email addresses and, for email sign-in, passwords. The privacy policy names it.
- Asgardeo is on the sign-in path: if it is down, nobody can sign in (existing 30-day sessions keep working).
- Account takeover risk moves to Asgardeo's linking rules (threat T18). Required settings: email verification on for self-registration, and no automatic association of federated users with local accounts by email.
- Signing out of KubeDojo does not sign the learner out of Asgardeo; `prompt=login` makes that harmless for the next sign-in. Full RP-initiated logout can be added later.
- Test sign-in (`AUTH_TEST_MODE=1`) maps a numeric test ID to a stable UUID, so tests still never need Asgardeo.

## Alternatives considered

- **Add Google to Auth.js next to GitHub:** no new vendor, but no email/password without storing passwords ourselves, and per-provider config and account rules multiply.
- **Auth.js email (magic link) provider:** needs a database adapter, an email service and storing email addresses, contrary to PR1.
- **Self-hosted WSO2 Identity Server or Keycloak:** the same broker model without the vendor, but another stateful service to run, patch and back up.
- **Asgardeo's hosted login page behind a single "Sign in" button:** fewer changes, but an extra page for every sign-in and learners can't see the options before leaving KubeDojo.
