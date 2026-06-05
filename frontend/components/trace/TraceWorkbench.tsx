"use client";

import dynamic from "next/dynamic";
import { useCallback, useState } from "react";
import type { PodDetail } from "@/lib/api";
import type { SigmaGraph } from "@/lib/graph";
import type { PathSummary, TraceGraph } from "@/lib/trace";
import { teardownTrace } from "@/lib/trace";
import { TraceFlowCanvas } from "./TraceFlowCanvas";

const SigmaTopologyGraph = dynamic(
  () => import("@/components/SigmaTopologyGraph").then((m) => m.SigmaTopologyGraph),
  {
    ssr: false,
    loading: () => (
      <div className="h-[420px] flex items-center justify-center text-sm text-siem-muted">Loading Sigma graph…</div>
    ),
  }
);

type Props = {
  authSessionId: string;
  traceId: string;
  targetPod: PodDetail;
  graph: TraceGraph;
  sigmaGraph?: SigmaGraph | null;
  onEnd?: () => void;
  embedded?: boolean;
};

type TraceView = "cop" | "sigma";

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
  targetPod,
  graph,
  sigmaGraph,
  onEnd,
  embedded = true,
}: Props) {
  const [view, setView] = useState<TraceView>("cop");
  const [paused, setPaused] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const endTrace = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      await teardownTrace(authSessionId, traceId);
      onEnd?.();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, [authSessionId, traceId, onEnd]);

  const paths = graph.paths ?? [];
  const ingress = paths.filter((p: PathSummary) => p.direction === "ingress");
  const egress = paths.filter((p: PathSummary) => p.direction === "egress");
  const host = paths.filter((p: PathSummary) => p.direction === "host");

  return (
    <div className={embedded ? "space-y-4" : "min-h-screen flex flex-col bg-siem-bg text-siem-text"}>
      <div className="flex flex-wrap items-center gap-3 siem-card px-4 py-3">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-siem-text">
            {targetPod.namespace}/{targetPod.name}
          </p>
          <p className="text-xs text-siem-muted font-mono truncate">
            {targetPod.pod_ip || "no IP"} · {targetPod.node_name || "node pending"} · {traceId.slice(0, 8)}
          </p>
        </div>
        <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30 shrink-0">
          discovery
        </span>
        <div className="ml-auto flex gap-2 items-center flex-wrap">
            <div className="flex rounded-siem border border-siem-border overflow-hidden text-xs">
              <button
                type="button"
                className={`px-3 py-1.5 ${view === "cop" ? "fluent-nav-active" : "fluent-nav-idle"}`}
                onClick={() => setView("cop")}
              >
                Packet cop
              </button>
              <button
                type="button"
                className={`px-3 py-1.5 ${view === "sigma" ? "fluent-nav-active" : "fluent-nav-idle"}`}
                onClick={() => setView("sigma")}
                disabled={!sigmaGraph?.nodes?.length}
              >
                Sigma graph
              </button>
            </div>
            {view === "cop" && (
              <label className="flex items-center gap-2 text-xs text-siem-muted">
                <input type="checkbox" checked={paused} onChange={(e) => setPaused(e.target.checked)} />
                Pause animation
              </label>
            )}
            <button type="button" className="siem-btn-ghost text-sm" disabled={busy} onClick={() => endTrace()}>
              End trace
            </button>
          </div>
        </div>
        {error && <p className="text-sm text-siem-err mt-2 px-4">{error}</p>}

      <div className="space-y-4 max-w-[1600px]">
        <section className="siem-card p-4">
          <div className="flex flex-wrap gap-4 mb-4 text-sm">
            <div>
              <span className="text-siem-muted">Nodes </span>
              <span className="font-semibold">{graph.nodes.length}</span>
            </div>
            <div>
              <span className="text-siem-muted">Paths </span>
              <span className="font-semibold">{paths.length}</span>
            </div>
            <div>
              <span className="text-siem-muted">Namespaces </span>
              <span className="font-mono text-xs">{graph.namespaces?.join(", ")}</span>
            </div>
          </div>
          {view === "cop" ? (
            <TraceFlowCanvas graph={graph} animate={!paused} />
          ) : (
            <div className="h-[480px] rounded-siem border border-siem-border overflow-hidden">
              <SigmaTopologyGraph
                graph={sigmaGraph ?? null}
                topology={null}
                selectedEdgeId={null}
                onSelectEdge={() => undefined}
              />
            </div>
          )}
        </section>

        <section className="grid md:grid-cols-2 gap-4">
          <div className="siem-card p-4">
            <h2 className="text-sm font-semibold mb-3">Discovered paths</h2>
            <PathTable title="Ingress" rows={ingress} />
            <PathTable title="Egress" rows={egress} />
            <PathTable title="Host" rows={host} />
          </div>
          <div className="siem-card p-4 border-l-4 border-siem-err/60">
            <h2 className="text-sm font-semibold mb-2">Packet cop (next)</h2>
            <p className="text-sm text-siem-muted">
              Live correlation and drop diagnosis will attach here once capture + eBPF hop evidence is wired to this
              trace session. Discovery skeleton is namespace-scoped from your RBAC token.
            </p>
          </div>
        </section>
      </div>
    </div>
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
