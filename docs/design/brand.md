# Brand

## Logo (provisional)

No approved logo existed when Part 03 was built, so the project uses a **provisional mark**: an isometric cube (the "Kube") wearing a cyan belt with a knot (the "Dojo") on a navy tile. Source: [`apps/web/app/icon.svg`](../../apps/web/app/icon.svg); React component: `components/brand/logo.tsx`.

- The mark intentionally does **not** resemble the Kubernetes helm wheel or heptagon (see risk R7 in the [risk register](../architecture/risk-register.md)).
- Replace it by editing `app/icon.svg` and the matching paths in `logo.tsx`, then run `node scripts/generate-icons.mjs` in `apps/web` to regenerate `favicon.ico` (16/32/48) and `apple-icon.png` (180).
- The Open Graph/Twitter card is generated at request time by `app/opengraph-image.tsx`.

## Wordmark

"Kube" in the foreground color + "Dojo" in the highlight color, set in Geist semibold as live text (not an image), so it scales and is read by screen readers.

## Voice

Plain, direct and honest. Explain what things are for before how to use them. Never overstate: say "simulated", "coming soon" or "in development" where true, never show sample data as if it were real, and never promise certification outcomes.

## Trademark

Every page footer and the About page state that KubeDojo is independent and not affiliated with the CNCF or The Linux Foundation, and that Kubernetes is a registered trademark of The Linux Foundation. Check the Linux Foundation's trademark guidelines before public launch.
