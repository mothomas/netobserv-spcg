import type { FlowTopology, TopologyEdge } from "./ai";

export function formatSrtt(ns?: number): string {
  if (!ns || ns <= 0) return "—";
  const ms = ns / 1e6;
  if (ms < 1) return `${ms.toFixed(2)} ms`;
  return `${ms.toFixed(1)} ms`;
}

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function edgeStroke(health: string): string {
  switch (health) {
    case "dropped":
      return "#dc2626";
    case "degraded":
      return "#d97706";
    default:
      return "#94a3b8";
  }
}

export function edgeLabel(e: TopologyEdge): string {
  const parts: string[] = [];
  if (e.proto) parts.push(e.proto);
  if (e.dst_port) parts.push(String(e.dst_port));
  parts.push(`${e.count} pkts`);
  return parts.join(" · ");
}

export function emptyTopology(): FlowTopology {
  return { nodes: [], edges: [], namespaces: [], edge_details: {} };
}

/** Ensure topology arrays are never null (API/JSON may omit or null them). */
export function normalizeTopology(t: FlowTopology | null | undefined): FlowTopology {
  if (!t) return emptyTopology();
  return {
    ...t,
    nodes: t.nodes ?? [],
    edges: t.edges ?? [],
    namespaces: t.namespaces ?? [],
    edge_details: t.edge_details ?? {},
  };
}

export function mergeTopology(
  prev: FlowTopology | null | undefined,
  next: FlowTopology | undefined
): FlowTopology {
  if (!next || next.nodes.length === 0) return prev ?? emptyTopology();
  return next;
}
