// @vitest-environment node
import { describe, expect, it } from "vitest";
import { buildGraph, changedSince, describeEdges, nodeId } from "@/lib/cluster-graph";
import type { SimResource } from "@/lib/lab-format";

let uid = 0;
const meta = (name: string, namespace?: string, extra: object = {}) => ({
  name,
  namespace,
  uid: `uid-${name}-${uid++}`,
  resourceVersion: "1",
  creationTimestamp: "2026-01-01T00:00:00Z",
  ...extra,
});
const res = (r: object) => r as unknown as SimResource;

function cluster() {
  const dep = res({
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: meta("web", "default"),
    spec: { replicas: 2 },
    status: { readyReplicas: 2 },
  });
  const owner = (o: SimResource) => [
    { uid: o.metadata.uid, kind: o.kind, name: o.metadata.name, apiVersion: "v1", controller: true },
  ];
  const rs = res({
    apiVersion: "apps/v1",
    kind: "ReplicaSet",
    metadata: meta("web-abc", "default", { ownerReferences: owner(dep) }),
    spec: { replicas: 2 },
  });
  const old = res({
    apiVersion: "apps/v1",
    kind: "ReplicaSet",
    metadata: meta("web-old", "default", { ownerReferences: owner(dep) }),
    spec: { replicas: 0 },
  });
  const pod = (name: string, ready: boolean) =>
    res({
      apiVersion: "v1",
      kind: "Pod",
      metadata: meta(name, "default", { labels: { app: "web" }, ownerReferences: owner(rs) }),
      spec: {
        nodeName: "node-2",
        containers: [
          { name: "web", env: [{ name: "MODE", valueFrom: { configMapKeyRef: { name: "settings", key: "mode" } } }] },
        ],
      },
      status: { phase: "Running", containerStatuses: [{ ready, restartCount: 0, state: { running: {} } }] },
    });
  return [
    dep,
    rs,
    old,
    pod("web-abc-1", true),
    pod("web-abc-2", false),
    res({
      apiVersion: "v1",
      kind: "Service",
      metadata: meta("web", "default"),
      spec: { selector: { app: "web" }, ports: [{ port: 80 }] },
    }),
    res({
      apiVersion: "discovery.k8s.io/v1",
      kind: "EndpointSlice",
      metadata: meta("web-x", "default", { labels: { "kubernetes.io/service-name": "web" } }),
      endpoints: [{ addresses: ["10.0.0.1"], conditions: { ready: true }, targetRef: { name: "web-abc-1" } }],
    }),
    res({ apiVersion: "v1", kind: "ConfigMap", metadata: meta("settings", "default"), data: { mode: "prod" } }),
    res({ apiVersion: "v1", kind: "Service", metadata: meta("kubernetes", "default"), spec: { selector: {} } }),
    res({
      apiVersion: "v1",
      kind: "Pod",
      metadata: meta("other", "shop"),
      spec: { containers: [{ name: "c" }] },
      status: {},
    }),
    res({
      apiVersion: "v1",
      kind: "Node",
      metadata: meta("node-2"),
      spec: {},
      status: { conditions: [{ type: "Ready", status: "True" }] },
    }),
  ];
}

describe("buildGraph", () => {
  it("relates owners, selectors, nodes and configuration", () => {
    const g = buildGraph(cluster(), "default");
    const rel = (relation: string) =>
      g.edges.filter((e) => e.relation === relation).map((e) => `${e.source} -> ${e.target}`);
    expect(rel("owns")).toEqual([
      "Deployment/default/web -> ReplicaSet/default/web-abc",
      "Deployment/default/web -> ReplicaSet/default/web-old",
      "ReplicaSet/default/web-abc -> Pod/default/web-abc-1",
      "ReplicaSet/default/web-abc -> Pod/default/web-abc-2",
    ]);
    const selects = g.edges.filter((e) => e.relation === "selects");
    expect(selects.map((e) => [e.target, e.ready])).toEqual([
      ["Pod/default/web-abc-1", true],
      ["Pod/default/web-abc-2", false],
    ]);
    expect(rel("runs-on")).toHaveLength(2);
    expect(rel("uses")).toEqual([
      "Pod/default/web-abc-1 -> ConfigMap/default/settings",
      "Pod/default/web-abc-2 -> ConfigMap/default/settings",
    ]);
    // Other namespaces and the API server's Service are left out; Nodes stay.
    expect(g.nodes.map((n) => n.id)).not.toContain("Pod/shop/other");
    expect(g.nodes.map((n) => n.id)).not.toContain("Service/default/kubernetes");
    expect(g.nodes.map((n) => n.id)).toContain(nodeId("Node", undefined, "node-2"));
    expect(g.nodes.find((n) => n.name === "web-old")?.idle).toBe(true);
  });

  it("lays out columns deterministically", () => {
    const a = buildGraph(cluster(), "default");
    const b = buildGraph([...cluster()].reverse(), "default");
    expect(a.nodes.map((n) => [n.name, n.x, n.y])).toEqual(b.nodes.map((n) => [n.name, n.x, n.y]));
    const x = (name: string) => a.nodes.find((n) => n.name === name)!.x;
    expect(x("web-abc")).toBeGreaterThan(a.nodes.find((n) => n.kind === "Deployment")!.x);
    expect(x("web-abc-1")).toBeGreaterThan(x("web-abc"));
  });

  it("shows every namespace when asked", () => {
    expect(buildGraph(cluster(), "").nodes.map((n) => n.id)).toContain("Pod/shop/other");
  });

  it("describes relationships in words and spots changes", () => {
    const g = buildGraph(cluster(), "default");
    expect(describeEdges(g)).toContain("Service web selects Pod web-abc-2 (not ready, gets no traffic)");
    expect(describeEdges(g)).toContain("Pod web-abc-1 runs on Node node-2");
    const prev = new Map(g.nodes.map((n) => [n.id, n.resourceVersion]));
    prev.set("Pod/default/web-abc-1", "0");
    prev.delete("ConfigMap/default/settings");
    expect([...changedSince(prev, g)].sort()).toEqual(["ConfigMap/default/settings", "Pod/default/web-abc-1"]);
    expect(changedSince(null, g).size).toBe(0);
  });
});
