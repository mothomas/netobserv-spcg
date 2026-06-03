"use client";

import dynamic from "next/dynamic";
import { useMemo, useState } from "react";
import type { CaptureSummary, FlowTopology, TopologyEdge } from "@/lib/ai";
import { FlowDetailsPanel } from "./FlowDetailsPanel";
import { CaptureStatsBar } from "./CaptureStatsBar";
import { PcapExportBar, type CapturePodRef } from "./PcapExportBar";
import { UI_BUILD_TAG } from "@/lib/sigmaGraphStyle";
import type { SigmaGraph, SigmaNode } from "@/lib/graph";
import type { TopologyNode } from "@/lib/ai";

const SigmaTopologyGraph = dynamic(
  () => import("./SigmaTopologyGraph").then((m) => m.SigmaTopologyGraph),
  {
    ssr: false,
    loading: () => (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Loading topology graph…
      </div>
    ),
  }
);

function asTopologyNode(n: TopologyNode | SigmaNode | undefined): TopologyNode | undefined {
  if (!n) return undefined;
  if ("kind" in n && typeof (n as TopologyNode).kind === "string") return n as TopologyNode;
  const s = n as SigmaNode;
  return { id: s.id, label: s.label, namespace: "", kind: s.type || "Pod" };
}

type Props = {
  topology: FlowTopology | null;
  sigmaGraph: SigmaGraph | null;
  captureSummary?: CaptureSummary | null;
  trackedPodIds: string[];
  loading?: boolean;
  graphDegraded?: boolean;
  onRefresh?: () => void;
  onEndSession?: () => void;
  sessionLabel?: string;
  capturePods?: CapturePodRef[];
  exportBusy?: boolean;
  onDownloadPod?: (pod: CapturePodRef) => void;
  onDownloadMerged?: () => void;
  onOpenS3?: () => void;
  s3Export?: import("@/lib/api").S3ExportInfo | null;
};

export function ObservabilityWorkbench({
  topology,
  sigmaGraph,
  captureSummary,
  trackedPodIds,
  loading,
  graphDegraded,
  onRefresh,
  onEndSession,
  sessionLabel,
  capturePods,
  exportBusy,
  onDownloadPod,
  onDownloadMerged,
  onOpenS3,
  s3Export,
}: Props) {
  const [selectedEdge, setSelectedEdge] = useState<TopologyEdge | null>(null);

  const edgeDetails = sigmaGraph?.edge_details ?? topology?.edge_details;

  const detail = useMemo(() => {
    if (!selectedEdge || !edgeDetails) return null;
    return edgeDetails[selectedEdge.id] ?? null;
  }, [selectedEdge, edgeDetails]);

  const fromNode = useMemo(
    () =>
      asTopologyNode(
        topology?.nodes?.find((n) => n.id === selectedEdge?.from) ??
          sigmaGraph?.nodes?.find((n) => n.id === selectedEdge?.from)
      ),
    [selectedEdge, topology, sigmaGraph]
  );

  const toNode = useMemo(
    () =>
      asTopologyNode(
        topology?.nodes?.find((n) => n.id === selectedEdge?.to) ??
          sigmaGraph?.nodes?.find((n) => n.id === selectedEdge?.to)
      ),
    [selectedEdge, topology, sigmaGraph]
  );

  return (
    <section className="siem-card overflow-hidden flex flex-col min-h-[720px]">
      <header className="app-shell-header flex flex-wrap items-center justify-between gap-3 px-5 py-4 shrink-0">
        <div>
          <h2 className="text-base font-semibold text-siem-text">Incident capture</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            Neo4j graph · Sigma render · tenant-scoped
            {sessionLabel ? ` · ${sessionLabel.slice(0, 12)}…` : ""}
            <span className="text-siem-muted/70"> · ui {UI_BUILD_TAG}</span>
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
        </div>
      </header>

      <CaptureStatsBar summary={captureSummary ?? null} loading={loading} />

      {graphDegraded && (
        <p className="px-5 py-2 text-xs text-amber-400 border-b border-siem-border bg-amber-500/5">
          High packet rate — graph and stats use sampled data (top flows only). Pause the stress source or stop capture for full fidelity.
        </p>
      )}

      {(capturePods && capturePods.length > 0 && onDownloadPod && onDownloadMerged) || s3Export?.enabled ? (
        <PcapExportBar
          pods={capturePods ?? []}
          busy={exportBusy}
          s3Export={s3Export}
          onDownloadPod={onDownloadPod ?? (() => undefined)}
          onDownloadMerged={onDownloadMerged ?? (() => undefined)}
          onOpenS3={onOpenS3}
        />
      ) : null}

      <div className="relative fluent-graph-stage overflow-hidden h-[62vh] min-h-[560px]">
        <SigmaTopologyGraph
          graph={sigmaGraph}
          topology={topology}
          selectedEdgeId={selectedEdge?.id ?? null}
          onSelectEdge={setSelectedEdge}
        />

        <aside className={`flow-inspector-drawer ${selectedEdge ? "is-open" : ""}`}>
          <div className="px-4 py-3 border-b border-siem-border shrink-0 flex items-start justify-between gap-2">
            <div>
              <p className="text-sm font-semibold text-siem-text">Flow inspector</p>
              <p className="text-[10px] text-siem-muted mt-0.5">Selected link · direction & sequence</p>
            </div>
            <button
              type="button"
              className="h-7 w-7 rounded-full border border-siem-border text-siem-muted hover:text-siem-text hover:bg-siem-bg transition shrink-0"
              onClick={() => setSelectedEdge(null)}
              aria-label="Close flow inspector"
            >
              ×
            </button>
          </div>
          <div className="flex-1 overflow-auto">
            <FlowDetailsPanel
              edge={selectedEdge}
              detail={detail}
              edgeDetails={edgeDetails}
              fromNode={fromNode}
              toNode={toNode}
              onClose={() => setSelectedEdge(null)}
            />
          </div>
        </aside>
      </div>
    </section>
  );
}
