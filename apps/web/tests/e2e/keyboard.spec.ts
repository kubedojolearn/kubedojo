import { expect, test } from "@playwright/test";

test("skip link is the first tab stop and moves focus to main content", async ({ page }) => {
  await page.goto("/learn");
  await page.keyboard.press("Tab");
  const skip = page.getByRole("link", { name: "Skip to main content" });
  await expect(skip).toBeFocused();
  await expect(skip).toBeVisible();
  await page.keyboard.press("Enter");
  await expect(page.locator("main#main")).toBeFocused();
});

test("focused links show a visible outline", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab"); // skip link
  await page.keyboard.press("Tab"); // logo
  const outline = await page.evaluate(() => {
    const el = document.activeElement as HTMLElement;
    const s = getComputedStyle(el);
    return { style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
  });
  expect(outline.style).not.toBe("none");
  expect(outline.width).toBeGreaterThanOrEqual(2);
});

test("? opens the shortcuts dialog and Escape closes it", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Shift+?");
  const dialog = page.getByRole("dialog", { name: "Keyboard shortcuts" });
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
});

test("g then l navigates to the curriculum", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("g");
  await page.keyboard.press("l");
  await expect(page).toHaveURL(/\/learn$/);
  await expect(page.getByRole("heading", { level: 1, name: "Curriculum" })).toBeVisible();
});

test("shortcuts do not fire while typing", async ({ page }) => {
  await page.goto("/design-system");
  await page.getByLabel("Handle").fill("g?l");
  await expect(page).toHaveURL(/\/design-system$/);
  await expect(page.getByRole("dialog")).toHaveCount(0);
});

test("unavailable controls stay focusable and explain why", async ({ page }) => {
  await page.goto("/login");
  const button = page.getByRole("button", { name: "Continue with GitHub" });
  await expect(button).toHaveAttribute("aria-disabled", "true");
  await expect(button).toHaveAccessibleDescription("Sign-in isn't available yet.");
  await button.focus();
  await expect(button).toBeFocused();
  // Playwright refuses to click aria-disabled elements; activate it the way a keyboard user would.
  await page.keyboard.press("Enter");
  await page.keyboard.press("Space");
  await expect(page).toHaveURL(/\/login$/);
  await expect(button).toBeFocused();
});

test("confirm dialog starts focus on Cancel", async ({ page }) => {
  await page.goto("/design-system");
  await page.getByRole("button", { name: "Reset lab…" }).click();
  const dialog = page.getByRole("alertdialog", { name: "Reset this lab?" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Cancel" })).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
});
