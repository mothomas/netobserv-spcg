import { authHeaders } from "./api";

export type AIProvider =
  | "openai"
  | "anthropic"
  | "gemini"
  | "ollama"
  | "openai_compatible"
  | "cursor";

export type FlowEdge = {
  from: string;
  to: string;
  proto?: string;
  src_port?: number;
  dst_port?: number;
  count: number;
};

export type FlowGraph = {
  nodes: string[];
  edges: FlowEdge[];
  mermaid: string;
};

export type TopologyNode = {
  id: string;
  label: string;
  kind?: string;
  namespace: string;
  pod?: string;
  owner_kind?: string;
  owner_name?: string;
  host_name?: string;
  host_ip?: string;
};

export type TopologyEdge = {
  id: string;
  from: string;
  to: string;
  health: "healthy" | "degraded" | "dropped" | string;
  proto?: string;
  src_port?: number;
  dst_port?: number;
  count: number;
  bytes: number;
  packets: number;
  srtt_ns?: number;
  max_srtt_ns?: number;
  drop_cause?: string;
  drop_diagnosis?: string;
  tcp_flags?: string[];
  tcp_state?: string;
};

export type SequenceStep = {
  rel_us: number;
  lane: string;
  label: string;
  flags?: string[];
};

export type EdgeDetail = {
  edge_id: string;
  srtt_ns?: number;
  bytes: number;
  packets: number;
  tcp_flags?: string[];
  tcp_state?: string;
  drop_cause?: string;
  drop_diagnosis?: string;
  sequence?: SequenceStep[];
};

export type FlowTopology = {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  namespaces: string[];
  edge_details?: Record<string, EdgeDetail>;
};

export type CaptureSummary = {
  event_count: number;
  total_packets: number;
  total_bytes: number;
  flow_edges: number;
  unique_nodes?: number;
  external_peers?: number;
  tracked_pods?: number;
  capture_duration_sec?: number;
  events_per_sec?: number;
  avg_packet_bytes?: number;
  avg_srtt_ms?: number;
  protocols: Record<string, number>;
  health?: Record<string, number>;
  tcp_flags?: Record<string, number>;
  top_ports?: { proto: string; port: number; count: number; bytes: number }[];
  top_dns?: { name: string; count: number }[];
  top_talkers?: { id: string; label: string; kind?: string; packets: number; bytes: number }[];
  drop_edges?: number;
  dns_queries?: number;
};

export type AIContextResponse = {
  session_id: string;
  event_count: number;
  jsonl_lines: number;
  jsonl_preview: string;
  flow_graph: FlowGraph;
  topology?: FlowTopology;
  capture_summary?: CaptureSummary;
  tracked_pod_ids?: string[];
  scrub_legend: Record<string, string>;
};

export type AIChatResponse = {
  reply: string;
  flow_graph?: FlowGraph;
  topology?: FlowTopology;
  capture_summary?: CaptureSummary;
  jsonl_lines?: number;
  scrub_legend?: Record<string, string>;
};

export type AIVerifyResponse = {
  auth_ok: boolean;
  capture_ok: boolean;
  llm_ok: boolean;
  ready: boolean;
  auth_error?: string;
  capture_error?: string;
  llm_error?: string;
  capture_events?: number;
  capture_bytes?: number;
  llm_preview?: string;
};

export const PROVIDER_META: Record<
  AIProvider,
  { label: string; defaultModel: string; defaultEndpoint: string; keyHint: string }
> = {
  openai: {
    label: "ChatGPT (OpenAI)",
    defaultModel: "gpt-4o-mini",
    defaultEndpoint: "https://api.openai.com/v1/chat/completions",
    keyHint: "sk-… API key",
  },
  anthropic: {
    label: "Claude (Anthropic)",
    defaultModel: "claude-3-5-haiku-20241022",
    defaultEndpoint: "https://api.anthropic.com/v1/messages",
    keyHint: "Anthropic API key",
  },
  gemini: {
    label: "Gemini (Google)",
    defaultModel: "gemini-2.0-flash",
    defaultEndpoint: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
    keyHint: "Google AI API key",
  },
  ollama: {
    label: "Ollama (local)",
    defaultModel: "llama3.2",
    defaultEndpoint: "http://127.0.0.1:11434/api/chat",
    keyHint: "Optional bearer token",
  },
  openai_compatible: {
    label: "OpenAI-compatible (Azure / Copilot Studio)",
    defaultModel: "gpt-4o-mini",
    defaultEndpoint: "",
    keyHint: "Gateway API key",
  },
  cursor: {
    label: "Cursor (Cloud Agent)",
    defaultModel: "composer-2.5",
    defaultEndpoint: "https://api.cursor.com",
    keyHint: "Cursor API key (cursor_… or crsr_…) from Dashboard → Integrations",
  },
};

function aiBody(authSessionId: string, captureSessionId: string, extra: Record<string, unknown> = {}) {
  return JSON.stringify({ capture_session_id: captureSessionId, session_id: captureSessionId, ...extra });
}

export async function verifyAI(
  authSessionId: string,
  captureSessionId: string,
  opts: {
    provider: AIProvider;
    model: string;
    api_key?: string;
    api_endpoint?: string;
    proxy_url?: string;
    test_llm?: boolean;
  }
): Promise<AIVerifyResponse> {
  const res = await fetch("/api/v1/ai/verify", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: aiBody(authSessionId, captureSessionId, opts),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function fetchAIContext(
  authSessionId: string,
  captureSessionId: string,
  maxLines = 400
): Promise<AIContextResponse> {
  const res = await fetch("/api/v1/ai/context", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: aiBody(authSessionId, captureSessionId, { max_lines: maxLines }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function sendAIChat(
  authSessionId: string,
  captureSessionId: string,
  body: {
    message?: string;
    provider: AIProvider;
    model: string;
    proxy_url?: string;
    api_endpoint?: string;
    api_key?: string;
    reset_chat?: boolean;
    flush_session?: boolean;
  }
): Promise<AIChatResponse> {
  const res = await fetch("/api/v1/ai/chat", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: aiBody(authSessionId, captureSessionId, body),
  });
  if (!res.ok) throw new Error(await res.text());
  if (res.status === 204) return { reply: "" };
  return res.json();
}
