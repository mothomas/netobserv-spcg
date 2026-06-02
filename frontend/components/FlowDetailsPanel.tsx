"use client";

import type { EdgeDetail, TopologyEdge, TopologyNode } from "@/lib/ai";
import { formatBytes, formatSrtt } from "@/lib/topology";
import { SequenceLadder } from "./SequenceLadder";

type Props = {
  edge: TopologyEdge | null;
  detail: EdgeDetail | null;
  fromNode?: TopologyNode;
  toNode?: TopologyNode;
  onClose?: () => void;
};

export function FlowDetailsPanel({ edge, detail, fromNode, toNode, onClose }: Props) {
  if (!edge) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-center px-8">
        <p className="text-sm font-medium text-siem-text">Flow inspection</p>
        <p className="text-sm text-siem-muted mt-2 max-w-xs">
          Select a directed edge on the topology map to view TCP metrics, kernel drop causes, and the packet ladder.
        </p>
      </div>
    );
  }

  const hasDrop = !!(edge.drop_cause || detail?.drop_cause);
  const dropCause = detail?.drop_cause || edge.drop_cause;
  const dropDx = detail?.drop_diagnosis || edge.drop_diagnosis;
  const tcpFlags = detail?.tcp_flags ?? edge.tcp_flags ?? [];
  const bytes = detail?.bytes ?? edge.bytes;
  const packets = detail?.packets ?? edge.packets;
  const srtt = detail?.srtt_ns ?? edge.srtt_ns;
  const protocol = edge.proto || "—";
  const ports = edge.dst_port ? `${edge.src_port || "*"} -> ${edge.dst_port}` : "—";

  return (
    <div className="h-full overflow-auto p-5 space-y-5">
      <div>
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-siem-text">Selected flow</h3>
          <div className="flex items-center gap-2">
            <HealthPill health={edge.health} />
            {onClose && (
              <button
                type="button"
                className="h-7 w-7 rounded-full border border-siem-border text-siem-muted hover:text-siem-text hover:bg-siem-bg transition"
                onClick={onClose}
                aria-label="Close flow panel"
              >
                ×
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <EndpointCard
          title="Source"
          node={fromNode}
          fallback={edge.from}
          iconTone="text-siem-info"
        />
        <EndpointCard
          title="Destination"
          node={toNode}
          fallback={edge.to}
          iconTone="text-siem-accentHi"
        />
      </div>

      {hasDrop && (
        <div className="rounded-md border border-siem-err/40 bg-siem-err/10 p-4">
          <p className="siem-label text-siem-err">Drop alert</p>
          <p className="text-sm font-mono text-siem-err mt-2 break-all">{dropCause}</p>
          {dropDx && <p className="text-sm text-siem-err/90 mt-2">{dropDx}</p>}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <Metric label="SRTT" value={formatSrtt(srtt)} muted={!srtt} accent="blue" />
        <Metric label="Bytes" value={formatBytes(bytes)} muted={!bytes} />
        <Metric label="Packets" value={String(packets)} muted={!packets} />
        <Metric label="Protocol" value={protocol} muted={protocol === "—"} />
        <Metric label="Ports" value={ports} muted={ports === "—"} mono />
        <Metric label="TCP / drop state" value={detail?.tcp_state || edge.tcp_state || "—"} muted={!detail?.tcp_state && !edge.tcp_state} />
      </div>

      <div className="rounded-md border border-siem-border bg-siem-card px-3 py-2.5">
        <div className="siem-label mb-2">TCP flags</div>
        <TcpFlagPills active={tcpFlags} />
      </div>

      {(fromNode || toNode) && (
        <div className="rounded-md border border-siem-border bg-siem-bg p-3 text-xs space-y-2">
          {fromNode && (
            <K8sRow
              title="Source"
              pod={fromNode.pod}
              kind={fromNode.owner_kind}
              owner={fromNode.owner_name}
              host={fromNode.host_name}
              node={fromNode.host_ip}
            />
          )}
          {toNode && (
            <K8sRow
              title="Destination"
              pod={toNode.pod}
              kind={toNode.owner_kind}
              owner={toNode.owner_name}
              host={toNode.host_name}
              node={toNode.host_ip}
            />
          )}
        </div>
      )}

      <div>
        <h4 className="siem-label mb-2">Sequence / ladder</h4>
        <SequenceLadder
          steps={detail?.sequence ?? []}
          srcLabel={fromNode?.namespace || edge.from}
          hostLabel={fromNode?.host_name || "Host network stack"}
          dstLabel={toNode?.namespace || edge.to}
        />
      </div>
    </div>
  );
}

function Metric({
  label,
  value,
  accent,
  span,
  muted,
  mono,
}: {
  label: string;
  value: string;
  accent?: "blue";
  span?: number;
  muted?: boolean;
  mono?: boolean;
}) {
  return (
    <div
      className={`rounded-md border border-siem-border bg-siem-card px-3 py-2.5 ${span === 2 ? "col-span-2" : ""}`}
    >
      <div className="siem-label">{label}</div>
      <div
        className={`text-sm font-semibold mt-0.5 ${
          muted
            ? "text-siem-muted/70"
            : accent === "blue"
              ? "text-siem-accentHi"
              : "text-siem-text"
        } ${mono ? "font-mono" : ""}`}
      >
        {value}
      </div>
    </div>
  );
}

function EndpointCard({
  title,
  node,
  fallback,
  iconTone,
}: {
  title: string;
  node?: TopologyNode;
  fallback: string;
  iconTone: string;
}) {
  const entity = node?.pod || node?.owner_name || fallback;
  const kind = node?.owner_kind || node?.kind || "peer";
  const ip = node?.host_ip || ipFromEndpointID(fallback) || "—";
  return (
    <div className="rounded-md border border-white/10 bg-siem-panel/80 px-3 py-2.5">
      <div className="siem-label">{title}</div>
      <div className="mt-1.5 flex items-center gap-2">
        <span className={`text-sm ${iconTone}`}>◉</span>
        <span className="text-sm text-siem-text truncate">
          {kind}/{entity}
        </span>
      </div>
      <div className="mt-1 text-xs font-mono text-siem-muted truncate">
        {node?.namespace || "external"} · {ip}
      </div>
    </div>
  );
}

function ipFromEndpointID(id: string): string | null {
  if (!id) return null;
  if (id.startsWith("ext/")) return id.slice(4);
  return null;
}

const FLAG_SET = ["SYN", "ACK", "FIN", "RST", "PSH", "URG", "ECE", "CWR"];

function TcpFlagPills({ active }: { active: string[] }) {
  const set = new Set(active.map((f) => f.toUpperCase()));
  return (
    <div className="flex flex-wrap gap-1.5">
      {FLAG_SET.map((flag) => {
        const on = set.has(flag);
        return (
          <span
            key={flag}
            className={`px-2 py-0.5 rounded-full border text-[10px] font-mono ${
              on
                ? "bg-siem-accent/20 border-siem-accent/40 text-siem-accentHi"
                : "bg-siem-bg border-siem-border text-siem-muted/70"
            }`}
          >
            {flag}
          </span>
        );
      })}
    </div>
  );
}

function HealthPill({ health }: { health: string }) {
  const tone =
    health === "dropped"
      ? "bg-siem-err/15 text-siem-err border-siem-err/35"
      : health === "degraded"
        ? "bg-siem-warn/15 text-siem-warn border-siem-warn/35"
        : "bg-siem-ok/15 text-siem-ok border-siem-ok/35";
  return <span className={`text-[10px] uppercase tracking-wide border rounded-full px-2.5 py-0.5 ${tone}`}>{health}</span>;
}

function K8sRow({
  title,
  pod,
  kind,
  owner,
  host,
  node,
}: {
  title: string;
  pod?: string;
  kind?: string;
  owner?: string;
  host?: string;
  node?: string;
}) {
  return (
    <div>
      <span className="font-semibold text-siem-text">{title}: </span>
      <span className="text-siem-muted">
        {pod || owner || "—"}
        {kind && owner ? ` (${kind}/${owner})` : ""}
        {host ? ` · node ${host}` : ""}
        {node ? ` · ${node}` : ""}
      </span>
    </div>
  );
}
