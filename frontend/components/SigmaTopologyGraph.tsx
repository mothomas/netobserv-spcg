"use client";

import { useEffect, useMemo, useState } from "react";
import Graph from "graphology";
import { NodeImageProgram } from "@sigma/node-image";
import { NodeCircleProgram } from "sigma/rendering";
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
import type { FlowTopology, TopologyEdge } from "@/lib/ai";
import type { SigmaGraph } from "@/lib/graph";
import { resolveIPGeo } from "@/lib/networkplot/geo";
import {
  compilePublicIpFlagPairs,
  loadIpCountryMap,
  lookupCountryFromMap,
  pairFromCountry,
  type PublicIpFlagPair,
} from "@/lib/publicIpFlags";
import {
  computeNodeDegrees,
  edgeColor,
  extractPublicIp,
  flagNodeImageDataUrl,
  ipCircleColor,
  isPodNode,
  podColor,
  podNodeImageDataUrl,
  scaledNodeSize,
} from "@/lib/sigmaGraphStyle";

type Props = {
  graph: SigmaGraph | null;
  topology: FlowTopology | null;
  selectedEdgeId: string | null;
  onSelectEdge: (edge: TopologyEdge | null) => void;
  onPublicIpFlags?: (pairs: PublicIpFlagPair[]) => void;
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

function buildGraphology(graph: SigmaGraph): Graph {
  const degrees = computeNodeDegrees(graph);
  const maxDegree = Math.max(0, ...degrees.values());
  const g = new Graph({ multi: true });

  for (const n of graph.nodes) {
    if (g.hasNode(n.id)) continue;
    const degree = degrees.get(n.id) ?? 0;
    const size = scaledNodeSize(Math.max(8, n.size), degree, maxDegree);
    const publicIp = extractPublicIp(n);

    if (isPodNode(n)) {
      const color = podColor(n.id);
      g.addNode(n.id, {
        x: n.x,
        y: n.y,
        label: n.label,
        size: Math.max(size, 16),
        color,
        type: "image",
        image: podNodeImageDataUrl(color),
        nodeKind: "pod",
        baseSize: size,
      });
      continue;
    }

    if (publicIp) {
      g.addNode(n.id, {
        x: n.x,
        y: n.y,
        label: publicIp,
        size: Math.max(size, 14),
        color: "#0f172a",
        type: "circle",
        nodeKind: "public-ip",
        publicIp,
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
      color: ipCircleColor(ipKey),
      type: "circle",
      nodeKind: "endpoint",
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
        color: edgeColor(key),
        baseColor: edgeColor(key),
        topologyEdgeId: e.topology_edge_id || e.id,
      });
    } catch {
      // skip duplicate parallel keys
    }
  }
  return g;
}

function graphSignature(graph: SigmaGraph): string {
  const nodeSig = graph.nodes.map((n) => `${n.id}:${n.label}:${n.type}:${n.tracked}`).join("|");
  const edgeSig = (graph.edges ?? [])
    .map((e) => `${e.id}:${e.source}:${e.target}:${e.topology_edge_id}`)
    .join("|");
  return `${nodeSig}::${edgeSig}`;
}

function GraphLoader({ graph, signature }: { graph: SigmaGraph; signature: string }) {
  const loadGraph = useLoadGraph();

  useEffect(() => {
    loadGraph(buildGraphology(graph), true);
  }, [graph, signature, loadGraph]);

  return null;
}

function SigmaGraphGeoEnricher({
  graph,
  signature,
  onPublicIpFlags,
}: {
  graph: SigmaGraph;
  signature: string;
  onPublicIpFlags?: (pairs: PublicIpFlagPair[]) => void;
}) {
  const sigma = useSigma();

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      const map = await loadIpCountryMap();
      const g = sigma.getGraph();
      const publicIps = graph.nodes.map(extractPublicIp).filter((ip): ip is string => !!ip);
      const resolved: Record<string, string> = {};

      for (const node of graph.nodes) {
        const ip = extractPublicIp(node);
        if (!ip || !g.hasNode(node.id)) continue;

        const mapped = lookupCountryFromMap(ip, map);
        if (mapped) {
          resolved[ip] = mapped;
          const pair = pairFromCountry(ip, mapped);
          g.setNodeAttribute(node.id, "type", "image");
          g.setNodeAttribute(node.id, "image", flagNodeImageDataUrl(pair.countryCode, pair.flag));
          g.setNodeAttribute(node.id, "label", ip);
        }
      }

      if (!cancelled) {
        onPublicIpFlags?.(compilePublicIpFlagPairs(publicIps, map, resolved));
        sigma.refresh();
      }

      await Promise.all(
        graph.nodes.map(async (node) => {
          const ip = extractPublicIp(node);
          if (!ip || !g.hasNode(node.id) || resolved[ip]) return;

          const geo = await resolveIPGeo(ip);
          const cc = geo?.countryCode || "ZZ";
          if (cancelled || !g.hasNode(node.id)) return;

          resolved[ip] = cc;
          const pair = pairFromCountry(ip, cc);
          g.setNodeAttribute(node.id, "type", "image");
          g.setNodeAttribute(node.id, "image", flagNodeImageDataUrl(pair.countryCode, pair.flag));
          g.setNodeAttribute(node.id, "label", ip);
        })
      );

      if (!cancelled) {
        onPublicIpFlags?.(compilePublicIpFlagPairs(publicIps, map, resolved));
        sigma.refresh();
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [graph, signature, sigma, onPublicIpFlags]);

  return null;
}

function SigmaGraphBehaviors({
  topology,
  selectedEdgeId,
  onSelectEdge,
}: {
  topology: FlowTopology | null;
  selectedEdgeId: string | null;
  onSelectEdge: (edge: TopologyEdge | null) => void;
}) {
  const registerEvents = useRegisterEvents();
  const setSettings = useSetSettings();
  const sigma = useSigma();
  const [draggedNode, setDraggedNode] = useState<string | null>(null);

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
      clickEdge: ({ edge }) => {
        const g = sigma.getGraph();
        const attrs = g.getEdgeAttributes(edge);
        const eid = String(attrs.topologyEdgeId || edge);
        const te =
          topology?.edges?.find((x) => x.id === eid) ??
          ({
            id: eid,
            from: g.source(edge),
            to: g.target(edge),
            health: "healthy",
            count: 0,
            bytes: 0,
            packets: 0,
          } as TopologyEdge);
        onSelectEdge(te);
      },
      clickStage: () => onSelectEdge(null),
    });
  }, [registerEvents, sigma, draggedNode, topology, onSelectEdge]);

  useEffect(() => {
    setSettings({
      nodeReducer: (node, data) => {
        const g = sigma.getGraph();
        const next = { ...data };
        const baseSize = g.getNodeAttribute(node, "baseSize") ?? data.size ?? 10;
        next.size = baseSize;
        if (g.getNodeAttribute(node, "highlighted")) {
          next.highlighted = true;
          next.size = baseSize * 1.28;
          next.zIndex = 2;
        }
        return next;
      },
      edgeReducer: (edge, data) => {
        const g = sigma.getGraph();
        const attrs = g.getEdgeAttributes(edge);
        const eid = String(attrs.topologyEdgeId || "");
        const baseColor = String(attrs.baseColor || attrs.color || edgeColor(edge));
        const noLabel = { ...data, label: "" };
        if (selectedEdgeId && eid && eid !== selectedEdgeId) {
          return { ...noLabel, color: "#334155", size: 1 };
        }
        if (selectedEdgeId && eid === selectedEdgeId) {
          return { ...noLabel, color: "#c4ecff", size: 4, zIndex: 1 };
        }
        return { ...noLabel, color: baseColor, size: Math.max(2, data.size ?? 2) };
      },
    });
  }, [selectedEdgeId, setSettings, sigma, draggedNode]);

  return null;
}

