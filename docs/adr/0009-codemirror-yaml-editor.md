# ADR-0009: CodeMirror 6 for the lab's YAML editor

- **Status:** Accepted · **Date:** 2026-11-08
- Amends the master plan's technology table, which named Monaco.

## Context

Part 13 adds a YAML editor to every lab: learners write manifests there and run `kubectl apply -f`. The plan named Monaco (the VS Code editor). Monaco is several megabytes, loads its language services in web workers (by default from a CDN), has weak support for touch screens, and its screen-reader mode has to be switched on. The lab page must work on phones and meet WCAG 2.2 AA.

## Decision

Use **CodeMirror 6** with its YAML grammar and lint packages (`@codemirror/*`, bundled; no CDN or workers).

- Syntax highlighting, line numbers, undo history and bracket matching; Tab indents, and Escape then Tab moves focus on, which the editor states under the editor.
- YAML syntax errors are found in the browser with the `yaml` package and shown as lint diagnostics. The API parses and validates again on `kubectl apply`; the browser check is only a convenience.
- The editor is dark in both app themes, using the terminal palette, so it reads as part of the lab rather than the page.

## Consequences

- About 150 KB (gzip) on the lab page only; nothing loads from third-party hosts.
- No Kubernetes schema completion in V1. Field errors come from the simulator when the learner applies, worded like the API server's.

## Alternatives considered

- **Monaco:** richer, but heavy, worker- and CDN-dependent, and weaker on mobile and screen readers.
- **A plain textarea:** fully accessible and dependency-free, but no highlighting or line numbers, which matter when YAML indentation is the lesson.
