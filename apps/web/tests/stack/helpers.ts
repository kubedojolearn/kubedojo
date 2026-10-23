import { expect, type Page } from "@playwright/test";

/** A unique test identity per call, so runs never collide in a persistent database. */
export function testIdentity(prefix = "e2e") {
  const n = Math.floor(Math.random() * 1e12) + 1e12;
  return { githubId: String(n), login: `${prefix}-${n.toString(36)}` };
}

/**
 * Signs in through the test provider (AUTH_TEST_MODE=1), which still provisions via the Go API.
 * New accounts are sent to onboarding first; it is skipped unless `onboarding: "keep"`.
 */
export async function signIn(
  page: Page,
  who: { githubId: string; login: string },
  callbackUrl = "/dashboard",
  { onboarding = "skip" }: { onboarding?: "skip" | "keep" } = {},
) {
  await page.goto(`/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
  const form = page.getByRole("form", { name: "Test sign-in" });
  await form.getByLabel("GitHub ID").fill(who.githubId);
  await form.getByLabel("Login").fill(who.login);
  await form.getByRole("button", { name: "Sign in as test user" }).click();
  await expect(page).not.toHaveURL(/\/login/);
  if (onboarding === "skip") {
    // Completes onboarding without answers, then returns to the requested page.
    const res = await page.request.patch("/api/bff/v1/users/me", {
      headers: { Origin: new URL(page.url()).origin, "Content-Type": "application/json" },
      data: { completeOnboarding: true },
    });
    expect(res.ok()).toBe(true);
    // The post-sign-in redirect to onboarding may still be landing; go to the target either way.
    await page.goto(callbackUrl);
    await expect(page).not.toHaveURL(/\/onboarding/);
  }
}
