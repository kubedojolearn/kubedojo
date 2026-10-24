import { expect, test } from "@playwright/test";
import { signIn, testIdentity } from "./helpers";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL (make up)");

test("opting in puts you on the leaderboard, and opting out takes you off", async ({ page }) => {
  const who = testIdentity("ranked");
  await signIn(page, who, "/settings");
  await page.getByLabel("Appear on the leaderboard").check();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved")).toBeVisible();

  await page.goto("/leaderboard");
  await expect(page.getByText(`You're listed as @${who.login}.`)).toBeVisible();
  await expect(page.getByRole("table")).toContainText(`@${who.login}`);

  await page.goto("/settings");
  await page.getByLabel("Appear on the leaderboard").uncheck();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved")).toBeVisible();
  await page.goto("/leaderboard");
  await expect(page.getByText("You're not listed.")).toBeVisible();
  await expect(page.getByText(`@${who.login}`, { exact: true })).toHaveCount(0);
});

test("the data export downloads your own data", async ({ page }) => {
  const who = testIdentity("exporter");
  await signIn(page, who, "/settings");
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Download my data" }).click();
  const file = await download;
  expect(file.suggestedFilename()).toMatch(new RegExp(`^kubedojo-${who.login}-\\d{4}-\\d{2}-\\d{2}\\.json$`));
  const body = JSON.parse(await (await import("node:fs/promises")).readFile((await file.path())!, "utf8"));
  expect(body.profile.handle).toBe(who.login);
  expect(body.identities[0].providerSubject).toBe(who.githubId);
});

test("deleting the account needs the handle, signs you out and can't be undone by signing in", async ({ page }) => {
  const who = testIdentity("leaver");
  await signIn(page, who, "/settings");
  await page.getByRole("button", { name: "Delete my account…" }).click();
  const dialog = page.getByRole("dialog", { name: "Delete your account?" });
  const confirm = dialog.getByRole("button", { name: "Delete account" });
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel(/Type .* to confirm/).fill("someone-else");
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel(/Type .* to confirm/).fill(who.login);
  await confirm.click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("link", { name: "Sign in" }).first()).toBeVisible();

  // Signing in again with the same GitHub account starts a brand-new account.
  await signIn(page, who, "/dashboard");
  const me = await (await page.request.get("/api/bff/v1/users/me")).json();
  expect(me.data.handle).not.toBe(who.login);
});
