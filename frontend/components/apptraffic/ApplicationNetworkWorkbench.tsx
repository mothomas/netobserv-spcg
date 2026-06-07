"use client";

import { useMemo, type ReactNode } from "react";
import type { CaptureSummary, FlowTopology, TopologyEdge, TopologyNode } from "@/lib/ai";
import { CaptureStatsBar } from "@/components/CaptureStatsBar";
import { LayerScopeBanner } from "@/components/layout/LayerScopeBanner";
import { LAYER_SCOPES } from "@/lib/sections";
import { formatBytes } from "@/lib/topology";

type TrafficRow = {
  id: string;
  direction: "ingress" | "reply" | "internal" | "other";
  fromLabel: string;
  toLabel: string;
  proto: string;
  port: string;
  count: number;
  bytes: number;
  health: string;
};

type Props = {
  scopeLabel: string;
  scopeSummary: string;
  captureSessionId: string;
  trackedPodIds: string[];
  topology: FlowTopology | null;
  captureSummary: CaptureSummary | null;
  loading?: boolean;
  captureActive?: boolean;
  onRefresh?: () => void;
  onOpenCapture?: () => void;
};

const APP_PORTS = new Set([80, 443, 8080, 8443, 8000, 9000, 3000, 5000, 9090]);

function nodeLabel(nodes: TopologyNode[] | undefined, id: string): string {
  const n = nodes?.find((x) => x.id === id);
  return n?.label || id;
}

function isTracked(id: string, tracked: Set<string>, nodes: TopologyNode[] | undefined): boolean {
  if (tracked.has(id)) return true;
  const n = nodes?.find((x) => x.id === id);
  if (!n) return false;
  if (n.pod && n.namespace) return tracked.has(`${n.namespace}/${n.pod}`);
  return false;
}

function isExternal(id: string, nodes: TopologyNode[] | undefined): boolean {
  const n = nodes?.find((x) => x.id === id);
  return n?.kind === "External" || id.startsWith("ext:") || id.includes("external");
}

function inferAppProto(edge: TopologyEdge): string {
  const port = edge.dst_port || edge.src_port || 0;
  if (edge.proto?.toUpperCase() === "UDP" && port === 53) return "DNS";
  if (port === 443 || port === 8443) return "TLS/HTTPS";
  if (port === 80 || port === 8080 || port === 8000) return "HTTP";
  if (edge.proto?.toUpperCase() === "TCP" && APP_PORTS.has(port)) return "App TCP";
  return edge.proto?.toUpperCase() || "FLOW";
}

function classifyEdge(
  edge: TopologyEdge,
  tracked: Set<string>,
  nodes: TopologyNode[] | undefined
): TrafficRow["direction"] {
  const fromTracked = isTracked(edge.from, tracked, nodes);
  const toTracked = isTracked(edge.to, tracked, nodes);
  if (isExternal(edge.from, nodes) && toTracked) return "ingress";
  if (fromTracked && isExternal(edge.to, nodes)) return "reply";
  if (fromTracked && toTracked) return "internal";
  return "other";
}

export function ApplicationNetworkWorkbench({
  scopeLabel,
  scopeSummary,
  captureSessionId,
  trackedPodIds,
  topology,
  captureSummary,
  loading,
  captureActive,
  onRefresh,
  onOpenCapture,
}: Props) {
  const tracked = useMemo(() => new Set(trackedPodIds), [trackedPodIds]);
  const nodes = topology?.nodes;

  const rows = useMemo(() => {
    return (topology?.edges ?? [])
      .map((e) => ({
        id: e.id,
        direction: classifyEdge(e, tracked, nodes),
        fromLabel: nodeLabel(nodes, e.from),
        toLabel: nodeLabel(nodes, e.to),
        proto: inferAppProto(e),
        port: e.dst_port ? String(e.dst_port) : e.src_port ? String(e.src_port) : "—",
        count: e.count,
        bytes: e.bytes,
        health: e.health || "healthy",
      }))
      .sort((a, b) => b.count - a.count);
  }, [topology, tracked, nodes]);

  const ingress = rows.filter((r) => r.direction === "ingress");
  const replies = rows.filter((r) => r.direction === "reply");
  const internal = rows.filter((r) => r.direction === "internal");

  const tlsSni = captureSummary?.packet_analytics?.tls_sni ?? [];
  const topDns = captureSummary?.top_dns ?? [];
  const appPorts = (captureSummary?.top_ports ?? []).filter((p) => APP_PORTS.has(p.port) || p.port === 53);

  return (
    <section className="siem-card overflow-hidden flex flex-col min-h-[720px]" aria-label="Application network">
      <header className="app-shell-header flex flex-wrap items-center justify-between gap-3 px-5 py-4 shrink-0 border-b border-siem-border">
        <div>
          <h2 className="text-base font-semibold text-siem-text">Application network</h2>
          <p className="text-xs text-siem-muted mt-0.5 font-mono">
            {scopeSummary}
            <span className="mx-2 opacity-30">·</span>
            capture {captureSessionId.slice(0, 8)}…
          </p>
        </div>
        <div className="flex gap-2 flex-wrap items-center">
          {captureActive && (
            <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30">live</span>
          )}
          <button type="button" className="siem-btn-ghost text-xs" onClick={onRefresh} disabled={loading}>
            {loading ? "Refreshing…" : "Refresh"}
          </button>
          {onOpenCapture && (
            <button type="button" className="siem-btn-ghost text-xs" onClick={onOpenCapture}>
              Capture scope
            </button>
          )}
        </div>
      </header>

      <div className="px-5 pt-4">
        <LayerScopeBanner layer={LAYER_SCOPES.apptraffic} detail={scopeLabel} compact />
      </div>

      <CaptureStatsBar summary={captureSummary} loading={loading} />

      <div className="grid lg:grid-cols-3 gap-0 border-b border-siem-border shrink-0">
        <SignalPanel title="TLS / HTTPS (SNI)">
          {tlsSni.length ? (
            <SignalList items={tlsSni.map((s) => ({ key: s.host, label: s.host, value: String(s.count) }))} />
          ) : (
            <EmptySignal text="No TLS ClientHello in capture window." />
          )}
        </SignalPanel>
        <SignalPanel title="DNS">
          {topDns.length ? (
            <SignalList items={topDns.map((d) => ({ key: d.name, label: d.name, value: String(d.count) }))} />
          ) : (
            <EmptySignal text="No DNS queries observed." />
          )}
        </SignalPanel>
        <SignalPanel title="Application ports">
          {appPorts.length ? (
            <SignalList
              items={appPorts.map((p) => ({
                key: `${p.proto}-${p.port}`,
                label: `${p.proto}/${p.port}`,
                value: `${p.count} · ${formatBytes(p.bytes)}`,
              }))}
            />
          ) : (
            <EmptySignal text="Waiting for HTTP/TLS/DNS on app ports." />
          )}
        </SignalPanel>
      </div>

      <div className="p-5 flex-1 overflow-auto space-y-6">
        <TrafficSection
          title="Ingress to workload"
          hint="Traffic arriving at selected pods (from services, ingress, or external peers)."
          rows={ingress}
          loading={loading}
          empty="No ingress flows yet — send traffic to the selected pods."
        />
        <TrafficSection
          title="Replies & egress"
          hint="Responses and outbound connections from selected pods."
          rows={replies}
          loading={loading}
          empty="No reply/egress flows observed yet."
        />
        <TrafficSection
          title="In-scope calls"
          hint="Calls between tracked workloads inside your capture selection."
          rows={internal}
          loading={loading}
          empty="No internal calls between selected workloads yet."
        />
      </div>
    </section>
  );
}

