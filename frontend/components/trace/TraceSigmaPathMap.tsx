"use client";

import { useEffect, useMemo, useState } from "react";
import Graph from "graphology";
import { NodeCircleProgram } from "sigma/rendering";
import { NodeImageProgram } from "@sigma/node-image";
import {
  ControlsContainer,
  SigmaContainer,
  useLoadGraph,
  useRegisterEvents,
  useSetSettings,
  useSigma,
  ZoomControl,
} from "@react-sigma/core";
import "@react-sigma/core/lib/style.css";
import type { SigmaGraph } from "@/lib/graph";
import type { TraceGraph } from "@/lib/trace";
import {
  computeNodeDegrees,
  edgeColor,
  ipCircleColor,
  isPodNode,
  podColor,
  podNodeImageDataUrl,
  scaledNodeSize,
} from "@/lib/sigmaGraphStyle";

type Props = {
  sigmaGraph: SigmaGraph | null;
  traceGraph?: TraceGraph | null;
  selectedPathIds: string[];
  showContext: boolean;
};

const SIGMA_SETTINGS = {
  allowInvalidContainer: true,
  renderEdgeLabels: false,
  renderLabels: true,
  labelFont: "Segoe UI, Inter, system-ui, sans-serif",
  defaultNodeColor: "#64748b",
  defaultEdgeColor: "#64748b",
  labelDensity: 0.07,
  labelGridCellSize: 80,
  enableEdgeEvents: true,
  zIndex: true,
  labelColor: { color: "#b0bdd4" },
  stagePadding: 32,
  minEdgeThickness: 1.5,
  nodeProgramClasses: {
    circle: NodeCircleProgram,
    image: NodeImageProgram,
  },
};

function pathRefMaps(traceGraph?: TraceGraph | null) {
  const nodeRefs = new Map<string, string[]>();
  const edgeRefs = new Map<string, string[]>();
  const nodeTrack = new Map<string, string>();
  if (!traceGraph) return { nodeRefs, edgeRefs, nodeTrack };

  for (const n of traceGraph.nodes) {
    if (n.path_refs?.length) nodeRefs.set(n.id, n.path_refs);
    if (n.track) nodeTrack.set(n.id, n.track);
  }
  for (const e of traceGraph.edges) {
    if (e.path_refs?.length) edgeRefs.set(e.id, e.path_refs);
  }
  for (const opt of traceGraph.path_options ?? []) {
    for (const hop of opt.hop_ids ?? []) {
      const cur = nodeRefs.get(hop) ?? [];
      if (!cur.includes(opt.id)) nodeRefs.set(hop, [...cur, opt.id]);
    }
    for (const eid of opt.edge_ids ?? []) {
      const cur = edgeRefs.get(eid) ?? [];
      if (!cur.includes(opt.id)) edgeRefs.set(eid, [...cur, opt.id]);
    }
  }
  return { nodeRefs, edgeRefs, nodeTrack };
}

function buildGraphology(
  graph: SigmaGraph,
  nodeRefs: Map<string, string[]>,
  edgeRefs: Map<string, string[]>,
  nodeTrack: Map<string, string>
): Graph {
  const degrees = computeNodeDegrees(graph);
  const maxDegree = Math.max(0, ...degrees.values());
  const g = new Graph({ multi: true });

  for (const n of graph.nodes) {
    if (g.hasNode(n.id)) continue;
    const degree = degrees.get(n.id) ?? 0;
    const size = scaledNodeSize(Math.max(8, n.size), degree, maxDegree);
    const track = nodeTrack.get(n.id) || n.type || "";
    const pathRefs = nodeRefs.get(n.id) ?? [];

    if (isPodNode(n)) {
      const color = n.color || podColor(n.id);
      g.addNode(n.id, {
        x: n.x,
        y: n.y,
        label: n.label,
        size: Math.max(size, 16),
        color,
        type: "image",
        image: podNodeImageDataUrl(color),
        nodeTrack: track,
        pathRefs,
        baseSize: size,
      });
      continue;
    }

    const ipKey = /^\d{1,3}(\.\d{1,3}){3}$/.test(n.label || "") ? n.label : n.id;
    g.addNode(n.id, {
      x: n.x,
      y: n.y,
      label: n.label,
      size,
      color: n.color || ipCircleColor(ipKey),
      type: "circle",
      nodeTrack: track,
      pathRefs,
      baseSize: size,
    });
  }

  for (const e of graph.edges ?? []) {
    if (!g.hasNode(e.source) || !g.hasNode(e.target)) continue;
    const key = e.id || `${e.source}->${e.target}`;
    if (g.hasEdge(key)) continue;
    try {
      g.addDirectedEdgeWithKey(key, e.source, e.target, {
        size: Math.max(2, e.size),
        color: e.color || edgeColor(key),
        baseColor: e.color || edgeColor(key),
        topologyEdgeId: e.topology_edge_id || e.id,
        pathRefs: edgeRefs.get(e.id) ?? edgeRefs.get(e.topology_edge_id) ?? [],
      });
    } catch {
      // skip duplicate parallel keys
    }
  }
  return g;
}

function graphSignature(graph: SigmaGraph, selected: string[], showContext: boolean): string {
  const nodeSig = graph.nodes.map((n) => `${n.id}:${n.label}`).join("|");
  const edgeSig = (graph.edges ?? []).map((e) => `${e.id}:${e.source}:${e.target}`).join("|");
  return `${nodeSig}::${edgeSig}::${selected.join(",")}::${showContext}`;
}

