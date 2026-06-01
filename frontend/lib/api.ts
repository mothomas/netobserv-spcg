export type NamespaceRow = { name: string; status: string };

export type OwnerRef = { kind: string; name: string; uid: string };

export type PodDetail = {
  namespace: string;
  name: string;
  uid: string;
  status: string;
  pod_ip: string;
  node_name?: string;
  label_selector?: string;
  owners: OwnerRef[];
  primary_owner?: OwnerRef;
};

export type ControllerSummary = {
  kind: string;
  name: string;
  namespace: string;
  status: string;
  ready?: string;
  label_selector?: string;
};

export type NamespaceWorkloads = {
  namespace: string;
  pods: PodDetail[];
  deployments: ControllerSummary[];
  statefulsets: ControllerSummary[];
  daemonsets: ControllerSummary[];
};

export type CaptureSelection = {
  type: "pod" | "owner";
  namespace: string;
  pod_name?: string;
  pod_uid?: string;
  owner_kind?: string;
  owner_name?: string;
  label_selector?: string;
  port?: number;
};

export type AuthMode = "token" | "kubeconfig";

export type LoginResponse = {
  session_id: string;
  mode: string;
  cluster?: string;
};

export function authHeaders(sessionId: string): HeadersInit {
  return {
    "X-SPCG-Session": sessionId,
    "Content-Type": "application/json",
  };
}

export async function loginWithToken(token: string): Promise<LoginResponse> {
  const res = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: "token", token }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

function kubeconfigPayload(content: string): string {
  const trimmed = content.trim();
  if (!trimmed) return "";
  // Accept raw YAML or already-base64 payloads (matches backend DecodeKubeconfigUpload).
  if (!trimmed.includes("\n") && /^[A-Za-z0-9+/=]+$/.test(trimmed)) {
    return trimmed;
  }
  return btoa(unescape(encodeURIComponent(trimmed)));
}

export async function loginWithKubeconfig(content: string): Promise<LoginResponse> {
  const res = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: "kubeconfig", kubeconfig: kubeconfigPayload(content) }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function logout(sessionId: string): Promise<void> {
  await fetch("/api/v1/auth/logout", {
    method: "POST",
    headers: authHeaders(sessionId),
  });
}

export async function fetchNamespaces(sessionId: string): Promise<NamespaceRow[]> {
  const res = await fetch("/api/v1/namespaces", { headers: authHeaders(sessionId) });
  if (!res.ok) throw new Error(await res.text());
  const data = await res.json();
  if (!Array.isArray(data)) throw new Error("unexpected namespaces response");
  return data;
}

function normalizeWorkloads(data: NamespaceWorkloads[]): NamespaceWorkloads[] {
  return data.map((g) => ({
    ...g,
    pods: (g.pods ?? []).map((p) => ({ ...p, owners: p.owners ?? [] })),
    deployments: g.deployments ?? [],
    statefulsets: g.statefulsets ?? [],
    daemonsets: g.daemonsets ?? [],
  }));
}

export async function fetchWorkloads(sessionId: string, namespaces: string[]): Promise<NamespaceWorkloads[]> {
  const res = await fetch("/api/v1/workloads", {
    method: "POST",
    headers: authHeaders(sessionId),
    body: JSON.stringify({ namespaces }),
  });
  if (!res.ok) throw new Error(await res.text());
  const data = await res.json();
  if (!Array.isArray(data)) throw new Error("unexpected workloads response");
  return normalizeWorkloads(data);
}

export function ownerLabel(pod: PodDetail): string {
  const owners = pod.owners ?? [];
  const o = pod.primary_owner || owners[0];
  if (!o) return "—";
  return `${o.kind}/${o.name}`;
}

export function ownerKey(ns: string, kind: string, name: string): string {
  return `${ns}/${kind}/${name}`;
}

export function podUnderOwner(pod: PodDetail, kind: string, name: string): boolean {
  if (pod.primary_owner?.kind === kind && pod.primary_owner?.name === name) return true;
  return (pod.owners ?? []).some((o) => o.kind === kind && o.name === name);
}
