"use client";

import type { FlowEdge, FlowGraph } from "@/lib/ai";

type Props = {
  graph: FlowGraph | null;
};

/** Lightweight SVG flow view (no external diagram runtime). */
export function FlowDiagram({ graph }: Props) {
  if (!graph || graph.edges.length === 0) {
    return (
      <p className="text-sm text-slate-400 p-4">
        No K8s-enriched flows yet. Start capture and wait for packets with SrcK8S/DstK8S metadata from the
        netobserv sensor.
      </p>
    );
  }

  const nodes = graph.nodes.length > 0 ? graph.nodes : uniqueNodes(graph.edges);
  const positions = layoutNodes(nodes);
  const maxX = Math.max(...Object.values(positions).map((p) => p.x), 1);
  const w = Math.min(900, 120 + maxX * 200);
  const h = 80 + nodes.length * 48;

  return (
    <div className="overflow-auto p-2">
      <svg width={w} height={h} className="text-slate-200">
        <defs>
          <marker id="arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
            <path d="M0,0 L6,3 L0,6 Z" fill="#60a5fa" />
          </marker>
        </defs>
        {graph.edges.map((e, i) => {
          const from = positions[e.from] || { x: 0, y: i * 40 };
          const to = positions[e.to] || { x: 2, y: i * 40 + 20 };
          const x1 = 40 + from.x * 180;
          const y1 = 30 + from.y * 44;
          const x2 = 40 + to.x * 180;
          const y2 = 30 + to.y * 44;
          const label = edgeLabel(e);
          const mx = (x1 + x2) / 2;
          const my = (y1 + y2) / 2 - 8;
          return (
            <g key={i}>
              <line x1={x1 + 80} y1={y1 + 16} x2={x2 + 10} y2={y2 + 16} stroke="#3b82f6" strokeWidth={1.5} markerEnd="url(#arrow)" />
              <text x={mx + 40} y={my + 16} fill="#94a3b8" fontSize={10} textAnchor="middle">
                {label}
              </text>
            </g>
          );
        })}
        {nodes.map((n) => {
          const p = positions[n] || { x: 0, y: 0 };
          const x = 40 + p.x * 180;
          const y = 30 + p.y * 44;
          return (
            <g key={n}>
              <rect x={x} y={y} width={160} height={32} rx={6} fill="#141b2d" stroke="#475569" />
              <text x={x + 8} y={y + 20} fill="#e2e8f0" fontSize={11}>
                {truncate(n, 22)}
              </text>
            </g>
          );
        })}
      </svg>
      <details className="mt-2 text-xs text-slate-500">
        <summary className="cursor-pointer">Mermaid source</summary>
        <pre className="mt-1 p-2 bg-spcg-bg rounded overflow-auto">{graph.mermaid}</pre>
      </details>
    </div>
  );
}

function uniqueNodes(edges: FlowEdge[]): string[] {
  const s = new Set<string>();
  for (const e of edges) {
    s.add(e.from);
    s.add(e.to);
  }
  return [...s];
}

function layoutNodes(nodes: string[]): Record<string, { x: number; y: number }> {
  const pos: Record<string, { x: number; y: number }> = {};
  nodes.forEach((n, i) => {
    pos[n] = { x: i % 4, y: Math.floor(i / 4) };
  });
  return pos;
}

function edgeLabel(e: FlowEdge): string {
  let l = e.proto || "";
  if (e.dst_port) l += (l ? ":" : "") + String(e.dst_port);
  if (!l) l = `${e.count} pkts`;
  else l += ` (${e.count})`;
  return l;
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1) + "…" : s;
}
