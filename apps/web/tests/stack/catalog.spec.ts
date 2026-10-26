import { expect, test } from "@playwright/test";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL and imported content (make up)");

test("the curriculum shows every belt and links every published lesson", async ({ page }) => {
  await page.goto("/learn");
  const courses = page.getByRole("main").getByRole("heading", { level: 2 });
  await expect(courses).toHaveText([
    "Kubernetes fundamentals",
    "Pods and workloads",
    "Networking",
    "Configuration",
    "Cluster operations",
    "Troubleshooting",
  ]);
  const workloads = page.getByRole("region", { name: "Pods and workloads" });
  await expect(workloads).toContainText("yellow belt");
  await expect(workloads).toContainText("6 of 6 lessons open");
  await expect(workloads.getByRole("link", { name: "Scale a Deployment" })).toHaveAttribute(
    "href",
    "/learn/workloads/scaling-deployments",
  );
  await expect(workloads.getByRole("link", { name: "Launch your first Pod" })).toHaveAttribute(
    "href",
    "/learn/workloads/launch-a-pod",
  );
  await expect(page.getByRole("region", { name: "Troubleshooting" })).toContainText("5 of 5 lessons open");
});

test("a course lists its published lessons in order", async ({ page }) => {
  await page.goto("/learn");
  await page.getByRole("link", { name: "Pods and workloads" }).click();
  await expect(page).toHaveURL(/\/learn\/workloads$/);
  await expect(page.getByRole("heading", { level: 1, name: "Pods and workloads" })).toBeVisible();
  const lessons = page.getByRole("region", { name: "Lessons" }).getByRole("listitem");
  await expect(lessons).toHaveCount(6);
  await expect(lessons.nth(0)).toContainText("Launch your first Pod");
  await expect(lessons.nth(1)).toContainText("Inspect a running Pod");
  await expect(lessons.nth(4)).toContainText("Scale a Deployment");
  await expect(page.getByText(/being written for this course/)).toHaveCount(0);
});

test("a lesson shows learner content and nothing hidden", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const response = await page.goto("/learn/workloads/scaling-deployments");
  const html = (await response?.text()) ?? "";
  for (const hidden of ["Try: kubectl scale", "assertions", "readyReplicas", "failureMessage", "solution"]) {
    expect(html, `page source leaks ${hidden}`).not.toContain(hidden);
  }

  await expect(page.getByRole("heading", { level: 1, name: "Scale a Deployment" })).toBeVisible();
  await expect(page.getByRole("region", { name: "By the end you'll be able to" })).toContainText(
    "Change the desired replica count",
  );
  await expect(page.getByRole("link", { name: "Create a Deployment", exact: true }).first()).toHaveAttribute(
    "href",
    "/learn/workloads/create-a-deployment",
  );
  await expect(page.getByRole("region", { name: "Lab goals" })).toContainText("The nginx Deployment wants 3 replicas");
  await expect(page.getByRole("region", { name: "Key terms" }).getByText("ReplicaSet", { exact: true })).toBeVisible();
  await expect(page.getByRole("link", { name: /Scaling a Deployment \(Kubernetes docs\)/ })).toHaveAttribute(
    "rel",
    "noreferrer",
  );

  await page.getByRole("button", { name: "Copy code" }).first().click();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe("kubectl get deployment nginx");

  // Keyboard: [ goes to the previous lesson.
  await page.locator("body").press("[");
  await expect(page).toHaveURL(/\/learn\/workloads\/create-a-deployment$/);
  await expect(page.getByRole("heading", { level: 1, name: "Create a Deployment" })).toBeVisible();
  await page.locator("body").press("]");
  await expect(page).toHaveURL(/\/learn\/workloads\/scaling-deployments$/);
});

test("unknown courses and lessons are not found", async ({ page }) => {
  for (const path of ["/learn/workloads/nope", "/learn/nope", "/learn/Bad_Slug"]) {
    await page.goto(path);
    await expect(page.getByRole("heading", { level: 1, name: "Page not found" })).toBeVisible();
  }
});

test("the glossary defines every term", async ({ page }) => {
  await page.goto("/learn/glossary");
  await expect(page.getByRole("heading", { level: 1, name: "Glossary" })).toBeVisible();
  await expect(page.getByRole("term")).toHaveCount(29);
});
