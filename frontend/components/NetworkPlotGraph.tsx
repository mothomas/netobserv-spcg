"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import type cytoscape from "cytoscape";
import type { Core, EdgeSingular, StylesheetJson } from "cytoscape";
import type { FlowTopology, TopologyEdge } from "@/lib/ai";
import { CYTOSCAPE_STYLES } from "@/lib/networkplot/cytoscape-styles";
import { flowTopologyToNetworkPlot } from "@/lib/networkplot/convert";

type Props = {
  topology: FlowTopology | null;
  trackedPodIds: string[];
  selectedEdgeId: string | null;
  onSelectEdge: (edge: TopologyEdge | null) => void;
};

const LEGEND: [string, string][] = [
  ["pod", "Selected pod (solid)"],
  ["service-clusterip", "Direct peer (dashed)"],
  ["external", "External peer"],
  ["direct", "Observed flow (green)"],
  ["scheduled", "Degraded flow (amber)"],
  ["snat", "Drop / error (orange dashed)"],
];

export function NetworkPlotGraph({ topology, trackedPodIds, selectedEdgeId, onSelectEdge }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const cyRef = useRef<Core | null>(null);
  const [ready, setReady] = useState(false);

  const plot = useMemo(
    () => flowTopologyToNetworkPlot(topology, trackedPodIds),
    [topology, trackedPodIds]
  );

  useEffect(() => {
    if (!containerRef.current || !plot) return;
    let destroyed = false;

    (async () => {
      const cytoscape = (await import("cytoscape")).default;
      const dagre = (await import("cytoscape-dagre")).default;
      cytoscape.use(dagre);

      if (destroyed || !containerRef.current) return;
      if (cyRef.current) {
        cyRef.current.destroy();
        cyRef.current = null;
      }

      const cy = cytoscape({
        container: containerRef.current,
        elements: [...plot.nodes, ...plot.edges],
        style: CYTOSCAPE_STYLES as StylesheetJson,
        layout: {
          name: "dagre",
          rankDir: "TB",
          nodeSep: 50,
          rankSep: 90,
          animate: false,
          fit: false,
          padding: 32,
          nodeDimensionsIncludeLabels: true,
        } as cytoscape.LayoutOptions,
        wheelSensitivity: 0.3,
      });

      const fit = () => {
        if (cy.elements().length) {
          cy.resize();
          cy.fit(cy.elements(), 40);
        }
      };
      cy.on("layoutstop", fit);
      cy.ready(fit);

      cy.on("tap", (e) => {
        if (e.target === cy) {
          onSelectEdge(null);
          cy.elements().removeClass("faded hl-edge");
          return;
        }
      });

      cy.on("tap", "edge", (evt) => {
        const edge = evt.target as EdgeSingular;
        const edgeId = edge.data("topologyEdgeId") as string;
        const te = topology?.edges.find((x) => x.id === edgeId) ?? null;
        onSelectEdge(te);
        cy.elements().addClass("faded");
        edge.removeClass("faded").addClass("hl-edge");
        edge.source().removeClass("faded");
        edge.target().removeClass("faded");
      });

      cyRef.current = cy;
      setReady(true);
    })();

    return () => {
      destroyed = true;
      if (cyRef.current) {
        cyRef.current.destroy();
        cyRef.current = null;
      }
      setReady(false);
    };
  }, [plot, topology, onSelectEdge]);

  useEffect(() => {
    const cy = cyRef.current;
    if (!cy || !ready) return;
    cy.edges().removeClass("hl-edge faded");
    if (selectedEdgeId) {
      const edge = cy.edges(`[topologyEdgeId = "${selectedEdgeId}"]`).first() as EdgeSingular | undefined;
      if (edge) {
        cy.elements().addClass("faded");
        edge.removeClass("faded").addClass("hl-edge");
        edge.source().removeClass("faded");
        edge.target().removeClass("faded");
      }
    }
  }, [selectedEdgeId, ready]);

  if (!trackedPodIds.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-slate-500 px-6 text-center">
        Select pods in the workspace before capture to scope the topology to your tenant.
      </div>
    );
  }

  if (!plot?.nodes.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-slate-500 px-6 text-center">
        No observed flows involving selected pods yet. Run capture, generate traffic, then refresh.
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-[360px] bg-[#f8f9fa]">
      <aside className="w-[200px] shrink-0 border-r border-slate-200 bg-white p-3 overflow-y-auto text-xs">
        <div className="flex gap-1 mb-3">
          <button
            type="button"
            className="px-2 py-1 rounded border border-slate-300 hover:bg-slate-50"
            onClick={() => cyRef.current?.zoom(cyRef.current.zoom() * 1.2)}
          >
            +
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-slate-300 hover:bg-slate-50"
            onClick={() => cyRef.current?.zoom(cyRef.current.zoom() / 1.2)}
          >
            −
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-slate-300 hover:bg-slate-50"
            onClick={() => cyRef.current?.fit(cyRef.current.elements(), 40)}
          >
            fit
          </button>
        </div>
        <p className="font-semibold text-slate-700 mb-2">Legend</p>
        {LEGEND.map(([key, label]) => (
          <div key={key} className="text-slate-600 mb-1.5 leading-snug">
            {label}
          </div>
        ))}
        <p className="mt-3 text-[10px] text-slate-400 border-t border-slate-100 pt-2">
          networkplot · K8s icons · tenant-scoped
        </p>
      </aside>
      <div className="flex-1 relative min-w-0">
        <div ref={containerRef} className="absolute inset-0" />
        <p className="absolute bottom-1 right-2 text-[10px] text-slate-400 pointer-events-none">
          Click edge for flow details
        </p>
      </div>
    </div>
  );
}
