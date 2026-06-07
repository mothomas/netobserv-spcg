import type { CaptureSelection, PodDetail } from "@/lib/api";
import { apiFetch, authHeaders } from "@/lib/api";
import { apiUrl } from "@/lib/apiBase";
import type { SigmaGraph } from "@/lib/graph";
import { normalizeSigmaGraph } from "@/lib/graph";
export { syncAppUrl } from "@/lib/sections";

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
  layer?: "logical" | "physical" | string;
  namespace?: string;
  rank: number;
  track?: "ingress" | "egress" | "anchor" | "shared" | "context" | string;
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
  y?: number;
  height?: number;
  track?: string;
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
  stats?: {
    total_nodes?: number;
    focused_nodes?: number;
    logical_nodes?: number;
    physical_nodes?: number;
    pruned_nodes?: number;
  };
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

export type EdgePaintState = "THEORY_ONLY" | "ACTIVE_GREEN" | "DROPPED_RED";

export type ProbeAttachInterface = {
  name: string;
  primary: boolean;
  cni?: string;
};

export type ProbeFireResponse = {
  probe_id: string;
  trace_id: string;
  paint_token: string;
  icmp_id: number;
  interface: string;
  mode: "simulate" | "capture" | "live" | string;
  primary_edges: number;
  capture_linked?: boolean;
  capture_auto_started?: boolean;
};

export type ProbeEvent = {
  type: "probe_started" | "edge_update" | "probe_finished" | "error" | "snapshot" | string;
  trace_id: string;
  probe_id?: string;
  edge_id?: string;
  state?: EdgePaintState;
  hook?: string;
  seq?: number;
  message?: string;
  drop_reason?: string;
  verified?: number;
  total?: number;
  edge_states?: Record<string, EdgePaintState>;
};

export async function fetchProbeInterfaces(
  authSessionId: string,
  traceId: string
): Promise<ProbeAttachInterface[]> {
  const res = await apiFetch(
    `/api/v1/trace/probe/interfaces?trace_id=${encodeURIComponent(traceId)}`,
    { method: "GET", headers: authHeaders(authSessionId) }
  );
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `probe interfaces failed (${res.status})`);
  }
  const data = (await res.json()) as { interfaces?: ProbeAttachInterface[] };
  return data.interfaces ?? [];
}

export async function fireTraceProbe(
  authSessionId: string,
  traceId: string,
  opts: { interface: string; simulate?: boolean; demoDrop?: boolean }
): Promise<ProbeFireResponse> {
  const res = await apiFetch("/api/v1/trace/probe/fire", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({
      trace_id: traceId,
      interface: opts.interface,
      simulate: opts.simulate ?? false,
      demo_drop: opts.demoDrop ?? false,
    }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `probe fire failed (${res.status})`);
  }
  return (await res.json()) as ProbeFireResponse;
}

export async function streamTraceProbeEvents(
  authSessionId: string,
  traceId: string,
  onEvent: (ev: ProbeEvent) => void,
  signal?: AbortSignal
): Promise<void> {
  const res = await fetch(
    apiUrl(`/api/v1/trace/probe/events?trace_id=${encodeURIComponent(traceId)}`),
    { method: "GET", headers: authHeaders(authSessionId), signal }
  );
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `probe events failed (${res.status})`);
  }
  const reader = res.body?.getReader();
  if (!reader) throw new Error("probe event stream unavailable");
  const decoder = new TextDecoder();
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const parts = buf.split("\n\n");
    buf = parts.pop() || "";
    for (const block of parts) {
      if (!block.trim()) continue;
      const dataLine = block.split("\n").find((l) => l.startsWith("data:"));
      if (!dataLine) continue;
      const data = dataLine.replace(/^data:\s*/, "");
      try {
        onEvent(JSON.parse(data) as ProbeEvent);
      } catch {
        /* ignore malformed frames */
      }
    }
  }
}
