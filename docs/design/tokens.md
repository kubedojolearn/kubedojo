# Design tokens

Source of truth: [`apps/web/lib/design-tokens.ts`](../../apps/web/lib/design-tokens.ts). [`app/globals.css`](../../apps/web/app/globals.css) declares the same values as CSS variables, and [`tests/unit/design-tokens.test.ts`](../../apps/web/tests/unit/design-tokens.test.ts) fails if they drift or if any listed pair drops below WCAG 2.2 AA (4.5:1 text, 3:1 UI).

## Color roles

The palette is GitDojo's, so both products look like one family.

| Role | Dark (default) | Light | Use |
| --- | --- | --- | --- |
| background | `#0B0D10` | `#F4F6F9` | Page |
| surface | `#111419` | `#F9FAFB` | Banded sections, quiet wells |
| card / popover | `#151920` / `#1A1F27` | `#FFFFFF` | Raised surfaces |
| elevated / hover | `#1A1F27` / `#202630` | `#EEF1F5` / `#E5E9EF` | Chips and keycaps / hover fill |
| foreground | `#F4F7FB` | `#111827` | Headings and body text |
| foreground-secondary | `#AEB6C2` | `#3D4756` | Copy under a heading |
| muted-foreground | `#8A93A1` | `#556070` | Captions, meta text (4.5:1 on every surface up to `hover`) |
| faint | `#6F7886` | `#78828F` | Icons and markers only (3:1), never text |
| primary | `#4A67E0` | `#3A5AD6` | Filled buttons (white text ≥ 4.5:1) |
| link / highlight / ring | `#6C8CFF` | `#3A5AD6` | Links, the brand word, eyebrows, focus outline |
| success / warning / danger | `#4FD18B` / `#F5C451` / `#F06A78` | `#11703A` / `#8A5A00` / `#C0263A` | Status text and icons |
| border / border-subtle / border-strong | `#262C35` / `#1D222A` / `#343C48` | `#D6DCE4` / `#E4E8EE` / `#C2C9D3` | Dividers; header and section rules; outline buttons |
| input | `#6F7886` | `#78828F` | Form field borders (3:1) |
| sim-badge | `#2B2414` on `#F5C451` | `#FBF1D9` on `#6B4500` | "Simulated", "In development" labels |

`--primary-soft`, `--highlight-soft`, `--glow` and `--shade` are translucent companions (soft badge fills, hero glow, shadows); they are CSS-only.

Rules:

- Status is never conveyed by color alone: pair it with an icon or text (`statusStyles` in the tokens file).
- Light theme uses darker variants of green/amber/red/cyan; the dark-theme hues fail contrast on white.

## Typography

| Token | Font | Use |
| --- | --- | --- |
| `--font-sans` (and `--font-heading`) | Inter (next/font, self-hosted) | Headings, UI and content |
| `--font-mono` | JetBrains Mono (not preloaded) | Terminal, code, kubectl, eyebrows |

Type scale (shared with GitDojo): `text-display` 56/64, `text-h1` 40/48, `text-h2` 30/38, `text-h3` 22/30, `text-h4` 18/26, `text-body-lg` 17/28, `text-body` 15/24 (the body default), `text-small` 13/20, `text-caption` 12/18, `text-micro` 11/16. Headings are bold or semibold with tight tracking, use `text-balance`; paragraphs `text-pretty`. `lib/utils.ts` teaches `cn` these sizes, so `text-small` and a text color can be merged without one dropping the other.

## Spacing, radius, elevation

- Tailwind's 4 px spacing scale. Page gutters 16/24 px (`px-4 sm:px-6`), max content width 1280 px (`Container`).
- Radius: `sm` 6, `md` 8 (controls), `lg` 10 (cards, panels), `xl` 12 (hero windows, the sign-in card).
- Elevation is borders first. Shadows only on overlays and the hero/sign-in windows (`shadow-shade/40`).
- Belt colors are plain CSS variables `--belt-1` … `--belt-6` (decorative, always paired with text).

## Breakpoints

| Width | Behavior |
| --- | --- |
| < 768 | Search is an icon button; single-column grids |
| < 1280 | Header links in a menu panel under the header (☰) |
| ≥ 1280 (`xl`) | Header links inline |
| ≥ 1024 (`lg`) | Lab split view; two-column marketing sections |

## Terminal

`terminal` in the tokens file is the xterm.js theme: background `#080A0D`, foreground `#D6DCE5`, cursor blue, ANSI colors all ≥ 4.5:1 on the background. The terminal is dark in both app themes.

## Cluster map

`graphNodes` assigns each kind a color, an icon and a shape (Pod circle, Service hexagon, Deployment/ReplicaSet/Node rounded rectangles, ConfigMap/Secret document, Namespace frame). Shape and icon make kinds distinguishable without color.

## Motion

Transitions are short (≤ 200 ms) and decorative only. `prefers-reduced-motion` and the in-app setting (`html.reduce-motion`, Part 08) reduce all animations and transitions to effectively zero.
