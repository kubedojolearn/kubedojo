"use client";

import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { Download } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { bff } from "@/lib/bff-client";
import { cn } from "@/lib/utils";

export interface TerminalEntry {
  key: string;
  input: string;
  stdout: string;
  stderr: string;
  exitCode: number;
  pending?: boolean;
}

interface CommandResult {
  requestId: string;
  sequence: number;
  input: string;
  stdout: string;
  stderr: string;
  exitCode: number;
}

const MAX_INPUT = 16384;

/** One command and its output, as a shell would show it. */
function Entry({ entry }: { entry: TerminalEntry }) {
  return (
    <div className="whitespace-pre-wrap break-words">
      <div>
        <span aria-hidden className="select-none text-terminal-prompt">
          ${" "}
        </span>
        <span className="sr-only">Command: </span>
        {entry.input}
      </div>
      {entry.pending ? (
        <div className="text-terminal-muted">Running…</div>
      ) : (
        <>
          {entry.stdout && <div>{entry.stdout.replace(/\n$/, "")}</div>}
          {entry.stderr && <div className="text-terminal-error">{entry.stderr.replace(/\n$/, "")}</div>}
        </>
      )}
    </div>
  );
}

/**
 * The lab terminal. Commands go to the API through the BFF; each has a client-generated request
 * ID, so a retry after a network error never runs a command twice. `clear` is handled here.
 */
export function Terminal({ labId, active, initial }: { labId: string; active: boolean; initial: TerminalEntry[] }) {
  const [entries, setEntries] = useState<TerminalEntry[]>(initial);
  const [value, setValue] = useState("");
  const [busy, setBusy] = useState(false);
  const [historyIndex, setHistoryIndex] = useState<number | null>(null);
  const history = useRef<string[]>(initial.map((e) => e.input));
  const scroller = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);

  useEffect(() => {
    scroller.current?.scrollTo({ top: scroller.current.scrollHeight });
  }, [entries]);

  async function run(command: string) {
    const key = crypto.randomUUID();
    setEntries((es) => [...es, { key, input: command, stdout: "", stderr: "", exitCode: 0, pending: true }]);
    let res = await bff<CommandResult>(`labs/${labId}/commands`, {
      method: "POST",
      body: { requestId: key, input: command },
    });
    // A network failure may hide a command that ran: retrying with the same ID is safe.
    if (!res.ok && res.status === 0) {
      res = await bff<CommandResult>(`labs/${labId}/commands`, {
        method: "POST",
        body: { requestId: key, input: command },
      });
    }
    setEntries((es) =>
      es.map((e) =>
        e.key !== key
          ? e
          : res.ok
            ? { ...e, pending: false, stdout: res.data.stdout, stderr: res.data.stderr, exitCode: res.data.exitCode }
            : { ...e, pending: false, stderr: res.message, exitCode: 1 },
      ),
    );
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    const command = value.trim();
    if (!command || busy) return;
    setValue("");
    setHistoryIndex(null);
    history.current = [...history.current, command];
    if (command === "clear") {
      setEntries([]);
      return;
    }
    setBusy(true);
    try {
      await run(command);
    } finally {
      setBusy(false);
      input.current?.focus();
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    const h = history.current;
    if (e.key === "ArrowUp" && h.length > 0) {
      e.preventDefault();
      const i = historyIndex === null ? h.length - 1 : Math.max(0, historyIndex - 1);
      setHistoryIndex(i);
      setValue(h[i] ?? "");
    } else if (e.key === "ArrowDown" && historyIndex !== null) {
      e.preventDefault();
      const i = historyIndex + 1;
      if (i >= h.length) {
        setHistoryIndex(null);
        setValue("");
      } else {
        setHistoryIndex(i);
        setValue(h[i] ?? "");
      }
    } else if (e.key === "l" && e.ctrlKey) {
      e.preventDefault();
      setEntries([]);
    }
  }

  async function downloadTranscript() {
    // Built from the stored history, where Secret values are redacted.
    const all: CommandResult[] = [];
    let cursor: string | null = null;
    for (let page = 0; page < 10; page++) {
      const res: Response = await fetch(
        `/api/bff/v1/labs/${labId}/commands?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
      ).catch(() => new Response(null, { status: 0 }));
      if (!res.ok) {
        toast.error("Couldn't download the transcript", { description: "Please try again." });
        return;
      }
      const body = (await res.json()) as { data: CommandResult[]; meta: { nextCursor: string | null } };
      all.push(...body.data);
      cursor = body.meta.nextCursor;
      if (!cursor) break;
    }
    const text = all
      .reverse()
      .map((c) => `$ ${c.input}\n${c.stdout}${c.stderr}`)
      .join("\n");
    const header = `# KubeDojo lab ${labId}: simulated cluster transcript (Secret values redacted)\n\n`;
    const url = URL.createObjectURL(new Blob([header + text], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `kubedojo-lab-${labId.slice(0, 8)}-transcript.txt`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
        <p>
          Simulated cluster: output is generated by KubeDojo. Read commands work now; changing the cluster arrives next.
          Type <kbd className="font-mono">help</kbd> to see what&apos;s available.
        </p>
        <Button variant="ghost" size="sm" onClick={downloadTranscript}>
          <Download /> Transcript
        </Button>
      </div>
      <div
        ref={scroller}
        role="log"
        aria-label="Terminal output"
        aria-live="polite"
        tabIndex={0}
        className="h-[min(60svh,32rem)] overflow-y-auto rounded-lg bg-terminal p-3 font-mono text-[13px] leading-relaxed text-terminal-foreground"
      >
        {entries.length === 0 && <p className="text-terminal-muted">Try: kubectl get pods</p>}
        <div className="space-y-2">
          {entries.map((e) => (
            <Entry key={e.key} entry={e} />
          ))}
        </div>
      </div>
      <form onSubmit={submit} className="flex items-center gap-2 rounded-lg bg-terminal px-3 py-2 font-mono text-sm">
        <span aria-hidden className="text-terminal-prompt">
          $
        </span>
        <input
          ref={input}
          aria-label="Command"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          maxLength={MAX_INPUT}
          disabled={!active}
          placeholder={active ? "kubectl get pods" : "This lab has ended"}
          autoComplete="off"
          autoCapitalize="off"
          spellCheck={false}
          className={cn(
            "min-w-0 flex-1 bg-transparent text-terminal-foreground outline-none placeholder:text-terminal-muted",
            "focus-visible:ring-0",
          )}
        />
        <Button type="submit" size="sm" variant="secondary" disabled={!active || busy || !value.trim()}>
          Run
        </Button>
      </form>
    </div>
  );
}
