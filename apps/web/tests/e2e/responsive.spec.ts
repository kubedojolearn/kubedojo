import { expect, test } from "@playwright/test";
import { allRoutes } from "./routes";

const widths = [320, 375, 768, 1024, 1440];

for (const width of widths) {
  test.describe(`${width}px`, () => {
    test.use({ viewport: { width, height: 900 } });
    for (const route of allRoutes) {
      test(`${route} has no horizontal page scroll`, async ({ page }) => {
        await page.goto(route);
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
        expect(overflow).toBeLessThanOrEqual(0);
      });
    }
  });
}
