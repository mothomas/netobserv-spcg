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

export type S3CaptureConfig = {
  enabled: boolean;
  endpoint?: string;
  region?: string;
  bucket: string;
  prefix?: string;
  access_key_id: string;
  secret_access_key: string;
  session_token?: string;
  force_path_style?: boolean;
  proxy_url?: string;
};

export type S3ExportInfo = {
  enabled: boolean;
  bucket?: string;
  object_key?: string;
  object_url?: string;
  bytes?: number;
  upload_done: boolean;
};

export type AuthMode = "kubeconfig";

export type LoginResponse = {
  session_id: string;
  mode: string;
  cluster?: string;
};

import { apiUrl, publicApiBase } from "./apiBase";
import { runtimeAuthMethods } from "./authMode";

export { apiUrl } from "./apiBase";

export type AuthConfigResponse = {
  methods: string[];
  public_api_base?: string;
  openshift?: { authorize_path: string; authorize_url?: string; error?: string };
};

/** Set API base at runtime (server-side / legacy; browser uses same-origin /api proxy). */
export function setPublicApiBase(base: string): void {
  if (typeof window === "undefined" || !base) return;
  (window as Window & { __SPCG_API_BASE__?: string }).__SPCG_API_BASE__ = base.replace(/\/$/, "");
}

function clientAuthConfigFallback(detail: string): AuthConfigResponse | null {
  const methods = runtimeAuthMethods();
  if (!methods.length) return null;
  const base = publicApiBase();
  const cfg: AuthConfigResponse = { methods, ...(base ? { public_api_base: base } : {}) };
  if (methods.includes("openshift")) {
    cfg.openshift = {
      authorize_path: "/api/v1/auth/openshift/authorize",
      ...(base ? { authorize_url: `${base}/api/v1/auth/openshift/authorize` } : {}),
      error: detail,
    };
  }
  return cfg;
}

export async function fetchAuthConfig(): Promise<AuthConfigResponse> {
  let res: Response;
  try {
    // Always same-origin: middleware/route serve auth/config from pod env when portal proxy is off.
    res = await fetch("/api/v1/auth/config", { cache: "no-store", credentials: "include" });
  } catch (err) {
    const fb = clientAuthConfigFallback(err instanceof Error ? err.message : String(err));
    if (fb) return fb;
    throw err;
  }
  const bodyText = await res.text();
  if (!res.ok) {
    const short =
      bodyText.startsWith("<!DOCTYPE") || bodyText.startsWith("<html")
        ? `HTTP ${res.status} from UI (portal unreachable or middleware error). Check spcg-ui-portal pod image and readiness.`
        : bodyText.slice(0, 500) || `HTTP ${res.status}`;
    const fb = clientAuthConfigFallback(short);
    if (fb) return fb;
    throw new Error(short);
  }
  let cfg: AuthConfigResponse;
  try {
    cfg = JSON.parse(bodyText) as AuthConfigResponse;
  } catch {
    const fb = clientAuthConfigFallback("Invalid JSON from /api/v1/auth/config");
    if (fb) return fb;
    throw new Error("Invalid JSON from /api/v1/auth/config — is spcg-ui-portal running the correct image?");
  }
  if (cfg.public_api_base) {
    setPublicApiBase(cfg.public_api_base);
  }
  return cfg;
}

/** Redirect browser to OpenShift OAuth (use spcg-api authorize_url when configured). */
export function startOpenShiftLogin(authorizePath: string, authorizeUrl?: string): void {
  if (authorizeUrl) {
    window.location.href = authorizeUrl;
    return;
  }
  window.location.href = apiUrl(authorizePath.startsWith("/") ? authorizePath : `/${authorizePath}`);
}

/** After OAuth callback stored session in sessionStorage. */
export function takePendingOpenShiftLogin(): LoginResponse | null {
  if (typeof window === "undefined") return null;
  const raw = sessionStorage.getItem("spcg_pending_auth");
  if (!raw) return null;
  sessionStorage.removeItem("spcg_pending_auth");
  try {
    return JSON.parse(raw) as LoginResponse;
  } catch {
    return null;
  }
}

export function authHeaders(sessionId: string): HeadersInit {
  return {
    "X-SPCG-Session": sessionId,
    "Content-Type": "application/json",
  };
}

export function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(apiUrl(path), { ...init, credentials: init?.credentials ?? "include" });
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
  const res = await apiFetch("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ mode: "kubeconfig", kubeconfig: kubeconfigPayload(content) }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function logout(sessionId: string): Promise<void> {
  await apiFetch("/api/v1/auth/logout", {
    method: "POST",
    headers: authHeaders(sessionId),
  });
}

