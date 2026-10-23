import { expect, type Page } from "@playwright/test";

/** A unique test identity per call, so runs never collide in a persistent database. */
export function testIdentity(prefix = "e2e") {
  const n = Math.floor(Math.random() * 1e12) + 1e12;
  return { githubId: String(n), login: `${prefix}-${n.toString(36)}` };
}

/** Signs in through the test provider (AUTH_TEST_MODE=1), which still provisions via the Go API. */
export async function signIn(page: Page, who: { githubId: string; login: string }, callbackUrl = "/dashboard") {
  await page.goto(`/login?callbackUrl=${encodeURIComponent(callbackUrl)}`);
  const form = page.getByRole("form", { name: "Test sign-in" });
  await form.getByLabel("GitHub ID").fill(who.githubId);
  await form.getByLabel("Login").fill(who.login);
  await form.getByRole("button", { name: "Sign in as test user" }).click();
  await expect(page).not.toHaveURL(/\/login/);
}
