import { expect, test } from "@playwright/test";
import { signIn, testIdentity } from "./helpers";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL and imported content (make up)");

test("new learners are onboarded once, and every question is optional", async ({ page }) => {
  const who = testIdentity("onboard");
  await signIn(page, who, "/dashboard", { onboarding: "keep" });
  await expect(page).toHaveURL(/\/onboarding$/);
  await expect(page.getByRole("heading", { level: 1, name: "Welcome to KubeDojo" })).toBeVisible();
  await page.getByLabel(/New to Kubernetes/).check();
  await page.getByLabel("Get better at fixing things").check();
  await page.getByLabel("Your handle").fill(`${who.login}-x`);
  await page.getByRole("button", { name: "Continue" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByRole("button", { name: `Account menu for ${who.login}-x` })).toBeVisible();

  // Onboarding doesn't come back.
  await page.goto("/onboarding");
  await expect(page).toHaveURL(/\/dashboard$/);
});

test("skipping onboarding works too", async ({ page }) => {
  await signIn(page, testIdentity("skipper"), "/dashboard", { onboarding: "keep" });
  await page.getByRole("button", { name: "Skip for now" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
});

test("a new learner's dashboard shows honest empty states", async ({ page }) => {
  await signIn(page, testIdentity("fresh"));
  await expect(page.getByText("Recommended next")).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "How a cluster is put together" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Your stats" })).toContainText("0 days");
  await expect(page.getByRole("region", { name: "Your stats" })).toContainText("0 XP");
  await expect(page.getByText("No activity yet")).toBeVisible();
  await expect(page.getByText("No achievements yet")).toBeVisible();
  await expect(page.getByRole("progressbar", { name: "Pods and workloads: 0% complete" })).toBeVisible();
  await expect(page.getByText("0 of 2 lessons")).toBeVisible();
});

test("opening a lesson updates continue-learning and activity", async ({ page }) => {
  await signIn(page, testIdentity("reader"));
  const opened = page.waitForResponse((r) => r.url().endsWith("/api/bff/v1/users/me/progress/opened"));
  await page.goto("/learn/workloads/scaling-deployments");
  expect((await opened).status()).toBe(204);
  await page.goto("/dashboard");
  await expect(page.getByText("Continue where you left off")).toBeVisible();
  await expect(page.getByRole("heading", { level: 3, name: "Scale a Deployment" })).toBeVisible();
  await expect(page.getByRole("region", { name: "Recent activity" })).toContainText("Opened scaling deployments");
  // Opening is not completing.
  await expect(page.getByText("0 of 2 lessons")).toBeVisible();
});

test("progress can't be written from the browser", async ({ page }) => {
  await signIn(page, testIdentity("cheater"));
  const origin = new URL(page.url()).origin;
  const headers = { Origin: origin, "Content-Type": "application/json" };
  expect(
    (await page.request.patch("/api/bff/v1/users/me/progress", { headers, data: { completedLessons: 30 } })).status(),
  ).toBe(405);
  expect(
    (
      await page.request.post("/api/bff/v1/users/me/progress/opened", {
        headers,
        data: { courseSlug: "workloads", lessonSlug: "launch-a-pod" },
      })
    ).status(),
  ).toBe(404);
  expect(
    (
      await page.request.post("/api/bff/v1/users/me/progress/opened", {
        headers,
        data: { courseSlug: "workloads", lessonSlug: "scaling-deployments", completed: true },
      })
    ).status(),
  ).toBe(400);
  const progress = await (await page.request.get("/api/bff/v1/users/me/progress")).json();
  expect(progress.data.totals.completedLessons).toBe(0);
});

test("profiles are private until their owner opts in", async ({ browser }) => {
  const owner = testIdentity("owner");
  const ownerPage = await browser.newPage();
  await signIn(ownerPage, owner);
  const visitor = await browser.newPage();
  await signIn(visitor, testIdentity("visitor"));

  await visitor.goto(`/profile/${owner.login}`);
  await expect(visitor.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();
  await ownerPage.goto(`/profile/${owner.login}`);
  await expect(ownerPage.getByText("Your profile is private")).toBeVisible();

  await ownerPage.goto("/settings");
  await ownerPage.getByLabel("Public profile").check();
  await ownerPage.getByRole("button", { name: "Save settings" }).click();
  await expect(ownerPage.getByText("Settings saved")).toBeVisible();

  await visitor.goto(`/profile/${owner.login}`);
  await expect(visitor.getByRole("heading", { level: 1, name: owner.login })).toBeVisible();
  await expect(visitor.getByText("0 XP")).toBeVisible();
});

test("reduced motion follows the account setting", async ({ page }) => {
  await signIn(page, testIdentity("calm"), "/settings");
  await page.getByLabel("Reduce motion").check();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved")).toBeVisible();
  await page.goto("/dashboard");
  await expect(page.locator("html")).toHaveClass(/\breduce-motion\b/);
});
