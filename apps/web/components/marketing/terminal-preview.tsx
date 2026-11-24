"use client";

import { RotateCcw } from "lucide-react";
import { motion, useInView } from "motion/react";
import { useCallback, useEffect, useRef, useState } from "react";
import { useCalmMotion } from "@/components/marketing/motion";
import { cn } from "@/lib/utils";

type Line = { kind: "prompt" | "out" | "warn" | "note"; text: string };

// A read-only troubleshooting session. Every command here works in today's simulator.
const session: Line[] = [
  { kind: "prompt", text: "kubectl get pods" },
  { kind: "out", text: "NAME                     READY   STATUS             RESTARTS   AGE" },
  { kind: "out", text: "nginx-4fzxcz7snh-tbjxs   1/1     Running            0          60m" },
  { kind: "warn", text: "typo                     0/1     ImagePullBackOff   0          5m" },
  { kind: "prompt", text: "kubectl events --types=Warning --for pod/typo" },
  { kind: "out", text: "LAST SEEN   TYPE      REASON   OBJECT     MESSAGE" },
  { kind: "out", text: '5m          Warning   Failed   pod/typo   Failed to pull image "ngnix:1.27": not found' },
  { kind: "note", text: "ngnix, not nginx. Spotting that is where the troubleshooting course begins." },
];

const DONE = { line: session.length, chars: 0 };
const TYPE_MS = 34; // per character of a typed command
const OUTPUT_MS = 110; // per line of output
const ENTER_MS = 420; // pause after a command, before its output

/**
 * A picture of a lab, styled like the real one (apps/web/components/lab), that types out its
 * session once it scrolls into view. It is not an interactive terminal, and the caption says so.
 * Screen readers get the whole session at once; the typed copy is hidden from them.
 */
