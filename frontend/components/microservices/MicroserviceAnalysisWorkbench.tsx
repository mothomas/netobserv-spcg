"use client";

import { useMemo, type ReactNode } from "react";
import type { CaptureSummary, FlowTopology, TopologyEdge, TopologyNode } from "@/lib/ai";
import { CaptureStatsBar } from "@/components/CaptureStatsBar";
import { endpointLabel } from "@/lib/trace";
import type { TraceEndpoint } from "@/lib/trace";
import { formatBytes, formatSrtt } from "@/lib/topology";

type ServiceCall = {
  id: string;
  from: string;
  to: string;
  fromLabel: string;
  toLabel: string;
  proto: string;
  port: string;
  count: number;
  bytes: number;
  health: string;
  srtt?: number;
};

type Props = {
  traceId: string;
  source: TraceEndpoint;
  destination: TraceEndpoint;
  captureSessionId: string;
  topology: FlowTopology | null;
  captureSummary: CaptureSummary | null;
  loading?: boolean;
  captureActive?: boolean;
  onRefresh?: () => void;
  onStopCapture?: () => void;
};

const L7_PORTS = new Set([80, 443, 8080, 8443, 8000, 9000, 3000, 5000, 9090]);

function nodeLabel(nodes: TopologyNode[] | undefined, id: string): string {
  const n = nodes?.find((x) => x.id === id);
  return n?.label || id;
}

function inferL7Kind(edge: TopologyEdge): string {
  const port = edge.dst_port || edge.src_port || 0;
  if (edge.proto?.toUpperCase() === "UDP" && port === 53) return "DNS";
  if (port === 443 || port === 8443) return "TLS/HTTPS";
  if (port === 80 || port === 8080 || port === 8000) return "HTTP";
  if (edge.proto?.toUpperCase() === "TCP" && L7_PORTS.has(port)) return "App TCP";
  return edge.proto?.toUpperCase() || "FLOW";
}

