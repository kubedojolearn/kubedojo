import { summarize, type SimResource, type Tone } from "@/lib/lab-format";

/**
 * The cluster map's model: which objects to draw, where, and how they relate. Pure and
 * deterministic, so the same snapshot always draws the same map (and it can be unit tested).
 */

export type Relation = "owns" | "selects" | "runs-on" | "uses";

export interface GraphNode {
  id: string; // kind/namespace/name
  kind: string;
  namespace?: string;
  name: string;
  status: string;
  tone: Tone;
  resourceVersion: string;
  /** Scaled to zero, e.g. an old ReplicaSet kept for rollbacks. */
  idle: boolean;
  x: number;
  y: number;
}

export interface GraphEdge {
  id: string;
  source: string;
  target: string;
  relation: Relation;
  /** For selects: the Pod is a ready endpoint of the Service. */
  ready?: boolean;
}

export interface ClusterGraph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/** Columns, left to right: what selects, what owns, what runs, what it reads, where it runs. */
const COLUMNS: Record<string, number> = {
  Service: 0,
  Deployment: 1,
  ReplicaSet: 2,
  Pod: 3,
  ConfigMap: 4,
  Secret: 4,
  Node: 5,
};
export const COLUMN_WIDTH = 250;
export const ROW_HEIGHT = 84;

type Obj = Record<string, unknown>;
const obj = (v: unknown): Obj => (v && typeof v === "object" && !Array.isArray(v) ? (v as Obj) : {});
const arr = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const str = (v: unknown): string => (typeof v === "string" ? v : "");

export const nodeId = (kind: string, namespace: string | undefined, name: string) =>
  `${kind}/${namespace ?? ""}/${name}`;

function selects(selector: Obj, labels: Obj): boolean {
  const keys = Object.keys(selector);
  return keys.length > 0 && keys.every((k) => labels[k] === selector[k]);
}

/** ConfigMaps and Secrets a Pod's containers reference (env valueFrom and envFrom). */
function configRefs(pod: Obj): { kind: string; name: string }[] {
  const out: { kind: string; name: string }[] = [];
  for (const c of arr(obj(pod.spec).containers).map(obj)) {
    for (const e of arr(c.env).map(obj)) {
      const from = obj(e.valueFrom);
      const cm = str(obj(from.configMapKeyRef).name);
      const secret = str(obj(from.secretKeyRef).name);
      if (cm) out.push({ kind: "ConfigMap", name: cm });
      if (secret) out.push({ kind: "Secret", name: secret });
    }
    for (const e of arr(c.envFrom).map(obj)) {
      const cm = str(obj(e.configMapRef).name);
      const secret = str(obj(e.secretRef).name);
      if (cm) out.push({ kind: "ConfigMap", name: cm });
      if (secret) out.push({ kind: "Secret", name: secret });
    }
  }
  return out;
}

/**
 * Builds the map for one namespace ("" for all namespaces). Namespaced objects outside it are
 * left out; Nodes are always shown, since every Pod runs on one.
 */
