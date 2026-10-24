# ADR-0007: Three challenge goal types — state, answer, observation

- **Status:** Accepted · **Date:** 2026-10-22
- Resolves plan-review item 1.

## Context

The plan grades only by asserting on resource state ("the evaluator reads state, never an exact required command string"). About a third of the planned lessons teach investigation (`get`, `describe`, `logs`, `events`) and change nothing, so a learner who does nothing would pass them.

## Decision

A challenge has one or more goals; each goal is exactly one of three types. All goals must pass for the lesson to complete.

### `state`

Assertions on one simulated resource, or that it does not exist.

```yaml
- id: deployment-ready
  type: state
  target: { kind: Deployment, namespace: default, name: nginx }   # or labelSelector (must match exactly one)
  assertions:
    - { path: spec.replicas, operator: equals, value: 3 }
    - { path: status.readyReplicas, operator: equals, value: 3 }
```

`target.absent: true` asserts the resource does not exist.

### `answer`

The learner types or picks an answer in the goals panel. The expected value is computed **on the server, at grading time, from the session's current state**, or fixed in the hidden spec. Expected values never leave the server.

```yaml
- id: node-count
  type: answer
  prompt: How many nodes are in this cluster?
  input: number                      # text | number | choice
  expected:
    count: { kind: Node }            # one of: static | path | count
- id: pod-ip
  type: answer
  prompt: What is the IP address of Pod `api`?
  input: text
  expected:
    path: { target: { kind: Pod, namespace: default, name: api }, path: status.podIP }
- id: event-reason
  type: answer
  prompt: Which reason does the Warning event for Pod `etl` show?
  input: choice
  choices: [BackOff, FailedScheduling, Killing, Pulled]
  expected: { static: FailedScheduling }
```

Comparison normalizes surrounding whitespace; `text` answers may set `caseSensitive: false`. Answers are stored in `lab_answers` (bounded length). Each goal allows at most 10 submissions per session, and `choice` goals at most `len(choices) - 1`, so brute force can't pass them.

### `observation`

A command with a given **parsed** shape ran successfully (exit code 0) in this session. Matching is on the parser's structured output, never on the raw text, so aliases, flag order and quoting don't matter.

```yaml
- id: described-pod
  type: observation
  command: { verb: describe, kind: Pod, name: api }   # optional: namespace, output, subverb
```

Use observation goals sparingly and pair them with an answer or state goal where possible: they show that the learner used the tool, not that they understood the output.

### Assertion DSL (shared by `state` goals and `path` answers)

- **Paths:** dot-separated fields, `[n]` for list indexes, `["key.with.dots"]` for map keys (e.g. `metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]`). No wildcards, filters or expressions.
- **Operators:** `equals`, `notEquals`, `exists`, `notExists`, `in`, `contains` (list element or substring), `greaterThanOrEqual`, `lessThanOrEqual`, `lengthEquals`, `lengthGreaterThanOrEqual`. Quantity-aware comparison for CPU/memory strings (`500m` ≤ `1`).
- No scripts, JSONPath, CEL, regex, or any executable content. New operators require a schema change and review.

## Consequences

- Every lesson can be graded, including read-only ones, without matching command strings.
- The goals panel UI needs inputs for answer goals (Part 16) and `POST /labs/{id}/answers`.
- Path-based answers stay correct even though seeded names and IPs differ per session.

## Alternatives considered

- **Command-string matching:** brittle and teaches memorization. Rejected by the plan.
- **Quizzes outside the lab:** disconnects the question from the cluster the learner is looking at.
