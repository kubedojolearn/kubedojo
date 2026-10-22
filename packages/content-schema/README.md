# @kubedojo/content-schema

JSON Schemas (2020-12) and a validator for KubeDojo lesson content.

| Schema | Validates | Lives at |
| --- | --- | --- |
| [course.schema.json](schemas/course.schema.json) | A course and its ordered lesson list | `content/courses/<slug>.yaml` |
| [challenge.schema.json](schemas/challenge.schema.json) | A lesson: learner-facing text, starting fixture, goals, hints, reference solution, reward | `content/challenges/<course>/<lesson>.yaml` |
| [fixture.schema.json](schemas/fixture.schema.json) | The simulated cluster a lab starts from | `content/fixtures/*.yaml` |
| [common.schema.json](schemas/common.schema.json) | Shared definitions (slugs, kinds, labels, virtual files) | — |

Goal types and the assertion language are defined in [ADR-0007](../../docs/adr/0007-challenge-goal-types.md); what the simulator supports is in [simulator-compatibility.md](../../docs/architecture/simulator-compatibility.md).

## Validate

```sh
# A full content tree: schema checks plus cross-file rules
node bin/validate.js ../../content

# The examples in this package (course lesson lists may be incomplete)
node bin/validate.js examples --partial

# Every file in a directory must FAIL (type from the file-name prefix)
node bin/validate.js --expect-invalid examples/invalid

# Unit tests (validator rules and the WebSocket protocol schema)
node --test
```

Cross-file rules on top of the schemas:

- `id` equals `<course>/<lesson>`, and the file path matches.
- The course lists the lesson; every lesson a course lists has a challenge file (skipped with `--partial`).
- `initialFixture` exists; prerequisites exist and aren't self-references.
- Goal IDs are unique; cluster-scoped kinds (Node, Namespace) have no namespace in targets.
- Every answer goal has an answer in `solution.answers`, and choice answers are among the choices.
- Fixture resources are unique and correctly namespaced.
- Duplicate YAML keys are errors.

## What is hidden from learners

`goals[*].target`, `assertions` and `expected`, `hints` (until revealed one by one), and `solution` never leave the API. Learners see `goals[*].description`, `prompt`, `input` and `choices`. See the `LessonDetail` schema in [openapi.yaml](../../docs/api/openapi.yaml).

## Examples

- [`examples/challenges/workloads/scaling-deployments.yaml`](examples/challenges/workloads/scaling-deployments.yaml) — a state goal (the master plan's sample, in the final format)
- [`examples/challenges/fundamentals/cluster-architecture.yaml`](examples/challenges/fundamentals/cluster-architecture.yaml) — answer goals (count and choice)
- [`examples/invalid/`](examples/invalid) — files that must be rejected, one rule each