export function buildGraph(resources: SimResource[], namespace: string): ClusterGraph {
  const inScope = resources.filter(
    (r) =>
      r.kind in COLUMNS &&
      (r.kind === "Node" || namespace === "" || r.metadata.namespace === namespace) &&
      // The API server's own Service selects nothing a learner made.
      !(r.kind === "Service" && r.metadata.name === "kubernetes" && r.metadata.namespace === "default"),
  );
  const byUid = new Map<string, SimResource>();
  for (const r of inScope) byUid.set(r.metadata.uid, r);
  const id = (r: SimResource) => nodeId(r.kind, r.metadata.namespace, r.metadata.name);

  // Ready endpoints per Service, from its EndpointSlice.
  const readyPods = new Map<string, Set<string>>();
  for (const r of resources) {
    if (r.kind !== "EndpointSlice") continue;
    const svc = str(obj(r.metadata.labels)["kubernetes.io/service-name"]);
    const set = new Set<string>();
    for (const e of arr((r as Obj).endpoints).map(obj)) {
      if (obj(e.conditions).ready === true) set.add(str(obj(e.targetRef).name));
    }
    readyPods.set(`${r.metadata.namespace}/${svc}`, set);
  }

  const edges: GraphEdge[] = [];
  const add = (source: string, target: string, relation: Relation, ready?: boolean) =>
    edges.push({
      id: `${relation}:${source}->${target}`,
      source,
      target,
      relation,
      ...(ready === undefined ? {} : { ready }),
    });

  for (const r of inScope) {
    const o = r as SimResource & Obj;
    for (const ref of r.metadata.ownerReferences ?? []) {
      const owner = byUid.get(ref.uid);
      if (owner && ref.controller) add(id(owner), id(r), "owns");
    }
    if (r.kind === "Pod") {
      const nodeName = str(obj(o.spec).nodeName);
      if (nodeName) add(id(r), nodeId("Node", undefined, nodeName), "runs-on");
      const seen = new Set<string>();
      for (const ref of configRefs(o)) {
        const target = nodeId(ref.kind, r.metadata.namespace, ref.name);
        if (!seen.has(target) && inScope.some((x) => id(x) === target)) {
          seen.add(target);
          add(id(r), target, "uses");
        }
      }
    }
    if (r.kind === "Service") {
      const selector = obj(obj(o.spec).selector);
      const ready = readyPods.get(`${r.metadata.namespace}/${r.metadata.name}`) ?? new Set<string>();
      for (const p of inScope) {
        if (
          p.kind === "Pod" &&
          p.metadata.namespace === r.metadata.namespace &&
          selects(selector, obj(p.metadata.labels))
        ) {
          add(id(r), id(p), "selects", ready.has(p.metadata.name));
        }
      }
    }
  }

  // Order each column so related objects line up: Pods follow their ReplicaSet, ReplicaSets
  // their Deployment, newest revision first.
  const ownerOf = new Map(edges.filter((e) => e.relation === "owns").map((e) => [e.target, e.source]));
  const rootKey = (nid: string): string => {
    const chain: string[] = [];
    let cur: string | undefined = nid;
    while (cur && chain.length < 4) {
      chain.unshift(cur);
      cur = ownerOf.get(cur);
    }
    return chain.join(" > ");
  };
  const columns = new Map<number, SimResource[]>();
  for (const r of inScope) {
    const col = COLUMNS[r.kind]!;
    columns.set(col, [...(columns.get(col) ?? []), r]);
  }
  const nodes: GraphNode[] = [];
  for (const [col, rs] of [...columns.entries()].sort((a, b) => a[0] - b[0])) {
    rs.sort((a, b) => {
      const ka = col === 4 ? `${a.kind} ${id(a)}` : rootKey(id(a));
      const kb = col === 4 ? `${b.kind} ${id(b)}` : rootKey(id(b));
      return ka < kb ? -1 : ka > kb ? 1 : 0;
    });
    rs.forEach((r, row) => {
      const row2 = summarize(r);
      const spec = obj((r as Obj).spec);
      const idle = (r.kind === "ReplicaSet" || r.kind === "Deployment") && spec.replicas === 0;
      nodes.push({
        id: id(r),
        kind: r.kind,
        namespace: r.metadata.namespace,
        name: r.metadata.name,
        status: row2.status,
        tone: row2.tone,
        resourceVersion: r.metadata.resourceVersion,
        idle,
        x: col * COLUMN_WIDTH,
        y: row * ROW_HEIGHT,
      });
    });
  }
  edges.sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
  return { nodes, edges };
}

const verbs: Record<Relation, string> = {
  owns: "owns",
  selects: "selects",
  "runs-on": "runs on",
  uses: "reads configuration from",
};

/** A sentence per relationship, for screen readers and for anyone who prefers text. */
export function describeEdges(graph: ClusterGraph): string[] {
  const byId = new Map(graph.nodes.map((n) => [n.id, n]));
  const label = (nid: string) => {
    const n = byId.get(nid);
    return n ? `${n.kind} ${n.name}` : nid.split("/").filter(Boolean).join(" ");
  };
  return graph.edges.map((e) => {
    const extra = e.relation === "selects" ? (e.ready ? " (ready endpoint)" : " (not ready, gets no traffic)") : "";
    return `${label(e.source)} ${verbs[e.relation]} ${label(e.target)}${extra}`;
  });
}

/** Nodes whose resourceVersion differs from the previous snapshot's: what the last command changed. */
export function changedSince(prev: Map<string, string> | null, graph: ClusterGraph): Set<string> {
  const out = new Set<string>();
  if (!prev) return out;
  for (const n of graph.nodes) {
    if (prev.get(n.id) !== n.resourceVersion) out.add(n.id);
  }
  return out;
}
