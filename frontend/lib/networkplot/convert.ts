import type { FlowTopology, TopologyEdge, TopologyNode } from "@/lib/ai";
import { iconUrl, inferNodeType, RANK_BY_TYPE, STYLE_BY_TYPE } from "./theme";

export type CyElement = { data: Record<string, string | number | boolean> };

export type NetworkPlotElements = {
  nodes: CyElement[];
  edges: CyElement[];
  title: string;
};

/** Strict tenant boundary: only selected pods + one-hop flow peers. */
export function isolateToTrackedPods(topology: FlowTopology, trackedIds: Set<string>): FlowTopology {
  if (trackedIds.size === 0) return { nodes: [], edges: [], namespaces: [], edge_details: {} };
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

function nodeToCy(n: TopologyNode, trackedIds: Set<string>): CyElement {
  const ntype = inferNodeType(n.kind, n.label);
  const style = STYLE_BY_TYPE[ntype] ?? STYLE_BY_TYPE.generic;
  const tracked = trackedIds.has(n.id);
  const lines = [n.label];
  if (n.host_ip) lines.push(n.host_ip);
  if (n.owner_kind && n.owner_name) lines.push(`${n.owner_kind}/${n.owner_name}`);
  return {
    data: {
      id: n.id,
      label: lines.join("\n"),
      name: n.label,
      type: ntype,
      namespace: n.namespace || "",
      rank: RANK_BY_TYPE[ntype] ?? 5,
      iconUrl: iconUrl(ntype),
      card: style.bg,
      accent: style.accent,
      border: style.border,
      textColor: style.text,
      directPod: tracked ? "true" : "false",
      namespaceOnly: tracked ? "false" : "true",
      tracked: tracked ? "true" : "false",
    },
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
  if (!isolated.nodes.length) return { nodes: [], edges: [], title: title || "Observed flows" };

  const nodes = isolated.nodes.map((n) => nodeToCy(n, tracked));
  const edges: CyElement[] = isolated.edges.map((e, i) => ({
    data: {
      id: e.id || `e${i}_${e.from}_${e.to}`,
      source: e.from,
      target: e.to,
      label: edgeLabel(e),
      edgeType: edgeType(e.health),
      topologyEdgeId: e.id,
    },
  }));

  return {
    nodes,
    edges,
    title: title || `Capture · ${trackedIds.length} selected pod(s)`,
  };
}
