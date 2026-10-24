# Writing lessons

Lessons are files in Git, reviewed like code and published as immutable versions ([ADR-0005](../adr/0005-versioned-content.md)). This guide covers the format, the review workflow and the accessibility rules.

## Where things live

```text
content/
  courses/<course>.yaml              course metadata and the ordered list of lesson slugs
  challenges/<course>/<lesson>.yaml  one file per lesson: text, goals, hints, solution
  fixtures/<name>.yaml               the simulated cluster a lab starts from
  glossary.yaml                      short definitions linked from lessons
```

Schemas: [packages/content-schema](../../packages/content-schema/README.md). Templates to copy, one for each goal type:

| Template | Shows |
| --- | --- |
| [`fundamentals/cluster-architecture`](../../content/challenges/fundamentals/cluster-architecture.yaml) | **Answer** goals: a count computed from the cluster, and a multiple-choice concept question |
| [`workloads/inspect-a-pod`](../../content/challenges/workloads/inspect-a-pod.yaml) | An **observation** goal plus answers found by reading `describe` and logs |
| [`workloads/scaling-deployments`](../../content/challenges/workloads/scaling-deployments.yaml) | A **state** goal checked against the resulting cluster, plus a starter manifest in the editor |

## Anatomy of a lesson file

| Field | Visible to learners? | Notes |
| --- | --- | --- |
| `id`, `version`, `course`, `lesson` | IDs only | `id` is `<course>/<lesson>`. **Bump `version` whenever you change anything**; the importer refuses changed content under an old version. |
| `title`, `summary`, `estimatedMinutes`, `objectives`, `prerequisites`, `references`, `glossary` | Yes | Prerequisites are lesson IDs; unpublished ones show as "coming soon". |
| `body` | Yes | Markdown (GitHub-flavoured). Raw HTML is dropped, not rendered. |
| `initialFixture`, `initialFiles`, `terminal.allowedFamilies` | No (used to build the lab) | Fixtures may only use kinds and behaviors in the [compatibility matrix](../architecture/simulator-compatibility.md). |
| `goals[*].description`, `prompt`, `input`, `choices` | Yes | Never reveal the answer in a description or prompt. |
| `goals[*].target`, `assertions`, `expected`, `failureMessage` | **No** | Graded on the server ([ADR-0007](../adr/0007-challenge-goal-types.md)). `failureMessage` appears only after a failed check. |
| `hints` | One at a time, on request | Order from gentle nudge to near-solution. |
| `solution` | **No** | Replayed in CI to prove the lesson can be passed. Give `answers` for static answers; path and count answers are computed. |
| `reward` | Totals only | Base XP, first-try bonus and per-hint penalty. |

## Writing well

- **One idea per lesson.** Explain what something is *for* before how to use it.
- **Short.** Aim for 300–600 words of body text; the lab is where learning happens.
- **Grade state, not keystrokes.** Prefer state goals. Use answer goals for investigation lessons, and observation goals only alongside them.
- **Be honest about the simulator.** End with a `> **Simulated cluster:** …` note whenever the lesson relies on simulated behavior (instant readiness, fixed logs). It renders as a callout.
- **Plain language.** Define jargon the first time, or link it through `glossary`. Use second person ("you") and active voice.
- **Real commands only.** Every command in the body must work in the simulator; CI replays the solution, and reviewers run the body's examples.

## Accessibility

- Use headings in order: the lesson title is the page's `h1`, so body sections start at `##`.
- Write descriptive link text ("Scaling a Deployment"), never "click here".
- Tables need a header row. Don't use tables for layout.
- Don't convey meaning by color or emoji alone.
- Put commands in code blocks (they get a copy button), not in images.

## Workflow

1. Branch and edit files under `content/`. If you add a term, add it to `glossary.yaml` and run `pnpm --filter @kubedojo/web gen:glossary`.
2. Validate locally: `make content-validate`. Unwritten lessons a course lists are reported as *planned*; that's fine until Part 17 makes them required (`--complete`).
3. Preview: `make up` (or `make dev`) imports and publishes your content locally. Open `/learn`.
4. Open a pull request. CI validates the schemas and cross-file rules, and (from Part 17) replays every solution in the simulator.
5. A reviewer with Kubernetes experience checks accuracy, and a second reviewer checks clarity and accessibility.
6. After merge and deploy, the new version is imported as a **draft**. An admin publishes it (`api content import -publish` today; the admin screen in Part 18). Learners mid-lab keep the version they started with.
