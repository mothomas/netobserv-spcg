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

export async function loginWithKubeconfig(content: string): Promise<LoginResponse> {
  const res = await fetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: "kubeconfig", kubeconfig: content }),
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
  return res.json();
}

export async function fetchWorkloads(sessionId: string, namespaces: string[]): Promise<NamespaceWorkloads[]> {
  const res = await fetch("/api/v1/workloads", {
    method: "POST",
    headers: authHeaders(sessionId),
    body: JSON.stringify({ namespaces }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export function ownerLabel(pod: PodDetail): string {
  const o = pod.primary_owner || pod.owners[0];
  if (!o) return "—";
  return `${o.kind}/${o.name}`;
}

export function ownerKey(ns: string, kind: string, name: string): string {
  return `${ns}/${kind}/${name}`;
}

export function podUnderOwner(pod: PodDetail, kind: string, name: string): boolean {
  if (pod.primary_owner?.kind === kind && pod.primary_owner?.name === name) return true;
  return pod.owners.some((o) => o.kind === kind && o.name === name);
}
