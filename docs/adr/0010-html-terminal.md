# ADR-0010: The lab terminal is HTML, not xterm.js

- **Status:** Accepted · **Date:** 2026-11-08
- Amends the master plan's technology table, which named xterm.js.

## Context

The plan chose xterm.js for the lab terminal. xterm.js emulates a real terminal: it renders a character grid (canvas or DOM rows), handles escape sequences and keystrokes itself, and exposes text to screen readers only through an optional accessibility mode that mirrors the buffer into a hidden element. KubeDojo's terminal has no shell, no TTY and no programs that draw with escape sequences; every command is one line in, one result out. The V1 promise is WCAG 2.2 AA, including the terminal.

## Decision

Keep the terminal as **plain HTML**: a labelled text input, and a `role="log"` region where each command and its output is a block of text (`aria-live="polite"`).

- Screen readers read new output as it arrives, and every past command is text that can be found, selected and copied.
- History (↑/↓), `clear` and Ctrl+L work as in a shell. Colors come from the terminal palette in `lib/design-tokens.ts`.
- Part 14 adds the live connection (the lab WebSocket, protocol v1) behind the same UI, with HTTP as the fallback.

## Consequences

- No terminal-emulator features (cursor addressing, full-screen programs), which the simulator has no use for.
- A smaller page: no emulator bundle or web fonts for its grid.

## Alternatives considered

- **xterm.js with its screen-reader mode:** closer to a real terminal, but its accessibility depends on a mode learners must know to turn on, and it adds weight for features the simulator can't use.
