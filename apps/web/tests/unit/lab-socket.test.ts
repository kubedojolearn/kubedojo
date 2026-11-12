import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { backoff, LabSocket, type SocketStatus } from "@/lib/lab-socket";

class FakeSocket {
  static last: FakeSocket | null = null;
  readyState = 0;
  sent: Record<string, unknown>[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: ((ev: { code: number }) => void) | null = null;
  constructor(public url: string) {
    FakeSocket.last = this;
  }
  send(data: string) {
    this.sent.push(JSON.parse(data) as Record<string, unknown>);
  }
  close() {}
  // Test helpers.
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  receive(m: object) {
    this.onmessage?.({ data: JSON.stringify(m) });
  }
  drop(code: number) {
    this.readyState = 3;
    this.onclose?.({ code });
  }
}

function setup(version = 5) {
  const statuses: SocketStatus[] = [];
  const versions: number[] = [];
  const s = new LabSocket({
    labId: "lab",
    version,
    getTicket: async () => ({ ticket: "t".repeat(43), url: "/api/v1/ws/labs/lab", protocolVersion: 1 }),
    onStatus: (st) => statuses.push(st),
    onVersion: (v) => versions.push(v),
    createSocket: (url) => new FakeSocket(url) as unknown as WebSocket,
    random: () => 0.5,
  });
  return { s, statuses, versions };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe("LabSocket", () => {
  beforeEach(() => {
    vi.stubGlobal("WebSocket", { OPEN: 1 });
  });
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("authenticates, runs commands and reports newer versions", async () => {
    const { s, statuses, versions } = setup();
    s.start();
    await flush();
    const ws = FakeSocket.last!;
    expect(ws.url).toMatch(/^ws:\/\/.*\/api\/v1\/ws\/labs\/lab$/);
    ws.open();
    expect(ws.sent[0]).toEqual({ type: "auth", protocolVersion: 1, ticket: "t".repeat(43) });
    ws.receive({
      type: "auth.ok",
      protocolVersion: 1,
      sessionId: "lab",
      snapshotVersion: 6,
      heartbeatIntervalMs: 25000,
    });
    expect(statuses.at(-1)).toBe("open");
    expect(versions).toEqual([6]); // something changed while connecting

    const result = s.run("r1", "kubectl get pods");
    expect(ws.sent[1]).toEqual({ type: "command.run", requestId: "r1", input: "kubectl get pods" });
    ws.receive({
      type: "command.result",
      requestId: "r1",
      sequence: 1,
      stdout: "ok\n",
      stderr: "",
      exitCode: 0,
      errorCode: null,
      mutated: false,
      snapshotVersion: 6,
    });
    await expect(result).resolves.toMatchObject({ stdout: "ok\n" });

    ws.receive({ type: "state.changed", snapshotVersion: 8, eventCount: 1 });
    ws.receive({ type: "state.changed", snapshotVersion: 7, eventCount: 1 }); // out of order: ignored
    expect(versions).toEqual([6, 8]);
  });

  it("rejects pending commands when the socket drops, then reconnects with backoff", async () => {
    vi.useFakeTimers();
    const { s, statuses } = setup();
    s.start();
    await vi.advanceTimersByTimeAsync(0);
    const first = FakeSocket.last!;
    first.open();
    first.receive({
      type: "auth.ok",
      protocolVersion: 1,
      sessionId: "lab",
      snapshotVersion: 5,
      heartbeatIntervalMs: 25000,
    });
    const pending = s.run("r1", "kubectl get pods");
    first.drop(1006);
    await expect(pending).rejects.toThrow("closed");
    expect(statuses.at(-1)).toBe("reconnecting");
    await vi.advanceTimersByTimeAsync(backoff(0, () => 0.5));
    expect(FakeSocket.last).not.toBe(first);
    s.stop();
  });

  it("stops for good when the lab ends or another tab takes over", async () => {
    const { s, statuses } = setup();
    s.start();
    await flush();
    FakeSocket.last!.open();
    FakeSocket.last!.drop(4410);
    expect(statuses.at(-1)).toBe("ended");
    await expect(s.run("r", "x")).rejects.toThrow();
  });

  it("backs off exponentially with jitter, capped at 30 s", () => {
    expect(backoff(0, () => 0.5)).toBe(500);
    expect(backoff(1, () => 0.5)).toBe(1000);
    expect(backoff(3, () => 0)).toBe(3200);
    expect(backoff(20, () => 1)).toBe(36000);
    expect(backoff(20, () => 0.5)).toBe(30000);
  });
});
