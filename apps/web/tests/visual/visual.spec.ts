import { expect, test } from "@playwright/test";

// Run with `make test-visual` (inside mcr.microsoft.com/playwright) so fonts and rendering match CI.
// Update baselines with `make test-visual-update` and review the image diffs in the PR.

const pages = [
  { name: "design-system", path: "/design-system", fullPage: true },
  { name: "home", path: "/", fullPage: true },
  { name: "login", path: "/login", fullPage: false },
  { name: "paths", path: "/paths", fullPage: false },
];

for (const scheme of ["dark", "light"] as const) {
  test.describe(scheme, () => {
    test.use({ colorScheme: scheme, viewport: { width: 1280, height: 800 } });
    for (const p of pages) {
      test(p.name, async ({ page }) => {
        await page.goto(p.path);
        await page.evaluate(() => document.fonts.ready);
        await expect(page).toHaveScreenshot(`${p.name}-${scheme}.png`, { fullPage: p.fullPage });
      });
    }
  });
}

test.describe("mobile", () => {
  test.use({ colorScheme: "dark", viewport: { width: 375, height: 800 } });
  test("home", async ({ page }) => {
    await page.goto("/");
    await page.evaluate(() => document.fonts.ready);
    await expect(page).toHaveScreenshot("home-mobile-dark.png");
  });
});
