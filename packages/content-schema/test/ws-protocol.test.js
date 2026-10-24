// Checks docs/api/ws-protocol.schema.json accepts the documented frames and rejects malformed ones.
import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import Ajv2020 from "ajv/dist/2020.js";
import addFormats from "ajv-formats";

const schemaPath = new URL("../../../docs/api/ws-protocol.schema.json", import.meta.url);
const ajv = new Ajv2020({ strict: true, strictTypes: false, allErrors: true });
addFormats(ajv);
const validate = ajv.compile(JSON.parse(readFileSync(schemaPath, "utf8")));

const id = "0b6f4c1e-8f53-4f3c-9a51-2d7e3c1a9b10";
const ticket = "a".repeat(43);

test("documented frames are valid", () => {
  const frames = [
    { type: "auth", protocolVersion: 1, ticket },
    { type: "auth.ok", protocolVersion: 1, sessionId: id, snapshotVersion: 42, heartbeatIntervalMs: 25000 },
    { type: "command.run", requestId: id, input: "kubectl get pods" },
    {
      type: "command.result",
      requestId: id,
      sequence: 7,
      stdout: "...",
      stderr: "",
      exitCode: 0,
      errorCode: null,
      mutated: false,
      snapshotVersion: 43,
    },
    {
      type: "command.result",
      requestId: id,
      sequence: 8,
      stdout: "",
      stderr: "Pipes are not supported in the simulator",
      exitCode: 1,
      errorCode: "UNSUPPORTED_COMMAND",
      mutated: false,
      snapshotVersion: 43,
      deduplicated: true,
    },
    { type: "state.changed", snapshotVersion: 43, eventCount: 3 },
    { type: "heartbeat", timestamp: "2026-10-22T10:00:00Z" },
    { type: "heartbeat" },
    { type: "error", code: "RATE_LIMITED", message: "Slow down", requestId: id, retryAfterMs: 1000 },
  ];
  for (const frame of frames)
    assert.ok(validate(frame), `${JSON.stringify(frame)}: ${JSON.stringify(validate.errors)}`);
});

test("malformed frames are rejected", () => {
  const frames = [
    { type: "auth", ticket },
    { type: "auth", protocolVersion: 2, ticket },
    { type: "command.run", requestId: "not-a-uuid", input: "kubectl get pods" },
    { type: "command.run", requestId: id, input: "" },
    { type: "command.run", requestId: id, input: "x".repeat(16385) },
    { type: "command.run", requestId: id, input: "kubectl get pods", userId: "someone-else" },
    { type: "state.changed", snapshotVersion: 0, eventCount: 1 },
    { type: "error", code: "NOT_A_CODE", message: "x" },
    { type: "unknown" },
  ];
  for (const frame of frames)
    assert.equal(validate(frame), false, `${JSON.stringify(frame).slice(0, 80)} should be rejected`);
});
