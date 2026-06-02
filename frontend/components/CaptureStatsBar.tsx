"use client";

import type { ReactNode } from "react";
import type { CaptureSummary } from "@/lib/ai";
import { formatBytes, formatSrtt } from "@/lib/topology";
import { TrafficBurstChart } from "./TrafficBurstChart";

type Props = {
  summary: CaptureSummary | null;
  loading?: boolean;
};

const PEER_LABELS: Record<string, string> = {
  k8s_service: "ClusterIP",
  k8s_pod: "Pod IP",
  external: "External",
  cluster_dns: "Cluster DNS",
  link_local: "Link-local",
  loopback: "Loopback",
};

export function CaptureStatsBar({ summary, loading }: Props) {
  if (loading && !summary) {
    return (
      <div className="px-5 py-4 border-b border-siem-border text-sm text-siem-muted">
        Computing incident metrics…
      </div>
    );
  }
  if (!summary || summary.event_count === 0) {
    return (
      <div className="px-5 py-4 border-b border-siem-border text-sm text-siem-muted">
        No packet metrics yet. Start capture, generate traffic, then refresh flows.
      </div>
    );
  }

  const pa = summary.packet_analytics;
  const protos = Object.entries(summary.protocols || {}).sort((a, b) => b[1] - a[1]);
  const health = Object.entries(summary.health || {}).sort((a, b) => b[1] - a[1]);
  const icmp = Object.entries(pa?.icmp || {}).sort((a, b) => b[1] - a[1]);
  const peers = Object.entries(pa?.peer_classes || {}).sort((a, b) => b[1] - a[1]);
  const dnsFail = Object.entries(pa?.dns_failures || {}).sort((a, b) => b[1] - a[1]);

  const mbps =
    summary.capture_duration_sec && summary.capture_duration_sec > 0
      ? (summary.total_bytes * 8) / summary.capture_duration_sec / 1e6
      : 0;

  return (
    <div className="border-b border-siem-border bg-siem-panel/50 space-y-4 px-5 py-4">
      <p className="text-[11px] text-siem-muted">
        Metrics reflect selected pods and capture window only — not a substitute for 24/7 NPM baselines.
      </p>

      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-8 gap-2">
        <Metric label="Events" value={String(summary.event_count)} />
        <Metric label="Packets" value={String(summary.total_packets)} />
        <Metric label="Bytes" value={formatBytes(summary.total_bytes)} />
        <Metric label="Avg rate" value={mbps > 0 ? `${mbps.toFixed(2)} Mbps` : "—"} />
        <Metric label="Flow edges" value={String(summary.flow_edges)} />
        <Metric label="Duration" value={summary.capture_duration_sec ? `${summary.capture_duration_sec.toFixed(1)}s` : "—"} />
        <Metric label="Failed TCP" value={String(pa?.tcp_failed_handshakes ?? 0)} tone={pa?.tcp_failed_handshakes ? "err" : undefined} />
        <Metric label="DNS errors" value={String(Object.values(pa?.dns_failures || {}).reduce((a, b) => a + b, 0))} tone={dnsFail.length ? "warn" : undefined} />
      </div>

      <div className="grid lg:grid-cols-3 gap-4">
        <Panel title="Transport health">
          <div className="grid grid-cols-2 gap-2 text-xs">
            <Mini label="SYN" value={pa?.tcp_syn} />
            <Mini label="SYN-ACK" value={pa?.tcp_syn_ack} />
            <Mini label="RST" value={pa?.tcp_rst} tone={pa?.tcp_rst ? "err" : undefined} />
            <Mini label="FIN" value={pa?.tcp_fin} />
            <MiniText label="Avg RTT" value={summary.avg_srtt_ms ? formatSrtt(Math.round(summary.avg_srtt_ms * 1e6)) : "—"} />
            <Mini label="Drop edges" value={summary.drop_edges} tone={summary.drop_edges ? "err" : undefined} />
          </div>
          {health.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {health.map(([h, n]) => (
                <Chip key={h} label={`${h} ${n}`} tone={h === "dropped" ? "err" : h === "degraded" ? "warn" : "ok"} />
              ))}
            </div>
          )}
        </Panel>

        <Panel title="DNS & discovery">
          <Mini label="Queries" value={summary.dns_queries} />
          {dnsFail.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {dnsFail.map(([code, n]) => (
                <Chip key={code} label={`${code} ${n}`} tone="err" />
              ))}
            </div>
          )}
          {summary.top_dns && summary.top_dns.length > 0 && (
            <p className="mt-2 text-[11px] text-siem-muted leading-relaxed">
              {summary.top_dns.slice(0, 4).map((d) => `${d.name} (${d.count})`).join(" · ")}
            </p>
          )}
          {pa?.tls_sni && pa.tls_sni.length > 0 && (
            <p className="mt-2 text-[11px] text-siem-muted">
              <span className="text-siem-text font-medium">TLS SNI: </span>
              {pa.tls_sni.map((s) => s.host).join(", ")}
            </p>
          )}
        </Panel>

        <Panel title="Peer classes">
          {peers.length === 0 ? (
            <p className="text-xs text-siem-muted">—</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {peers.map(([k, n]) => (
                <Chip key={k} label={`${PEER_LABELS[k] || k} ${n}`} />
              ))}
            </div>
          )}
          {protos.length > 0 && (
            <div className="mt-3 flex flex-wrap gap-1.5">
              {protos.map(([p, n]) => (
                <Chip key={p} label={`${p} ${n}`} />
              ))}
            </div>
          )}
          {icmp.length > 0 && (
            <p className="mt-2 text-[11px] text-siem-muted">
              ICMP: {icmp.map(([t, n]) => `${t} (${n})`).join(" · ")}
            </p>
          )}
        </Panel>
      </div>

      {summary.top_ports && summary.top_ports.length > 0 && (
        <p className="text-xs text-siem-muted">
          <span className="text-siem-text font-medium">Top ports: </span>
          {summary.top_ports.map((p) => `${p.proto}/${p.port} (${p.count})`).join(" · ")}
        </p>
      )}

      {pa?.time_buckets && pa.time_buckets.length > 0 && (
        <TrafficBurstChart
          buckets={pa.time_buckets}
          durationSec={summary.capture_duration_sec}
          peakBucketPackets={pa.peak_bucket_packets}
        />
      )}
    </div>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: "warn" | "err" }) {
  const v =
    tone === "err" ? "text-siem-err" : tone === "warn" ? "text-siem-warn" : "text-siem-text";
  return (
    <div className="siem-card px-3 py-2">
      <p className="siem-label">{label}</p>
      <p className={`text-sm font-semibold tabular-nums mt-0.5 ${v}`}>{value}</p>
    </div>
  );
}

