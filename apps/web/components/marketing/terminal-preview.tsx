import { CircleCheck } from "lucide-react";

type Line = { kind: "prompt" | "out" | "ok"; text: string };

const session: Line[] = [
  { kind: "prompt", text: "kubectl get deployments" },
  { kind: "out", text: "NAME    READY   UP-TO-DATE   AVAILABLE   AGE" },
  { kind: "out", text: "nginx   1/1     1            1           1h" },
  { kind: "prompt", text: "kubectl scale deployment nginx --replicas=3" },
  { kind: "out", text: "deployment.apps/nginx scaled" },
  { kind: "prompt", text: "kubectl get pods" },
  { kind: "out", text: "NAME                     READY   STATUS    RESTARTS   AGE" },
  { kind: "out", text: "nginx-6c8d7b9f4d-2xk7p   1/1     Running   0          1h" },
  { kind: "out", text: "nginx-6c8d7b9f4d-h5w9q   1/1     Running   0          10s" },
  { kind: "out", text: "nginx-6c8d7b9f4d-r8m2v   1/1     Running   0          10s" },
  { kind: "ok", text: "Goal met: nginx wants 3 replicas and all 3 are ready." },
];

/**
 * A static, illustrative lab session for the home page. It is a picture of the product, not an
 * interactive terminal, and is labelled as such.
 */
export function TerminalPreview() {
  return (
    <figure className="overflow-hidden rounded-xl border border-white/10 bg-terminal text-terminal-foreground shadow-glow">
      <div className="flex items-center gap-2 border-b border-white/10 px-4 py-2.5">
        <span aria-hidden className="size-3 rounded-full bg-[#FF7B7B]" />
        <span aria-hidden className="size-3 rounded-full bg-[#FACC15]" />
        <span aria-hidden className="size-3 rounded-full bg-[#4ADE80]" />
        <span className="ml-2 font-mono text-xs text-[#8293B0]">workloads/scaling-deployments</span>
        <span className="ml-auto rounded bg-[#3B2F0B] px-1.5 py-0.5 text-[10px] font-medium tracking-wide text-[#FCD34D] uppercase">
          Simulated
        </span>
      </div>
      <pre className="overflow-x-auto p-4 font-mono text-[12.5px] leading-relaxed sm:text-[13px]">
        <code>
          {session.map((line, i) => {
            if (line.kind === "prompt")
              return (
                <span key={i} className="block">
                  <span className="text-[#5EEAD4]">$ </span>
                  {line.text}
                </span>
              );
            if (line.kind === "ok")
              return (
                <span key={i} className="mt-2 flex items-center gap-1.5 text-[#4ADE80]">
                  <CircleCheck aria-hidden className="size-3.5" />
                  {line.text}
                </span>
              );
            return (
              <span key={i} className="block text-[#D6E2F5]/90">
                {line.text}
              </span>
            );
          })}
        </code>
      </pre>
      <figcaption className="border-t border-white/10 px-4 py-2 text-xs text-[#8293B0]">
        Example of a lab session. The cluster is simulated: no containers run.
      </figcaption>
    </figure>
  );
}
