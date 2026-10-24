# Brand

## Logo

A torii gate (the dojo) framing three cubes (the cluster), tied with a belt. The master artwork is [`apps/web/brand/kubedojo-logo-source.png`](../../apps/web/brand/kubedojo-logo-source.png); it is not served.

`pnpm --filter @kubedojo/web brand:assets` regenerates every file the site uses from it (outputs are committed):

| File | Use |
| --- | --- |
| `public/brand/kubedojo-mark.png` | The mark on its navy tile: header, footer, sidebar (`LogoMark`) |
| `public/brand/kubedojo-logo.webp` | The full lockup: sign-in panel and design system (`LogoLockup`) |
| `app/icon.png`, `app/apple-icon.png`, `app/favicon.ico` | Browser and home-screen icons |

The Open Graph/Twitter card (`app/opengraph-image.tsx`) embeds the mark. The mark doesn't imitate the Kubernetes helm wheel or heptagon (risk R7 in the [risk register](../architecture/risk-register.md)).

## Wordmark

"Kube" in the foreground color + "Dojo" in the highlight color, set in Bricolage Grotesque bold as live text (not an image), so it scales and is read by screen readers.

## Belts

Each course is a belt, in curriculum order: white, yellow, orange, green, blue, black (`components/brand/belt.tsx`). The belt shows where a course sits and is the learner's rank on the dashboard. Belts are decorative: they always appear next to the course title or the belt name, never alone.

## Visual language

Ink and paper: the logo's navy as the dark background, warm paper for light. Hairline borders and divided lists rather than floating cards, small corner radii, no glows or gradient text. Mono labels (`white belt · course 1`) mark structure. Pages show real product content (real kubectl output, the real syllabus) instead of generic feature tiles.

## Voice

Plain, direct and honest. Explain what things are for before how to use them. Never overstate: say "simulated", "coming soon" or "in development" where true, never show sample data as if it were real, and never promise certification outcomes.

## Trademark

Every page footer and the About page state that KubeDojo is independent and not affiliated with the CNCF or The Linux Foundation, and that Kubernetes is a registered trademark of The Linux Foundation. Check the Linux Foundation's trademark guidelines before public launch.
