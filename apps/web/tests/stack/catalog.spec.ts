import { expect, test } from "@playwright/test";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL and imported content (make up)");

test("the curriculum lists published courses and filters them", async ({ page }) => {
  await page.goto("/learn");
  const courses = page.getByRole("list").filter({ has: page.getByRole("heading", { name: "Pods and workloads" }) });
  await expect(courses.getByRole("heading", { level: 2 })).toHaveText([
    "Kubernetes fundamentals",
    "Pods and workloads",
  ]);
  await expect(page.getByText("2 of 6 lessons available")).toBeVisible();

  await page.getByLabel("Search courses").fill("pods");
  await expect(page.getByRole("status")).toHaveText("1 of 2 courses");
  await page.getByLabel("Search courses").fill("");
  await page.getByLabel("intermediate").check();
  await expect(page.getByText("No courses match")).toBeVisible();
});

test("a course lists its published lessons in order", async ({ page }) => {
  await page.goto("/learn");
  await page.getByRole("link", { name: "Pods and workloads" }).click();
  await expect(page).toHaveURL(/\/learn\/workloads$/);
  await expect(page.getByRole("heading", { level: 1, name: "Pods and workloads" })).toBeVisible();
  const lessons = page.getByRole("region", { name: "Lessons" }).getByRole("listitem");
  await expect(lessons).toHaveCount(2);
  await expect(lessons.nth(0)).toContainText("Inspect a running Pod");
  await expect(lessons.nth(1)).toContainText("Scale a Deployment");
  await expect(page.getByText("4 more lessons are being written for this course.")).toBeVisible();
});

test("a lesson shows learner content and nothing hidden", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"]);
  const response = await page.goto("/learn/workloads/scaling-deployments");
  const html = (await response?.text()) ?? "";
  for (const hidden of ["Try: kubectl scale", "assertions", "readyReplicas", "failureMessage", "solution"]) {
    expect(html, `page source leaks ${hidden}`).not.toContain(hidden);
  }

  await expect(page.getByRole("heading", { level: 1, name: "Scale a Deployment" })).toBeVisible();
  await expect(page.getByRole("region", { name: "You will learn to" })).toContainText(
    "Change the desired replica count",
  );
  await expect(page.getByText("create a deployment (coming soon)")).toBeVisible();
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
  await expect(page).toHaveURL(/\/learn\/workloads\/inspect-a-pod$/);
  await expect(page.getByRole("heading", { level: 1, name: "Inspect a running Pod" })).toBeVisible();
  await page.locator("body").press("]");
  await expect(page).toHaveURL(/\/learn\/workloads\/scaling-deployments$/);
});

test("unpublished and unknown lessons are not found", async ({ page }) => {
  for (const path of [
    "/learn/workloads/launch-a-pod",
    "/learn/networking",
    "/learn/workloads/nope",
    "/learn/Bad_Slug",
  ]) {
    await page.goto(path);
    await expect(page.getByRole("heading", { name: "We couldn't find that page" })).toBeVisible();
  }
});

test("the glossary defines every term", async ({ page }) => {
  await page.goto("/learn/glossary");
  await expect(page.getByRole("heading", { level: 1, name: "Glossary" })).toBeVisible();
  await expect(page.getByRole("term")).toHaveCount(19);
});
