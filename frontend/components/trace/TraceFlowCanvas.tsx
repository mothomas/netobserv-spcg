"use client";

import type { TraceEdge, TraceGraph, TraceNode } from "@/lib/trace";

type Props = {
  graph: TraceGraph;
  animate?: boolean;
};

function edgePath(from: TraceNode, to: TraceNode, nodeW: number, nodeH: number): string {
  const x1 = from.x + nodeW;
  const y1 = from.y + nodeH / 2;
  const x2 = to.x;
  const y2 = to.y + nodeH / 2;
  const mx = (x1 + x2) / 2;
  return `M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`;
}

function FlowPacket({
  pathD,
  color,
  outline,
  dur,
  delay,
  frozen,
  dropColor,
}: {
  pathD: string;
  color: string;
  outline: string;
  dur: number;
  delay: number;
  frozen?: boolean;
  dropColor?: string;
}) {
  if (!pathD) return null;
  const size = 14;
  const half = size / 2;
  const pathId = `${frozen ? "frozen" : "motion"}-${delay}`;
  const stopFill = dropColor ?? color;
  return (
    <g>
      <path id={pathId} d={pathD} fill="none" stroke="none" />
      <rect x={-half} y={-half} width={size} height={size} fill={color} stroke={outline} strokeWidth={2} rx={2}>
        {frozen ? (
          <>
            <animateMotion dur="3.4s" fill="freeze" begin={`${delay}s`} keyPoints="0;0.55;0.55" keyTimes="0;0.82;1" calcMode="linear">
              <mpath href={`#${pathId}`} />
            </animateMotion>
            <animate attributeName="fill" values={`${color};${color};${stopFill}`} keyTimes="0;0.82;1" dur="3.4s" begin={`${delay}s`} fill="freeze" />
          </>
        ) : (
          <animateMotion dur={`${dur}s`} repeatCount="indefinite" begin={`${delay}s`} calcMode="linear">
            <mpath href={`#${pathId}`} />
          </animateMotion>
        )}
      </rect>
    </g>
  );
}

export function TraceFlowCanvas({ graph, animate = true }: Props) {
  const nodeMap = new Map(graph.nodes.map((n) => [n.id, n]));
  const defaultW = graph.nodes[0]?.width ?? 148;
  const defaultH = graph.nodes[0]?.height ?? 72;
  const width = Math.max(graph.width, 1400);
  const height = Math.max(graph.height, 420);

  const primaryEdges = graph.edges.filter((e) => e.primary);
  const dropEdge = graph.edges.find((e) => e.drop);

  function pathForEdge(e: TraceEdge): string {
    const a = nodeMap.get(e.from);
    const b = nodeMap.get(e.to);
    if (!a || !b) return "";
    return edgePath(a, b, a.width || defaultW, a.height || defaultH);
  }

  return (
    <div className="space-y-3">
      <div
        className="overflow-auto rounded-siem border border-siem-border-hi max-h-[62vh] min-h-[480px]"
        style={{ background: "var(--siem-graph-bg)" }}
      >
        <svg
          width={width}
          height={height}
          viewBox={`0 0 ${width} ${height}`}
          preserveAspectRatio="xMinYMid meet"
          className="block"
          role="img"
          aria-label="Source to destination path map"
        >
          {graph.lanes?.map((lane) => (
            <g key={`${lane.rank}-${lane.label}`}>
              <rect
                x={lane.x}
                y={12}
                width={lane.width}
                height={height - 24}
                fill="rgba(255,255,255,0.02)"
                stroke="var(--siem-border)"
                rx={8}
              />
              <text x={lane.x + 14} y={34} fill="var(--siem-muted)" fontSize={12} fontWeight={600}>
                {lane.label}
              </text>
            </g>
          ))}

          {graph.edges.map((e) => {
            const d = pathForEdge(e);
            const dim = !e.primary && !e.drop;
            const stroke = e.drop ? "var(--siem-err)" : e.primary ? "var(--siem-accent)" : "var(--siem-border-hi)";
            return (
              <path
                key={e.id}
                d={d}
                fill="none"
                stroke={stroke}
                strokeWidth={e.drop ? 4 : e.primary ? 3.5 : 1.5}
                strokeDasharray={e.drop ? "10 6" : e.primary ? undefined : "4 8"}
                opacity={dim ? 0.22 : 1}
                strokeLinecap="round"
              />
            );
          })}

          {animate &&
            primaryEdges.slice(0, 8).map((e, i) => (
              <FlowPacket
                key={`pkt-${e.id}`}
                pathD={pathForEdge(e)}
                color="var(--siem-ok)"
                outline="var(--siem-graph-bg)"
                dur={2.2 + i * 0.08}
                delay={i * 0.35}
              />
            ))}
          {animate && dropEdge && (
            <FlowPacket
              pathD={pathForEdge(dropEdge)}
              color="var(--siem-ok)"
              dropColor="var(--siem-err)"
              outline="var(--siem-graph-bg)"
              dur={3.4}
              delay={2.8}
              frozen
            />
          )}

          {graph.nodes.map((n) => {
            const w = n.width || defaultW;
            const h = n.height || defaultH;
            const endpoint = n.tracked;
            const onPath = n.focused || endpoint;
            const dim = !onPath;
            return (
              <g key={n.id} transform={`translate(${n.x}, ${n.y})`} opacity={dim ? 0.28 : 1}>
                <rect
                  width={w}
                  height={h}
                  rx={8}
                  fill={endpoint ? "rgba(96, 205, 255, 0.16)" : onPath ? "rgba(96, 205, 255, 0.08)" : "var(--siem-card)"}
                  stroke={endpoint ? "var(--siem-accent)" : onPath ? "var(--siem-accent)" : "var(--siem-border-hi)"}
                  strokeWidth={endpoint ? 3 : onPath ? 2.5 : 1.5}
                />
                <text x={w / 2} y={28} textAnchor="middle" fill="var(--siem-text)" fontSize={14} fontWeight={700}>
                  {truncateLabel(n.label, 18)}
                </text>
                <text x={w / 2} y={50} textAnchor="middle" fill="var(--siem-muted)" fontSize={11}>
                  {truncateLabel(n.detail || n.kind, 22)}
                </text>
              </g>
            );
          })}
        </svg>
      </div>

      <div className="flex flex-wrap gap-4 text-xs text-siem-muted px-1">
        <LegendSwatch color="var(--siem-accent)" label="Focused path (source → destination)" />
        <LegendSwatch color="var(--siem-border-hi)" label="Discovered context (dimmed)" dashed />
        <LegendSwatch color="var(--siem-ok)" label="Packet animation on focused path" square />
      </div>
    </div>
  );
}

function LegendSwatch({ color, label, dashed, square }: { color: string; label: string; dashed?: boolean; square?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2">
      {square ? (
        <span className="inline-block w-3 h-3 rounded-sm" style={{ background: color }} />
      ) : (
        <span
          className="inline-block w-8 h-0.5"
          style={{
            background: dashed ? "transparent" : color,
            borderTop: dashed ? `2px dashed ${color}` : undefined,
          }}
        />
      )}
      {label}
    </span>
  );
}

function truncateLabel(s: string, max: number) {
  if (s.length <= max) return s;
  return `${s.slice(0, max - 1)}…`;
}
