import type { CaptureSelection, PodDetail } from "@/lib/api";
import { apiFetch, authHeaders } from "@/lib/api";
import type { SigmaGraph } from "@/lib/graph";
import { normalizeSigmaGraph } from "@/lib/graph";

export type TraceNode = {
  id: string;
  label: string;
  kind: string;
  namespace?: string;
  rank: number;
  x: number;
  y: number;
  width: number;
  height: number;
  tracked: boolean;
  status?: string;
  detail?: string;
};

export type TraceEdge = {
  id: string;
  from: string;
  to: string;
  edge_type: string;
  primary?: boolean;
  drop?: boolean;
  label?: string;
};

export type PathSummary = {
  direction: string;
  resource: string;
  namespace: string;
  kind: string;
  status: string;
  detail?: string;
};

export type TraceLane = {
  label: string;
  rank: number;
  x: number;
  width: number;
};

export type TraceGraph = {
  trace_id?: string;
  nodes: TraceNode[];
  edges: TraceEdge[];
  paths: PathSummary[];
  namespaces: string[];
  lanes?: TraceLane[];
  width: number;
  height: number;
};

export type TraceStartResponse = {
  trace_id: string;
  target_pod: PodDetail;
  graph: TraceGraph;
  sigma_graph?: SigmaGraph | null;
};

export type TraceLaunchContext = {
  session_id: string;
  trace_id: string;
  namespaces: string[];
  selections: CaptureSelection[];
};

export function syncAppUrl(section: "workspace" | "flow" | "trace" | "ai", traceId?: string | null): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (section === "workspace") {
    url.searchParams.delete("section");
    url.searchParams.delete("trace_id");
  } else {
    url.searchParams.set("section", section);
    if (section === "trace" && traceId) url.searchParams.set("trace_id", traceId);
    else url.searchParams.delete("trace_id");
  }
  const qs = url.searchParams.toString();
  window.history.replaceState({}, "", qs ? `${url.pathname}?${qs}` : url.pathname);
}

export async function startTrace(
  authSessionId: string,
  namespaces: string[],
  selections: CaptureSelection[]
): Promise<TraceStartResponse> {
  const res = await apiFetch("/api/v1/trace/start", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ namespaces, selections }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace start failed (${res.status})`);
  }
  const data = (await res.json()) as TraceStartResponse;
  return { ...data, sigma_graph: normalizeSigmaGraph(data.sigma_graph) };
}

export async function fetchTraceGraph(
  authSessionId: string,
  traceId: string
): Promise<{ trace_id: string; target_pod: PodDetail; graph: TraceGraph; sigma_graph: SigmaGraph | null }> {
  const res = await apiFetch(`/api/v1/trace/graph?trace_id=${encodeURIComponent(traceId)}`, {
    method: "GET",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace graph failed (${res.status})`);
  }
  const data = (await res.json()) as TraceStartResponse;
  return {
    trace_id: data.trace_id,
    target_pod: data.target_pod,
    graph: data.graph,
    sigma_graph: normalizeSigmaGraph(data.sigma_graph),
  };
}

export async function teardownTrace(authSessionId: string, traceId: string): Promise<void> {
  const res = await apiFetch(`/api/v1/trace/teardown/${encodeURIComponent(traceId)}`, {
    method: "POST",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace teardown failed (${res.status})`);
  }
}
