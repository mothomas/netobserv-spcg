"use client";

import { useMemo } from "react";
import type { EdgePaintState, TraceEdge, TraceGraph, TraceNode } from "@/lib/trace";

type Props = {
  graph: TraceGraph;
  animate?: boolean;
  edgeStates?: Record<string, EdgePaintState>;
};

const VIEW_PAD = 56;

function edgePath(from: TraceNode, to: TraceNode, nodeW: number, nodeH: number): string {
  const fw = from.width || nodeW;
  const fh = from.height || nodeH;
  const tw = to.width || nodeW;
  const th = to.height || nodeH;
  const x1 = from.x + fw;
  const y1 = from.y + fh / 2;
  const x2 = to.x;
  const y2 = to.y + th / 2;
  const dx = Math.max(40, Math.abs(x2 - x1) * 0.42);
  if (Math.abs(y1 - y2) > 36) {
    const midX = (x1 + x2) / 2;
    return `M ${x1} ${y1} C ${midX} ${y1}, ${midX} ${y2}, ${x2} ${y2}`;
  }
  return `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;
}

function edgePaintStroke(state: EdgePaintState | undefined, e: TraceEdge): string {
  if (e.drop || state === "DROPPED_RED") return "var(--siem-err)";
  if (state === "ACTIVE_GREEN") return "var(--siem-ok)";
  if (e.primary) return "var(--siem-accent)";
  return "var(--siem-border-hi)";
}

function trackStroke(track?: string) {
  switch (track) {
    case "ingress":
      return "rgba(96, 205, 255, 0.55)";
    case "egress":
      return "rgba(52, 211, 153, 0.45)";
    case "anchor":
      return "rgba(96, 205, 255, 0.75)";
    default:
      return "var(--siem-border)";
  }
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
  const size = 12;
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

function computeViewBox(nodes: TraceNode[], defaultW: number, defaultH: number) {
  if (!nodes.length) {
    return { minX: 0, minY: 0, width: 1200, height: 320 };
  }
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const n of nodes) {
    const w = n.width || defaultW;
    const h = n.height || defaultH;
    minX = Math.min(minX, n.x);
    minY = Math.min(minY, n.y);
    maxX = Math.max(maxX, n.x + w);
    maxY = Math.max(maxY, n.y + h);
  }
  return {
    minX: minX - VIEW_PAD,
    minY: minY - VIEW_PAD,
    width: maxX - minX + VIEW_PAD * 2,
    height: maxY - minY + VIEW_PAD * 2,
  };
}

export function TraceFlowCanvas({ graph, animate = true, edgeStates }: Props) {
  const nodeMap = new Map(graph.nodes.map((n) => [n.id, n]));
  const defaultW = graph.nodes[0]?.width ?? 148;
  const defaultH = graph.nodes[0]?.height ?? 72;

  const viewBox = useMemo(
    () => computeViewBox(graph.nodes, defaultW, defaultH),
    [graph.nodes, defaultW, defaultH]
  );

  const primaryEdges = graph.edges.filter((e) => e.primary);
  const dropEdge = graph.edges.find((e) => e.drop);
  const paintedEdges = primaryEdges.filter((e) => edgeStates?.[e.id] === "ACTIVE_GREEN");
  const hasProbePaint = paintedEdges.length > 0;

  function pathForEdge(e: TraceEdge): string {
    const a = nodeMap.get(e.from);
    const b = nodeMap.get(e.to);
    if (!a || !b) return "";
    return edgePath(a, b, a.width || defaultW, a.height || defaultH);
  }

  const swimlanes = (graph.lanes ?? []).filter((lane) => lane.y != null && lane.height != null);

  return (
    <div className="space-y-3">
      <div
        className="overflow-auto rounded-siem border border-siem-border-hi max-h-[68vh] min-h-[420px]"
        style={{ background: "var(--siem-graph-bg)" }}
      >
        <svg
          width={viewBox.width}
          height={viewBox.height}
          viewBox={`${viewBox.minX} ${viewBox.minY} ${viewBox.width} ${viewBox.height}`}
          preserveAspectRatio="xMidYMid meet"
          className="block mx-auto"
          role="img"
          aria-label="Source to destination path map"
        >
          {swimlanes.map((lane) => (
            <g key={`${lane.track}-${lane.label}`}>
              <rect
                x={lane.x}
                y={(lane.y ?? 0) + VIEW_PAD * 0.15}
                width={lane.width}
                height={lane.height ?? 80}
                fill={
                  lane.track === "ingress"
                    ? "rgba(96, 205, 255, 0.04)"
                    : lane.track === "egress"
                      ? "rgba(52, 211, 153, 0.04)"
                      : "rgba(255,255,255,0.02)"
                }
                stroke="var(--siem-border)"
                rx={10}
              />
              <text
                x={lane.x + 14}
                y={(lane.y ?? 0) + VIEW_PAD * 0.15 + 20}
                fill="var(--siem-muted)"
                fontSize={10}
                fontWeight={600}
                letterSpacing="0.05em"
              >
                {lane.label.toUpperCase()}
              </text>
            </g>
          ))}

          {graph.edges.map((e) => {
            const d = pathForEdge(e);
            const dim = !e.primary && !e.drop;
            const paint = edgeStates?.[e.id];
            const stroke = edgePaintStroke(paint, e);
            const verified = paint === "ACTIVE_GREEN";
            return (
              <path
                key={e.id}
                d={d}
                fill="none"
                stroke={stroke}
                strokeWidth={e.drop || paint === "DROPPED_RED" ? 3 : verified ? 3.5 : e.primary ? 2.5 : 1.25}
                strokeDasharray={e.drop ? "8 5" : verified ? undefined : e.primary ? undefined : "3 6"}
                opacity={dim && !verified ? 0.18 : 0.95}
                strokeLinecap="round"
              />
            );
          })}

          {animate && !hasProbePaint &&
            primaryEdges.slice(0, 8).map((e, i) => (
              <FlowPacket
                key={`pkt-${e.id}`}
                pathD={pathForEdge(e)}
                color="var(--siem-ok)"
                outline="var(--siem-graph-bg)"
                dur={2.4 + i * 0.08}
                delay={i * 0.35}
              />
            ))}
          {animate && hasProbePaint &&
            paintedEdges.map((e, i) => (
              <FlowPacket
                key={`probe-${e.id}`}
                pathD={pathForEdge(e)}
                color="var(--siem-ok)"
                outline="var(--siem-graph-bg)"
                dur={1.4}
                delay={i * 0.08}
              />
            ))}
          {graph.edges
            .filter((e) => edgeStates?.[e.id] === "DROPPED_RED")
            .map((e) => (
              <FlowPacket
                key={`drop-${e.id}`}
                pathD={pathForEdge(e)}
                color="var(--siem-ok)"
                dropColor="var(--siem-err)"
                outline="var(--siem-graph-bg)"
                dur={2.2}
                delay={0.2}
                frozen
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
            const isContext = n.track === "context" || (!onPath && !n.track);
            const track = n.track ?? (onPath ? "egress" : "context");
            return (
              <g key={n.id} transform={`translate(${n.x}, ${n.y})`} opacity={dim ? 0.35 : 1}>
                {onPath && track !== "context" && (
                  <rect x={0} y={0} width={4} height={h} rx={2} fill={trackStroke(track)} />
                )}
                <rect
                  x={onPath && track !== "context" ? 4 : 0}
                  y={0}
                  width={onPath && track !== "context" ? w - 4 : w}
                  height={h}
                  rx={isContext ? 999 : 10}
                  fill={
                    endpoint
                      ? "rgba(96, 205, 255, 0.14)"
                      : onPath
                        ? "rgba(255,255,255,0.04)"
                        : "rgba(255,255,255,0.02)"
                  }
                  stroke={endpoint ? "var(--siem-accent)" : onPath ? trackStroke(track) : "var(--siem-border)"}
                  strokeWidth={endpoint ? 2 : onPath ? 1.5 : 1}
                />
                <text
                  x={w / 2}
                  y={isContext ? h / 2 + 4 : 26}
                  textAnchor="middle"
                  fill="var(--siem-text)"
                  fontSize={isContext ? 10 : 13}
                  fontWeight={onPath ? 700 : 500}
                >
                  {truncateLabel(n.label, isContext ? 14 : 18)}
                </text>
                {!isContext && (
                  <text x={w / 2} y={48} textAnchor="middle" fill="var(--siem-muted)" fontSize={10}>
                    {truncateLabel(n.detail || n.kind, 24)}
                  </text>
                )}
              </g>
            );
          })}
        </svg>
      </div>

      <div className="flex flex-wrap gap-4 text-xs text-siem-muted px-1">
        <LegendSwatch color="rgba(96, 205, 255, 0.75)" label="Ingress paths (into workload)" />
        <LegendSwatch color="rgba(52, 211, 153, 0.65)" label="Egress paths (to destination)" />
        <LegendSwatch color="var(--siem-border-hi)" label="Context (dimmed)" dashed />
        <LegendSwatch color="var(--siem-err)" label="Blocked hop (policy drop)" dashed />
        <LegendSwatch color="var(--siem-ok)" label="Verified hop (probe paint)" square />
        <LegendSwatch color="var(--siem-ok)" label="Live packet on focused hop" square />
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
