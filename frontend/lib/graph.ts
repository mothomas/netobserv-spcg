import type { EdgeDetail } from "@/lib/ai";
import { apiFetch, authHeaders } from "@/lib/api";

export type SigmaNode = {
  id: string;
  label: string;
  x: number;
  y: number;
  size: number;
  color: string;
  border?: string;
  tracked: boolean;
  type?: string;
};

export type SigmaEdge = {
  id: string;
  source: string;
  target: string;
  label: string;
  color: string;
  size: number;
  topology_edge_id: string;
  edge_type: string;
  external_ip?: string;
  country_code?: string;
};

export type SigmaGraph = {
  capture_id: string;
  nodes: SigmaNode[];
  edges: SigmaEdge[];
  edge_details?: Record<string, EdgeDetail>;
};

export async function fetchGraphTopology(
  authSessionId: string,
  captureSessionId: string
): Promise<SigmaGraph> {
  const res = await apiFetch("/api/v1/graph/topology", {
    method: "POST",
    headers: {
      ...authHeaders(authSessionId),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ capture_session_id: captureSessionId }),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `graph fetch failed (${res.status})`);
  }
  return res.json() as Promise<SigmaGraph>;
}

export function normalizeSigmaGraph(g: SigmaGraph | null | undefined): SigmaGraph | null {
  if (!g) return null;
  return {
    ...g,
    nodes: g.nodes ?? [],
    edges: g.edges ?? [],
    edge_details: g.edge_details ?? {},
  };
}
