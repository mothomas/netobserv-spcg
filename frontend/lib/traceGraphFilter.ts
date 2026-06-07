import type { SigmaGraph } from "@/lib/graph";
import type { TraceGraph, TraceNode } from "@/lib/trace";

export type TraceGraphFilter = {
  focusPath: boolean;
  logical: boolean;
  physical: boolean;
};

export const DEFAULT_TRACE_FILTER: TraceGraphFilter = {
  focusPath: true,
  logical: true,
  physical: true,
};

export function filterTraceGraph(graph: TraceGraph, filter: TraceGraphFilter): TraceGraph {
  const nodes = graph.nodes.filter((n) => nodeVisible(n, filter));
  const ids = new Set(nodes.map((n) => n.id));
  const edges = graph.edges.filter((e) => ids.has(e.from) && ids.has(e.to));
  return { ...graph, nodes, edges };
}

function nodeVisible(n: TraceNode, filter: TraceGraphFilter): boolean {
  if (filter.focusPath && !n.focused && !n.tracked) return false;
  const layer = n.layer || "logical";
  if (layer === "physical" && !filter.physical) return false;
  if (layer === "logical" && !filter.logical) return false;
  return true;
}

export function filterSigmaGraph(graph: SigmaGraph | null | undefined, visibleNodeIds: Set<string>): SigmaGraph | null {
  if (!graph) return null;
  const nodes = graph.nodes.filter((n) => visibleNodeIds.has(n.id));
  const ids = new Set(nodes.map((n) => n.id));
  const edges = graph.edges.filter((e) => ids.has(e.source) && ids.has(e.target));
  return { ...graph, nodes, edges };
}
