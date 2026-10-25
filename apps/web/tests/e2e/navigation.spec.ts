import { expect, test } from "@playwright/test";

test("the header navigation reaches every main page and marks the current one", async ({ page }) => {
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "Main" });
  for (const [label, heading] of [
    ["Learn", "Curriculum"],
    ["Learning path", "Learning path"],
    ["Leaderboard", "Leaderboard"],
    ["Changelog", "Changelog"],
  ] as const) {
    await nav.getByRole("link", { name: label, exact: true }).click();
    await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    await expect(nav.getByRole("link", { name: label, exact: true })).toHaveAttribute("aria-current", "page");
  }
});

test("footer links reach the project and legal pages", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Project" }).getByRole("link", { name: "About" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "About KubeDojo" })).toBeVisible();
  await page.getByRole("navigation", { name: "Project" }).getByRole("link", { name: "FAQ" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Frequently asked questions" })).toBeVisible();
  await page.getByRole("navigation", { name: "Legal" }).getByRole("link", { name: "Privacy" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Privacy policy" })).toBeVisible();
  await page.getByRole("navigation", { name: "Legal" }).getByRole("link", { name: "Terms" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Terms of use" })).toBeVisible();
});

test.describe("mobile", () => {
  test.use({ viewport: { width: 375, height: 800 } });

  test("the menu opens, navigates and closes", async ({ page }) => {
    await page.goto("/");
    const menu = page.getByRole("navigation", { name: "Mobile" });
    // The button works once the page has hydrated; retry until it does.
    await expect(async () => {
      if (!(await menu.isVisible())) await page.getByRole("button", { name: "Open menu" }).click();
      await expect(menu).toBeVisible({ timeout: 1_000 });
    }).toPass();
    await menu.getByRole("link", { name: "Leaderboard" }).click();
    await expect(page).toHaveURL(/\/leaderboard$/);
    await expect(page.getByRole("heading", { level: 1, name: "Leaderboard" })).toBeVisible();
    await expect(menu).toBeHidden();
  });

  test("Escape closes the menu and returns focus to its button", async ({ page }) => {
    await page.goto("/learn");
    const menu = page.getByRole("navigation", { name: "Mobile" });
    await expect(async () => {
      if (!(await menu.isVisible())) await page.getByRole("button", { name: "Open menu" }).click();
      await expect(menu).toBeVisible({ timeout: 1_000 });
    }).toPass();
    await page.keyboard.press("Escape");
    await expect(menu).toBeHidden();
    await expect(page.getByRole("button", { name: "Open menu" })).toBeFocused();
  });
});

test("signed-in pages share the site header and footer, without admin links for visitors", async ({ page }) => {
  await page.goto("/leaderboard");
  const nav = page.getByRole("navigation", { name: "Main" });
  await expect(nav.getByRole("link", { name: "Leaderboard" })).toHaveAttribute("aria-current", "page");
  await expect(nav.getByRole("link", { name: "Dashboard" })).not.toHaveAttribute("aria-current", "page");
  await expect(page.getByRole("link", { name: "Content", exact: true })).toHaveCount(0);
  await expect(page.getByRole("contentinfo")).toBeVisible();
  await expect(page.getByRole("link", { name: "Sign in", exact: true })).toBeVisible();
});

for (const path of ["/dashboard", "/settings", "/lab/00000000-0000-4000-8000-000000000000", "/admin/content"]) {
  test(`${path} sends signed-out visitors to sign-in`, async ({ page }) => {
    await page.goto(path);
    await expect(page).toHaveURL(
      new RegExp(`/login\\?callbackUrl=${encodeURIComponent(path).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`),
    );
    await expect(page.getByRole("heading", { level: 1, name: "Sign in" })).toBeVisible();
  });
}

test("unknown routes show the not-found page with a 404 status", async ({ page }) => {
  const res = await page.goto("/does-not-exist");
  expect(res?.status()).toBe(404);
  await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
});

test("pages have titles and social metadata", async ({ page }) => {
  await page.goto("/faq");
  await expect(page).toHaveTitle("FAQ · KubeDojo");
  await expect(page.locator('meta[property="og:image"]')).toHaveAttribute("content", /opengraph-image/);
  const og = await page.request.get("/opengraph-image");
  expect(og.headers()["content-type"]).toBe("image/png");
});

test("security headers are set", async ({ request }) => {
  const res = await request.get("/");
  expect(res.headers()["x-content-type-options"]).toBe("nosniff");
  expect(res.headers()["x-frame-options"]).toBe("DENY");
  expect(res.headers()["referrer-policy"]).toBe("strict-origin-when-cross-origin");
  expect(res.headers()["x-powered-by"]).toBeUndefined();
});
