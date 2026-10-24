# Lab WebSocket protocol — version 1

- **Endpoint:** `wss://<web origin>/api/v1/ws/labs/{labId}` (routed by the edge proxy to Go)
- **Schema:** [ws-protocol.schema.json](ws-protocol.schema.json) (JSON Schema 2020-12)
- **Decision record:** [ADR-0003](../adr/0003-auth-bff.md), [ADR-0004](../adr/0004-pg-state-redis-events.md)

The socket is a fast path for running commands and hearing about state changes. It is never the source of truth: everything it says can be recovered from REST (`GET /snapshot`, `GET /commands`).

## Connection

1. Get a ticket: `POST /api/bff/labs/{labId}/ws-ticket` → `{ticket, expiresAt, url, protocolVersion: 1}`. Tickets are single-use and expire after 30 s.
2. Open `wss://<origin>{url}`. The server rejects the upgrade with `403` if `Origin` is not in `WS_ALLOWED_ORIGINS`, and `429` if the IP exceeds 30 upgrades per minute.
3. Within **3 seconds**, send:

   ```json
   {"type":"auth","protocolVersion":1,"ticket":"…"}
   ```

4. Server redeems the ticket atomically, checks it was issued for this user and this `labId`, and replies:

   ```json
   {"type":"auth.ok","protocolVersion":1,"sessionId":"…","snapshotVersion":42,"heartbeatIntervalMs":25000}
   ```

5. Client fetches `GET /api/bff/labs/{labId}/snapshot` (always, after every successful auth).

Any other first frame, a late auth, or an invalid/used/expired ticket → `{"type":"error","code":"UNAUTHENTICATED","fatal":true,…}` then close code **4401**.

## Messages

| Direction | `type` | Purpose |
| --- | --- | --- |
| C → S | `auth` | First frame only |
| C → S | `command.run` | Run one terminal input. `requestId` is a client UUID. |
| C → S | `heartbeat` | Optional; any frame counts as liveness |
| S → C | `auth.ok` | Authenticated; current version |
| S → C | `command.result` | Output of one command. Always sent for a `command.run` that was accepted for execution, including unsupported or invalid input (non-zero `exitCode`, `errorCode` set). |
| S → C | `state.changed` | Lab version moved (from this or another connection, or a REST call). Notification only. |
| S → C | `heartbeat` | Every `heartbeatIntervalMs` |
| S → C | `error` | Protocol-level problem: malformed frame, rate limit, session no longer active, auth failure |

### Why unsupported commands are `command.result`, not `error`

The master plan's example sent `{"type":"error","code":"UNSUPPORTED_COMMAND"}` for a piped command. Version 1 instead returns a `command.result` with `exitCode: 1`, the explanation on `stderr` and `errorCode: "UNSUPPORTED_COMMAND"`. That keeps every terminal input tied to its `requestId`, recorded in command history with a sequence number, and rendered by the same code path as kubectl's own errors. `error` frames are reserved for problems with the connection or the frame itself.

## Rules

- **At most once per `requestId`.** Execution is deduplicated in PostgreSQL (`UNIQUE (session_id, request_id)`). Re-sending a `requestId` returns the stored result with `"deduplicated": true`.
- **No automatic replay.** After a reconnect the client never re-sends commands on its own. A command whose result never arrived is shown as "result unknown"; the learner (or the UI, on request) may re-send the *same* `requestId` to fetch the stored result safely.
- **Version gaps.** The client tracks the highest `snapshotVersion` it has rendered. On `state.changed` or `command.result` with a higher version, it refetches the snapshot. Out-of-order notifications with a lower version are ignored.
- **Commands are serialized per lab** on the server (row lock). A client may pipeline commands; results arrive in execution order.
- **Rate limit:** 100 commands per minute per user. Excess frames get `{"type":"error","code":"RATE_LIMITED","requestId":"…","retryAfterMs":…}` and are not executed.
- **Limits:** frame ≤ 32 KiB (`input` ≤ 16 KiB); at most 3 open sockets per user per lab (the oldest is closed with 4409).
- **Session state:** if the lab leaves the `active` state (completed, abandoned or expired; a reset keeps it active), the server sends `error` with `SESSION_EXPIRED` or `CONFLICT`, `fatal: true`, and closes with 4410.
- **Liveness:** the server closes connections with no frames for 2 × `heartbeatIntervalMs` (close 4408).

## Close codes

| Code | Meaning | Client action |
| --- | --- | --- |
| 1000 | Normal | — |
| 1001 | Server shutting down | Reconnect with backoff |
| 1009 | Frame too large | Fix client; do not retry the same frame |
| 4400 | Protocol error (malformed JSON, unknown `type`, schema violation) | Report bug; reconnect |
| 4401 | Not authenticated | Get a new ticket and reconnect |
| 4408 | Idle timeout | Reconnect |
| 4409 | Replaced by a newer connection | Do not reconnect from this tab automatically |
| 4410 | Lab no longer active | Show lab ended state; do not reconnect |
| 4429 | Rate limited at connect | Reconnect after backoff |

Reconnect backoff: 0.5 s, 1 s, 2 s, 4 s … capped at 30 s, with ±20% jitter, and a fresh ticket each time.

## Versioning

`protocolVersion` is in `auth` and `auth.ok`. A server that doesn't support the client's version replies `error` `PROTOCOL_ERROR` (fatal) and closes with 4400. Adding optional fields is backward compatible and doesn't bump the version; removing or changing fields does.
