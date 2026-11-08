"use client";

import "@xyflow/react/dist/style.css";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTheme } from "next-themes";
import {
  Background,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import { Box, Copy, FileCog, KeyRound, Layers, Network, Server, type LucideIcon } from "lucide-react";
import { ResourceList } from "@/components/lab/resource-list";
import { Button } from "@/components/ui/button";
import { buildGraph, changedSince, describeEdges, type GraphNode } from "@/lib/cluster-graph";
import { graphNodes } from "@/lib/design-tokens";
import type { SimResource, Tone } from "@/lib/lab-format";
import { cn } from "@/lib/utils";

const icons: Record<string, LucideIcon> = {
  Deployment: Layers,
  ReplicaSet: Copy,
  Pod: Box,
  Service: Network,
  ConfigMap: FileCog,
  Secret: KeyRound,
  Node: Server,
};

// Shape and icon tell kinds apart, so color is never the only signal (WCAG 1.4.1).
const shapes: Record<string, string> = {
  Pod: "rounded-full",
  Service: "rounded-md border-double border-4",
  ConfigMap: "rounded-sm border-dashed",
  Secret: "rounded-sm border-dashed",
};

const toneClass: Record<Tone, string> = {
  ok: "text-success",
  warn: "text-warning",
  error: "text-danger",
  neutral: "text-muted-foreground",
};

// React Flow needs node data as a plain object type (an interface has no index signature).
type MapData = { [K in keyof GraphNode]: GraphNode[K] } & { changed: boolean };
type MapNode = Node<MapData, "resource">;

function ResourceNode({ data }: NodeProps<MapNode>) {
  const Icon = icons[data.kind] ?? Box;
  const color = graphNodes[data.kind as keyof typeof graphNodes]?.color;
  return (
    <div
      className={cn(
        "w-[210px] border-2 bg-card px-3 py-2 text-left shadow-sm transition-shadow",
        shapes[data.kind] ?? "rounded-md",
        data.idle && "opacity-60",
        data.changed && "ring-4 ring-highlight/60",
      )}
      style={{ borderColor: color }}
    >
      <Handle type="target" position={Position.Left} className="!size-1.5 !border-0 !bg-border-strong" />
      <div className="flex items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
        <Icon aria-hidden className="size-3.5 shrink-0" style={{ color }} />
        {data.kind}
        {data.namespace && <span className="truncate">· {data.namespace}</span>}
      </div>
      <div className="truncate font-mono text-xs font-semibold text-foreground" title={data.name}>
        {data.name}
      </div>
      <div className={cn("truncate text-[11px]", toneClass[data.tone])} title={data.status}>
        {data.status || " "}
      </div>
      <Handle type="source" position={Position.Right} className="!size-1.5 !border-0 !bg-border-strong" />
    </div>
  );
}

const nodeTypes = { resource: ResourceNode };

const edgeStyle = {
  owns: { stroke: "var(--color-foreground-secondary)" },
  selects: { stroke: "#A855F7", strokeDasharray: "6 4" },
  "runs-on": { stroke: "var(--color-border-strong)", strokeDasharray: "2 4" },
  uses: { stroke: "#14B8A6", strokeDasharray: "2 3" },
} as const;

/**
 * The cluster map: Services, Deployments, ReplicaSets, Pods, their configuration and Nodes,
 * joined by what owns, selects, runs on or reads what. Objects the last command changed are
 * highlighted. The same relationships are listed as text below the map, and the list view
 * shows every object, for keyboard and screen-reader users.
 */
export function ClusterPanel({
  resources,
  currentNamespace,
  version,
}: {
  resources: SimResource[];
  currentNamespace: string;
  version: number;
}) {
  const { resolvedTheme } = useTheme();
  const [view, setView] = useState<"map" | "list">("map");
  const namespaces = useMemo(
    () => resources.filter((r) => r.kind === "Namespace").map((r) => r.metadata.name),
    [resources],
  );
  const [namespace, setNamespace] = useState(namespaces.includes(currentNamespace) ? currentNamespace : "default");
  const graph = useMemo(() => buildGraph(resources, namespace), [resources, namespace]);

  // Highlight what changed since the previous snapshot of this lab, for a few seconds.
  const previous = useRef<{ version: number; rv: Map<string, string> } | null>(null);
  const [changed, setChanged] = useState<Set<string>>(new Set());
  useEffect(() => {
    const prev = previous.current;
    const diff = prev && version > prev.version ? changedSince(prev.rv, graph) : new Set<string>();
    previous.current = { version, rv: new Map(graph.nodes.map((n) => [n.id, n.resourceVersion])) };
    setChanged(diff);
    if (diff.size === 0) return;
    const t = setTimeout(() => setChanged(new Set()), 4000);
    return () => clearTimeout(t);
    // Only a new lab version marks changes; switching namespace doesn't.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version]);

  const nodes: MapNode[] = graph.nodes.map((n) => ({
    id: n.id,
    type: "resource",
    position: { x: n.x, y: n.y },
    data: { ...n, changed: changed.has(n.id) },
    ariaLabel: `${n.kind} ${n.name}${n.namespace ? ` in ${n.namespace}` : ""}: ${n.status}${changed.has(n.id) ? " (just changed)" : ""}`,
    draggable: false,
    connectable: false,
  }));
  const edges: Edge[] = graph.edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    style: { ...edgeStyle[e.relation], strokeWidth: 1.5, opacity: e.relation === "selects" && !e.ready ? 0.45 : 1 },
    markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14 },
    focusable: false,
  }));
  const sentences = describeEdges(graph);

  return (
    <div className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <div role="group" aria-label="View" className="inline-flex rounded-md border p-0.5">
          {(["map", "list"] as const).map((v) => (
            <Button
              key={v}
              size="sm"
              variant={view === v ? "secondary" : "ghost"}
              aria-pressed={view === v}
              onClick={() => setView(v)}
            >
              {v === "map" ? "Map" : "List"}
            </Button>
          ))}
        </div>
        {view === "map" && (
          <label className="flex items-center gap-2 text-sm">
            Namespace
            <select
              value={namespace}
              onChange={(e) => setNamespace(e.target.value)}
              className="h-8 rounded-md border bg-background px-2 text-sm"
            >
              <option value="">All namespaces</option>
              {namespaces.map((ns) => (
                <option key={ns} value={ns}>
                  {ns}
                </option>
              ))}
            </select>
          </label>
        )}
        <p className="ml-auto text-xs text-muted-foreground">Simulated cluster. Nothing here runs on a real one.</p>
      </div>

      {view === "list" ? (
        <ResourceList resources={resources} />
      ) : (
        <>
          <div className="h-[min(65svh,34rem)] rounded-lg border bg-background" data-testid="cluster-map">
            <ReactFlow
              nodes={nodes}
              edges={edges}
              nodeTypes={nodeTypes}
              colorMode={resolvedTheme === "dark" ? "dark" : "light"}
              nodesDraggable={false}
              nodesConnectable={false}
              edgesFocusable={false}
              fitView
              fitViewOptions={{ padding: 0.15 }}
              minZoom={0.2}
              ariaLabelConfig={{
                "node.a11yDescription.default": "Use the list view or the relationships below for details.",
              }}
            >
              <Background gap={24} size={1} />
              <Controls showInteractive={false} />
            </ReactFlow>
          </div>
          <ul aria-label="Legend" className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
            <li>
              <span aria-hidden className="mr-1 inline-block w-6 border-t-2 align-middle" style={edgeStyle.owns} />
              owns
            </li>
            <li>
              <span
                aria-hidden
                className="mr-1 inline-block w-6 border-t-2 border-dashed align-middle"
                style={{ borderColor: "#A855F7" }}
              />
              selects (faded: not ready, no traffic)
            </li>
            <li>
              <span
                aria-hidden
                className="mr-1 inline-block w-6 border-t-2 border-dotted align-middle"
                style={{ borderColor: "#14B8A6" }}
              />
              reads configuration
            </li>
            <li>
              <span aria-hidden className="mr-1 inline-block w-6 border-t-2 border-dotted align-middle" />
              runs on
            </li>
            <li>Highlighted: changed by your last command</li>
          </ul>
          <details className="rounded-lg border p-3 text-sm">
            <summary className="cursor-pointer font-medium">Relationships as text ({sentences.length})</summary>
            {sentences.length === 0 ? (
              <p className="mt-2 text-muted-foreground">Nothing in this namespace is related to anything else yet.</p>
            ) : (
              <ul className="mt-2 grid list-disc gap-1 pl-5">
                {sentences.map((s) => (
                  <li key={s}>{s}</li>
                ))}
              </ul>
            )}
          </details>
        </>
      )}
    </div>
  );
}
