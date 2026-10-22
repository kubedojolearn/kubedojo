## What and why

<!-- Which plan part (e.g. Part 05) or issue does this implement? What changed for learners or operators? -->

## How it was tested

<!-- Commands you ran and their results. Say plainly if something was not run. -->

- [ ] `make check`
- [ ] `make test-e2e` (if UI changed)

## Review checklist

See [docs/architecture/review-checklist.md](../docs/architecture/review-checklist.md). In particular:

- [ ] No client-supplied identity, role, XP, score or completion is trusted
- [ ] Lab data queries enforce ownership in SQL
- [ ] No user text reaches a shell, interpreter or `dangerouslySetInnerHTML`
- [ ] API/WS/content contract changes are in `docs/api` or `packages/content-schema` first
- [ ] Logs contain no secrets, YAML or Secret values
- [ ] Docs updated
