import { expect, test, type Page } from "@playwright/test";
import { signIn, testIdentity } from "./helpers";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL and imported content (make up)");

const LESSON = "/learn/workloads/scaling-deployments";

async function startLab(page: Page): Promise<string> {
  await page.goto(LESSON);
  await page.getByRole("button", { name: "Start lab" }).click();
  await expect(page).toHaveURL(/\/lab\/[0-9a-f-]{36}$/);
  return new URL(page.url()).pathname.split("/").pop()!;
}

function post(page: Page, path: string, headers: Record<string, string> = {}) {
  return page.request.post(`/api/bff/v1/${path}`, { headers: { Origin: new URL(page.url()).origin, ...headers } });
}

test("starting a lab loads the lesson's simulated cluster", async ({ page }) => {
  await signIn(page, testIdentity("lab"));
  const labId = await startLab(page);

  await expect(page.getByRole("heading", { level: 1, name: "Scale a Deployment" })).toBeVisible();
  const header = page.getByRole("main").locator("header").first();
  await expect(header.getByText("Active", { exact: true })).toBeVisible();
  await expect(header.getByText("Version 1")).toBeVisible();
  const cluster = page.getByRole("tabpanel", { name: "Cluster" });
  await expect(cluster.getByRole("heading", { name: /^Deployment/ })).toBeVisible();
  await expect(cluster.getByText("1/1 ready").first()).toBeVisible();
  await expect(cluster.getByText(/^default\/nginx-[a-z0-9]{10}-[a-z0-9]{5}$/)).toBeVisible();
  await expect(cluster.getByText("Running · 1/1 ready")).toBeVisible();

  await page.getByRole("tab", { name: "Events" }).click();
  await expect(page.getByRole("tabpanel", { name: "Events" })).toContainText("Successfully assigned default/nginx-");

  // Starting the lesson again reopens the same lab.
  expect(await startLab(page)).toBe(labId);
});

test("reset and end are confirmed, idempotent and leave the lab read-only", async ({ page }) => {
  await signIn(page, testIdentity("reset"));
  const labId = await startLab(page);

  // The API insists on an idempotency key, and a retried key doesn't reset twice.
  expect((await post(page, `labs/${labId}/reset`)).status()).toBe(400);
  const key = { "Idempotency-Key": `stack-${Date.now()}` };
  const first = await post(page, `labs/${labId}/reset`, key);
  const retry = await post(page, `labs/${labId}/reset`, key);
  expect(first.status()).toBe(200);
  expect((await retry.json()).data.version).toBe((await first.json()).data.version);
  expect((await first.json()).data.version).toBe(2);

  await page.reload();
  await page.getByRole("button", { name: "Reset", exact: true }).click();
  await expect(page.getByRole("alertdialog", { name: "Reset this lab?" })).toBeVisible();
  await page.getByRole("button", { name: "Reset lab" }).click();
  await expect(page.getByText("Lab reset to its starting state")).toBeVisible();
  await expect(page.getByText("Version 3")).toBeVisible();

  await page.getByRole("button", { name: "End lab" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "End lab" }).click();
  await expect(page.getByRole("main").locator("header").first().getByText("Ended", { exact: true })).toBeVisible();
  await expect(page.getByRole("status").filter({ hasText: "This lab is read-only" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Reset", exact: true })).toHaveCount(0);
  expect((await post(page, `labs/${labId}/reset`, { "Idempotency-Key": `stack-after-${Date.now()}` })).status()).toBe(
    409,
  );
});

test("another learner's lab is not found", async ({ browser }) => {
  const owner = await browser.newPage();
  await signIn(owner, testIdentity("owner"));
  const labId = await startLab(owner);

  const intruder = await browser.newPage();
  await signIn(intruder, testIdentity("intruder"));
  // The page streams, so the not-found UI arrives with status 200 (and noindex); the API says 404.
  await intruder.goto(`/lab/${labId}`);
  await expect(intruder.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();
  const api = await intruder.request.get(`/api/bff/v1/labs/${labId}/snapshot`);
  expect(api.status()).toBe(404);
  expect((await post(intruder, `labs/${labId}/finish`, { "Idempotency-Key": "intruder-key-1" })).status()).toBe(404);
});