function TrafficSection({
  title,
  hint,
  rows,
  loading,
  empty,
}: {
  title: string;
  hint: string;
  rows: TrafficRow[];
  loading?: boolean;
  empty: string;
}) {
  return (
    <div>
      <h3 className="text-sm font-semibold text-siem-text">{title}</h3>
      <p className="text-xs text-siem-muted mb-3">{hint}</p>
      {rows.length ? <TrafficTable rows={rows} /> : <p className="text-sm text-siem-muted">{loading ? "Analyzing…" : empty}</p>}
    </div>
  );
}

function TrafficTable({ rows }: { rows: TrafficRow[] }) {
  return (
    <table className="w-full text-sm">
      <thead className="text-xs text-siem-muted border-b border-siem-border">
        <tr>
          <th className="text-left py-2 pr-3">From</th>
          <th className="text-left py-2 pr-3">To</th>
          <th className="text-left py-2 pr-3">App</th>
          <th className="text-left py-2 pr-3">Port</th>
          <th className="text-right py-2 pr-3">Flows</th>
          <th className="text-right py-2">Bytes</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((row) => (
          <tr key={row.id} className="border-t border-siem-border hover:bg-siem-panel/30">
            <td className="py-2 pr-3 font-mono text-xs max-w-[140px] truncate" title={row.fromLabel}>
              {row.fromLabel}
            </td>
            <td className="py-2 pr-3 font-mono text-xs max-w-[140px] truncate" title={row.toLabel}>
              {row.toLabel}
            </td>
            <td className="py-2 pr-3">
              <AppChip kind={row.proto} health={row.health} />
            </td>
            <td className="py-2 pr-3 font-mono text-xs text-siem-muted">{row.port}</td>
            <td className="py-2 pr-3 text-right font-mono text-xs">{row.count}</td>
            <td className="py-2 text-right font-mono text-xs">{formatBytes(row.bytes)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SignalPanel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="p-4 border-b lg:border-b-0 lg:border-r border-siem-border last:border-r-0">
      <p className="text-[10px] uppercase tracking-wide text-siem-muted mb-3">{title}</p>
      {children}
    </div>
  );
}

function SignalList({ items }: { items: { key: string; label: string; value: string }[] }) {
  return (
    <ul className="space-y-1 text-xs font-mono">
      {items.map((item) => (
        <li key={item.key} className="flex justify-between gap-2">
          <span className="truncate text-siem-text">{item.label}</span>
          <span className="text-siem-muted shrink-0">{item.value}</span>
        </li>
      ))}
    </ul>
  );
}

function EmptySignal({ text }: { text: string }) {
  return <p className="text-xs text-siem-muted">{text}</p>;
}

function AppChip({ kind, health }: { kind: string; health: string }) {
  const tone =
    health === "dropped"
      ? "text-siem-err border-siem-err/40"
      : health === "degraded"
        ? "text-siem-warn border-siem-warn/40"
        : "text-siem-accent border-siem-accent/40";
  return <span className={`text-[10px] px-1.5 py-0.5 rounded-md border ${tone}`}>{kind}</span>;
}
