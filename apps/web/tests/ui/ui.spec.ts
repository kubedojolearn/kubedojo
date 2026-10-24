import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";
import { signIn, testIdentity } from "../stack/helpers";

/*
 * UI quality gate against the full stack (make up + AUTH_TEST_MODE=1): every page, signed out and
 * signed in, at desktop and phone widths, with real API data. Each page must have one h1, no
 * serious accessibility violations, no horizontal scroll, no broken images, no browser errors
 * and no server errors; every internal link on it must resolve.
 */

test.skip(!process.env.E2E_BASE_URL, "UI checks need E2E_BASE_URL and imported content (make up)");

const LESSON = "/learn/workloads/scaling-deployments";
const publicRoutes = [
  "/",
  "/learn",
  "/learn/workloads",
  LESSON,
  "/learn/glossary",
  "/about",
  "/faq",
  "/changelog",
  "/privacy",
  "/terms",
  "/login",
  "/design-system",
  "/this-page-does-not-exist",
];
const appRoutes = [
  "/dashboard",
  "/paths",
  "/leaderboard",
  "/settings",
  "/admin",
  "/admin/content",
  "/admin/audit",
  LESSON,
];

/** Collects problems the page reports while it is being checked. */
function watch(page: Page): string[] {
  const problems: string[] = [];
  page.on("console", (m) => {
    // The 404 route answers 404 on purpose; the browser logs that as a failed resource.
    if (m.type() === "error" && !/status of 404/.test(m.text())) problems.push(`console error: ${m.text()}`);
  });
  page.on("pageerror", (e) => problems.push(`uncaught: ${e.message}`));
  page.on("response", (r) => {
    if (r.status() >= 500) problems.push(`${r.status()} from ${r.url()}`);
  });
  return problems;
}

async function checkPage(page: Page, route: string, links: Set<string>) {
  const problems = watch(page);
  await page.goto(route);
  await page.waitForLoadState("networkidle");

  await expect(page.locator("h1"), `${route}: exactly one h1`).toHaveCount(1);

  const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"]).analyze();
  const blocking = axe.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  expect(
    blocking.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(" ")).join(", ")}`),
    `${route}: accessibility`,
  ).toEqual([]);

  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${route}: horizontal scroll`).toBeLessThanOrEqual(0);

  const broken = await page.evaluate(() =>
    Array.from(document.images)
      .filter((img) => img.complete && img.naturalWidth === 0)
      .map((img) => img.currentSrc || img.src),
  );
  expect(broken, `${route}: broken images`).toEqual([]);

  for (const href of await page.locator("a[href^='/']").evaluateAll((as) => as.map((a) => a.getAttribute("href")!))) {
    links.add(href.split("#")[0]!);
  }
  expect(problems, `${route}: browser and server errors`).toEqual([]);
}

async function checkLinks(page: Page, links: Set<string>) {
  const failures: string[] = [];
  for (const href of links) {
    if (href === "" || href.startsWith("/api/")) continue;
    const res = await page.request.get(href, { maxRedirects: 5 });
    // The deliberate 404 example and pages that only exist after sign-in are allowed to 404/redirect.
    if (res.status() >= 400 && !href.includes("this-page-does-not-exist")) failures.push(`${res.status()} ${href}`);
  }
  expect(failures, "internal links").toEqual([]);
}

for (const [label, viewport] of [
  ["desktop", { width: 1440, height: 900 }],
  ["phone", { width: 390, height: 844 }],
] as const) {
  test.describe(label, () => {
    test.use({ viewport });

    test("signed-out pages", async ({ page }) => {
      test.setTimeout(120_000);
      const links = new Set<string>();
      for (const route of publicRoutes) await checkPage(page, route, links);
      await checkLinks(page, links);
    });

    test("signed-in pages", async ({ page }) => {
      test.setTimeout(150_000);
      const who = testIdentity("ui");
      await signIn(page, who);
      const links = new Set<string>();
      for (const route of [...appRoutes, `/profile/${who.login}`]) await checkPage(page, route, links);

      // Search finds published lessons from the real catalog.
      const search = page.getByRole("dialog", { name: "Search KubeDojo" });
      await expect(async () => {
        if (!(await search.isVisible())) await page.keyboard.press("ControlOrMeta+k");
        await expect(search).toBeVisible({ timeout: 1_000 });
      }).toPass();
      await search.getByRole("combobox").fill("scale");
      await expect(search.getByRole("option", { name: /Scale a Deployment/ })).toBeVisible();
      await page.keyboard.press("Escape");

      // A real lab: start it from the lesson, run a command, then check the lab page itself.
      await page.goto(LESSON);
      await page.getByRole("button", { name: "Start lab" }).click();
      await expect(page).toHaveURL(/\/lab\/[0-9a-f-]{36}$/);
      const terminal = page.getByRole("textbox", { name: "Command" });
      await terminal.fill("kubectl get deployments");
      await terminal.press("Enter");
      await expect(page.getByRole("log", { name: "Terminal output" })).toContainText("READY");
      await checkPage(page, new URL(page.url()).pathname, links);
      await checkLinks(page, links);
    });
  });
}
