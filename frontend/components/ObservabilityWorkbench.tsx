"use client";

import { useMemo, useState } from "react";
import type { CaptureSummary, FlowTopology, TopologyEdge } from "@/lib/ai";
import { NetworkPlotGraph } from "./NetworkPlotGraph";
import { FlowDetailsPanel } from "./FlowDetailsPanel";
import { CaptureStatsBar } from "./CaptureStatsBar";

type Props = {
  topology: FlowTopology | null;
  captureSummary?: CaptureSummary | null;
  trackedPodIds: string[];
  loading?: boolean;
  onRefresh?: () => void;
  onOpenAnalyst?: () => void;
  sessionLabel?: string;
};

export function ObservabilityWorkbench({
  topology,
  captureSummary,
  trackedPodIds,
  loading,
  onRefresh,
  onOpenAnalyst,
  sessionLabel,
}: Props) {
  const [selectedEdge, setSelectedEdge] = useState<TopologyEdge | null>(null);

  const detail = useMemo(() => {
    if (!selectedEdge || !topology?.edge_details) return null;
    return topology.edge_details[selectedEdge.id] ?? null;
  }, [selectedEdge, topology]);

  const fromNode = topology?.nodes.find((n) => n.id === selectedEdge?.from);
  const toNode = topology?.nodes.find((n) => n.id === selectedEdge?.to);

  return (
    <section className="rounded-2xl border border-slate-200 bg-white shadow-sm overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 px-5 py-4 border-b border-slate-100 bg-slate-50/80">
        <div>
          <h2 className="text-base font-semibold text-slate-900">Packet observability</h2>
          <p className="text-xs text-slate-500 mt-0.5">
            Selected pods and direct flow neighbors only · click an edge for PCAP summary
            {sessionLabel ? ` · ${sessionLabel.slice(0, 12)}…` : ""}
          </p>
        </div>
        <div className="flex gap-2">
          {onRefresh && (
            <button
              type="button"
              className="px-3 py-1.5 rounded-lg border border-slate-300 text-sm text-slate-700 hover:bg-white transition"
              onClick={onRefresh}
              disabled={loading}
            >
              {loading ? "Loading…" : "Refresh flows"}
            </button>
          )}
          {onOpenAnalyst && (
            <button
              type="button"
              className="px-3 py-1.5 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 transition"
              onClick={onOpenAnalyst}
            >
              AI network analyst
            </button>
          )}
        </div>
      </header>

      <CaptureStatsBar summary={captureSummary ?? null} loading={loading} />

      <div className="flex flex-col lg:flex-row min-h-[480px]">
        <div className="lg:w-[60%] border-b lg:border-b-0 lg:border-r border-slate-200 min-h-[360px]">
          <NetworkPlotGraph
            topology={topology}
            trackedPodIds={trackedPodIds}
            selectedEdgeId={selectedEdge?.id ?? null}
            onSelectEdge={setSelectedEdge}
          />
        </div>
        <div className="lg:w-[40%] bg-slate-50/50 min-h-[280px]">
          <FlowDetailsPanel edge={selectedEdge} detail={detail} fromNode={fromNode} toNode={toNode} />
        </div>
      </div>
    </section>
  );
}
