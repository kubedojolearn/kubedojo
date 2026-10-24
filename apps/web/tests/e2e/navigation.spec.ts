import { expect, test } from "@playwright/test";

test("main navigation reaches every marketing page", async ({ page }) => {
  await page.goto("/");
  const nav = page.getByRole("navigation", { name: "Main" });
  for (const [label, heading] of [
    ["Curriculum", "Curriculum"],
    ["About", "About KubeDojo"],
    ["FAQ", "Frequently asked questions"],
    ["Changelog", "Changelog"],
  ] as const) {
    await nav.getByRole("link", { name: label }).click();
    await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
    await expect(nav.getByRole("link", { name: label })).toHaveAttribute("aria-current", "page");
  }
});

test("footer links reach the legal pages", async ({ page }) => {
  await page.goto("/");
  await page.getByRole("navigation", { name: "Legal" }).getByRole("link", { name: "Privacy" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Privacy policy" })).toBeVisible();
  await page.getByRole("navigation", { name: "Legal" }).getByRole("link", { name: "Terms" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Terms of use" })).toBeVisible();
});

test.describe("mobile", () => {
  test.use({ viewport: { width: 375, height: 800 } });

  test("menu drawer opens, navigates and closes", async ({ page }) => {
    await page.goto("/");
    await page.getByRole("button", { name: "Open menu" }).click();
    const drawer = page.getByRole("dialog");
    await expect(drawer).toBeVisible();
    await drawer.getByRole("link", { name: "FAQ" }).click();
    await expect(page).toHaveURL(/\/faq$/);
    await expect(drawer).toBeHidden();
  });

  test("app navigation drawer works", async ({ page }) => {
    await page.goto("/paths");
    await page.getByRole("button", { name: "Open navigation" }).click();
    await page.getByRole("dialog").getByRole("link", { name: "Leaderboard" }).click();
    await expect(page.getByRole("heading", { level: 1, name: "Leaderboard" })).toBeVisible();
  });
});

test("app sidebar marks the current page and hides admin links from signed-out visitors", async ({ page }) => {
  await page.goto("/leaderboard");
  const nav = page.getByRole("navigation", { name: "App" });
  await expect(nav.getByRole("link", { name: "Leaderboard" })).toHaveAttribute("aria-current", "page");
  await expect(nav.getByRole("link", { name: "Dashboard" })).not.toHaveAttribute("aria-current", "page");
  await expect(nav.getByRole("link", { name: "Content" })).toHaveCount(0);
});

for (const path of ["/dashboard", "/settings", "/lab/00000000-0000-4000-8000-000000000000", "/admin/content"]) {
  test(`${path} sends signed-out visitors to sign-in`, async ({ page }) => {
    await page.goto(path);
    await expect(page).toHaveURL(
      new RegExp(`/login\\?callbackUrl=${encodeURIComponent(path).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`),
    );
    await expect(page.getByRole("heading", { level: 1, name: "Sign in to KubeDojo" })).toBeVisible();
  });
}

test("unknown routes show the not-found page with a 404 status", async ({ page }) => {
  const res = await page.goto("/does-not-exist");
  expect(res?.status()).toBe(404);
  await expect(page.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();
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
