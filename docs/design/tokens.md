# Design tokens

Source of truth: [`apps/web/lib/design-tokens.ts`](../../apps/web/lib/design-tokens.ts). [`app/globals.css`](../../apps/web/app/globals.css) declares the same values as CSS variables, and [`tests/unit/design-tokens.test.ts`](../../apps/web/tests/unit/design-tokens.test.ts) fails if they drift or if any listed pair drops below WCAG 2.2 AA (4.5:1 text, 3:1 UI).

## Color roles

| Role | Dark (default brand) | Light | Use |
| --- | --- | --- | --- |
| background | `#09111F` navy | `#F7F9FC` | Page |
| card / popover | `#111C2E` surface | `#FFFFFF` | Raised surfaces |
| foreground | `#E6EDF7` | `#0B1324` | Body text |
| muted-foreground | `#9AABC4` | `#4A5A75` | Secondary text |
| primary | `#326CE5` Kubernetes blue | `#326CE5` | Primary buttons, selected state (white text 4.76:1) |
| link | `#8CB4FF` | `#1F56C9` | Inline links. **Don't use `primary` for text on dark backgrounds** (3.97:1). |
| highlight | `#22D3EE` cyan | `#0E7490` | Accents, brand word, emphasis |
| ring | `#22D3EE` | `#0E7490` | Focus outline (2 px, 2 px offset) |
| success / warning / danger | `#22C55E` / `#FBBF24` / `#F87171` | `#15803D` / `#B45309` / `#B91C1C` | Status text and icons |
| border / input | `#22314D` / `#5B6F92` | `#D5DDEA` / `#7A8AA6` | Dividers / form field borders (3:1) |
| sim-badge | `#3B2F0B` on `#FCD34D` | `#FEF3C7` on `#78350F` | "Simulated", "In development" labels |

Rules:

- Status is never conveyed by color alone: pair it with an icon or text (`statusStyles` in the tokens file).
- Light theme uses darker variants of green/amber/red/cyan; the dark-theme hues fail contrast on white.

## Typography

| Token | Font | Use |
| --- | --- | --- |
| `--font-sans` | Geist (next/font, self-hosted) | UI and content |
| `--font-mono` | JetBrains Mono (not preloaded) | Terminal, code, kubectl |

Scale (Tailwind): 48/60 display (`text-5xl`/`6xl`), 30–36 headings, 20 titles, 16 body, 14 secondary, 12 captions. Headings use `text-balance`, paragraphs `text-pretty`.

## Spacing, radius, elevation

- Tailwind's 4 px spacing scale. Page gutters 16/24/32 px (`px-4 sm:px-6 lg:px-8`), max content width 72 rem.
- Radius base `0.625rem`; cards `rounded-xl`, controls `rounded-lg`, badges `rounded-full`.
- Elevation is mostly borders; `shadow-glow` is reserved for the terminal hero.

## Breakpoints

| Width | Behavior |
| --- | --- |
| < 768 | Marketing nav in a drawer; single-column grids |
| ≥ 768 (`md`) | Marketing nav inline |
| < 1024 | App sidebar in a drawer; lab panels as tabs |
| ≥ 1024 (`lg`) | Sidebar visible; lab split view |

## Terminal

`terminal` in the tokens file is the xterm.js theme: background `#050B16`, foreground `#D6E2F5`, cursor cyan, ANSI colors all ≥ 4.5:1 on the background. The terminal is dark in both app themes.

## Cluster map

`graphNodes` assigns each kind a color, an icon and a shape (Pod circle, Service hexagon, Deployment/ReplicaSet/Node rounded rectangles, ConfigMap/Secret document, Namespace frame). Shape and icon make kinds distinguishable without color.

## Motion

Transitions are short (≤ 200 ms) and decorative only. `prefers-reduced-motion` and the in-app setting (`html.reduce-motion`, Part 08) reduce all animations and transitions to effectively zero.
