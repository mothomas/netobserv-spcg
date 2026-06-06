import type { CaptureSelection, PodDetail } from "@/lib/api";
import { apiFetch, authHeaders } from "@/lib/api";
import type { SigmaGraph } from "@/lib/graph";
import { normalizeSigmaGraph } from "@/lib/graph";

export type TraceEndpoint = {
  mode: "ip" | "namespace";
  ip?: string;
  external?: boolean;
  namespace?: string;
  type?: "pod" | "owner";
  pod_name?: string;
  pod_uid?: string;
  owner_kind?: string;
  owner_name?: string;
  label_selector?: string;
};

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
  focused?: boolean;
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
  source: TraceEndpoint;
  destination: TraceEndpoint;
  source_pods: PodDetail[];
  dest_pods?: PodDetail[];
  target_pod: PodDetail;
  graph: TraceGraph;
  sigma_graph?: SigmaGraph | null;
  capture_session_id?: string;
  capture_active?: boolean;
};

export type TraceStatusResponse = {
  trace_id: string;
  capture_session_id?: string;
  capture_active?: boolean;
  capture_events?: number;
  source?: TraceEndpoint;
  destination?: TraceEndpoint;
  source_pods?: number;
};

export type TraceCaptureStartResponse = {
  trace_id: string;
  capture_session_id: string;
  capture_active: boolean;
  resolved_pods?: number;
  sensor_filters?: number;
  already_running?: boolean;
};

export function endpointLabel(ep: TraceEndpoint): string {
  if (ep.mode === "ip") {
    if (!ep.ip || ep.ip === "external") return "External IP";
    return ep.external ? `${ep.ip} (external)` : ep.ip;
  }
  if (ep.type === "owner" && ep.owner_kind && ep.owner_name) {
    return `${ep.namespace}/${ep.owner_kind}/${ep.owner_name}`;
  }
  if (ep.pod_name) return `${ep.namespace}/${ep.pod_name}`;
  return ep.namespace || "namespace";
}

export function syncAppUrl(
  section: "workspace" | "flow" | "trace" | "microservices" | "ai",
  traceId?: string | null
): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (section === "workspace") {
    url.searchParams.delete("section");
    url.searchParams.delete("trace_id");
  } else {
    url.searchParams.set("section", section);
    if ((section === "trace" || section === "microservices") && traceId) {
      url.searchParams.set("trace_id", traceId);
    } else {
      url.searchParams.delete("trace_id");
    }
  }
  const qs = url.searchParams.toString();
  window.history.replaceState({}, "", qs ? `${url.pathname}?${qs}` : url.pathname);
}

export async function startTrace(
  authSessionId: string,
  namespaces: string[],
  source: TraceEndpoint,
  destination: TraceEndpoint
): Promise<TraceStartResponse> {
  const res = await apiFetch("/api/v1/trace/start", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ namespaces, source, destination }),
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
): Promise<TraceStartResponse> {
  const res = await apiFetch(`/api/v1/trace/graph?trace_id=${encodeURIComponent(traceId)}`, {
    method: "GET",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace graph failed (${res.status})`);
  }
  const data = (await res.json()) as TraceStartResponse;
  return { ...data, sigma_graph: normalizeSigmaGraph(data.sigma_graph) };
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

export async function startTraceCapture(
  authSessionId: string,
  traceId: string
): Promise<TraceCaptureStartResponse> {
  const res = await apiFetch("/api/v1/trace/capture/start", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ trace_id: traceId }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace capture start failed (${res.status})`);
  }
  return (await res.json()) as TraceCaptureStartResponse;
}

export async function stopTraceCapture(authSessionId: string, traceId: string): Promise<void> {
  const res = await apiFetch("/api/v1/trace/capture/stop", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ trace_id: traceId }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace capture stop failed (${res.status})`);
  }
}

export async function fetchTraceStatus(
  authSessionId: string,
  traceId: string
): Promise<TraceStatusResponse> {
  const res = await apiFetch(`/api/v1/trace/status?trace_id=${encodeURIComponent(traceId)}`, {
    method: "GET",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `trace status failed (${res.status})`);
  }
  return (await res.json()) as TraceStatusResponse;
}

export function defaultSourceEndpoint(): TraceEndpoint {
  return { mode: "namespace", type: "pod", namespace: "", pod_name: "" };
}

export function defaultDestEndpoint(): TraceEndpoint {
  return { mode: "ip", ip: "external", external: true };
}

export function sourceEndpointFromSelection(sel: CaptureSelection): TraceEndpoint {
  if (sel.type === "owner") {
    return {
      mode: "namespace",
      type: "owner",
      namespace: sel.namespace,
      owner_kind: sel.owner_kind,
      owner_name: sel.owner_name,
      label_selector: sel.label_selector,
    };
  }
  return {
    mode: "namespace",
    type: "pod",
    namespace: sel.namespace,
    pod_name: sel.pod_name,
    pod_uid: sel.pod_uid,
  };
}

export function validateTraceEndpoints(source: TraceEndpoint, dest: TraceEndpoint): string | null {
  if (source.mode === "namespace") {
    if (!source.namespace) return "Select a source namespace.";
    if (source.type === "owner" && (!source.owner_kind || !source.owner_name)) {
      return "Select a source Deployment, StatefulSet, or DaemonSet.";
    }
    if (source.type === "pod" && !source.pod_name) return "Select a source pod or workload.";
  } else if (source.mode === "ip" && !source.ip?.trim()) {
    return "Enter a source IP address.";
  }
  if (dest.mode === "namespace") {
    if (!dest.namespace) return "Select a destination namespace.";
    if (dest.type === "owner" && (!dest.owner_kind || !dest.owner_name)) {
      return "Select a destination workload.";
    }
    if (dest.type === "pod" && !dest.pod_name) return "Select a destination pod or workload.";
  } else if (dest.mode === "ip" && !dest.ip?.trim()) {
    return "Enter a destination IP or choose External.";
  }
  return null;
}
