# Quality baseline (Part 03)

Recorded 2026-10-22 against the production standalone build (`node scripts/start-standalone.mjs`) on the developer machine. Lighthouse 12, default mobile profile (simulated throttling: slow 4G, 4× CPU), two runs per page; ranges shown.

| Page | Performance | Accessibility | Best practices | SEO | FCP | LCP | TBT | CLS |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `/` | 92–93 | 100 | 100 | 100 | 1.2 s | 3.2–3.3 s | 60–80 ms | 0 |
| `/learn` | 93 | 100 | 100 | 100 | 1.2 s | 3.1 s | 80 ms | 0 |
| `/faq` | 96–99 | 100 | 100 | 100 | 0.9 s | 2.3–2.8 s | 40–50 ms | 0 |
| `/dashboard` | 94–99 | 100 | 100 | 63* | 0.9 s | 2.3–3.0 s | 40–80 ms | 0 |

\* App pages are deliberately `noindex`; the SEO score reflects that.

## Against the budgets in [requirements.md](../architecture/requirements.md#performance)

- **P1 LCP ≤ 2.5 s: not met yet** on `/` and `/learn` in lab conditions (3.1–3.3 s). The LCP element is the hero paragraph, and nearly all of the time is render delay, not network loading. Changing the body font to `display: optional` made no difference, so fonts are ruled out. The likely cause is the client JavaScript every page hydrates from the root layout (Radix dropdown, dialog and sheet; Sonner; keyboard shortcuts), about 320 KB total transfer. **Follow-up:** lazy-load the shortcuts dialog, toaster and theme menu, and re-measure. Lab measurements also overstate field LCP; field data (p75) is the real budget once deployed.
- **P2 INP:** not measurable in Lighthouse navigation mode; TBT (a proxy) is 40–120 ms.
- **CLS:** 0 on all pages.

## Accessibility

- axe (WCAG 2.0/2.1/2.2 A and AA plus best practices) on all 19 routes in both themes: **0 serious or critical violations** (`tests/e2e/accessibility.spec.ts`).
- Token contrast: all text pairs ≥ 4.5:1 and UI pairs ≥ 3:1 in both themes (`tests/unit/design-tokens.test.ts`).
- No horizontal page scroll at 320, 375, 768, 1024 and 1440 px on any route (`tests/e2e/responsive.spec.ts`).
- Keyboard: skip link first, visible 2 px focus ring, shortcuts suppressed while typing, focus starts on Cancel in confirmations (`tests/e2e/keyboard.spec.ts`).
- Not yet done: a manual screen-reader pass with VoiceOver and NVDA, which is due before launch per [requirements.md](../architecture/requirements.md).

## Reproduce

```sh
cd apps/web && pnpm build && node scripts/start-standalone.mjs 3100 &
CHROME_PATH="$(node -e 'console.log(require("@playwright/test").chromium.executablePath())')" \
  npx lighthouse@12 http://localhost:3100/ --chrome-flags="--headless=new" --view
```
