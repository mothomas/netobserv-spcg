"use client";

import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import type { SigmaGraph } from "@/lib/graph";
import type { EdgePaintState, PathSummary, TraceEndpoint, TraceGraph } from "@/lib/trace";
import { endpointLabel } from "@/lib/trace";
import { LayerScopeBanner } from "@/components/layout/LayerScopeBanner";
import { LAYER_SCOPES } from "@/lib/sections";
import { DEFAULT_TRACE_FILTER, filterSigmaGraph, filterTraceGraph } from "@/lib/traceGraphFilter";
import { TraceFlowCanvas } from "./TraceFlowCanvas";
import { TraceGraphFilters } from "./TraceGraphFilters";
import { TraceProbePanel } from "./TraceProbePanel";
import { TraceStatsBar } from "./TraceStatsBar";

const SigmaTopologyGraph = dynamic(
  () => import("@/components/SigmaTopologyGraph").then((m) => m.SigmaTopologyGraph),
  {
    ssr: false,
    loading: () => (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Loading Sigma graph…
      </div>
    ),
  }
);

export type TraceView = "cop" | "sigma";

type Props = {
  authSessionId: string;
  traceId: string;
  source: TraceEndpoint;
  destination: TraceEndpoint;
  sourcePodCount: number;
  destPodCount: number;
  graph: TraceGraph;
  sigmaGraph?: SigmaGraph | null;
  view: TraceView;
  paused: boolean;
};

function statusBadge(status: string) {
  if (status === "discovered") {
    return (
      <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-ok border border-siem-ok/30">discovered</span>
    );
  }
  if (status === "out_of_scope") {
    return (
      <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-muted border border-siem-border">out of scope</span>
    );
  }
  return (
    <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-muted border border-siem-border">{status}</span>
  );
}

export function TraceWorkbench({
  authSessionId,
  traceId,
  source,
  destination,
  sourcePodCount,
  destPodCount,
  graph,
  sigmaGraph,
  view,
  paused,
}: Props) {
  const [filter, setFilter] = useState(DEFAULT_TRACE_FILTER);
  const [edgeStates, setEdgeStates] = useState<Record<string, EdgePaintState>>({});
  const [probeActive, setProbeActive] = useState(false);
  const [verifiedHops, setVerifiedHops] = useState(0);
  const [totalHops, setTotalHops] = useState(0);
  const displayGraph = useMemo(() => filterTraceGraph(graph, filter), [graph, filter]);
  const visibleIds = useMemo(() => new Set(displayGraph.nodes.map((n) => n.id)), [displayGraph]);
  const displaySigma = useMemo(() => filterSigmaGraph(sigmaGraph, visibleIds), [sigmaGraph, visibleIds]);

  const paths = graph.paths ?? [];
  const ingress = paths.filter((p: PathSummary) => p.direction === "ingress");
  const egress = paths.filter((p: PathSummary) => p.direction === "egress");
  const host = paths.filter((p: PathSummary) => p.direction === "host");

  const headerBadge =
    probeActive ? (
      <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-accent border border-siem-accent/40 shrink-0 animate-pulse">
        verifying
      </span>
    ) : verifiedHops > 0 && totalHops > 0 && verifiedHops >= totalHops ? (
      <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30 shrink-0">verified</span>
    ) : verifiedHops > 0 ? (
      <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-warn border border-siem-warn/30 shrink-0">
        partial · {verifiedHops}/{totalHops}
      </span>
    ) : (
      <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30 shrink-0">discovery</span>
    );

  return (
    <section
      className="siem-card overflow-hidden flex flex-col min-h-[720px]"
      aria-label="Packet Trace workspace"
    >
      <header className="app-shell-header flex flex-wrap items-center justify-between gap-3 px-5 py-4 shrink-0 border-b border-siem-border">
        <div>
          <h2 className="text-base font-semibold text-siem-text">Source → Destination</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            {endpointLabel(source)}
            <span className="mx-2 opacity-50">→</span>
            {endpointLabel(destination)}
            <span className="mx-2 opacity-30">·</span>
            {sourcePodCount} source pod{sourcePodCount === 1 ? "" : "s"}
            {destPodCount > 0 ? ` · ${destPodCount} dest pod${destPodCount === 1 ? "" : "s"}` : ""}
            {` · trace ${traceId.slice(0, 8)}…`}
          </p>
        </div>
        {headerBadge}
      </header>

      <div className="px-5 py-4 border-b border-siem-border bg-siem-panel/20">
        <TraceProbePanel
          authSessionId={authSessionId}
          traceId={traceId}
          onEdgeStates={setEdgeStates}
          onProbingChange={setProbeActive}
          onVerifiedChange={(v, t) => {
            setVerifiedHops(v);
            setTotalHops(t);
          }}
        />
      </div>

      <TraceStatsBar graph={graph} />

      <div className="px-5 py-3 border-b border-siem-border bg-siem-panel/30">
        <TraceGraphFilters value={filter} stats={graph.stats} onChange={setFilter} />
      </div>

      <div className="relative fluent-graph-stage overflow-hidden flex-1 min-h-[560px]">
        {view === "cop" ? (
          <div className="h-full overflow-x-auto p-4">
            <TraceFlowCanvas
              graph={displayGraph}
              animate={!paused && !probeActive}
              edgeStates={edgeStates}
            />
          </div>
        ) : (
          <SigmaTopologyGraph
            graph={displaySigma ?? null}
            topology={null}
            selectedEdgeId={null}
            onSelectEdge={() => undefined}
          />
        )}
      </div>

      <div className="grid md:grid-cols-2 gap-0 border-t border-siem-border shrink-0">
        <div className="p-4 md:border-r border-siem-border">
          <h3 className="text-sm font-semibold mb-3">Discovered paths</h3>
          <PathTable title="Ingress" rows={ingress} />
          <PathTable title="Egress" rows={egress} />
          <PathTable title="Host" rows={host} />
          {!paths.length && (
            <p className="text-sm text-siem-muted">No external paths matched this pod in selected namespaces.</p>
          )}
        </div>
        <div className="p-4 border-l-4 border-siem-border md:border-l-0 md:border-t-0 border-l-siem-accent/40 bg-siem-panel/20">
          <LayerScopeBanner layer={LAYER_SCOPES.trace} compact />
          <p className="text-sm text-siem-muted mt-3">
            Run discovery, then <strong className="text-siem-text font-medium">Verify path</strong> — capture,
            probe, and hop paint run automatically. Enable{" "}
            <strong className="text-siem-text font-medium">Demo policy block</strong> to show a red drop on the final
            hop.
          </p>
        </div>
      </div>
    </section>
  );
}

function PathTable({ title, rows }: { title: string; rows: PathSummary[] }) {
  if (!rows.length) return null;
  return (
    <div className="mb-4">
      <p className="text-xs text-siem-muted uppercase tracking-wide mb-2">{title}</p>
      <table className="w-full text-sm">
        <tbody>
          {rows.map((r) => (
            <tr key={`${r.kind}-${r.resource}-${r.namespace}`} className="border-t border-siem-border">
              <td className="py-2 pr-2 font-mono text-xs">{r.resource}</td>
              <td className="py-2 pr-2 text-siem-muted">{r.kind}</td>
              <td className="py-2 pr-2 text-siem-muted">{r.namespace || "—"}</td>
              <td className="py-2">{statusBadge(r.status)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
