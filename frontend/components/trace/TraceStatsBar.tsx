import type { PathSummary, TraceGraph } from "@/lib/trace";

type Props = {
  graph: TraceGraph;
  phase?: string;
};

function countByDirection(paths: PathSummary[], direction: string) {
  return paths.filter((p) => p.direction === direction).length;
}

export function TraceStatsBar({ graph, phase = "discovery" }: Props) {
  const paths = graph.paths ?? [];
  const ingress = countByDirection(paths, "ingress");
  const egress = countByDirection(paths, "egress");
  const host = countByDirection(paths, "host");

  return (
    <div className="flex flex-wrap gap-x-6 gap-y-2 px-5 py-3 border-b border-siem-border bg-siem-panel/40 text-sm">
      <Stat label="Phase" value={phase} mono={false} />
      <Stat label="Nodes" value={String(graph.nodes.length)} />
      <Stat label="Paths" value={String(paths.length)} />
      <Stat label="Ingress" value={String(ingress)} />
      <Stat label="Egress" value={String(egress)} />
      <Stat label="Host" value={String(host)} />
      {graph.namespaces?.length ? (
        <Stat label="Namespaces" value={graph.namespaces.join(", ")} />
      ) : null}
    </div>
  );
}

function Stat({ label, value, mono = true }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <span className="text-siem-muted">{label} </span>
      <span className={`font-semibold text-siem-text ${mono ? "font-mono text-xs" : ""}`}>{value}</span>
    </div>
  );
}
