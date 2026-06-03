"use client";

import { useMemo } from "react";
import type { EdgeDetail, TopologyEdge, TopologyNode } from "@/lib/ai";
import { conversationSteps, endpointLabel } from "@/lib/flowSequence";
import { formatBytes, formatSrtt } from "@/lib/topology";
import { SequenceLadder } from "./SequenceLadder";

type Props = {
  edge: TopologyEdge | null;
  detail: EdgeDetail | null;
  edgeDetails?: Record<string, EdgeDetail> | null;
  fromNode?: TopologyNode;
  toNode?: TopologyNode;
  onClose?: () => void;
};

export function FlowDetailsPanel({ edge, detail, edgeDetails, fromNode, toNode, onClose }: Props) {
  const srcLabel = endpointLabel(fromNode, edge?.from ?? "—");
  const dstLabel = endpointLabel(toNode, edge?.to ?? "—");

  const steps = useMemo(() => {
    if (!edge) return [];
    const fromDetail = detail?.sequence ?? [];
    if (fromDetail.some((s) => s.direction === "reverse")) return fromDetail;
    const merged = conversationSteps(edge, edgeDetails);
    return merged.length > 0 ? merged : fromDetail;
  }, [edge, detail, edgeDetails]);

  if (!edge) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-center px-8">
        <p className="text-sm font-medium text-siem-text">Flow inspection</p>
        <p className="text-sm text-siem-muted mt-2 max-w-xs">
          Select a directed edge on the topology map to view flow direction, TCP metrics, and the message sequence for that link only.
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
  const ports =
    edge.src_port || edge.dst_port
      ? `${edge.src_port || "*"} → ${edge.dst_port || "*"}`
      : "—";

  const forwardCount = steps.filter((s) => (s.direction || s.lane) !== "reverse").length;
  const replyCount = steps.length - forwardCount;

  return (
    <div className="h-full overflow-auto p-5 space-y-5">
      <div>
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-siem-text">Selected link</h3>
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

      <div className="flow-direction-banner">
        <EndpointPill title="Source" label={srcLabel} sub={fromNode?.host_ip || ipFromEndpointID(edge.from) || "—"} />
        <div className="flow-direction-arrow" aria-label="Flow direction">
          <span className="flow-direction-line" />
          <span className="flow-direction-head">▶</span>
          <span className="flow-direction-caption">{protocol !== "—" ? protocol.toUpperCase() : "FLOW"}</span>
          {ports !== "—" && <span className="flow-direction-ports">{ports}</span>}
        </div>
        <EndpointPill title="Destination" label={dstLabel} sub={toNode?.host_ip || ipFromEndpointID(edge.to) || "—"} align="right" />
      </div>

      {steps.length > 0 && (
        <div className="flex flex-wrap gap-2 text-[10px]">
          <span className="flow-seq-chip">{steps.length} messages on this link</span>
          {forwardCount > 0 && <span className="flow-seq-chip is-forward">{forwardCount} source → dest</span>}
          {replyCount > 0 && <span className="flow-seq-chip is-reverse">{replyCount} dest → source</span>}
        </div>
      )}

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
        <div className="siem-label mb-2">TCP flags (link aggregate)</div>
        <TcpFlagPills active={tcpFlags} />
      </div>

      <div>
        <h4 className="siem-label mb-2">Message sequence (this link only)</h4>
        <SequenceLadder steps={steps} srcLabel={srcLabel} dstLabel={dstLabel} proto={protocol} ports={ports} />
      </div>
    </div>
  );
}

function EndpointPill({
  title,
  label,
  sub,
  align,
}: {
  title: string;
  label: string;
  sub: string;
  align?: "right";
}) {
  return (
    <div className={`flow-endpoint-pill ${align === "right" ? "text-right" : ""}`}>
      <div className="siem-label">{title}</div>
      <div className="text-sm text-siem-text font-medium truncate mt-0.5" title={label}>
        {label}
      </div>
      <div className="text-[10px] font-mono text-siem-muted truncate mt-0.5">{sub}</div>
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
