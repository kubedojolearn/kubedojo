// @vitest-environment node
import { describe, expect, it } from "vitest";
import { groupResources, summarize, type SimResource } from "@/lib/lab-format";

const meta = (name: string, namespace?: string) => ({
  name,
  namespace,
  uid: "u",
  resourceVersion: "1",
  creationTimestamp: "2026-01-01T00:00:00Z",
});

const pod = (status: object, spec: object = { containers: [{ name: "c" }], nodeName: "node-2" }): SimResource =>
  ({ apiVersion: "v1", kind: "Pod", metadata: meta("web-1", "default"), spec, status }) as SimResource;

describe("summarize", () => {
  it("describes a running pod", () => {
    const row = summarize(
      pod({ phase: "Running", containerStatuses: [{ ready: true, restartCount: 0, state: { running: {} } }] }),
    );
    expect(row).toMatchObject({ status: "Running · 1/1 ready", tone: "ok" });
  });

  it("surfaces a waiting reason and restarts", () => {
    const row = summarize(
      pod({
        phase: "Running",
        containerStatuses: [{ ready: false, restartCount: 3, state: { waiting: { reason: "CrashLoopBackOff" } } }],
      }),
    );
    expect(row).toMatchObject({ status: "CrashLoopBackOff · 0/1 ready · 3 restarts", tone: "error" });
  });

  it("marks unschedulable pods", () => {
    const row = summarize(
      pod(
        { phase: "Pending", conditions: [{ type: "PodScheduled", status: "False", reason: "Unschedulable" }] },
        { containers: [{ name: "c" }] },
      ),
    );
    expect(row).toMatchObject({ status: "Pending (unschedulable) · 0/1 ready", tone: "error" });
  });

  it("describes deployments by readiness", () => {
    const d = (ready: number) =>
      summarize({
        apiVersion: "apps/v1",
        kind: "Deployment",
        metadata: meta("web", "default"),
        spec: { replicas: 3 },
        status: { readyReplicas: ready },
      } as SimResource);
    expect(d(3)).toMatchObject({ status: "3/3 ready", tone: "ok" });
    expect(d(1).tone).toBe("warn");
    expect(d(0).tone).toBe("error");
  });

  it("describes services, slices, nodes and config", () => {
    expect(
      summarize({
        apiVersion: "v1",
        kind: "Service",
        metadata: meta("web", "default"),
        spec: { type: "NodePort", clusterIP: "10.96.0.10", ports: [{ port: 80, nodePort: 30080, protocol: "TCP" }] },
      } as SimResource).status,
    ).toBe("NodePort 10.96.0.10 80:30080/TCP");
    expect(
      summarize({
        apiVersion: "discovery.k8s.io/v1",
        kind: "EndpointSlice",
        metadata: meta("web-x", "default"),
        endpoints: [],
      } as SimResource),
    ).toMatchObject({ status: "0/0 endpoints ready", tone: "warn" });
    expect(
      summarize({
        apiVersion: "v1",
        kind: "Node",
        metadata: meta("node-1"),
        spec: { unschedulable: true },
        status: { conditions: [{ type: "Ready", status: "True" }] },
      } as SimResource),
    ).toMatchObject({ status: "Ready,SchedulingDisabled", tone: "warn" });
    expect(
      summarize({
        apiVersion: "v1",
        kind: "ConfigMap",
        metadata: meta("cfg", "default"),
        data: { a: "1" },
      } as SimResource).status,
    ).toBe("1 key");
  });
});

describe("groupResources", () => {
  it("orders kinds for learners and names within a kind", () => {
    const groups = groupResources([
      { apiVersion: "v1", kind: "Namespace", metadata: meta("default") },
      pod({ phase: "Running" }),
      { apiVersion: "apps/v1", kind: "Deployment", metadata: meta("b", "default") },
      { apiVersion: "apps/v1", kind: "Deployment", metadata: meta("a", "default") },
    ] as SimResource[]);
    expect(groups.map((g) => g.kind)).toEqual(["Deployment", "Pod", "Namespace"]);
    expect(groups[0]!.rows.map((r) => r.name)).toEqual(["a", "b"]);
  });
});
