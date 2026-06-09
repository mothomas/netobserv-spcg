"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import type { SigmaGraph } from "@/lib/graph";
import type { PathSummary, TraceEndpoint, TraceGraph } from "@/lib/trace";
import { endpointLabel } from "@/lib/trace";
import { TraceFlowCanvas } from "./TraceFlowCanvas";
import { TracePathOptionList } from "./TracePathOptionList";
import { TraceStatsBar } from "./TraceStatsBar";

const TraceSigmaPathMap = dynamic(
  () => import("./TraceSigmaPathMap").then((m) => m.TraceSigmaPathMap),
  {
    ssr: false,
    loading: () => (
      <div className="h-full flex items-center justify-center text-sm text-siem-muted px-6 text-center">
        Loading Sigma path map…
      </div>
    ),
  }
);

export type TraceView = "cop" | "sigma";

type Props = {
  traceId: string;
  source: TraceEndpoint;
  destination: TraceEndpoint;
  sourcePodCount: number;
  destPodCount: number;
  graph: TraceGraph;
  sigmaGraph?: SigmaGraph | null;
  view: TraceView;
  paused: boolean;
  captureActive?: boolean;
  captureBusy?: boolean;
  onStartCapture?: () => void;
  onOpenL7?: () => void;
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

function defaultPathSelection(graph: TraceGraph): string[] {
  const opts = graph.path_options ?? [];
  if (!opts.length) return [];
  const ingress = opts.find((p) => p.direction === "ingress");
  const egress = opts.find((p) => p.direction === "egress");
  const out: string[] = [];
  if (ingress) out.push(ingress.id);
  if (egress) out.push(egress.id);
  return out;
}

export function TraceWorkbench({
  traceId,
  source,
  destination,
  sourcePodCount,
  destPodCount,
  graph,
  sigmaGraph,
  view,
  paused,
  captureActive,
  captureBusy,
  onStartCapture,
  onOpenL7,
}: Props) {
  const paths = graph.paths ?? [];
  const pathOptions = graph.path_options ?? [];
  const ingress = paths.filter((p: PathSummary) => p.direction === "ingress");
  const egress = paths.filter((p: PathSummary) => p.direction === "egress");
  const host = paths.filter((p: PathSummary) => p.direction === "host");

  const [selectedPathIds, setSelectedPathIds] = useState<string[]>(() => defaultPathSelection(graph));
  const [showContext, setShowContext] = useState(true);

  useEffect(() => {
    setSelectedPathIds(defaultPathSelection(graph));
  }, [graph.trace_id, pathOptions.length]);

  const hasPathOptions = pathOptions.length > 0;

  const togglePath = (id: string) => {
    setSelectedPathIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  };

  const graphStage = useMemo(() => {
    if (view === "cop") {
      return (
        <TraceFlowCanvas
          graph={graph}
          animate={!paused}
          selectedPathIds={selectedPathIds}
          showContext={showContext}
        />
      );
    }
    return (
      <TraceSigmaPathMap
        sigmaGraph={sigmaGraph ?? null}
        traceGraph={graph}
        selectedPathIds={selectedPathIds}
        showContext={showContext}
      />
    );
  }, [view, graph, sigmaGraph, paused, selectedPathIds, showContext]);

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
        <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30 shrink-0">
          discovery
        </span>
      </header>

      <TraceStatsBar graph={graph} />

      <div className="relative fluent-graph-stage overflow-hidden flex-1 min-h-[560px] flex">
        {hasPathOptions && (
          <aside className="w-[272px] shrink-0 border-r border-siem-border bg-siem-panel/30 overflow-y-auto p-4 hidden lg:block">
            <h3 className="text-sm font-semibold text-siem-text mb-3">Path options</h3>
            <TracePathOptionList
              options={pathOptions}
              selected={selectedPathIds}
              onToggle={togglePath}
              onClear={() => setSelectedPathIds([])}
            />
            <label className="flex items-center gap-2 text-xs text-siem-muted mt-4 pt-4 border-t border-siem-border">
              <input type="checkbox" checked={showContext} onChange={(e) => setShowContext(e.target.checked)} />
              Show context (policies, BGP, NAD)
            </label>
          </aside>
        )}
        <div className="flex-1 min-w-0 min-h-[560px] relative">{graphStage}</div>
      </div>

      <div className="grid md:grid-cols-2 gap-0 border-t border-siem-border shrink-0">
        <div className="p-4 md:border-r border-siem-border">
          <h3 className="text-sm font-semibold mb-3">Discovered paths</h3>
          {hasPathOptions && (
            <div className="lg:hidden mb-4 pb-4 border-b border-siem-border">
              <TracePathOptionList
                options={pathOptions}
                selected={selectedPathIds}
                onToggle={togglePath}
                onClear={() => setSelectedPathIds([])}
              />
            </div>
          )}
          <PathTable title="Ingress" rows={ingress} />
          <PathTable title="Egress" rows={egress} />
          <PathTable title="Host" rows={host} />
          {!paths.length && (
            <p className="text-sm text-siem-muted">No external paths matched this pod in selected namespaces.</p>
          )}
        </div>
        <div className="p-4 border-l-4 border-siem-border md:border-l-0 md:border-t-0 border-l-siem-accent/40 bg-siem-panel/20">
          <h3 className="text-sm font-semibold mb-2">On demand: live capture & L7</h3>
          <p className="text-sm text-siem-muted mb-4">
            Start eBPF capture on resolved source pods only. Service-to-service calls, TLS SNI, DNS, and app ports
            appear in L7 analysis while the trace session stays open.
          </p>
          <div className="flex flex-wrap gap-2">
            {!captureActive ? (
              <button
                type="button"
                className="siem-btn-primary text-sm"
                disabled={captureBusy || !onStartCapture}
                onClick={onStartCapture}
              >
                {captureBusy ? "Starting capture…" : "Start live capture on path"}
              </button>
            ) : (
              <>
                <span className="text-[10px] px-2 py-1 rounded-md text-siem-ok border border-siem-ok/30 self-center">
                  capture live
                </span>
                {onOpenL7 && (
                  <button type="button" className="siem-btn-ghost text-sm" onClick={onOpenL7}>
                    Open L7 analysis
                  </button>
                )}
              </>
            )}
          </div>
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
