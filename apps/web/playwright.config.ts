import { defineConfig, devices } from "@playwright/test";

const port = Number(process.env.E2E_PORT ?? 3100);
const baseURL = process.env.E2E_BASE_URL ?? `http://localhost:${port}`;
// E2E_SKIP_BUILD=1 serves an existing standalone build (used inside the Playwright container,
// where the host's build is reused because native build tooling differs by platform).
const serve = `node scripts/start-standalone.mjs ${port}`;

export default defineConfig({
  testDir: "tests",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: { baseURL, trace: "retain-on-failure" },
  // Baselines have no platform suffix: they are only produced inside the pinned Playwright image.
  snapshotPathTemplate: "{testDir}/visual/__screenshots__/{arg}{ext}",
  expect: { toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: "disabled" } },
  projects: [
    { name: "chromium", testMatch: "e2e/**/*.spec.ts", use: { ...devices["Desktop Chrome"] } },
    { name: "visual", testMatch: "visual/**/*.spec.ts", use: { ...devices["Desktop Chrome"] } },
    // Full stack (edge + web + API + Postgres + Redis) with AUTH_TEST_MODE=1; run via `make test-stack`.
    { name: "stack", testMatch: "stack/**/*.spec.ts", use: { ...devices["Desktop Chrome"] } },
    // Every page against the full stack: a11y, layout, images, links and errors; run via `make test-ui`.
    { name: "ui", testMatch: "ui/**/*.spec.ts", use: { ...devices["Desktop Chrome"] } },
  ],
  webServer: process.env.E2E_BASE_URL
    ? undefined
    : {
        command: process.env.E2E_SKIP_BUILD ? serve : `pnpm build && ${serve}`,
        // Signed-out browsing needs only a session secret; no API is running for these tests.
        env: {
          AUTH_SECRET: process.env.AUTH_SECRET ?? "playwright-only-secret-not-used-anywhere-else",
          WEB_ORIGIN: baseURL,
        },
        url: `${baseURL}/api/healthz`,
        reuseExistingServer: !process.env.CI,
        timeout: 180_000,
      },
});
