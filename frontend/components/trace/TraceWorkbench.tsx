"use client";

import dynamic from "next/dynamic";
import type { PodDetail } from "@/lib/api";
import type { SigmaGraph } from "@/lib/graph";
import type { PathSummary, TraceGraph } from "@/lib/trace";
import { TraceFlowCanvas } from "./TraceFlowCanvas";
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
  traceId: string;
  targetPod: PodDetail;
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

export function TraceWorkbench({ traceId, targetPod, graph, sigmaGraph, view, paused }: Props) {
  const paths = graph.paths ?? [];
  const ingress = paths.filter((p: PathSummary) => p.direction === "ingress");
  const egress = paths.filter((p: PathSummary) => p.direction === "egress");
  const host = paths.filter((p: PathSummary) => p.direction === "host");

  return (
    <section
      className="siem-card overflow-hidden flex flex-col min-h-[720px]"
      aria-label="Packet Trace workspace"
    >
      <header className="app-shell-header flex flex-wrap items-center justify-between gap-3 px-5 py-4 shrink-0 border-b border-siem-border">
        <div>
          <h2 className="text-base font-semibold text-siem-text">Path discovery</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            {targetPod.namespace}/{targetPod.name}
            {targetPod.pod_ip ? ` · ${targetPod.pod_ip}` : ""}
            {targetPod.node_name ? ` · ${targetPod.node_name}` : ""}
            {` · trace ${traceId.slice(0, 8)}…`}
          </p>
        </div>
        <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30 shrink-0">
          discovery
        </span>
      </header>

      <TraceStatsBar graph={graph} />

      <div className="relative fluent-graph-stage overflow-hidden flex-1 min-h-[560px]">
        {view === "cop" ? (
          <div className="h-full overflow-x-auto p-4">
            <TraceFlowCanvas graph={graph} animate={!paused} />
          </div>
        ) : (
          <SigmaTopologyGraph
            graph={sigmaGraph ?? null}
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
          <h3 className="text-sm font-semibold mb-2">Next: live packet cop</h3>
          <p className="text-sm text-siem-muted">
            Phase A maps infrastructure from your RBAC token. Live hop correlation, drop diagnosis, and policy hints
            will attach here once capture and eBPF evidence are wired to this trace session.
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