/** Wipes capture PCAP/AI state; auth session remains until logout. */
export async function teardownCapture(authSessionId: string, captureSessionId: string): Promise<void> {
  const res = await apiFetch(`/api/v1/capture/teardown/${captureSessionId}`, {
    method: "POST",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok && res.status !== 204) throw new Error(await res.text());
}

/** Release a live ingest stream slot without deleting captured data (after Stop capture). */
export async function releaseCaptureStream(authSessionId: string, captureSessionId: string): Promise<void> {
  const res = await apiFetch(`/api/v1/capture/release-stream/${encodeURIComponent(captureSessionId)}`, {
    method: "POST",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok && res.status !== 204) throw new Error(await res.text());
}

/** Release all stuck stream slots for this login session. */
export async function releaseAllCaptureStreams(authSessionId: string): Promise<number> {
  const res = await apiFetch("/api/v1/capture/release-all-streams", {
    method: "POST",
    headers: authHeaders(authSessionId),
  });
  if (!res.ok) throw new Error(await res.text());
  const data = (await res.json()) as { released?: number };
  return data.released ?? 0;
}

function parseDownloadFilename(contentDisposition: string | null, fallback: string): string {
  if (!contentDisposition) return fallback;
  const star = /filename\*=UTF-8''([^;]+)/i.exec(contentDisposition);
  if (star) return decodeURIComponent(star[1]);
  const plain = /filename="?([^";]+)"?/i.exec(contentDisposition);
  return plain ? plain[1] : fallback;
}

function triggerBlobDownload(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

/** Download PCAP for one captured pod (requires auth session header). */
export async function downloadCapturePod(
  authSessionId: string,
  captureSessionId: string,
  podUid: string,
  podName?: string
): Promise<void> {
  const res = await apiFetch(
    `/api/v1/capture/download/${encodeURIComponent(captureSessionId)}?pod_uid=${encodeURIComponent(podUid)}`,
    { headers: { "X-SPCG-Session": authSessionId } }
  );
  if (!res.ok) throw new Error(await res.text());
  const blob = await res.blob();
  const fallback = podName ? `${podName}.pcapng` : `${podUid}.pcapng`;
  triggerBlobDownload(blob, parseDownloadFilename(res.headers.get("Content-Disposition"), fallback));
}

export async function fetchCaptureLimits(authSessionId: string): Promise<{
  limits: {
    max_concurrent_sessions: number;
    max_pods_per_session: number;
    max_capture_duration: string;
    max_capture_bytes: number;
    s3_offload_required: boolean;
  };
  active_capture_count: number;
}> {
  const res = await apiFetch("/api/v1/capture/limits", {
    headers: { "X-SPCG-Session": authSessionId },
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export type CaptureObservabilityResponse = {
  session_id: string;
  event_count: number;
  topology?: import("./ai").FlowTopology;
  capture_summary?: import("./ai").CaptureSummary;
  tracked_pod_ids?: string[];
  s3_export?: S3ExportInfo | null;
  graph_capped?: boolean;
  events_sampled?: boolean;
};

export async function fetchCaptureObservability(
  authSessionId: string,
  captureSessionId: string
): Promise<CaptureObservabilityResponse> {
  const res = await apiFetch("/api/v1/capture/observability", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ capture_session_id: captureSessionId, session_id: captureSessionId }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

/** Verify S3 bucket credentials before starting an S3-backed capture. */
export async function testS3Capture(authSessionId: string, cfg: S3CaptureConfig): Promise<void> {
  const res = await apiFetch("/api/v1/capture/s3/test", {
    method: "POST",
    headers: authHeaders(authSessionId),
    body: JSON.stringify({ ...cfg, enabled: true }),
  });
  if (!res.ok) throw new Error(await res.text());
}

/** Fetch a fresh presigned URL for the merged PCAP object in S3. */
export async function fetchS3ExportInfo(authSessionId: string, captureSessionId: string): Promise<S3ExportInfo> {
  const res = await apiFetch(`/api/v1/capture/s3/${encodeURIComponent(captureSessionId)}`, {
    headers: { "X-SPCG-Session": authSessionId },
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function openS3Export(authSessionId: string, captureSessionId: string): Promise<S3ExportInfo> {
  const info = await fetchS3ExportInfo(authSessionId, captureSessionId);
  if (info.object_url) {
    window.open(info.object_url, "_blank", "noopener,noreferrer");
  }
  return info;
}

/** Download merged PCAP for all pods in the capture session. */
export async function downloadCaptureMerged(authSessionId: string, captureSessionId: string): Promise<void> {
  const res = await apiFetch(`/api/v1/capture/merge/${encodeURIComponent(captureSessionId)}`, {
    headers: { "X-SPCG-Session": authSessionId },
  });
  if (!res.ok) throw new Error(await res.text());
  const ct = res.headers.get("Content-Type") || "";
  if (ct.includes("application/json")) {
    const info = (await res.json()) as S3ExportInfo;
    if (info.object_url) {
      window.open(info.object_url, "_blank", "noopener,noreferrer");
      return;
    }
    throw new Error("S3 export URL unavailable");
  }
  const blob = await res.blob();
  triggerBlobDownload(blob, parseDownloadFilename(res.headers.get("Content-Disposition"), "merged.pcapng"));
}

export async function fetchNamespaces(sessionId: string): Promise<NamespaceRow[]> {
  const res = await apiFetch("/api/v1/namespaces", { headers: authHeaders(sessionId) });
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
  const res = await apiFetch("/api/v1/workloads", {
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
