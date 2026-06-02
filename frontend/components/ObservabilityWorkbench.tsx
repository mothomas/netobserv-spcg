"use client";

import { useMemo, useState } from "react";
import type { CaptureSummary, FlowTopology, TopologyEdge } from "@/lib/ai";
import { NetworkPlotGraph } from "./NetworkPlotGraph";
import { FlowDetailsPanel } from "./FlowDetailsPanel";
import { CaptureStatsBar } from "./CaptureStatsBar";
import { PcapExportBar, type CapturePodRef } from "./PcapExportBar";

type Props = {
  topology: FlowTopology | null;
  captureSummary?: CaptureSummary | null;
  trackedPodIds: string[];
  loading?: boolean;
  onRefresh?: () => void;
  onOpenAnalyst?: () => void;
  onEndSession?: () => void;
  sessionLabel?: string;
  capturePods?: CapturePodRef[];
  exportBusy?: boolean;
  onDownloadPod?: (pod: CapturePodRef) => void;
  onDownloadMerged?: () => void;
};

export function ObservabilityWorkbench({
  topology,
  captureSummary,
  trackedPodIds,
  loading,
  onRefresh,
  onOpenAnalyst,
  onEndSession,
  sessionLabel,
  capturePods,
  exportBusy,
  onDownloadPod,
  onDownloadMerged,
}: Props) {
  const [selectedEdge, setSelectedEdge] = useState<TopologyEdge | null>(null);

  const detail = useMemo(() => {
    if (!selectedEdge || !topology?.edge_details) return null;
    return topology.edge_details[selectedEdge.id] ?? null;
  }, [selectedEdge, topology]);

  const fromNode = topology?.nodes.find((n) => n.id === selectedEdge?.from);
  const toNode = topology?.nodes.find((n) => n.id === selectedEdge?.to);

  return (
    <section className="siem-card overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 px-5 py-4 border-b border-siem-border bg-siem-panel">
        <div>
          <h2 className="text-base font-semibold text-siem-text">Incident capture</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            Tenant-scoped flows · edge drill-down
            {sessionLabel ? ` · ${sessionLabel.slice(0, 12)}…` : ""}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {onRefresh && (
            <button type="button" className="siem-btn-ghost" onClick={onRefresh} disabled={loading}>
              {loading ? "Loading…" : "Refresh flows"}
            </button>
          )}
          {onEndSession && (
            <button type="button" className="siem-btn-ghost text-siem-warn border-siem-warn/40" onClick={onEndSession}>
              End session
            </button>
          )}
          {onOpenAnalyst && (
            <button
              type="button"
              className="px-4 py-2 rounded-full text-sm font-semibold text-white bg-blue-700 hover:bg-blue-600 shadow-[0_8px_22px_rgba(37,99,235,0.35)] transition"
              onClick={onOpenAnalyst}
            >
              AI analyst
            </button>
          )}
        </div>
      </header>

      <CaptureStatsBar summary={captureSummary ?? null} loading={loading} />

      {capturePods && capturePods.length > 0 && onDownloadPod && onDownloadMerged && (
        <PcapExportBar
          pods={capturePods}
          busy={exportBusy}
          onDownloadPod={onDownloadPod}
          onDownloadMerged={onDownloadMerged}
        />
      )}

      <div className="relative h-[calc(100vh-260px)] min-h-[620px] bg-[#0d121c]">
        <div className="h-full border-t border-siem-border">
          <NetworkPlotGraph
            topology={topology}
            trackedPodIds={trackedPodIds}
            selectedEdgeId={selectedEdge?.id ?? null}
            onSelectEdge={setSelectedEdge}
          />
        </div>

        <div
          className={`absolute right-0 top-0 bottom-0 w-full md:w-[420px] xl:w-[480px] bg-siem-panel/95 backdrop-blur-sm border-l border-siem-border shadow-2xl transform transition-transform duration-300 ease-out ${
            selectedEdge ? "translate-x-0" : "translate-x-full"
          }`}
        >
          <FlowDetailsPanel
            edge={selectedEdge}
            detail={detail}
            fromNode={fromNode}
            toNode={toNode}
            onClose={() => setSelectedEdge(null)}
          />
        </div>
      </div>
    </section>
  );
}
