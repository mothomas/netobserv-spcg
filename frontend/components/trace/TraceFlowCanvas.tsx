"use client";

import type { TraceEdge, TraceGraph, TraceNode } from "@/lib/trace";

type Props = {
  graph: TraceGraph;
  animate?: boolean;
};

function edgePath(
  from: TraceNode,
  to: TraceNode,
  nodeW: number,
  nodeH: number
): string {
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
      <rect
        x={-half}
        y={-half}
        width={size}
        height={size}
        fill={color}
        stroke={outline}
        strokeWidth={2}
        rx={2}
      >
        {frozen ? (
          <>
            <animateMotion
              dur="3.4s"
              fill="freeze"
              begin={`${delay}s`}
              keyPoints="0;0.55;0.55"
              keyTimes="0;0.82;1"
              calcMode="linear"
            >
              <mpath href={`#${pathId}`} />
            </animateMotion>
            <animate
              attributeName="fill"
              values={`${color};${color};${stopFill}`}
              keyTimes="0;0.82;1"
              dur="3.4s"
              begin={`${delay}s`}
              fill="freeze"
            />
          </>
        ) : (
          <animateMotion
            dur={`${dur}s`}
            repeatCount="indefinite"
            begin={`${delay}s`}
            calcMode="linear"
          >
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
  const width = Math.max(graph.width, 1200);
  const height = Math.max(graph.height, 360);

  const primaryEdges = graph.edges.filter((e) => e.primary);
  const dropEdge = graph.edges.find((e) => e.drop);

  function pathForEdge(e: TraceEdge): string {
    const a = nodeMap.get(e.from);
    const b = nodeMap.get(e.to);
    if (!a || !b) return "";
    return edgePath(a, b, a.width || defaultW, a.height || defaultH);
  }

  return (
    <div
      className="overflow-x-auto rounded-siem border border-siem-border-hi"
      style={{ background: "var(--siem-graph-bg)" }}
    >
      <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} className="block min-w-full">
        {graph.lanes?.map((lane) => (
          <g key={lane.label}>
            <rect
              x={lane.x}
              y={12}
              width={lane.width}
              height={height - 24}
              fill="rgba(255,255,255,0.03)"
              stroke="var(--siem-border)"
              rx={8}
            />
            <text x={lane.x + 14} y={34} fill="var(--siem-muted)" fontSize={13} fontWeight={600}>
              {lane.label}
            </text>
          </g>
        ))}

        {graph.edges.map((e) => {
          const d = pathForEdge(e);
          const stroke = e.drop ? "var(--siem-err)" : e.primary ? "var(--siem-accent)" : "var(--siem-border-hi)";
          return (
            <path
              key={e.id}
              d={d}
              fill="none"
              stroke={stroke}
              strokeWidth={e.drop ? 4 : e.primary ? 3.5 : 2}
              strokeDasharray={e.drop ? "10 6" : e.primary ? undefined : "4 6"}
              opacity={e.primary || e.drop ? 1 : 0.55}
              strokeLinecap="round"
            />
          );
        })}

        {animate &&
          primaryEdges.slice(0, 6).map((e, i) => (
            <FlowPacket
              key={`pkt-${e.id}`}
              pathD={pathForEdge(e)}
              color="var(--siem-ok)"
              outline="var(--siem-graph-bg)"
              dur={2.4 + i * 0.1}
              delay={i * 0.45}
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
          const hl = n.tracked;
          return (
            <g key={n.id} transform={`translate(${n.x}, ${n.y})`}>
              <rect
                width={w}
                height={h}
                rx={8}
                fill={hl ? "rgba(96, 205, 255, 0.12)" : "var(--siem-card)"}
                stroke={hl ? "var(--siem-accent)" : "var(--siem-border-hi)"}
                strokeWidth={hl ? 3 : 2}
              />
              <text
                x={w / 2}
                y={30}
                textAnchor="middle"
                fill="var(--siem-text)"
                fontSize={15}
                fontWeight={700}
              >
                {n.label}
              </text>
              <text
                x={w / 2}
                y={52}
                textAnchor="middle"
                fill="var(--siem-muted)"
                fontSize={12}
              >
                {n.detail || n.kind}
              </text>
            </g>
          );
        })}

        <g transform="translate(24, 280)">
          <rect
            x={0}
            y={0}
            width={248}
            height={78}
            rx={8}
            fill="var(--siem-card)"
            stroke="var(--siem-border-hi)"
            strokeWidth={2}
          />
          <rect x={16} y={20} width={14} height={14} fill="var(--siem-ok)" rx={2} />
          <text x={40} y={32} fill="var(--siem-text)" fontSize={13} fontWeight={600}>
            In transit
          </text>
          <rect x={16} y={46} width={14} height={14} fill="var(--siem-err)" rx={2} />
          <text x={40} y={58} fill="var(--siem-text)" fontSize={13} fontWeight={600}>
            Dropped (frozen)
          </text>
        </g>
      </svg>
    </div>
  );
}
