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
  const width = 520;
  const laneH = 72;
  const topPad = 36;
  const height = topPad + LANES.length * laneH + 28;

  const laneTitle = (id: string) => {
    if (id === "src") return srcLabel;
    if (id === "host") return hostLabel;
    return dstLabel;
  };

  return (
    <div className="rounded-xl border border-slate-200 bg-white overflow-hidden">
      <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" role="img" aria-label="Packet sequence ladder">
        {LANES.map((lane, i) => {
          const y = topPad + i * laneH + laneH / 2;
          return (
            <g key={lane.id}>
              <line x1={48} y1={y} x2={width - 16} y2={y} stroke="#e2e8f0" strokeWidth={1} strokeDasharray="4 4" />
              <line x1={120 + i * 140} y1={topPad - 8} x2={120 + i * 140} y2={height - 20} stroke="#cbd5e1" strokeWidth={1} />
              <text x={8} y={y - 4} className="fill-slate-600 text-[9px] font-medium">
                {lane.title}
              </text>
              <text x={8} y={y + 10} className="fill-slate-400 text-[8px]">
                {laneTitle(lane.id).slice(0, 28)}
              </text>
            </g>
          );
        })}
        <line x1={48} y1={height - 18} x2={width - 16} y2={height - 18} stroke="#94a3b8" strokeWidth={1} />
        <text x={48} y={height - 6} className="fill-slate-500 text-[8px]">
          0 µs
        </text>
        <text x={width - 56} y={height - 6} className="fill-slate-500 text-[8px]">
          {maxUs} µs
        </text>
        {steps.map((s, idx) => {
          const laneIdx = LANES.findIndex((l) => l.id === s.lane);
          const li = laneIdx >= 0 ? laneIdx : 1;
          const y = topPad + li * laneH + laneH / 2;
          const x = 48 + (s.rel_us / maxUs) * (width - 80);
          const isReset = s.flags?.includes("RST");
          const isSyn = s.flags?.includes("SYN");
          const fill = isReset ? "#dc2626" : isSyn ? "#2563eb" : "#16a34a";
          return (
            <g key={idx}>
              <circle cx={x} cy={y} r={5} fill={fill} stroke="#fff" strokeWidth={1.5} />
              <text x={x} y={y - 10} textAnchor="middle" className="fill-slate-800 text-[8px] font-semibold">
                {s.label}
              </text>
              <text x={x} y={y + 18} textAnchor="middle" className="fill-slate-400 text-[7px]">
                +{s.rel_us}µs
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
