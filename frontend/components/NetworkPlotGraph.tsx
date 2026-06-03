"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FlowTopology, TopologyEdge } from "@/lib/ai";
import { flowTopologyToNetworkPlot } from "@/lib/networkplot/convert";
import { resolveIPGeo } from "@/lib/networkplot/geo";
import { edgeBezierPath, layoutRadialGraph } from "@/lib/networkplot/layout";

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

function countryColor(code: string): string {
  if (!code) return "#7dd3fc";
  const palette = ["#60a5fa", "#a78bfa", "#34d399", "#fbbf24", "#f472b6", "#22d3ee", "#fb7185", "#93c5fd"];
  let hash = 0;
  for (const ch of code) hash = (hash * 33 + ch.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

function linkStroke(l: { edgeType: string; countryCode: string }, selected: boolean, dimmed: boolean): string {
  if (dimmed) return "rgba(120,140,170,0.35)";
  if (l.edgeType === "snat") return "#fb7185";
  if (l.edgeType === "scheduled") return "#fbbf24";
  if (l.countryCode) return countryColor(l.countryCode);
  return "#7dd3fc";
}

export function NetworkPlotGraph({ topology, trackedPodIds, selectedEdgeId, onSelectEdge }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 800, height: 500 });
  const [flags, setFlags] = useState<Record<string, string>>({});
  const [countryCodeByEdge, setCountryCodeByEdge] = useState<Record<string, string>>({});
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const dragRef = useRef<{ active: boolean; x: number; y: number; px: number; py: number } | null>(null);

  const plot = useMemo(
    () => flowTopologyToNetworkPlot(topology, trackedPodIds),
    [topology, trackedPodIds]
  );

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const measure = () => {
      const rect = el.getBoundingClientRect();
      const w = Math.max(200, Math.floor(rect.width));
      const h = Math.max(200, Math.floor(rect.height));
      setSize((prev) => (prev.width === w && prev.height === h ? prev : { width: w, height: h }));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (!plot?.links?.length) return;
    let active = true;
    void (async () => {
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
    })();
    return () => {
      active = false;
    };
  }, [plot]);

  const layout = useMemo(() => {
    if (!plot?.nodes.length) return null;
    const links = plot.links.map((l) => ({
      ...l,
      flag: flags[l.id] || "",
      countryCode: countryCodeByEdge[l.id] || "",
    }));
    return layoutRadialGraph(plot.nodes, links, size.width, size.height);
  }, [plot, flags, countryCodeByEdge, size.width, size.height]);

  const nodeById = useMemo(() => {
    const m = new Map<string, { x: number; y: number; label: string }>();
    for (const n of layout?.nodes ?? []) m.set(n.id, n);
    return m;
  }, [layout]);

  const resetView = useCallback(() => {
    setPan({ x: 0, y: 0 });
    setZoom(1);
  }, []);

  useEffect(() => {
    resetView();
  }, [layout?.nodes.length, layout?.links.length, resetView]);

  const onWheel = (e: React.WheelEvent) => {
    e.preventDefault();
    const factor = e.deltaY > 0 ? 0.92 : 1.08;
    setZoom((z) => Math.min(4, Math.max(0.35, z * factor)));
  };

  const onPointerDown = (e: React.PointerEvent) => {
    if (e.button !== 0) return;
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
    dragRef.current = { active: true, x: e.clientX, y: e.clientY, px: pan.x, py: pan.y };
  };

  const onPointerMove = (e: React.PointerEvent) => {
    const d = dragRef.current;
    if (!d?.active) return;
    setPan({ x: d.px + (e.clientX - d.x), y: d.py + (e.clientY - d.y) });
  };

  const onPointerUp = () => {
    if (dragRef.current) dragRef.current.active = false;
  };

  if (!plot?.nodes.length) {
    return (
      <div className="h-full min-h-0 flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        No observed flows involving selected pods yet. Run capture, generate traffic, then refresh.
      </div>
    );
  }

  if (!layout) return null;

  return (
    <div className="flex h-full min-h-0 fluent-graph-stage overflow-hidden">
      <aside className="fluent-graph-legend">
        <div className="flex gap-1 mb-3">
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={() => setZoom((z) => Math.min(4, z * 1.15))}
          >
            +
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={() => setZoom((z) => Math.max(0.35, z / 1.15))}
          >
            −
          </button>
          <button
            type="button"
            className="px-2 py-1 rounded border border-siem-border text-siem-muted hover:bg-siem-card"
            onClick={resetView}
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
      </aside>

      <div
        ref={wrapRef}
        className="flex-1 min-w-0 min-h-0 relative overflow-hidden fluent-graph-stage cursor-grab active:cursor-grabbing"
        onWheel={onWheel}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerLeave={onPointerUp}
      >
        <svg
          width="100%"
          height="100%"
          viewBox={`0 0 ${layout.width} ${layout.height}`}
          className="block select-none"
          onClick={() => onSelectEdge(null)}
        >
          <g
            transform={`translate(${layout.width / 2 + pan.x},${layout.height / 2 + pan.y}) scale(${zoom}) translate(${-layout.width / 2},${-layout.height / 2})`}
          >
                {layout.links.map((l) => {
                  const s = nodeById.get(l.source);
                  const t = nodeById.get(l.target);
                  if (!s || !t) return null;
                  const selected = selectedEdgeId === l.topologyEdgeId;
                  const dimmed = Boolean(selectedEdgeId && !selected);
                  const d = edgeBezierPath(s.x, s.y, t.x, t.y, l.curvature);
                  const stroke = linkStroke(l, selected, dimmed);
                  const mx = (s.x + t.x) / 2;
                  const my = (s.y + t.y) / 2;
                  const label = `${l.flag ? `${l.flag} ` : ""}${l.label}`.trim();
                  return (
                    <g key={l.id}>
                      <path
                        d={d}
                        fill="none"
                        stroke="transparent"
                        strokeWidth={14}
                        style={{ cursor: "pointer" }}
                        onClick={(e) => {
                          e.stopPropagation();
                          const te = topology?.edges.find((x) => x.id === l.topologyEdgeId) ?? null;
                          onSelectEdge(te);
                        }}
                      />
                      <path
                        d={d}
                        fill="none"
                        stroke={stroke}
                        strokeWidth={selected ? 3 : 2}
                        strokeOpacity={dimmed ? 0.35 : 1}
                        pointerEvents="none"
                      />
                      <text
                        x={mx}
                        y={my - 6}
                        textAnchor="middle"
                        fontSize={9}
                        fill="#d7e6ff"
                        pointerEvents="none"
                      >
                        {label.length > 28 ? `${label.slice(0, 26)}…` : label}
                      </text>
                    </g>
                  );
                })}

                {layout.nodes.map((n) => {
                  const r = n.size || 8;
                  return (
                    <g key={n.id}>
                      <circle cx={n.x} cy={n.y} r={r + 3} fill="rgba(103,232,249,0.15)" />
                      <circle
                        cx={n.x}
                        cy={n.y}
                        r={r}
                        fill={n.tracked ? "#2b5fe2" : n.color}
                        stroke={n.border}
                        strokeWidth={n.tracked ? 2 : 1.2}
                      />
                      <text
                        x={n.x}
                        y={n.y + r + 12}
                        textAnchor="middle"
                        fontSize={10}
                        fontWeight={600}
                        fill="#eaf2ff"
                      >
                        {n.label.length > 22 ? `${n.label.slice(0, 20)}…` : n.label}
                      </text>
                    </g>
                  );
                })}
          </g>
        </svg>

        <p className="absolute top-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10">
          nodes {layout.nodes.length} · links {layout.links.length} · SVG
        </p>
        <p className="absolute bottom-1 right-2 text-[9px] text-siem-muted pointer-events-none z-10">
          Drag to pan · scroll to zoom · click edge for details
        </p>
      </div>
    </div>
  );
}