function SigmaGraphView({
  graph,
  topology,
  selectedEdgeId,
  onSelectEdge,
  onPublicIpFlags,
  publicIpFlags,
}: {
  graph: SigmaGraph;
  topology: FlowTopology | null;
  selectedEdgeId: string | null;
  onSelectEdge: (edge: TopologyEdge | null) => void;
  onPublicIpFlags?: (pairs: PublicIpFlagPair[]) => void;
  publicIpFlags: PublicIpFlagPair[];
}) {
  const signature = useMemo(() => graphSignature(graph), [graph]);

  return (
    <>
      <SigmaContainer className="sigma-topology-view !absolute !inset-0 !size-full" settings={SIGMA_SETTINGS}>
        <GraphLoader graph={graph} signature={signature} />
        <SigmaGraphGeoEnricher graph={graph} signature={signature} onPublicIpFlags={onPublicIpFlags} />
        <SigmaGraphBehaviors
          topology={topology}
          selectedEdgeId={selectedEdgeId}
          onSelectEdge={onSelectEdge}
        />
        <ControlsContainer position="bottom-right">
          <ZoomControl labels={{ zoomIn: "Zoom in", zoomOut: "Zoom out", reset: "Fit graph" }} />
        </ControlsContainer>
      </SigmaContainer>
      {publicIpFlags.length > 0 && (
        <div className="public-ip-flag-list">
          <div className="public-ip-flag-list-inner">
            {publicIpFlags.map(({ ip, flag, countryCode }) => (
              <span key={ip}>
                {ip} {flag} {countryCode}
              </span>
            ))}
          </div>
        </div>
      )}
    </>
  );
}

export function SigmaTopologyGraph({
  graph,
  topology,
  selectedEdgeId,
  onSelectEdge,
  onPublicIpFlags,
}: Props) {
  const [publicIpFlags, setPublicIpFlags] = useState<PublicIpFlagPair[]>([]);

  const handlePublicIpFlags = (pairs: PublicIpFlagPair[]) => {
    setPublicIpFlags(pairs);
    onPublicIpFlags?.(pairs);
  };

  if (!graph?.nodes?.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Syncing topology to graph store… refresh or run capture traffic.
      </div>
    );
  }

  return (
    <div className="h-full min-h-0 relative sigma-topology-shell fluent-graph-stage">
      <SigmaGraphView
        graph={graph}
        topology={topology}
        selectedEdgeId={selectedEdgeId}
        onSelectEdge={onSelectEdge}
        onPublicIpFlags={handlePublicIpFlags}
        publicIpFlags={publicIpFlags}
      />
      <p className="absolute top-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10">
        nodes {graph.nodes.length} · links {(graph.edges ?? []).length}
      </p>
      <p className="absolute bottom-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10 max-w-[40%] text-right">
        Click edge for flow details · drag nodes · scroll to zoom
      </p>
    </div>
  );
}
