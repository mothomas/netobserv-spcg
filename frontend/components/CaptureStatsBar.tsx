"use client";

import type { CaptureSummary } from "@/lib/ai";
import { formatBytes, formatSrtt } from "@/lib/topology";

type Props = {
  summary: CaptureSummary | null;
  loading?: boolean;
};

export function CaptureStatsBar({ summary, loading }: Props) {
  if (loading && !summary) {
    return (
      <div className="px-5 py-3 border-b border-slate-100 text-sm text-slate-500 bg-white">
        Computing flow stats…
      </div>
    );
  }
  if (!summary || summary.event_count === 0) {
    return (
      <div className="px-5 py-3 border-b border-slate-100 text-sm text-slate-500 bg-white">
        No flow stats yet — run capture and generate traffic, then refresh.
      </div>
    );
  }

  const protos = Object.entries(summary.protocols || {}).sort((a, b) => b[1] - a[1]);
  const health = Object.entries(summary.health || {}).sort((a, b) => b[1] - a[1]);
  const flags = Object.entries(summary.tcp_flags || {})
    .filter(([, n]) => n > 0)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 6);

  return (
    <div className="px-5 py-4 border-b border-slate-100 bg-white space-y-3">
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 text-sm">
        <Stat label="Events" value={String(summary.event_count)} />
        <Stat label="Packets" value={String(summary.total_packets)} />
        <Stat label="Bytes" value={formatBytes(summary.total_bytes)} />
        <Stat label="Flow edges" value={String(summary.flow_edges)} />
        <Stat
          label="Rate"
          value={
            summary.events_per_sec && summary.events_per_sec > 0
              ? `${summary.events_per_sec.toFixed(1)} evt/s`
              : "—"
          }
        />
        <Stat
          label="Duration"
          value={
            summary.capture_duration_sec && summary.capture_duration_sec > 0
              ? `${summary.capture_duration_sec.toFixed(1)}s`
              : "—"
          }
        />
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-6 gap-3 text-xs">
        <MiniStat label="Nodes" value={String(summary.unique_nodes ?? "—")} />
        <MiniStat label="External peers" value={String(summary.external_peers ?? "—")} />
        <MiniStat label="Tracked pods" value={String(summary.tracked_pods ?? "—")} />
        <MiniStat
          label="Avg pkt"
          value={
            summary.avg_packet_bytes && summary.avg_packet_bytes > 0
              ? formatBytes(Math.round(summary.avg_packet_bytes))
              : "—"
          }
        />
        <MiniStat
          label="Avg RTT"
          value={
            summary.avg_srtt_ms && summary.avg_srtt_ms > 0
              ? formatSrtt(Math.round(summary.avg_srtt_ms * 1e6))
              : "—"
          }
        />
        <MiniStat label="DNS queries" value={String(summary.dns_queries ?? 0)} />
      </div>

      {health.length > 0 && (
        <div className="flex flex-wrap gap-2">
          <span className="text-xs font-medium text-slate-600 w-full">Edge health</span>
          {health.map(([h, n]) => (
            <Chip key={h} tone={healthTone(h)} label={`${h} · ${n}`} />
          ))}
          {(summary.drop_edges ?? 0) > 0 && (
            <Chip tone="bad" label={`drops on ${summary.drop_edges} edge(s)`} />
          )}
        </div>
      )}

      {protos.length > 0 && (
        <div className="flex flex-wrap gap-2">
          <span className="text-xs font-medium text-slate-600 w-full">Protocols</span>
          {protos.map(([p, n]) => (
            <Chip key={p} label={`${p} · ${n}`} />
          ))}
        </div>
      )}

      {flags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          <span className="text-xs font-medium text-slate-600 w-full">TCP flags</span>
          {flags.map(([f, n]) => (
            <Chip key={f} label={`${f} · ${n}`} />
          ))}
        </div>
      )}

      {summary.top_ports && summary.top_ports.length > 0 && (
        <div className="text-xs text-slate-600">
          <span className="font-medium text-slate-700">Top ports: </span>
          {summary.top_ports
            .map((p) => `${p.proto || "?"}/${p.port} (${p.count})`)
            .join(" · ")}
        </div>
      )}

      {summary.top_dns && summary.top_dns.length > 0 && (
        <div className="text-xs text-slate-600">
          <span className="font-medium text-slate-700">DNS: </span>
          {summary.top_dns.map((d) => `${d.name} (${d.count})`).join(" · ")}
        </div>
      )}

      {summary.top_talkers && summary.top_talkers.length > 0 && (
        <div className="text-xs text-slate-600">
          <span className="font-medium text-slate-700">Top talkers: </span>
          {summary.top_talkers
            .slice(0, 5)
            .map((t) => {
              const kind = t.kind === "External" ? "ext" : t.kind || "pod";
              return `${t.label || t.id} [${kind}] ${formatBytes(t.bytes)}`;
            })
            .join(" · ")}
        </div>
      )}
    </div>
  );
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-100 bg-slate-50/80 px-3 py-2">
      <div className="text-slate-500 text-xs">{label}</div>
      <div className="font-semibold text-slate-900 tabular-nums">{value}</div>
    </div>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span className="text-slate-500">{label}</span>{" "}
      <span className="font-medium text-slate-800 tabular-nums">{value}</span>
    </div>
  );
}

function Chip({ label, tone }: { label: string; tone?: "ok" | "warn" | "bad" }) {
  const cls =
    tone === "bad"
      ? "bg-red-50 text-red-800 border-red-200"
      : tone === "warn"
        ? "bg-amber-50 text-amber-900 border-amber-200"
        : tone === "ok"
          ? "bg-emerald-50 text-emerald-800 border-emerald-200"
          : "bg-slate-100 text-slate-700 border-slate-200";
  return (
    <span className={`text-xs px-2 py-0.5 rounded-full border ${cls}`}>{label}</span>
  );
}

function healthTone(h: string): "ok" | "warn" | "bad" | undefined {
  if (h === "healthy") return "ok";
  if (h === "degraded") return "warn";
  if (h === "dropped") return "bad";
  return undefined;
}
