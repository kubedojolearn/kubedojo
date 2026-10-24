import type { Schemas } from "@kubedojo/api-client";

export type SimResource = Schemas["SimResource"];

export type Tone = "ok" | "warn" | "error" | "neutral";

export interface ResourceRow {
  key: string;
  kind: string;
  namespace?: string;
  name: string;
  status: string;
  tone: Tone;
}

/** Display order: what learners act on first. */
export const KIND_ORDER = [
  "Deployment",
  "ReplicaSet",
  "Pod",
  "Service",
  "EndpointSlice",
  "ConfigMap",
  "Secret",
  "Node",
  "Namespace",
];

type Obj = Record<string, unknown>;
const obj = (v: unknown): Obj => (v && typeof v === "object" && !Array.isArray(v) ? (v as Obj) : {});
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const num = (v: unknown): number => (typeof v === "number" ? v : 0);
const str = (v: unknown): string => (typeof v === "string" ? v : "");

function podStatus(spec: Obj, status: Obj): { status: string; tone: Tone } {
  const containers = arr(status.containerStatuses).map(obj);
  const total = arr(spec.containers).length;
  const ready = containers.filter((c) => c.ready === true).length;
  const restarts = containers.reduce((n, c) => n + num(c.restartCount), 0);
  const waiting = containers.map((c) => str(obj(obj(c.state).waiting).reason)).find((r) => r !== "");
  const unschedulable = arr(status.conditions)
    .map(obj)
    .some((c) => c.type === "PodScheduled" && c.status === "False");
  let phase = waiting || str(status.phase) || "Pending";
  if (unschedulable && !str(spec.nodeName)) phase = "Pending (unschedulable)";
  const tone: Tone =
    waiting || unschedulable
      ? "error"
      : phase === "Running" && ready === total
        ? "ok"
        : phase === "Running"
          ? "warn"
          : "neutral";
  return { status: `${phase} · ${ready}/${total} ready${restarts ? ` · ${restarts} restarts` : ""}`, tone };
}

function replicaStatus(spec: Obj, status: Obj): { status: string; tone: Tone } {
  const desired = typeof spec.replicas === "number" ? spec.replicas : 1;
  const ready = num(status.readyReplicas);
  return {
    status: `${ready}/${desired} ready`,
    tone: ready === desired ? "ok" : ready === 0 && desired > 0 ? "error" : "warn",
  };
}

/** One line describing a resource's state, in the spirit of `kubectl get`. */
export function summarize(r: SimResource): ResourceRow {
  const r2 = r as SimResource & Obj;
  const spec = obj(r2.spec);
  const status = obj(r2.status);
  const base = {
    key: `${r.kind}/${r.metadata.namespace ?? ""}/${r.metadata.name}`,
    kind: r.kind,
    namespace: r.metadata.namespace,
    name: r.metadata.name,
  };
  switch (r.kind) {
    case "Pod":
      return { ...base, ...podStatus(spec, status) };
    case "Deployment":
    case "ReplicaSet":
      return { ...base, ...replicaStatus(spec, status) };
    case "Service": {
      const ports = arr(spec.ports)
        .map(obj)
        .map((p) => `${num(p.port)}${p.nodePort ? `:${num(p.nodePort)}` : ""}/${str(p.protocol) || "TCP"}`)
        .join(", ");
      return {
        ...base,
        status: `${str(spec.type) || "ClusterIP"} ${str(spec.clusterIP)} ${ports}`.trim(),
        tone: "neutral",
      };
    }
    case "EndpointSlice": {
      const endpoints = arr(r2.endpoints).map(obj);
      const ready = endpoints.filter((e) => obj(e.conditions).ready === true).length;
      return {
        ...base,
        status: `${ready}/${endpoints.length} endpoints ready`,
        tone: endpoints.length === 0 ? "warn" : "ok",
      };
    }
    case "Node": {
      const readyCond = arr(status.conditions)
        .map(obj)
        .find((c) => c.type === "Ready");
      const ready = readyCond?.status === "True";
      const cordoned = spec.unschedulable === true;
      return {
        ...base,
        status: `${ready ? "Ready" : "NotReady"}${cordoned ? ",SchedulingDisabled" : ""}`,
        tone: ready && !cordoned ? "ok" : "warn",
      };
    }
    case "ConfigMap":
    case "Secret": {
      const n = Object.keys(obj(r2.data)).length;
      return { ...base, status: `${n} ${n === 1 ? "key" : "keys"}`, tone: "neutral" };
    }
    case "Namespace":
      return { ...base, status: str(status.phase) || "Active", tone: "neutral" };
  }
  return { ...base, status: "", tone: "neutral" };
}

/** Rows grouped by kind in KIND_ORDER, each group sorted by namespace then name. */
export function groupResources(resources: SimResource[]): { kind: string; rows: ResourceRow[] }[] {
  const byKind = new Map<string, ResourceRow[]>();
  for (const r of resources) {
    const row = summarize(r);
    byKind.set(row.kind, [...(byKind.get(row.kind) ?? []), row]);
  }
  const rank = (k: string) => (KIND_ORDER.includes(k) ? KIND_ORDER.indexOf(k) : KIND_ORDER.length);
  return [...byKind.entries()]
    .sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
    .map(([kind, rows]) => ({
      kind,
      rows: rows.sort((a, b) => (a.namespace ?? "").localeCompare(b.namespace ?? "") || a.name.localeCompare(b.name)),
    }));
}
