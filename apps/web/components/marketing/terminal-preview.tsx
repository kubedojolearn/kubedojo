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

/**
 * A static picture of a lab, styled like the real one (apps/web/components/lab). It is not an
 * interactive terminal, and the caption says so.
 */
export function TerminalPreview() {
  return (
    <figure className="rounded-xl border bg-card p-3 shadow-2xl shadow-shade/40">
      <div className="flex items-center gap-3 px-1 pb-3">
        <span aria-hidden className="flex gap-1.5">
          <span className="size-2.5 rounded-full bg-faint/50" />
          <span className="size-2.5 rounded-full bg-faint/50" />
          <span className="size-2.5 rounded-full bg-faint/50" />
        </span>
        <span className="text-small font-medium text-foreground-secondary">Image pull failure</span>
        <span className="ml-auto hidden font-mono text-caption text-muted-foreground sm:inline">
          troubleshooting/image-pull-failure
        </span>
      </div>
      <div className="overflow-hidden rounded-lg border border-white/10 bg-terminal text-terminal-foreground">
        <div className="flex items-center gap-4 border-b border-white/10 px-4 text-caption">
          <span className="border-b-2 border-[#6c8cff] py-2 font-medium text-white">Terminal</span>
          <span className="py-2 text-terminal-muted">Cluster</span>
          <span className="py-2 text-terminal-muted">Events</span>
        </div>
        <pre
          tabIndex={0}
          role="group"
          aria-label="Example terminal session"
          className="overflow-x-auto p-4 font-mono text-[12px] leading-relaxed lg:text-[12.5px] lg:whitespace-pre-wrap"
        >
          <code>
            {session.map((line, i) => {
              if (line.kind === "prompt")
                return (
                  <span key={i} className="mt-2 block first:mt-0">
                    <span className="text-terminal-prompt">$ </span>
                    {line.text}
                  </span>
                );
              if (line.kind === "note")
                return (
                  <span key={i} className="mt-3 block border-l-2 border-[#f5c451] pl-3 font-sans text-[#f5d58a]">
                    {line.text}
                  </span>
                );
              return (
                <span key={i} className={line.kind === "warn" ? "block text-terminal-error" : "block"}>
                  {line.text}
                </span>
              );
            })}
          </code>
        </pre>
      </div>
      <figcaption className="px-1 pt-3 text-caption text-muted-foreground">
        A lab, pictured. The cluster is simulated: nothing runs, nothing can break.
      </figcaption>
    </figure>
  );
}
