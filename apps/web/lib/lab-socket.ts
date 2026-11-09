/**
 * The lab WebSocket client (docs/api/websocket.md, protocol v1). The socket is a fast path:
 * commands still run at most once per requestId, and everything it says can be recovered over
 * REST, so the terminal falls back to HTTP whenever the socket isn't open.
 */

export interface SocketCommandResult {
  requestId: string;
  sequence: number;
  stdout: string;
  stderr: string;
  exitCode: number;
  errorCode: string | null;
  mutated: boolean;
  snapshotVersion: number;
  deduplicated?: boolean;
}

export type SocketStatus = "connecting" | "open" | "reconnecting" | "offline" | "ended";

interface Ticket {
  ticket: string;
  url: string;
  protocolVersion: number;
}

interface Options {
  labId: string;
  version: number;
  getTicket: () => Promise<Ticket | null>;
  onStatus: (s: SocketStatus) => void;
  /** A newer lab version exists (another tab, or a REST call): refetch the snapshot. */
  onVersion: (version: number) => void;
  /** Overridable for tests. */
  createSocket?: (url: string) => WebSocket;
  random?: () => number;
}

/** Reconnect delay: 0.5 s, 1 s, 2 s … capped at 30 s, ±20% jitter (websocket.md). */
export function backoff(attempt: number, random: () => number = Math.random): number {
  const base = Math.min(500 * 2 ** attempt, 30_000);
  return Math.round(base * (0.8 + 0.4 * random()));
}

// Close codes after which reconnecting can't help.
const FINAL = new Set([4409, 4410]);

export class LabSocket {
  private ws: WebSocket | null = null;
  private attempt = 0;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private stopped = false;
  private authed = false;
  private version: number;
  private pending = new Map<string, { resolve: (r: SocketCommandResult) => void; reject: (e: Error) => void }>();

  constructor(private readonly o: Options) {
    this.version = o.version;
  }

  start() {
    void this.connect();
  }

  stop() {
    this.stopped = true;
    if (this.timer) clearTimeout(this.timer);
    this.ws?.close(1000);
    this.failPending("closed");
  }

  get open(): boolean {
    return this.ws?.readyState === WebSocket.OPEN && this.authed;
  }

  /** Runs a command over the socket. Rejects if the socket closes first (the caller then asks REST, safely, with the same requestId). */
  run(requestId: string, input: string): Promise<SocketCommandResult> {
    return new Promise((resolve, reject) => {
      if (!this.open || !this.ws) {
        reject(new Error("socket not open"));
        return;
      }
      this.pending.set(requestId, { resolve, reject });
      this.ws.send(JSON.stringify({ type: "command.run", requestId, input }));
    });
  }

  private failPending(reason: string) {
    for (const p of this.pending.values()) p.reject(new Error(reason));
    this.pending.clear();
  }

  private seen(version: number) {
    if (version > this.version) {
      this.version = version;
      this.o.onVersion(version);
    }
  }

  private async connect() {
    if (this.stopped) return;
    this.o.onStatus(this.attempt === 0 ? "connecting" : "reconnecting");
    const ticket = await this.o.getTicket().catch(() => null);
    if (this.stopped) return;
    if (!ticket) return this.retry();
    const url = `${location.protocol === "https:" ? "wss:" : "ws:"}//${location.host}${ticket.url}`;
    let ws: WebSocket;
    try {
      ws = this.o.createSocket ? this.o.createSocket(url) : new WebSocket(url);
    } catch {
      return this.retry();
    }
    this.ws = ws;
    this.authed = false;
    ws.onopen = () => ws.send(JSON.stringify({ type: "auth", protocolVersion: 1, ticket: ticket.ticket }));
    ws.onmessage = (ev) => {
      let m: Record<string, unknown>;
      try {
        m = JSON.parse(String(ev.data)) as Record<string, unknown>;
      } catch {
        return;
      }
      switch (m.type) {
        case "auth.ok":
          this.authed = true;
          this.attempt = 0;
          this.o.onStatus("open");
          // Always resynchronise after (re)connecting: something may have changed meanwhile.
          this.seen(Number(m.snapshotVersion));
          break;
        case "command.result": {
          const r = m as unknown as SocketCommandResult;
          this.pending.get(r.requestId)?.resolve(r);
          this.pending.delete(r.requestId);
          // The terminal refreshes after its own changes; only remember the version.
          if (r.mutated) this.version = Math.max(this.version, r.snapshotVersion);
          else this.seen(r.snapshotVersion);
          break;
        }
        case "state.changed":
          this.seen(Number(m.snapshotVersion));
          break;
        case "error": {
          const id = typeof m.requestId === "string" ? m.requestId : null;
          if (id && this.pending.has(id)) {
            this.pending.get(id)!.reject(new Error(String(m.message ?? m.code)));
            this.pending.delete(id);
          }
          break;
        }
      }
    };
    ws.onclose = (ev) => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.authed = false;
      this.failPending("closed");
      if (this.stopped) return;
      if (FINAL.has(ev.code)) {
        this.o.onStatus("ended");
        return;
      }
      this.retry();
    };
  }

  private retry() {
    if (this.stopped) return;
    // After a few failures, say so: the terminal keeps working over HTTP meanwhile.
    this.o.onStatus(this.attempt >= 3 ? "offline" : "reconnecting");
    const delay = backoff(this.attempt, this.o.random);
    this.attempt++;
    this.timer = setTimeout(() => void this.connect(), delay);
  }
}
