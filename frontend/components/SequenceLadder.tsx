"use client";

import type { SequenceStep } from "@/lib/ai";

type Props = {
  steps: SequenceStep[];
  srcLabel: string;
  hostLabel: string;
  dstLabel: string;
};

const LANES = [
  { id: "src", title: "Source namespace" },
  { id: "host", title: "Node / network stack" },
  { id: "dst", title: "Target namespace" },
] as const;

export function SequenceLadder({ steps, srcLabel, hostLabel, dstLabel }: Props) {
  if (!steps.length) {
    return (
      <p className="text-sm text-slate-500 py-8 text-center">
        Select a flow edge to render packet sequence (SYN / ACK / RST) on a relative timeline.
      </p>
    );
  }

  const maxUs = Math.max(...steps.map((s) => s.rel_us), 1);
  const width = Math.max(860, steps.length * 78 + 180);
  const laneY = 96;
  const height = 220;

  const laneTitle = (id: string) => {
    if (id === "src") return srcLabel;
    if (id === "host") return hostLabel;
    return dstLabel;
  };

  return (
    <div className="rounded-md border border-siem-border bg-siem-bg overflow-x-auto">
      <svg width={width} height={height} className="min-w-full" role="img" aria-label="Packet sequence ladder timeline">
        <text x={12} y={22} className="fill-siem-text text-[11px] font-medium">
          Source
        </text>
        <text x={12} y={36} className="fill-siem-muted text-[10px]">
          {laneTitle("src").slice(0, 44)}
        </text>
        <text x={12} y={56} className="fill-siem-text text-[11px] font-medium">
          Host stack
        </text>
        <text x={12} y={70} className="fill-siem-muted text-[10px]">
          {laneTitle("host").slice(0, 44)}
        </text>
        <text x={12} y={90} className="fill-siem-text text-[11px] font-medium">
          Destination
        </text>
        <text x={12} y={104} className="fill-siem-muted text-[10px]">
          {laneTitle("dst").slice(0, 44)}
        </text>

        <line x1={140} y1={laneY} x2={width - 24} y2={laneY} stroke="#34d399" strokeWidth={2} strokeOpacity={0.6} />

        {steps.map((s, idx) => {
          const x = 140 + (s.rel_us / maxUs) * (width - 180);
          const labelAbove = idx % 2 === 0;
          const labelY = labelAbove ? laneY - 18 : laneY + 28;
          const tsY = labelAbove ? laneY - 5 : laneY + 45;
          const isReset = s.flags?.includes("RST");
          const isSyn = s.flags?.includes("SYN");
          const fill = isReset ? "#fb7185" : isSyn ? "#22d3ee" : "#34d399";
          return (
            <g key={`${s.rel_us}_${idx}`}>
              <line x1={x} y1={laneY - 12} x2={x} y2={laneY + 12} stroke="#1f2937" strokeWidth={1} />
              <circle cx={x} cy={laneY} r={5} fill={fill} stroke="#0b0f17" strokeWidth={2} />
              <rect x={x - 52} y={labelY - 10} width={104} height={14} rx={4} fill="#0b1220" fillOpacity={0.88} />
              <text x={x} y={labelY} textAnchor="middle" className="fill-siem-text text-[9px] font-medium">
                {s.label.slice(0, 22)}
              </text>
              <text x={x} y={tsY} textAnchor="middle" className="fill-siem-muted text-[8px] font-mono">
                +{s.rel_us}us
              </text>
            </g>
          );
        })}

        <text x={140} y={height - 12} className="fill-siem-muted text-[9px] font-mono">
          0us
        </text>
        <text x={width - 62} y={height - 12} className="fill-siem-muted text-[9px] font-mono">
          {maxUs}us
        </text>
      </svg>
    </div>
  );
}
