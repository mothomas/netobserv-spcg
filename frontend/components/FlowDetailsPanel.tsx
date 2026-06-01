"use client";

import type { EdgeDetail, TopologyEdge, TopologyNode } from "@/lib/ai";
import { formatBytes, formatSrtt } from "@/lib/topology";
import { SequenceLadder } from "./SequenceLadder";

type Props = {
  edge: TopologyEdge | null;
  detail: EdgeDetail | null;
  fromNode?: TopologyNode;
  toNode?: TopologyNode;
};

export function FlowDetailsPanel({ edge, detail, fromNode, toNode }: Props) {
  if (!edge) {
    return (
      <div className="h-full flex flex-col items-center justify-center text-center px-8">
        <p className="text-sm font-medium text-slate-700">Flow inspection</p>
        <p className="text-sm text-slate-500 mt-2 max-w-xs">
          Select a directed edge on the topology map to view TCP metrics, kernel drop causes, and the packet ladder.
        </p>
      </div>
    );
  }

  const hasDrop = !!(edge.drop_cause || detail?.drop_cause);
  const dropCause = detail?.drop_cause || edge.drop_cause;
  const dropDx = detail?.drop_diagnosis || edge.drop_diagnosis;

  return (
    <div className="h-full overflow-auto p-5 space-y-5">
      <div>
        <h3 className="text-sm font-semibold text-slate-900">Selected flow</h3>
        <p className="text-xs font-mono text-slate-600 mt-1">
          {edge.from} → {edge.to}
        </p>
        <span
          className={`inline-block mt-2 text-[10px] uppercase tracking-wide font-semibold px-2 py-0.5 rounded ${
            edge.health === "dropped"
              ? "bg-red-50 text-red-700 border border-red-200"
              : edge.health === "degraded"
                ? "bg-amber-50 text-amber-800 border border-amber-200"
                : "bg-slate-100 text-slate-600 border border-slate-200"
          }`}
        >
          {edge.health}
        </span>
      </div>

      {hasDrop && (
        <div className="rounded-xl border border-red-200 bg-red-50/80 p-4">
          <p className="text-xs font-semibold uppercase tracking-wide text-red-800">Drop alert</p>
          <p className="text-sm font-mono text-red-900 mt-2 break-all">{dropCause}</p>
          {dropDx && <p className="text-sm text-red-800 mt-2">{dropDx}</p>}
        </div>
      )}

      <div className="grid grid-cols-2 gap-3">
        <Metric label="sRTT (smoothed)" value={formatSrtt(detail?.srtt_ns ?? edge.srtt_ns)} accent="blue" />
        <Metric label="Bytes" value={formatBytes(detail?.bytes ?? edge.bytes)} />
        <Metric label="Packets" value={String(detail?.packets ?? edge.packets)} />
        <Metric label="Protocol" value={edge.proto || "—"} />
        <Metric label="Ports" value={edge.dst_port ? `${edge.src_port || "*"} → ${edge.dst_port}` : "—"} />
        <Metric
          label="TCP flags"
          value={(detail?.tcp_flags ?? edge.tcp_flags)?.join(", ") || "—"}
        />
        <Metric label="TCP / drop state" value={detail?.tcp_state || edge.tcp_state || "—"} span={2} />
      </div>

      {(fromNode || toNode) && (
        <div className="rounded-xl border border-slate-200 bg-slate-50/80 p-3 text-xs space-y-2">
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
        <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500 mb-2">Sequence / ladder</h4>
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
}: {
  label: string;
  value: string;
  accent?: "blue";
  span?: number;
}) {
  return (
    <div
      className={`rounded-lg border border-slate-200 bg-white px-3 py-2.5 ${span === 2 ? "col-span-2" : ""}`}
    >
      <div className="text-[10px] uppercase tracking-wide text-slate-500">{label}</div>
      <div
        className={`text-sm font-semibold mt-0.5 ${accent === "blue" ? "text-blue-700" : "text-slate-900"}`}
      >
        {value}
      </div>
    </div>
  );
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
      <span className="font-semibold text-slate-700">{title}: </span>
      <span className="text-slate-600">
        {pod || owner || "—"}
        {kind && owner ? ` (${kind}/${owner})` : ""}
        {host ? ` · node ${host}` : ""}
        {node ? ` · ${node}` : ""}
      </span>
    </div>
  );
}