function Mini({ label, value, tone }: { label: string; value?: number; tone?: "warn" | "err" }) {
  const v = value ?? 0;
  const c = tone === "err" ? "text-siem-err" : tone === "warn" ? "text-siem-warn" : "text-siem-text";
  return (
    <div>
      <span className="text-siem-muted">{label}</span>{" "}
      <span className={`font-mono font-medium ${c}`}>{v}</span>
    </div>
  );
}

function MiniText({ label, value, tone }: { label: string; value: string; tone?: "warn" | "err" }) {
  const c = tone === "err" ? "text-siem-err" : tone === "warn" ? "text-siem-warn" : "text-siem-text";
  return (
    <div>
      <span className="text-siem-muted">{label}</span>{" "}
      <span className={`font-mono font-medium ${c}`}>{value}</span>
    </div>
  );
}

function Panel({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="siem-card p-3">
      <p className="siem-label mb-2">{title}</p>
      {children}
    </div>
  );
}

function Chip({ label, tone }: { label: string; tone?: "ok" | "warn" | "err" }) {
  const cls =
    tone === "err"
      ? "bg-siem-err/15 text-siem-err border-siem-err/30"
      : tone === "warn"
        ? "bg-siem-warn/15 text-siem-warn border-siem-warn/30"
        : tone === "ok"
          ? "bg-siem-ok/15 text-siem-ok border-siem-ok/30"
          : "bg-siem-bg text-siem-muted border-siem-border";
  return <span className={`text-[10px] px-2 py-0.5 rounded border ${cls}`}>{label}</span>;
}
