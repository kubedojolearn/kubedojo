import { expect, test } from "@playwright/test";

test("home page renders", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1, name: /Master Kubernetes/ })).toBeVisible();
});

test("health endpoint reports build info without secrets", async ({ request }) => {
  const res = await request.get("/api/healthz");
  expect(res.status()).toBe(200);
  expect(res.headers()["cache-control"]).toContain("no-store");
  const body = await res.json();
  expect(Object.keys(body).sort()).toEqual(["commit", "status", "version"]);
  expect(body.status).toBe("ok");
});
