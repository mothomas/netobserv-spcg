"use client";

import dynamic from "next/dynamic";
import { useEffect, useMemo, useRef, useState } from "react";
import { forceCollide } from "d3-force";
import type { FlowTopology, TopologyEdge } from "@/lib/ai";
import { flowTopologyToNetworkPlot, type ForceLink, type ForceNode } from "@/lib/networkplot/convert";
import { resolveIPGeo } from "@/lib/networkplot/geo";

const ForceGraph2D = dynamic(() => import("react-force-graph-2d"), { ssr: false });

type Props = {
  topology: FlowTopology | null;
  trackedPodIds: string[];
  selectedEdgeId: string | null;
  onSelectEdge: (edge: TopologyEdge | null) => void;
};

const LEGEND: [string, string][] = [
  ["pod", "Tracked pod / workload"],
  ["service-clusterip", "K8s service peer"],
  ["external", "External endpoint"],
  ["direct", "Healthy traffic"],
  ["scheduled", "Degraded latency"],
  ["snat", "Dropped / reset flow"],
];

export function NetworkPlotGraph({ topology, trackedPodIds, selectedEdgeId, onSelectEdge }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const [size, setSize] = useState({ width: 900, height: 560 });
  const [flags, setFlags] = useState<Record<string, string>>({});
  const [countryCodeByEdge, setCountryCodeByEdge] = useState<Record<string, string>>({});

  const plot = useMemo(
    () => flowTopologyToNetworkPlot(topology, trackedPodIds),
    [topology, trackedPodIds]
  );

  useEffect(() => {
    if (!wrapRef.current) return;
    const ro = new ResizeObserver(([entry]) => {
      const w = Math.max(420, Math.round(entry.contentRect.width));
      const h = Math.max(520, Math.round(entry.contentRect.height));
      setSize({ width: w, height: h });
    });
    ro.observe(wrapRef.current);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (!plot?.links?.length) return;
    let active = true;
    const collectFlags = async () => {
      const next: Record<string, string> = {};
      const ccMap: Record<string, string> = {};
      await Promise.all(
        plot.links.map(async (l) => {
          if (!l.externalIp) return;
          const geo = await resolveIPGeo(l.externalIp);
          if (geo) {
            next[l.id] = geo.flagEmoji;
            ccMap[l.id] = geo.countryCode;
          }
        })
      );
      if (active) {
        setFlags(next);
        setCountryCodeByEdge(ccMap);
      }
    };
    void collectFlags();
    return () => {
      active = false;
    };
  }, [plot]);

  const graphData = useMemo(() => {
    const nodes = plot?.nodes ?? [];
    const links = (plot?.links ?? []).map((l) => ({
      ...l,
      flag: flags[l.id] || "",
      countryCode: countryCodeByEdge[l.id] || "",
    }));
    return { nodes, links };
  }, [plot, flags, countryCodeByEdge]);

  const countryColor = (code: string): string => {
    if (!code) return "#67e8f9";
    const palette = ["#60a5fa", "#a78bfa", "#34d399", "#fbbf24", "#f472b6", "#22d3ee", "#fb7185", "#93c5fd"];
    let hash = 0;
    for (const ch of code) hash = (hash * 33 + ch.charCodeAt(0)) | 0;
    return palette[Math.abs(hash) % palette.length];
  };

  useEffect(() => {
    if (!fgRef.current || !graphData.nodes.length) return;
    fgRef.current.d3Force("charge")?.strength(-420);
    fgRef.current.d3Force("link")?.distance((l: any) => Math.max(300, Number(l.distance || 330)));
    fgRef.current.d3Force("link")?.strength(0.17);
    fgRef.current.d3Force("center")?.strength(0.12);
    fgRef.current.d3Force(
      "collide",
      forceCollide((n: any) => {
        const r = Number(n.size || 8);
        return r + 28;
      }).iterations(2)
    );
    fgRef.current.d3VelocityDecay(0.32);
    fgRef.current.d3AlphaDecay(0.03);
    fgRef.current.d3ReheatSimulation();
    const id = window.setTimeout(() => {
      try {
        fgRef.current.zoomToFit(550, 16);
      } catch {
        // no-op
      }
    }, 180);
    return () => window.clearTimeout(id);
  }, [graphData.nodes.length, graphData.links.length]);

  if (!trackedPodIds.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Select pods in the workspace before capture to scope the topology to your tenant.
      </div>
    );
  }

  if (!plot?.nodes.length) {
    return (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        No observed flows involving selected pods yet. Run capture, generate traffic, then refresh.
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-[520px] bg-[#0d121c]">
      <aside className="w-[170px] shrink-0 border-r border-siem-border bg-siem-panel/95 p-3 overflow-y-auto text-xs">
        <div className="flex gap-1 mb-3">
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={() => fgRef.current?.zoom((fgRef.current.zoom() || 1) * 1.2, 220)}
          >
            +
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={() => fgRef.current?.zoom((fgRef.current.zoom() || 1) / 1.2, 220)}
          >
            −
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={() => fgRef.current?.zoomToFit(420, 24)}
          >
            fit
          </button>
        </div>
        <p className="font-semibold text-siem-text mb-2">Topology legend</p>
        {LEGEND.map(([key, label]) => (
          <div key={key} className="text-siem-muted mb-1.5 leading-snug">
            {label}
          </div>
        ))}
        <p className="mt-3 text-[10px] text-siem-muted border-t border-siem-border pt-2 leading-relaxed">
          Force layout keeps dense topologies readable; concentric fallback applies for very large graphs.
        </p>
      </aside>
      <div ref={wrapRef} className="flex-1 relative min-w-0">
        <ForceGraph2D
          ref={fgRef}
          width={size.width}
          height={size.height}
          graphData={graphData}
          cooldownTicks={180}
          nodeRelSize={5}
          linkCurvature={(l: any) => l.curvature || 0}
          linkWidth={(l: any) => l.width || 1}
          linkDirectionalParticles={(l: any) => (selectedEdgeId && l.topologyEdgeId === selectedEdgeId ? 3 : 0)}
          linkDirectionalParticleWidth={2}
          linkDirectionalParticleSpeed={() => 0.006}
          linkColor={(l: any) => {
            if (selectedEdgeId && l.topologyEdgeId !== selectedEdgeId) return "rgba(120,140,170,0.14)";
            if (l.edgeType === "snat") return "#fb7185";
            if (l.edgeType === "scheduled") return "#fbbf24";
            if (l.countryCode) return countryColor(String(l.countryCode));
            return "#67e8f9";
          }}
          onBackgroundClick={() => onSelectEdge(null)}
          onLinkClick={(l: any) => {
            const te = topology?.edges.find((x) => x.id === l.topologyEdgeId) ?? null;
            onSelectEdge(te);
          }}
          nodeCanvasObject={(node: any, ctx, globalScale) => {
            const n = node as ForceNode & { x?: number; y?: number };
            if (typeof n.x !== "number" || typeof n.y !== "number") return;
            const r = n.size || 8;
            const grd = ctx.createRadialGradient(n.x - r / 3, n.y - r / 3, 1, n.x, n.y, r + 7);
            grd.addColorStop(0, n.tracked ? "#c6d8ff" : n.color);
            grd.addColorStop(1, n.tracked ? "#2b5fe2" : "#0f2038");
            ctx.beginPath();
            ctx.arc(n.x, n.y, r + 2, 0, 2 * Math.PI);
            ctx.fillStyle = "rgba(103,232,249,0.16)";
            ctx.fill();
            ctx.beginPath();
            ctx.arc(n.x, n.y, r, 0, 2 * Math.PI);
            ctx.fillStyle = grd;
            ctx.fill();
            ctx.lineWidth = n.tracked ? 2.2 : 1.2;
            ctx.strokeStyle = n.border;
            ctx.stroke();

            const fontSize = Math.max(6.5, 9 / globalScale);
            const showLabel = n.tracked || globalScale > 1.75;
            ctx.font = `600 ${fontSize}px Inter, sans-serif`;
            ctx.textAlign = "center";
            ctx.fillStyle = "#eaf2ff";
            if (showLabel) {
              ctx.fillText(n.label, n.x, n.y + r + fontSize + 2);
            }
          }}
          linkCanvasObjectMode={() => "after"}
          linkCanvasObject={(link: any, ctx) => {
            if ((!link.label && !link.flag) || typeof link.source !== "object" || typeof link.target !== "object") return;
            const sx = (link.source as any).x;
            const sy = (link.source as any).y;
            const tx = (link.target as any).x;
            const ty = (link.target as any).y;
            if (![sx, sy, tx, ty].every((v) => typeof v === "number")) return;
            const mx = sx + (tx - sx) * 0.55;
            const my = sy + (ty - sy) * 0.55 - 8;
            const txt = selectedEdgeId && link.topologyEdgeId !== selectedEdgeId ? "" : `${link.flag ? `${link.flag} ` : ""}${link.label || ""}`.trim();
            if (!txt) return;
            ctx.font = "500 8px Inter, sans-serif";
            const pad = 4;
            const w = ctx.measureText(txt).width + pad * 2;
            ctx.fillStyle = "rgba(2,6,23,0.72)";
            ctx.fillRect(mx - w / 2, my - 10, w, 14);
            ctx.fillStyle = "#d7e6ff";
            ctx.textAlign = "center";
            ctx.fillText(txt, mx, my);
          }}
        />
        <p className="absolute bottom-1 right-2 text-[10px] text-siem-muted pointer-events-none">
          Click edge for flow details
        </p>
      </div>
    </div>
  );
}