export function TerminalPreview() {
  const calm = useCalmMotion();
  const ref = useRef<HTMLDivElement>(null);
  const inView = useInView(ref, { once: true, amount: 0.4 });
  // Which line is being written, and how many characters of it are visible.
  const [typed, setPos] = useState({ line: 0, chars: 0 });
  const [run, setRun] = useState(0);
  // With reduced motion, the whole session is shown at once.
  const pos = calm ? DONE : typed;
  const playing = !calm && inView && pos.line < session.length;

  useEffect(() => {
    if (!playing) return;
    const current = session[pos.line]!;
    let delay: number;
    let next: { line: number; chars: number };
    if (current.kind === "prompt" && pos.chars < current.text.length) {
      delay = pos.chars === 0 ? 500 : TYPE_MS + (pos.chars % 7 === 0 ? 40 : 0);
      next = { line: pos.line, chars: pos.chars + 1 };
    } else {
      delay = current.kind === "prompt" ? ENTER_MS : current.kind === "note" ? 600 : OUTPUT_MS;
      next = { line: pos.line + 1, chars: 0 };
    }
    const t = window.setTimeout(() => setPos(next), delay);
    return () => window.clearTimeout(t);
  }, [playing, pos, run]);

  const replay = useCallback(() => {
    setPos({ line: 0, chars: 0 });
    setRun((r) => r + 1);
  }, []);

  const finished = pos.line >= session.length;

  return (
    <figure className="relative rounded-2xl border border-border-strong/70 bg-card/80 p-3 shadow-2xl shadow-shade/40 backdrop-blur-xl">
      {/* A hairline of light along the top edge. */}
      <span
        aria-hidden
        className="pointer-events-none absolute inset-x-10 -top-px h-px bg-gradient-to-r from-transparent via-highlight/70 to-transparent"
      />
      <div className="flex items-center gap-3 px-1 pb-3">
        <span aria-hidden className="flex gap-1.5">
          <span className="size-2.5 rounded-full bg-[#f06a78]/80" />
          <span className="size-2.5 rounded-full bg-[#f5c451]/80" />
          <span className="size-2.5 rounded-full bg-[#4fd18b]/80" />
        </span>
        <span className="text-small font-medium text-foreground-secondary">Image pull failure</span>
        <span className="ml-auto hidden font-mono text-caption text-muted-foreground sm:inline">
          troubleshooting/image-pull-failure
        </span>
        <button
          type="button"
          onClick={replay}
          disabled={!finished || calm}
          aria-label="Replay the example session"
          className={cn(
            "-my-1 grid size-7 place-items-center rounded-md text-muted-foreground transition-[color,background-color,opacity] duration-150 hover:bg-hover hover:text-foreground disabled:pointer-events-none",
            finished && !calm ? "opacity-100" : "opacity-0",
          )}
        >
          <RotateCcw aria-hidden className="size-3.5" />
        </button>
      </div>
      <div ref={ref} className="overflow-hidden rounded-lg border border-white/10 bg-terminal text-terminal-foreground">
        <div className="flex items-center gap-4 border-b border-white/10 px-4 text-caption">
          <span className="relative py-2 font-medium text-white">
            Terminal
            <motion.span
              aria-hidden
              className="absolute inset-x-0 -bottom-px h-0.5 origin-left bg-[#6c8cff]"
              initial={{ scaleX: 0 }}
              animate={{ scaleX: inView || calm ? 1 : 0 }}
              transition={{ duration: calm ? 0 : 0.5, ease: [0.22, 1, 0.36, 1] }}
            />
          </span>
          <span className="py-2 text-terminal-muted">Cluster</span>
          <span className="py-2 text-terminal-muted">Events</span>
          <span className="ml-auto flex items-center gap-1.5 py-2 text-terminal-muted">
            <span aria-hidden className="relative flex size-1.5">
              <span className="absolute inline-flex size-full animate-ping rounded-full bg-[#4fd18b] opacity-60" />
              <span className="relative inline-flex size-1.5 rounded-full bg-[#4fd18b]" />
            </span>
            simulated
          </span>
        </div>
        <pre
          tabIndex={0}
          role="group"
          aria-label="Example terminal session"
          className="overflow-x-auto p-4 font-mono text-[12px] leading-relaxed lg:text-[13px] lg:whitespace-pre-wrap 2xl:p-5 2xl:text-[14px]"
        >
          {/* The full session, for assistive technology. */}
          <span className="sr-only">
            {session.map((l) => (l.kind === "prompt" ? `$ ${l.text}` : l.text)).join("\n")}
          </span>
          {/* Every line is laid out from the start, so typing never shifts the page. */}
          <code aria-hidden>
            {session.map((line, i) => {
              const state = i < pos.line ? "done" : i === pos.line ? "active" : "pending";
              const pending = state === "pending" || (state === "active" && line.kind !== "prompt");
              const visibleText =
                state === "active" && line.kind === "prompt" ? line.text.slice(0, pos.chars) : line.text;
              const hiddenText = state === "active" && line.kind === "prompt" ? line.text.slice(pos.chars) : "";
              const cursor = state === "active" && line.kind === "prompt";
              const body = (
                <>
                  {pending ? (
                    <span data-pending className="invisible">
                      {line.text}
                    </span>
                  ) : (
                    visibleText
                  )}
                  {cursor ? <Cursor /> : null}
                  {hiddenText ? (
                    <span data-pending className="invisible">
                      {hiddenText}
                    </span>
                  ) : null}
                </>
              );
              if (line.kind === "prompt")
                return (
                  <span key={i} className="mt-2 block first:mt-0">
                    <span
                      className={cn("text-terminal-prompt", state === "pending" && "invisible")}
                      data-pending={state === "pending" || undefined}
                    >
                      ${" "}
                    </span>
                    {body}
                  </span>
                );
              if (line.kind === "note")
                return (
                  <motion.span
                    key={i}
                    className="mt-3 block border-l-2 border-[#f5c451] pl-3 font-sans text-[#f5d58a]"
                    initial={false}
                    animate={pending ? { opacity: 0, x: -6 } : { opacity: 1, x: 0 }}
                    transition={{ duration: calm ? 0 : 0.45, ease: [0.22, 1, 0.36, 1] }}
                    data-pending={pending || undefined}
                  >
                    {line.text}
                  </motion.span>
                );
              return (
                <span
                  key={i}
                  className={cn(
                    "block",
                    line.kind === "warn" && "text-terminal-error",
                    line.kind === "warn" && !pending && !calm && "animate-[kd-flash_1.4s_ease-out_1]",
                  )}
                >
                  {body}
                </span>
              );
            })}
            {finished ? (
              <span className="mt-2 block">
                <span className="text-terminal-prompt">$ </span>
                <Cursor />
              </span>
            ) : null}
          </code>
        </pre>
      </div>
      <figcaption className="px-1 pt-3 text-caption text-muted-foreground">
        A lab, pictured. The cluster is simulated: nothing runs, nothing can break.
      </figcaption>
    </figure>
  );
}

function Cursor() {
  return (
    <span className="ml-px inline-block h-[1.1em] w-[0.55em] translate-y-[0.18em] animate-[kd-blink_1.1s_steps(1)_infinite] bg-terminal-prompt/90" />
  );
}
