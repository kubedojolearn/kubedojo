import { expect, test } from "@playwright/test";
import { signIn, testIdentity } from "./helpers";

test.skip(!process.env.E2E_BASE_URL, "stack tests need E2E_BASE_URL pointing at `make up` with AUTH_TEST_MODE=1");

test("signed-out visitors are sent to sign-in, then back", async ({ page }) => {
  await page.goto("/settings");
  await expect(page).toHaveURL(/\/login\?callbackUrl=%2Fsettings/);
  const who = testIdentity();
  await page.getByLabel("GitHub ID").fill(who.githubId);
  await page.getByLabel("Login").fill(who.login);
  await page.getByRole("button", { name: "Sign in as test user" }).click();
  await expect(page).toHaveURL(/\/settings$/);
  await expect(page.getByLabel("Handle")).toHaveValue(who.login);
});

test("sign-in provisions one account and repeat sign-in reuses it", async ({ page, context }) => {
  const who = testIdentity();
  await signIn(page, who);
  await expect(page.getByRole("heading", { level: 1, name: "Dashboard" })).toBeVisible();
  await expect(page.getByText(`Welcome back, ${who.login}.`)).toBeVisible();
  await expect(page.getByRole("button", { name: `Account menu for ${who.login}` })).toBeVisible();
  // Learners don't see admin navigation.
  await expect(page.getByRole("navigation", { name: "App" }).getByRole("link", { name: "Content" })).toHaveCount(0);

  await page.getByRole("button", { name: `Account menu for ${who.login}` }).click();
  await page.getByRole("menuitem", { name: "Sign out" }).click();
  await expect(page).toHaveURL(/\/$/);
  await context.clearCookies();

  await signIn(page, { githubId: who.githubId, login: "renamed-on-github" });
  await expect(page.getByRole("button", { name: `Account menu for ${who.login}` })).toBeVisible();
});

test("settings persist and the API's validation errors are shown", async ({ page }) => {
  await signIn(page, testIdentity(), "/settings");
  await page.getByLabel("Display name").fill("Playwright Learner");
  await page.getByLabel("Time zone").selectOption("Asia/Colombo");
  await page.getByLabel("Public profile").check();
  await page.getByRole("radio", { name: "light" }).check();
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("Settings saved")).toBeVisible();
  await expect(page.locator("html")).toHaveClass(/\blight\b/);
  await page.reload();
  await expect(page.getByLabel("Display name")).toHaveValue("Playwright Learner");
  await expect(page.getByLabel("Time zone")).toHaveValue("Asia/Colombo");
  await expect(page.getByLabel("Public profile")).toBeChecked();
  await expect(page.getByRole("radio", { name: "light" })).toBeChecked();

  await page.getByLabel("Handle").fill("admin");
  await page.getByRole("button", { name: "Save settings" }).click();
  await expect(page.getByText("is reserved")).toBeVisible();
  await expect(page.getByLabel("Handle")).toHaveAttribute("aria-invalid", "true");
});

test("handles are unique across accounts", async ({ browser }) => {
  const first = testIdentity("first");
  const a = await browser.newPage();
  await signIn(a, first);
  const b = await browser.newPage();
  await signIn(b, testIdentity("second"), "/settings");
  await b.getByLabel("Handle").fill(first.login.toUpperCase());
  await b.getByRole("button", { name: "Save settings" }).click();
  await expect(b.getByText("is already taken")).toBeVisible();
});

test("post-sign-in redirects stay on this site", async ({ page }) => {
  const who = testIdentity();
  await page.goto("/login?callbackUrl=https%3A%2F%2Fevil.example%2Fphish");
  await page.getByLabel("GitHub ID").fill(who.githubId);
  await page.getByLabel("Login").fill(who.login);
  await page.getByRole("button", { name: "Sign in as test user" }).click();
  await expect(page).toHaveURL(/localhost:8000\/dashboard$/);
});

test("the BFF requires a session and rejects cross-site mutations", async ({ page, request }) => {
  const anon = await request.get("/api/bff/v1/users/me");
  expect(anon.status()).toBe(401);

  await signIn(page, testIdentity());
  const forged = await page.request.patch("/api/bff/v1/users/me", {
    headers: { Origin: "https://evil.example", "Content-Type": "application/json" },
    data: { displayName: "pwned" },
  });
  expect(forged.status()).toBe(403);

  const smuggled = await page.request.patch("/api/bff/v1/users/me", {
    headers: { Origin: "http://localhost:8000", "Content-Type": "application/json" },
    data: { role: "admin" },
  });
  expect(smuggled.status()).toBe(400);
  const me = await (await page.request.get("/api/bff/v1/users/me")).json();
  expect(me.data.role).toBe("learner");

  // The internal API is not reachable through the edge, and provisioning is not reachable via the BFF.
  expect((await request.get("/api/v1/users/me")).status()).toBe(404);
  expect(
    (await page.request.post("/api/bff/v1/auth/provision", { headers: { Origin: "http://localhost:8000" } })).status(),
  ).toBe(404);
});

test("the session cookie is HttpOnly and the browser never sees an assertion", async ({ page, context }) => {
  const tokens: string[] = [];
  page.on("response", async (res) => {
    const auth = res.request().headers()["authorization"];
    if (auth) tokens.push(auth);
  });
  await signIn(page, testIdentity());
  await page.goto("/settings");
  const cookie = (await context.cookies()).find((c) => c.name.includes("session-token"));
  expect(cookie?.httpOnly).toBe(true);
  expect(cookie?.sameSite).toBe("Lax");
  expect(tokens).toEqual([]);
  const session = await (await page.request.get("/api/auth/session")).json();
  expect(Object.keys(session).sort()).toEqual(["expires", "subject"]);
});
