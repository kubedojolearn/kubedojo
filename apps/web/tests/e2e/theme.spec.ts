import { expect, test } from "@playwright/test";

test("follows the OS color scheme by default", async ({ browser }) => {
  for (const scheme of ["dark", "light"] as const) {
    const ctx = await browser.newContext({ colorScheme: scheme });
    const page = await ctx.newPage();
    await page.goto("/");
    await expect(page.locator("html")).toHaveClass(new RegExp(`\\b${scheme}\\b`));
    await ctx.close();
  }
});

test("theme choice persists across reloads", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "dark" });
  await page.goto("/");
  await page.getByTestId("theme-toggle").click();
  await page.getByRole("menuitemradio", { name: "Light" }).click();
  await expect(page.locator("html")).toHaveClass(/\blight\b/);
  await page.reload();
  await expect(page.locator("html")).toHaveClass(/\blight\b/);
});

test("settings theme picker changes the theme", async ({ page }) => {
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto("/settings");
  await page.getByText("Dark", { exact: true }).click();
  await expect(page.locator("html")).toHaveClass(/\bdark\b/);
  await expect(page.getByRole("radio", { name: "Dark" })).toBeChecked();
});

test("honors reduced motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  await page.getByRole("button", { name: "Open menu" }).isVisible();
  const duration = await page.evaluate(() => getComputedStyle(document.body).transitionDuration);
  expect(parseFloat(duration)).toBeLessThan(0.001);
});