function GraphLoader({
  graph,
  signature,
  traceGraph,
}: {
  graph: SigmaGraph;
  signature: string;
  traceGraph?: TraceGraph | null;
}) {
  const loadGraph = useLoadGraph();
  const { nodeRefs, edgeRefs, nodeTrack } = useMemo(() => pathRefMaps(traceGraph), [traceGraph]);

  useEffect(() => {
    loadGraph(buildGraphology(graph, nodeRefs, edgeRefs, nodeTrack), true);
  }, [graph, signature, loadGraph, nodeRefs, edgeRefs, nodeTrack]);

  return null;
}

function SigmaPathBehaviors({
  selectedPathIds,
  showContext,
  anchorNodeId,
}: {
  selectedPathIds: string[];
  showContext: boolean;
  anchorNodeId?: string;
}) {
  const registerEvents = useRegisterEvents();
  const setSettings = useSetSettings();
  const sigma = useSigma();
  const [draggedNode, setDraggedNode] = useState<string | null>(null);
  const selected = useMemo(() => new Set(selectedPathIds), [selectedPathIds]);
  const hasSelection = selected.size > 0;

  useEffect(() => {
    registerEvents({
      downNode: (e) => {
        setDraggedNode(e.node);
        sigma.getGraph().setNodeAttribute(e.node, "highlighted", true);
      },
      mousemovebody: (e) => {
        if (!draggedNode) return;
        const pos = sigma.viewportToGraph(e);
        sigma.getGraph().setNodeAttribute(draggedNode, "x", pos.x);
        sigma.getGraph().setNodeAttribute(draggedNode, "y", pos.y);
        e.preventSigmaDefault();
        e.original.preventDefault();
        e.original.stopPropagation();
      },
      mouseup: () => {
        if (draggedNode) {
          sigma.getGraph().removeNodeAttribute(draggedNode, "highlighted");
          setDraggedNode(null);
          sigma.setCustomBBox(sigma.getBBox());
        }
      },
      mousedown: () => {
        if (!sigma.getCustomBBox()) {
          sigma.setCustomBBox(sigma.getBBox());
        }
      },
    });
  }, [registerEvents, sigma, draggedNode]);

  useEffect(() => {
    setSettings({
      nodeReducer: (node, data) => {
        const g = sigma.getGraph();
        const next = { ...data };
        const baseSize = g.getNodeAttribute(node, "baseSize") ?? data.size ?? 10;
        const refs: string[] = g.getNodeAttribute(node, "pathRefs") ?? [];
        const track = String(g.getNodeAttribute(node, "nodeTrack") || "");
        const isAnchor = anchorNodeId ? node === anchorNodeId : track === "anchor";
        const lit = !hasSelection || refs.some((r) => selected.has(r)) || isAnchor;
        const context = track === "context" || track === "networkpolicy" || track === "nad" || track === "bgp-peer";

        next.size = baseSize;
        next.zIndex = 0;
        if (g.getNodeAttribute(node, "highlighted")) {
          next.highlighted = true;
          next.size = baseSize * 1.28;
          next.zIndex = 2;
        }
        if (!lit) {
          next.color = "#334155";
          next.label = "";
          next.zIndex = -1;
        }
        if (context && !showContext) {
          next.hidden = true;
        } else if (context && !lit) {
          next.color = "#475569";
          next.label = "";
        }
        return next;
      },
      edgeReducer: (edge, data) => {
        const g = sigma.getGraph();
        const refs: string[] = g.getEdgeAttribute(edge, "pathRefs") ?? [];
        const baseColor = String(g.getEdgeAttribute(edge, "baseColor") || data.color || "#64748b");
        const lit = !hasSelection || refs.some((r) => selected.has(r));
        if (!lit) {
          return { ...data, label: "", color: "#1e293b", size: 1 };
        }
        return { ...data, label: "", color: baseColor, size: Math.max(2, data.size ?? 2) };
      },
    });
  }, [selected, hasSelection, setSettings, sigma, draggedNode, showContext, anchorNodeId]);

  return null;
}

export function TraceSigmaPathMap({ sigmaGraph, traceGraph, selectedPathIds, showContext }: Props) {
  const signature = sigmaGraph ? graphSignature(sigmaGraph, selectedPathIds, showContext) : "";

  if (!sigmaGraph?.nodes?.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Run discovery to populate the path map.
      </div>
    );
  }

  return (
    <div className="h-full min-h-0 relative sigma-topology-shell fluent-graph-stage">
      <SigmaContainer className="sigma-topology-view !absolute !inset-0 !size-full" settings={SIGMA_SETTINGS}>
        <GraphLoader graph={sigmaGraph} signature={signature} traceGraph={traceGraph} />
        <SigmaPathBehaviors
          selectedPathIds={selectedPathIds}
          showContext={showContext}
          anchorNodeId={traceGraph?.anchor_node_id}
        />
        <ControlsContainer position="bottom-right">
          <ZoomControl labels={{ zoomIn: "Zoom in", zoomOut: "Zoom out", reset: "Fit graph" }} />
        </ControlsContainer>
      </SigmaContainer>
      <p className="absolute top-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10">
        nodes {sigmaGraph.nodes.length} · links {(sigmaGraph.edges ?? []).length}
      </p>
      <p className="absolute bottom-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10 max-w-[45%] text-right">
        Select paths in the panel · drag nodes · scroll to zoom
      </p>
    </div>
  );
}
