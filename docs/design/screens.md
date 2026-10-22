# V1 screens

- **Status:** Accepted (Part 03). Wireframes are low-fidelity; the implemented shell and the [design system gallery](../../apps/web/app/design-system/page.tsx) (`/design-system`) are the visual reference.
- Every screen has exactly one `h1`, a skip link to `#main`, and works from 320 px wide. Below 1024 px the app sidebar becomes a drawer; below 768 px marketing navigation becomes a drawer.
- Data-dependent screens list their **empty**, **loading** and **error** states. Loading uses skeletons plus a polite live region; errors use `ErrorState` (`role="alert"`) with a retry.

Legend: `[Button]`, `(link)`, `▢` card, `░` skeleton, `◷` disabled-with-reason (`UnavailableButton`).

## Public (marketing)

### Home `/` — implemented (Part 03)

```text
┌ Logo  Curriculum About FAQ Changelog ─────────── GH ☾ [Sign in] ☰ ┐
│ [In development] Free and open source                             │
│ Master Kubernetes, one command at a time.     ┌ terminal example ┐│
│ Learn by doing… nothing to install.           │ $ kubectl …       ││
│ [Explore the curriculum] [How it works]       │ ✓ Goal met        ││
│                                               └ "Simulated" label ┘│
│ How it works:  ▢ Learn the idea  ▢ Try it  ▢ Check your work      │
│ What you'll learn: 6 course cards (title, summary, level, count)  │
│ A simulator, and honest about it + 4 principle cards              │
│ Built in the open CTA                                             │
└ Footer: Learn / Project / Legal · license · trademark notice ─────┘
```

### Curriculum `/learn` — implemented as plan (Part 03), catalog in Part 07

Course cards with ordered lesson lists and a note that lessons open as they're published. Part 07 adds: search, difficulty filter, per-lesson links, progress badges for signed-in learners, prerequisites.

| State | Behavior |
| --- | --- |
| Loading (Part 07) | `CardListSkeleton` (6) |
| Empty (no published courses) | `EmptyState`: "No courses are published yet" |
| Error | `ErrorState` with retry |

### Course `/learn/[course]` — Part 07

```text
Breadcrumb: Curriculum › Pods and workloads
h1 Course title · level · lessons · est. time · [Start/Continue]
Description (Markdown)                         ▢ Your progress (signed in)
Ordered lessons: № · title · minutes · goal types · status ✓/●/○
```

### Lesson `/learn/[course]/[lesson]` — Part 07

```text
Breadcrumb › lesson
h1 Lesson title · minutes · prerequisites (links)
Objectives (list) · concept reading (Markdown, code blocks with Copy)
Goals (descriptions only — never assertions or answers)
[Start lab] (sign-in prompt if signed out) · (Previous) (Next)
References · glossary terms
```

### About, FAQ, Privacy, Terms, Changelog — implemented (Part 03)

Long-form pages use `Prose`. FAQ uses an accordion (all items independently expandable). Terms are marked **Draft** until reviewed before sign-in opens.

## Authentication

### Sign in `/login` — layout implemented; sign-in in Part 06

```text
        Logo
 ▢ [Coming soon]
   h1 Sign in to KubeDojo
   Save your progress…
   ◷ Continue with GitHub
   We only use your GitHub ID, username, name and avatar (privacy)
```

States (Part 06): OAuth cancelled → message + retry; provisioning failed → error with request reference; already signed in → redirect to `callbackUrl` (same-origin only).

## Signed-in app (shell implemented in Part 03)

```text
┌ Sidebar (≥1024px) ┬ Topbar: ☰(mobile) ◷Search…⌘K   ◷🔔 ☾ 👤 ┐
│ Logo              │ [Preview] notice (until Part 06)          │
│ Dashboard         │ main#main                                 │
│ Curriculum        │                                           │
│ Learning paths    │                                           │
│ Leaderboard       │                                           │
│ Settings          │                                           │
│ ADMIN (role only) │                                           │
│ Overview/Content/ │                                           │
│ Audit log         │                                           │
└───────────────────┴───────────────────────────────────────────┘
```

The Admin group is shown only to admins once roles exist (Part 06); the API enforces access regardless.

### Dashboard `/dashboard` — empty states implemented; data in Parts 08, 16, 18

```text
h1 Dashboard
▢ Continue learning (last lab + [Resume])   ▢ Streak 🔥   ▢ XP ✦
▢ Recommended next lesson                    ▢ Course progress bars
Recent achievements (row)  · Activity (list, paginated)
```

Empty (new learner): "No lessons started yet" + [Browse the curriculum]; streak/XP explain how to earn them. Never show sample numbers.

### Learning paths `/paths` — implemented (static plan)

"Kubernetes foundations": the six courses in order. Future paths are added by content PRs.

### Profile `/profile/[handle]` — Part 08

Public only if the owner opts in; otherwise 404 (not "private"). Shows avatar, display name, handle, joined date, XP, completed lessons, achievements.

### Settings `/settings` — appearance works now; rest in Parts 06/08/19

Sections: Appearance (theme — stored locally now, synced in Part 08; reduced motion), Profile (handle, display name), Privacy (public profile, leaderboard, analytics opt-out), Time zone (for streaks), Your data (export, delete with `ConfirmDialog` requiring the handle).

### Leaderboard `/leaderboard` — Part 18

Ranked table: rank, avatar, display name/handle, XP, lessons. Only opted-in learners. Signed-in learner sees their own rank row pinned if opted in; if not, a prompt explaining how to opt in.

### Lab `/lab/[sessionId]` — layout preview implemented; Parts 09–16

```text
h1 Lesson title · [Simulated] · status ●Connected     [Hint] [Reset…] [Check my work]
┌ Lesson & goals (resizable) ┬ Tabs: Terminal | YAML editor | Cluster map ┐
│ Concept summary            │ Terminal (xterm.js, screen-reader mode)    │
│ Goals ☐/☑ with feedback     │ Editor (Monaco; textarea fallback on touch)│
│ Answer inputs (answer goals)│ Map (React Flow) + [List view] toggle      │
│ Hints revealed so far       │                                            │
└────────────────────────────┴────────────────────────────────────────────┘
```

- **Below 1024 px:** lesson pane becomes a fourth tab; tabs wrap.
- **Connection states:** Connecting (skeleton), Connected, Reconnecting… (banner, commands queued disabled), Session ended (read-only, [Start again]).
- **Reset:** `ConfirmDialog`, focus on Cancel.
- **Check my work:** per-goal results announced in a live region; on pass, completion panel with XP and [Next lesson].
- **Cluster map list view:** a tree (Namespace → Deployment → ReplicaSet → Pods; Service → endpoints) with the same status icons; updates announced politely.

## Admin (Part 18)

### Overview `/admin`, Content `/admin/content`, Audit `/admin/audit`

```text
Content: table — lesson · current version · drafts · last imported · [Review]
  Review drawer: version metadata, SHA-256, validation report, diff vs current,
  [Publish…] / [Unpublish…] (ConfirmDialog, audited). No text editing (ADR-0005).
Audit: table — time · actor · action · entity · details (redacted), cursor-paginated
```

## Error pages

- **404:** "We couldn't find that page" with a light kubectl joke and [Go home]; HTTP 404.
- **Route error:** "Something went wrong", reference digest, [Try again]. Details are never shown to users.