export function MicroserviceAnalysisWorkbench({
  traceId,
  source,
  destination,
  captureSessionId,
  topology,
  captureSummary,
  loading,
  captureActive,
  onRefresh,
  onStopCapture,
}: Props) {
  const serviceCalls = useMemo(() => {
    const edges = topology?.edges ?? [];
    const nodes = topology?.nodes;
    const rows: ServiceCall[] = edges.map((e) => ({
      id: e.id,
      from: e.from,
      to: e.to,
      fromLabel: nodeLabel(nodes, e.from),
      toLabel: nodeLabel(nodes, e.to),
      proto: inferL7Kind(e),
      port: e.dst_port ? String(e.dst_port) : e.src_port ? String(e.src_port) : "—",
      count: e.count,
      bytes: e.bytes,
      health: e.health || "healthy",
      srtt: e.srtt_ns,
    }));
    return rows.sort((a, b) => b.count - a.count);
  }, [topology]);

  const l7Ports = useMemo(() => {
    return (captureSummary?.top_ports ?? []).filter((p) => L7_PORTS.has(p.port) || p.port === 53);
  }, [captureSummary]);

  const tlsSni = captureSummary?.packet_analytics?.tls_sni ?? [];
  const topDns = captureSummary?.top_dns ?? [];

  return (
    <section className="siem-card overflow-hidden flex flex-col min-h-[720px]" aria-label="L7 microservice analysis">
      <header className="app-shell-header flex flex-wrap items-center justify-between gap-3 px-5 py-4 shrink-0 border-b border-siem-border">
        <div>
          <h2 className="text-base font-semibold text-siem-text">L7 microservice analysis</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            {endpointLabel(source)}
            <span className="mx-2 opacity-50">→</span>
            {endpointLabel(destination)}
            <span className="mx-2 opacity-30">·</span>
            trace {traceId.slice(0, 8)}… · capture {captureSessionId.slice(0, 8)}…
          </p>
        </div>
        <div className="flex gap-2 flex-wrap items-center">
          {captureActive && (
            <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30">live ingest</span>
          )}
          <button type="button" className="siem-btn-ghost text-xs" onClick={onRefresh} disabled={loading}>
            {loading ? "Refreshing…" : "Refresh"}
          </button>
          {captureActive && onStopCapture && (
            <button type="button" className="siem-btn-ghost text-xs text-siem-warn border-siem-warn/40" onClick={onStopCapture}>
              Stop capture
            </button>
          )}
        </div>
      </header>

      <CaptureStatsBar summary={captureSummary} loading={loading} />

      <div className="grid lg:grid-cols-3 gap-0 border-b border-siem-border shrink-0">
        <L7Panel title="TLS / HTTPS (SNI)">
          {tlsSni.length ? (
            <ul className="space-y-1 text-xs font-mono">
              {tlsSni.map((s) => (
                <li key={s.host} className="flex justify-between gap-2">
                  <span className="truncate text-siem-text">{s.host}</span>
                  <span className="text-siem-muted shrink-0">{s.count}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-siem-muted">No TLS ClientHello observed yet.</p>
          )}
        </L7Panel>
        <L7Panel title="DNS queries">
          {topDns.length ? (
            <ul className="space-y-1 text-xs font-mono">
              {topDns.map((d) => (
                <li key={d.name} className="flex justify-between gap-2">
                  <span className="truncate text-siem-text">{d.name}</span>
                  <span className="text-siem-muted shrink-0">{d.count}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-siem-muted">No DNS queries in capture window.</p>
          )}
        </L7Panel>
        <L7Panel title="Application ports">
          {l7Ports.length ? (
            <ul className="space-y-1 text-xs font-mono">
              {l7Ports.map((p) => (
                <li key={`${p.proto}-${p.port}`} className="flex justify-between gap-2">
                  <span className="text-siem-text">
                    {p.proto}/{p.port}
                  </span>
                  <span className="text-siem-muted shrink-0">
                    {p.count} · {formatBytes(p.bytes)}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-xs text-siem-muted">Waiting for HTTP/TLS/DNS traffic on known app ports.</p>
          )}
        </L7Panel>
      </div>

      <div className="p-5 flex-1 overflow-auto">
        <h3 className="text-sm font-semibold text-siem-text mb-1">Service-to-service calls</h3>
        <p className="text-xs text-siem-muted mb-4">
          Directed flows between resolved workloads on the trace path — scoped to RBAC-selected source pods.
        </p>
        {serviceCalls.length ? (
          <table className="w-full text-sm">
            <thead className="text-xs text-siem-muted border-b border-siem-border">
              <tr>
                <th className="text-left py-2 pr-3">Source</th>
                <th className="text-left py-2 pr-3">Destination</th>
                <th className="text-left py-2 pr-3">L7</th>
                <th className="text-left py-2 pr-3">Port</th>
                <th className="text-right py-2 pr-3">Flows</th>
                <th className="text-right py-2 pr-3">Bytes</th>
                <th className="text-right py-2">RTT</th>
              </tr>
            </thead>
            <tbody>
              {serviceCalls.map((row) => (
                <tr key={row.id} className="border-t border-siem-border hover:bg-siem-panel/30">
                  <td className="py-2 pr-3 font-mono text-xs max-w-[140px] truncate" title={row.fromLabel}>
                    {row.fromLabel}
                  </td>
                  <td className="py-2 pr-3 font-mono text-xs max-w-[140px] truncate" title={row.toLabel}>
                    {row.toLabel}
                  </td>
                  <td className="py-2 pr-3">
                    <L7Chip kind={row.proto} health={row.health} />
                  </td>
                  <td className="py-2 pr-3 font-mono text-xs text-siem-muted">{row.port}</td>
                  <td className="py-2 pr-3 text-right font-mono text-xs">{row.count}</td>
                  <td className="py-2 pr-3 text-right font-mono text-xs">{formatBytes(row.bytes)}</td>
                  <td className="py-2 text-right font-mono text-xs text-siem-muted">
                    {row.srtt ? formatSrtt(row.srtt) : "—"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="text-sm text-siem-muted">
            {loading
              ? "Building service call map from live capture…"
              : "No service edges yet. Generate traffic between source and destination, then refresh."}
          </p>
        )}
      </div>
    </section>
  );
}

function L7Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="p-4 border-b lg:border-b-0 lg:border-r border-siem-border last:border-r-0">
      <p className="text-[10px] uppercase tracking-wide text-siem-muted mb-3">{title}</p>
      {children}
    </div>
  );
}

function L7Chip({ kind, health }: { kind: string; health: string }) {
  const tone =
    health === "dropped" ? "text-siem-err border-siem-err/40" : health === "degraded" ? "text-siem-warn border-siem-warn/40" : "text-siem-accent border-siem-accent/40";
  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-md border ${tone}`}>{kind}</span>
  );
}
