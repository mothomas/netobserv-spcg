import type { FlowTopology, TopologyEdge, TopologyNode } from "@/lib/ai";
import { inferNodeType, STYLE_BY_TYPE } from "./theme";

export type ForceNode = {
  id: string;
  label: string;
  namespace: string;
  type: string;
  tracked: boolean;
  color: string;
  border: string;
  textColor: string;
  size: number;
  x?: number;
  y?: number;
};

export type ForceLink = {
  id: string;
  source: string;
  target: string;
  topologyEdgeId: string;
  label: string;
  edgeType: "direct" | "scheduled" | "snat";
  width: number;
  curvature: number;
  distance: number;
  externalIp?: string;
};

export type NetworkPlotElements = {
  nodes: ForceNode[];
  links: ForceLink[];
  title: string;
};

/** Strict tenant boundary: only selected pods + one-hop flow peers. */
export function isolateToTrackedPods(topology: FlowTopology, trackedIds: Set<string>): FlowTopology {
  if (trackedIds.size === 0) return topology;
  const touches = (id: string) => trackedIds.has(id);
  const edges: TopologyEdge[] = [];
  const nodeIds = new Set<string>();
  for (const e of topology.edges) {
    if (touches(e.from) || touches(e.to)) {
      edges.push(e);
      nodeIds.add(e.from);
      nodeIds.add(e.to);
    }
  }
  const nodes = topology.nodes.filter((n) => nodeIds.has(n.id));
  if (!nodes.length || !edges.length) return topology;
  const ns = [...new Set(nodes.map((n) => n.namespace).filter(Boolean))].sort();
  const details: FlowTopology["edge_details"] = {};
  for (const e of edges) {
    if (topology.edge_details?.[e.id]) details[e.id] = topology.edge_details[e.id];
  }
  return { nodes, edges, namespaces: ns, edge_details: details };
}

function edgeType(health: string): string {
  if (health === "dropped") return "snat";
  if (health === "degraded") return "scheduled";
  return "direct";
}

function edgeLabel(e: TopologyEdge): string {
  const parts: string[] = [];
  if (e.proto) parts.push(e.proto);
  if (e.dst_port) parts.push(String(e.dst_port));
  parts.push(`${e.count} pkts`);
  if (e.drop_cause) parts.push(e.drop_cause.slice(0, 40));
  return parts.join(" · ");
}

function nodeToForce(n: TopologyNode, trackedIds: Set<string>): ForceNode {
  const ntype = inferNodeType(n.kind, n.label);
  const style = STYLE_BY_TYPE[ntype] ?? STYLE_BY_TYPE.generic;
  const tracked = trackedIds.has(n.id);
  const colorSeed = colorFromID(n.id);
  return {
    id: n.id,
    label: n.label,
    namespace: n.namespace || "",
    type: ntype,
    tracked,
    color: colorSeed,
    border: tracked ? "#94b4ff" : style.border,
    textColor: "#eaf2ff",
    size: tracked ? 10 : ntype === "external" ? 8 : 9,
  };
}

export function flowTopologyToNetworkPlot(
  topology: FlowTopology | null,
  trackedIds: string[],
  title?: string
): NetworkPlotElements | null {
  if (!topology) return null;
  const tracked = new Set(trackedIds);
  const isolated = isolateToTrackedPods(topology, tracked);
  if (!isolated.nodes.length) return { nodes: [], links: [], title: title || "Observed flows" };

  const nodes = isolated.nodes.map((n) => nodeToForce(n, tracked));
  const pairIndex = new Map<string, number>();
  const pairCount = new Map<string, number>();
  for (const e of isolated.edges) {
    const key = undirectedPair(e.from, e.to);
    pairCount.set(key, (pairCount.get(key) ?? 0) + 1);
  }

  const links: ForceLink[] = isolated.edges.map((e, i) => {
    const pairKey = undirectedPair(e.from, e.to);
    const idx = pairIndex.get(pairKey) ?? 0;
    pairIndex.set(pairKey, idx + 1);
    const total = pairCount.get(pairKey) ?? 1;
    const spread = total > 1 ? (idx - (total - 1) / 2) * 0.2 : 0;
    const width = Math.max(0.35, Math.min(1.2, Math.log2((e.packets || e.count || 1) + 1) * 0.2));
    const externalIp = externalPeerIP(e.from, e.to);
    const distance = externalIp ? 320 : 220;
    return {
      id: e.id || `e${i}_${e.from}_${e.to}`,
      source: e.from,
      target: e.to,
      label: edgeLabel(e),
      edgeType: edgeType(e.health) as ForceLink["edgeType"],
      topologyEdgeId: e.id,
      curvature: spread,
      width,
      distance,
      externalIp: externalIp || "",
    };
  });

  return {
    nodes,
    links,
    title: title || `Capture · ${trackedIds.length} selected pod(s)`,
  };
}

function undirectedPair(a: string, b: string): string {
  return a < b ? `${a}::${b}` : `${b}::${a}`;
}

function externalPeerIP(from: string, to: string): string | null {
  const a = from.startsWith("ext/") ? from.slice(4) : "";
  const b = to.startsWith("ext/") ? to.slice(4) : "";
  return a || b || null;
}

function colorFromID(id: string): string {
  let h = 0;
  for (const ch of id) h = (h * 37 + ch.charCodeAt(0)) | 0;
  const hue = Math.abs(h) % 360;
  return `hsl(${hue} 65% 58%)`;
}
