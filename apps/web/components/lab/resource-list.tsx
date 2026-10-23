import { groupResources, type SimResource, type Tone } from "@/lib/lab-format";
import { cn } from "@/lib/utils";

const toneClass: Record<Tone, string> = {
  ok: "text-success",
  warn: "text-warning",
  error: "text-danger",
  neutral: "text-muted-foreground",
};

/** The cluster as a list: every simulated resource grouped by kind, with a one-line status. */
export function ResourceList({ resources }: { resources: SimResource[] }) {
  const groups = groupResources(resources);
  return (
    <div className="space-y-5">
      {groups.map((g) => (
        <section key={g.kind} aria-labelledby={`kind-${g.kind}`}>
          <h3 id={`kind-${g.kind}`} className="text-sm font-semibold">
            {g.kind} <span className="font-normal text-muted-foreground">({g.rows.length})</span>
          </h3>
          <ul className="mt-2 divide-y rounded-lg border">
            {g.rows.map((r) => (
              <li key={r.key} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 px-3 py-2 text-sm">
                <span className="min-w-0 font-mono break-all">
                  {r.namespace && <span className="text-muted-foreground">{r.namespace}/</span>}
                  {r.name}
                </span>
                <span className={cn("ml-auto text-xs", toneClass[r.tone])}>{r.status}</span>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  );
}
