# Design tokens

Source of truth: [`apps/web/lib/design-tokens.ts`](../../apps/web/lib/design-tokens.ts). [`app/globals.css`](../../apps/web/app/globals.css) declares the same values as CSS variables, and [`tests/unit/design-tokens.test.ts`](../../apps/web/tests/unit/design-tokens.test.ts) fails if they drift or if any listed pair drops below WCAG 2.2 AA (4.5:1 text, 3:1 UI).

## Color roles

| Role | Dark (default brand) | Light | Use |
| --- | --- | --- | --- |
| background | `#040D1D` ink (the logo's navy) | `#F4F2EC` paper | Page |
| card / popover | `#0A1629` / `#0C1A30` | `#FFFDF8` | Raised surfaces |
| foreground | `#E8EEF8` | `#0B1324` | Body text |
| muted-foreground | `#97A7C0` | `#52565E` | Secondary text |
| primary | `#0165F6` logo blue | `#0157D6` | Primary buttons, selected state (white text ≥ 5:1) |
| link | `#7DB9FF` | `#0450C2` | Inline links. **Don't use `primary` for text on dark backgrounds.** |
| highlight | `#22E2F9` logo cyan | `#0B6E86` | Accents, brand word, emphasis |
| ring | `#22E2F9` | `#0B6E86` | Focus outline (2 px, 2 px offset) |
| success / warning / danger | `#3CCF7D` / `#F5B83D` / `#FF8A80` | `#17773B` / `#A14A06` / `#B42318` | Status text and icons |
| border / input | `#1A2B48` / `#5A7096` | `#D9D3C6` / `#8A8F98` | Dividers / form field borders (3:1) |
| sim-badge | `#33280A` on `#FCD34D` | `#F8EBC4` on `#6B3A0B` | "Simulated", "In development" labels |

Rules:

- Status is never conveyed by color alone: pair it with an icon or text (`statusStyles` in the tokens file).
- Light theme uses darker variants of green/amber/red/cyan; the dark-theme hues fail contrast on white.

## Typography

| Token | Font | Use |
| --- | --- | --- |
| `--font-heading` | Bricolage Grotesque (next/font, self-hosted, optical sizes) | h1–h3 and the wordmark |
| `--font-sans` | IBM Plex Sans (next/font, self-hosted) | UI and content |
| `--font-mono` | JetBrains Mono (not preloaded) | Terminal, code, kubectl |

Scale (Tailwind): 48/60 display (`text-5xl`/`6xl`), 30–36 headings, 20 titles, 16 body, 14 secondary, 12 captions. Headings use `text-balance`, paragraphs `text-pretty`.

## Spacing, radius, elevation

- Tailwind's 4 px spacing scale. Page gutters 16/24/32 px (`px-4 sm:px-6 lg:px-8`), max content width 72 rem.
- Radius base `0.375rem`; panels `rounded-lg`, controls `rounded-md`, badges are square tags (`3px`).
- Elevation is borders only (`shadow-panel` is a 1 px inner highlight). No glows.
- Belt colors are plain CSS variables `--belt-1` … `--belt-6` (decorative, always paired with text).

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
