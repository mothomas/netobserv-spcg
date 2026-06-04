"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AIDiagnosticModal } from "@/components/AIDiagnosticModal";
import { ObservabilityWorkbench } from "@/components/ObservabilityWorkbench";
import { S3CapturePanel, type CaptureTierLimits } from "@/components/S3CapturePanel";
import { AppShell } from "@/components/layout/AppShell";
import { Sidebar, type AppSection } from "@/components/layout/Sidebar";
import { type CaptureSummary, type FlowTopology } from "@/lib/ai";
import { emptyTopology, normalizeTopology } from "@/lib/topology";
import { fetchGraphTopology, normalizeSigmaGraph, type SigmaGraph } from "@/lib/graph";
import {
  fetchNamespaces,
  fetchWorkloads,
  fetchCaptureObservability,
  ownerKey,
  ownerLabel,
  podUnderOwner,
  type CaptureSelection,
  type ControllerSummary,
  type NamespaceRow,
  type NamespaceWorkloads,
  type PodDetail,
  loginWithKubeconfig,
  fetchAuthConfig,
  setPublicApiBase,
  startOpenShiftLogin,
  apiUrl,
  takePendingOpenShiftLogin,
  logout,
  teardownCapture,
  downloadCapturePod,
  downloadCaptureMerged,
  fetchCaptureLimits,
  releaseAllCaptureStreams,
  releaseCaptureStream,
  openS3Export,
  type AuthConfigResponse,
  type LoginResponse,
  type S3CaptureConfig,
  type S3ExportInfo,
  authHeaders,
} from "@/lib/api";

type ChunkEvent = {
  session_id: string;
  pod_name: string;
  sequence: number;
  chunk_size: number;
  packets_per_sec: number;
  cumulative_bytes: number;
};

type PodMetrics = {
  packetsPerSec: number;
  cumulativeBytes: number;
  lastChunkSize: number;
  lines: string[];
};

export default function Home() {
  const [authConfig, setAuthConfig] = useState<AuthConfigResponse | null>(null);
  const [kubeconfigText, setKubeconfigText] = useState("");
  const [session, setSession] = useState<LoginResponse | null>(null);
  const [loggedIn, setLoggedIn] = useState(false);
  const [namespaces, setNamespaces] = useState<NamespaceRow[]>([]);
  const [selectedNs, setSelectedNs] = useState<Record<string, boolean>>({});
  const [workloadGroups, setWorkloadGroups] = useState<NamespaceWorkloads[]>([]);
  const [workspaceReady, setWorkspaceReady] = useState(false);
  const [selectedPods, setSelectedPods] = useState<Record<string, boolean>>({});
  const [selectedOwners, setSelectedOwners] = useState<Record<string, boolean>>({});
  const [capturing, setCapturing] = useState(false);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [metrics, setMetrics] = useState<Record<string, PodMetrics>>({});
  const [showAI, setShowAI] = useState(false);
  const [activeSection, setActiveSection] = useState<AppSection>("workspace");
  const workspaceRef = useRef<HTMLDivElement>(null);
  const flowGraphRef = useRef<HTMLDivElement>(null);
  const [topology, setTopology] = useState<FlowTopology | null>(null);
  const [sigmaGraph, setSigmaGraph] = useState<SigmaGraph | null>(null);
  const [captureSummary, setCaptureSummary] = useState<CaptureSummary | null>(null);
  const [flowsLoading, setFlowsLoading] = useState(false);
  const [graphDegraded, setGraphDegraded] = useState(false);
  const flowsInFlightRef = useRef(false);
  const [capturePods, setCapturePods] = useState<{ name: string; namespace: string; uid?: string }[]>([]);
  const [trackedPodIds, setTrackedPodIds] = useState<string[]>([]);
  const abortRef = useRef<AbortController | null>(null);

  const [loginError, setLoginError] = useState<string | null>(null);
  const [captureError, setCaptureError] = useState<string | null>(null);
  const [exportBusy, setExportBusy] = useState(false);
  const [s3Config, setS3Config] = useState<S3CaptureConfig>({
    enabled: false,
    bucket: "",
    prefix: "",
    access_key_id: "",
    secret_access_key: "",
    endpoint: "",
    region: "",
    session_token: "",
    force_path_style: false,
  });
  const [s3Tested, setS3Tested] = useState(false);
  const [s3Export, setS3Export] = useState<S3ExportInfo | null>(null);
  const [tierLimits, setTierLimits] = useState<CaptureTierLimits | null>(null);

  const navigateSection = useCallback(
    (section: AppSection) => {
      setActiveSection(section);
      if (section === "ai") {
        if (sessionId) setShowAI(true);
        return;
      }
      setShowAI(false);
      const target = section === "flow" ? flowGraphRef : workspaceRef;
      requestAnimationFrame(() => {
        target.current?.scrollIntoView({ behavior: "smooth", block: "start" });
      });
    },
    [sessionId]
  );

  const finishLogin = useCallback(async (auth: LoginResponse) => {
    setSession(auth);
    const ns = await fetchNamespaces(auth.session_id);
    setNamespaces(ns);
    setLoggedIn(true);
  }, []);

  const loginWithKubeconfigForm = useCallback(async () => {
    setLoginError(null);
    try {
      const auth = await loginWithKubeconfig(kubeconfigText);
      await finishLogin(auth);
    } catch (e) {
      setLoginError(e instanceof Error ? e.message : String(e));
      throw e;
    }
  }, [kubeconfigText, finishLogin]);

  useEffect(() => {
    fetchAuthConfig()
      .then((cfg) => {
        if (cfg.public_api_base) setPublicApiBase(cfg.public_api_base);
        setAuthConfig(cfg);
        if (cfg.openshift?.error) setLoginError(cfg.openshift.error);
      })
      .catch((e) => setLoginError(e instanceof Error ? e.message : String(e)));
  }, []);

  useEffect(() => {
    const pending = takePendingOpenShiftLogin();
    if (!pending) return;
    setLoginError(null);
    finishLogin(pending).catch((e) => {
      setLoginError(e instanceof Error ? e.message : String(e));
    });
  }, [finishLogin]);

  const resetLocalState = useCallback(() => {
    abortRef.current?.abort();
    setCapturing(false);
    setSessionId(null);
    setTopology(null);
    setCaptureSummary(null);
    setMetrics({});
    setCapturePods([]);
    setCaptureError(null);
    setS3Export(null);
  }, []);

  const handleLogout = useCallback(async () => {
    abortRef.current?.abort();
    if (session?.session_id) {
      await logout(session.session_id).catch(() => undefined);
    }
    resetLocalState();
    setSession(null);
    setLoggedIn(false);
    setWorkspaceReady(false);
    setKubeconfigText("");
  }, [session, resetLocalState]);

  const endCaptureSession = useCallback(async () => {
    if (!session?.session_id || !sessionId) return;
    abortRef.current?.abort();
    setCapturing(false);
    await teardownCapture(session.session_id, sessionId).catch(() => undefined);
    resetLocalState();
  }, [session, sessionId, resetLocalState]);

  const selectedNamespaces = useMemo(
    () => Object.keys(selectedNs).filter((n) => selectedNs[n]),
    [selectedNs]
  );

  const allPods = useMemo(() => {
    const pods: PodDetail[] = [];
    for (const g of workloadGroups) {
      pods.push(...g.pods);
    }
    return pods;
  }, [workloadGroups]);

  const enterWorkspace = useCallback(async () => {
    if (selectedNamespaces.length === 0) return;
    if (!session) return;
    const w = await fetchWorkloads(session.session_id, selectedNamespaces);
    setWorkloadGroups(w);
    setWorkspaceReady(true);
    setSelectedPods({});
    setSelectedOwners({});
    fetchCaptureLimits(session.session_id)
      .then((r) =>
        setTierLimits({
          ...r.limits,
          active_streams: r.active_capture_count,
        })
      )
      .catch(() => setTierLimits(null));
  }, [session, selectedNamespaces]);

  const refreshTierLimits = useCallback(async () => {
    if (!session?.session_id) return;
    try {
      const r = await fetchCaptureLimits(session.session_id);
      setTierLimits({ ...r.limits, active_streams: r.active_capture_count });
    } catch {
      /* ignore */
    }
  }, [session?.session_id]);

  const handleReleaseAllStreams = useCallback(async () => {
    if (!session?.session_id) return;
    setCaptureError(null);
    try {
      const n = await releaseAllCaptureStreams(session.session_id);
      await refreshTierLimits();
      if (n > 0) {
        setCaptureError(null);
      }
    } catch (e) {
      setCaptureError(e instanceof Error ? e.message : String(e));
    }
  }, [session, refreshTierLimits]);

  const toggleNs = (name: string) => {
    setSelectedNs((prev) => ({ ...prev, [name]: !prev[name] }));
  };

  const togglePod = (uid: string) => {
    setSelectedPods((prev) => ({ ...prev, [uid]: !prev[uid] }));
  };

  const toggleOwner = (c: ControllerSummary) => {
    const key = ownerKey(c.namespace, c.kind, c.name);
    const next = !selectedOwners[key];
    setSelectedOwners((prev) => ({ ...prev, [key]: next }));
    setSelectedPods((prev) => {
      const copy = { ...prev };
      for (const g of workloadGroups) {
        if (g.namespace !== c.namespace) continue;
        for (const p of g.pods) {
          if (podUnderOwner(p, c.kind, c.name)) {
            if (next) copy[p.uid] = true;
            else delete copy[p.uid];
          }
        }
      }
      return copy;
    });
  };

  const selectedOwnerList = useMemo(() => {
    const list: ControllerSummary[] = [];
    for (const g of workloadGroups) {
      for (const c of [...g.deployments, ...g.statefulsets, ...g.daemonsets]) {
        if (selectedOwners[ownerKey(c.namespace, c.kind, c.name)]) list.push(c);
      }
    }
    return list;
  }, [workloadGroups, selectedOwners]);

  const selectedPodList = useMemo(
    () => allPods.filter((p) => selectedPods[p.uid]),
    [allPods, selectedPods]
  );

  const selectionPodIds = useMemo(
    () => selectedPodList.map((p) => `${p.namespace}/${p.name}`),
    [selectedPodList]
  );

  const captureSelections = useMemo((): CaptureSelection[] => {
    const selections: CaptureSelection[] = [];
    for (const c of selectedOwnerList) {
      selections.push({
        type: "owner",
        namespace: c.namespace,
        owner_kind: c.kind,
        owner_name: c.name,
        label_selector: c.label_selector,
        port: 0,
      });
    }
    for (const p of selectedPodList) {
      const covered = selectedOwnerList.some((c) => podUnderOwner(p, c.kind, c.name) && c.namespace === p.namespace);
      if (covered) continue;
      selections.push({
        type: "pod",
        namespace: p.namespace,
        pod_name: p.name,
        pod_uid: p.uid,
        label_selector: p.label_selector,
        port: 0,
      });
    }
    return selections;
  }, [selectedOwnerList, selectedPodList]);

  const hasSelection = captureSelections.length > 0;

  const s3Active = s3Config.enabled || !!tierLimits?.s3_offload_required;

  const startCapture = async () => {
    if (!hasSelection || !session) return;
    if (s3Active && !s3Tested) {
      setCaptureError("Test S3 connection before starting capture");
      return;
    }
    setCaptureError(null);
    setCapturing(true);
    setS3Export(s3Active ? { enabled: true, upload_done: false, bucket: s3Config.bucket } : null);
    setMetrics({});
    const pods = selectedPodList.map((p) => ({ namespace: p.namespace, name: p.name, uid: p.uid }));
    setCapturePods(pods);
    setTrackedPodIds(selectionPodIds);
    const prevCap = sessionId;
    abortRef.current?.abort();
    if (prevCap) {
      await releaseCaptureStream(session.session_id, prevCap).catch(() => undefined);
    }
    abortRef.current = new AbortController();

    let res: Response;
    try {
      res = await fetch(apiUrl("/api/v1/capture/stream"), {
        method: "POST",
        headers: authHeaders(session.session_id),
        body: JSON.stringify({
          namespaces: selectedNamespaces,
          selections: captureSelections,
          s3: s3Active ? { ...s3Config, enabled: true } : { enabled: false },
        }),
        signal: abortRef.current.signal,
      });
    } catch (e) {
      if ((e as Error).name !== "AbortError") {
        setCaptureError(e instanceof Error ? e.message : String(e));
      }
      setCapturing(false);
      return;
    }

    if (!res.ok) {
      const errText = await res.text().catch(() => res.statusText);
      setCaptureError(errText || `Capture failed (${res.status})`);
      setCapturing(false);
      await refreshTierLimits();
      return;
    }

    const reader = res.body?.getReader();
    if (!reader) {
      setCaptureError("Capture stream unavailable");
      setCapturing(false);
      return;
    }
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
        const eventLine = block.split("\n").find((l) => l.startsWith("event:"));
        const dataLine = block.split("\n").find((l) => l.startsWith("data:"));
        if (!dataLine) continue;
        const data = dataLine.replace(/^data:\s*/, "");
        if (eventLine?.includes("error")) {
          setCaptureError(data);
        }
        if (eventLine?.includes("limit")) {
          setCaptureError(data);
          setCapturing(false);
        }
        if (eventLine?.includes("session")) {
          try {
            const meta = JSON.parse(data);
            setSessionId(meta.session_id || data.trim());
            if (meta.s3_enabled) {
              setS3Export((prev) => ({ enabled: true, upload_done: false, bucket: s3Config.bucket, ...prev }));
            }
          } catch {
            setSessionId(data.trim());
          }
        }
        if (eventLine?.includes("s3_finalized")) {
          try {
            setS3Export(JSON.parse(data) as S3ExportInfo);
          } catch {
            /* ignore */
          }
        }
        if (eventLine?.includes("pod_refresh")) {
          try {
            const pr = JSON.parse(data) as { pods?: { namespace: string; name: string; uid?: string }[] };
            if (pr.pods?.length) {
              setCapturePods(pr.pods.map((p) => ({ namespace: p.namespace, name: p.name, uid: p.uid })));
            }
          } catch {
            /* ignore */
          }
        }
        if (eventLine?.includes("chunk")) {
          let ev: ChunkEvent;
          try {
            ev = JSON.parse(data) as ChunkEvent;
          } catch {
            continue;
          }
          setMetrics((m) => {
            const prev = m[ev.pod_name] || { packetsPerSec: 0, cumulativeBytes: 0, lastChunkSize: 0, lines: [] };
            const line = `[${ev.sequence}] chunk=${ev.chunk_size}B pps=${ev.packets_per_sec}`;
            return {
              ...m,
              [ev.pod_name]: {
                packetsPerSec: ev.packets_per_sec,
                cumulativeBytes: ev.cumulative_bytes,
                lastChunkSize: ev.chunk_size,
                lines: [...prev.lines.slice(-40), line],
              },
            };
          });
        }
      }
    }
    setCapturing(false);
    await refreshTierLimits();
  };

  const stopCapture = async () => {
    const cap = sessionId;
    abortRef.current?.abort();
    setCapturing(false);
    if (session?.session_id && cap) {
      await releaseCaptureStream(session.session_id, cap).catch(() => undefined);
    }
    await refreshTierLimits();
  };

  const loadFlowTopology = useCallback(async () => {
    if (!session?.session_id || !sessionId || flowsInFlightRef.current) return;
    flowsInFlightRef.current = true;
    setFlowsLoading(true);
    try {
      const obs = await fetchCaptureObservability(session.session_id, sessionId);
      setTopology(normalizeTopology(obs.topology));
      setCaptureSummary(obs.capture_summary ?? null);
      setGraphDegraded(!!(obs.graph_capped || obs.events_sampled));
      if (obs.s3_export) setS3Export(obs.s3_export);
      setTrackedPodIds(obs.tracked_pod_ids?.length ? obs.tracked_pod_ids : selectionPodIds);
      try {
        const g = await fetchGraphTopology(session.session_id, sessionId);
        setSigmaGraph(normalizeSigmaGraph(g));
      } catch {
        /* keep stats/topology from observability; graph may retry next tick */
      }
    } catch {
      setTopology(emptyTopology());
      setCaptureSummary(null);
    } finally {
      flowsInFlightRef.current = false;
      setFlowsLoading(false);
    }
  }, [session, sessionId, selectionPodIds]);

  useEffect(() => {
    if (sessionId && session?.session_id) {
      loadFlowTopology();
    }
  }, [sessionId, session?.session_id, loadFlowTopology]);

  useEffect(() => {
    if (!sessionId || !capturing || !session?.session_id) return;
    const id = window.setInterval(() => loadFlowTopology(), 8000);
    return () => window.clearInterval(id);
  }, [sessionId, capturing, session?.session_id, loadFlowTopology]);

  useEffect(() => {
    if (!workspaceReady || !session?.session_id) return;
    const id = window.setInterval(() => refreshTierLimits(), 5000);
    return () => window.clearInterval(id);
  }, [workspaceReady, session?.session_id, refreshTierLimits]);

  const exportPodList = useMemo((): { uid: string; name: string; namespace: string }[] => {
    if (capturePods.length > 0) {
      return capturePods
        .filter((p) => p.uid)
        .map((p) => ({ uid: p.uid!, name: p.name, namespace: p.namespace }));
    }
    return selectedPodList.map((p) => ({ uid: p.uid, name: p.name, namespace: p.namespace }));
  }, [capturePods, selectedPodList]);

  const handleDownloadPod = useCallback(
    async (pod: { uid: string; name: string }) => {
      if (!session?.session_id || !sessionId) return;
      setExportBusy(true);
      setCaptureError(null);
      try {
        await downloadCapturePod(session.session_id, sessionId, pod.uid, pod.name);
      } catch (e) {
        setCaptureError(e instanceof Error ? e.message : String(e));
      } finally {
        setExportBusy(false);
      }
    },
    [session, sessionId]
  );

  const handleOpenS3 = useCallback(async () => {
    if (!session?.session_id || !sessionId) return;
    setExportBusy(true);
    setCaptureError(null);
    try {
      const info = await openS3Export(session.session_id, sessionId);
      setS3Export(info);
    } catch (e) {
      setCaptureError(e instanceof Error ? e.message : String(e));
    } finally {
      setExportBusy(false);
    }
  }, [session, sessionId]);

  const captureStartBlocked = !hasSelection || (s3Active && !s3Tested);
  const handleDownloadMerged = useCallback(async () => {
    if (!session?.session_id || !sessionId) return;
    setExportBusy(true);
    setCaptureError(null);
    try {
      await downloadCaptureMerged(session.session_id, sessionId);
    } catch (e) {
      setCaptureError(e instanceof Error ? e.message : String(e));
    } finally {
      setExportBusy(false);
    }
  }, [session, sessionId]);

  if (!loggedIn) {
    return (
      <main className="min-h-screen flex items-center justify-center p-6 bg-siem-bg app-shell-root">
        <div className="w-full max-w-lg siem-card p-8">
          <div className="flex items-start justify-between gap-4 mb-6">
            <div className="flex items-center gap-3">
              <span className="fluent-logo-mark h-10 w-10">SPCG</span>
              <div>
                <h1 className="text-xl font-semibold text-siem-text">Secure Packet Capture Gateway</h1>
                <p className="text-sm text-siem-muted">Kubernetes · NetObserv eBPF</p>
              </div>
            </div>
          </div>
          <p className="text-siem-muted text-sm mb-4">
            {authConfig?.methods.includes("openshift")
              ? "Log in with your OpenShift username and password on the cluster login page. Access follows your RoleBindings."
              : "Upload a kubeconfig for your Kubernetes cluster. Credentials stay in session memory only and are wiped on sign out."}
          </p>
          {authConfig?.methods.includes("openshift") && (
            <button
              type="button"
              className="w-full siem-btn-primary py-2.5 mb-4"
              disabled={!authConfig.openshift?.authorize_url && !authConfig.openshift?.authorize_path}
              onClick={() => {
                setLoginError(null);
                if (!authConfig.openshift) return;
                startOpenShiftLogin(
                  authConfig.openshift.authorize_path,
                  authConfig.openshift.authorize_url
                );
              }}
            >
              Log in via OpenShift
            </button>
          )}
          {loginError && authConfig?.methods.includes("openshift") && !authConfig.methods.includes("kubeconfig") && (
            <p className="mb-3 text-sm text-siem-err whitespace-pre-wrap">{loginError}</p>
          )}
          {authConfig?.methods.includes("kubeconfig") && (
            <>
              {authConfig?.openshift && (
                <p className="text-xs text-siem-muted mb-2 text-center">— or use kubeconfig (lab / break-glass) —</p>
              )}
              <input
                type="file"
                accept=".yaml,.yml,.config"
                className="w-full text-sm mb-2 text-siem-muted"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (!f) return;
                  const reader = new FileReader();
                  reader.onload = () => setKubeconfigText(String(reader.result || ""));
                  reader.readAsText(f);
                }}
              />
              <textarea
                className="siem-input h-40 font-mono text-xs"
                placeholder="Paste kubeconfig YAML"
                value={kubeconfigText}
                onChange={(e) => setKubeconfigText(e.target.value)}
              />
              {loginError && <p className="mt-3 text-sm text-siem-err whitespace-pre-wrap">{loginError}</p>}
              <button
                type="button"
                className="mt-4 w-full fluent-tab-inactive py-2.5 border border-siem-border"
                onClick={() => loginWithKubeconfigForm().catch(() => undefined)}
              >
                Sign in with kubeconfig
              </button>
            </>
          )}
          {!authConfig && !loginError && (
            <p className="text-sm text-siem-muted">Loading sign-in options…</p>
          )}
          {authConfig && !authConfig.methods.includes("kubeconfig") && !authConfig.openshift && loginError && (
            <p className="mt-3 text-sm text-siem-err whitespace-pre-wrap">{loginError}</p>
          )}
          {authConfig?.methods.includes("openshift") && !authConfig.openshift?.authorize_path && (
            <p className="mt-3 text-sm text-siem-err">
              OpenShift login is enabled but OAuth is not configured on the portal (set OAUTH_CLIENT_ID and URLs).
            </p>
          )}
        </div>
      </main>
    );
  }

  if (!workspaceReady) {
    return (
      <main className="min-h-screen app-shell-root p-8 max-w-4xl mx-auto">
        <h1 className="text-xl font-semibold text-siem-text mb-2">Scope namespaces</h1>
        <p className="text-siem-muted text-sm mb-6">Tenant boundary — only selected namespaces appear in capture and topology.</p>
        <div className="grid gap-2 mb-6">
          {namespaces.map((n) => (
            <label
              key={n.name}
              className="fluent-scope-row siem-card border border-siem-border"
            >
              <input type="checkbox" checked={!!selectedNs[n.name]} onChange={() => toggleNs(n.name)} />
              <span className="font-mono text-siem-text">{n.name}</span>
              <span className="text-siem-muted text-sm ml-auto">{n.status}</span>
            </label>
          ))}
        </div>
        <button
          className="siem-btn-primary disabled:opacity-40"
          disabled={selectedNamespaces.length === 0}
          onClick={() => enterWorkspace().catch((e) => alert(String(e)))}
        >
          Enter workspace ({selectedNamespaces.length})
        </button>
      </main>
    );
  }

  const topbar = (
    <div className="flex flex-wrap items-center justify-between gap-4">
      <div>
        <h1 className="text-lg font-semibold text-siem-text">Investigation workspace</h1>
        <p className="text-xs text-siem-muted font-mono mt-0.5">
          {session?.cluster ? `${session.cluster} · ` : ""}
          {selectedNamespaces.join(", ")}
        </p>
      </div>
      <div className="flex gap-2">
        {!capturing ? (
          <button
            type="button"
            className="fluent-capture-start"
            onClick={startCapture}
            disabled={captureStartBlocked}
          >
            Start capture
          </button>
        ) : (
          <button type="button" className="fluent-capture-stop" onClick={stopCapture}>
            Stop capture
          </button>
        )}
      </div>
    </div>
  );

  return (
    <AppShell
      sidebar={
        <Sidebar
          product="SPCG"
          cluster={session?.cluster}
          sessionActive={!!session}
          captureActive={capturing}
          active={showAI ? "ai" : activeSection}
          flowAvailable={!!sessionId}
          aiAvailable={!!sessionId}
          onNavigate={navigateSection}
          onSignOut={() => handleLogout()}
        />
      }
      topbar={topbar}
    >
      <div className="space-y-6 max-w-[1400px]">

      <div ref={workspaceRef} className="space-y-6">
      {session && (
        <S3CapturePanel
          authSessionId={session.session_id}
          tierLimits={tierLimits}
          value={s3Config}
          onChange={setS3Config}
          tested={s3Tested}
          onTested={setS3Tested}
          disabled={capturing}
        />
      )}

      {workloadGroups.map((g) => (
        <section key={g.namespace} className="siem-card overflow-hidden">
          <div className="px-4 py-2 bg-siem-panel text-sm font-mono text-siem-text border-b border-siem-border">
            Namespace: {g.namespace}
          </div>

          <ControllerTable
            title="Deployments"
            rows={g.deployments}
            selectedOwners={selectedOwners}
            onToggle={toggleOwner}
          />
          <ControllerTable
            title="StatefulSets"
            rows={g.statefulsets}
            selectedOwners={selectedOwners}
            onToggle={toggleOwner}
          />
          <ControllerTable
            title="DaemonSets"
            rows={g.daemonsets}
            selectedOwners={selectedOwners}
            onToggle={toggleOwner}
          />

          <div className="px-4 py-2 text-xs text-siem-muted border-t border-siem-border">Pods</div>
          <table className="w-full text-sm">
            <thead className="bg-siem-panel text-siem-muted">
              <tr>
                <th className="p-3 w-10" />
                <th className="p-3 text-left">Pod</th>
                <th className="p-3 text-left">Owner</th>
                <th className="p-3 text-left">Status</th>
                <th className="p-3 text-left">Pod IP</th>
              </tr>
            </thead>
            <tbody>
              {g.pods.map((p) => (
                <tr key={p.uid} className="border-t border-siem-border hover:bg-siem-panel/50">
                  <td className="p-3">
                    <input type="checkbox" checked={!!selectedPods[p.uid]} onChange={() => togglePod(p.uid)} />
                  </td>
                  <td className="p-3 font-mono">{p.name}</td>
                  <td className="p-3 text-siem-muted">{ownerLabel(p)}</td>
                  <td className="p-3">
                    <StatusBadge status={p.status} />
                  </td>
                  <td className="p-3 font-mono">{p.pod_ip || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}

      {capturePods.length > 0 && (
        <p className="fluent-alert-info">
          Watching {capturePods.length} pod(s):{" "}
          {capturePods.map((p) => `${p.namespace}/${p.name}`).join(", ")}
          {capturing ? " · restarts update sensor filters automatically" : ""}
        </p>
      )}

      {captureError && (
        <div className="fluent-alert-err">
          <span>Capture error: {captureError}</span>
          {captureError.includes("concurrent") && session && (
            <button type="button" className="siem-btn-ghost text-xs" onClick={() => handleReleaseAllStreams()}>
              Clear stuck streams
            </button>
          )}
        </div>
      )}

      {(capturing || sessionId) && (
        <section className="space-y-3">
          <h2 className="siem-label">Live capture stream</h2>
          {Object.entries(metrics).map(([pod, m]) => (
            <div key={pod} className="siem-card p-4">
              <div className="flex justify-between text-sm mb-2">
                <span className="font-mono text-siem-text">{pod}</span>
                <span className="text-siem-muted">
                  {m.packetsPerSec} pkt/s · {m.cumulativeBytes} bytes
                </span>
              </div>
              <pre className="font-mono text-xs bg-siem-bg rounded-md p-3 max-h-32 overflow-auto text-siem-ok border border-siem-border">
                {m.lines.join("\n") || "Awaiting sensor packets…"}
              </pre>
            </div>
          ))}
        </section>
      )}

      </div>

      {sessionId && (
        <div ref={flowGraphRef} className="scroll-mt-6">
          <ObservabilityWorkbench
            topology={topology}
            sigmaGraph={sigmaGraph}
            captureSummary={captureSummary}
            trackedPodIds={trackedPodIds.length ? trackedPodIds : selectionPodIds}
            loading={flowsLoading}
            graphDegraded={graphDegraded}
            onRefresh={() => loadFlowTopology()}
            onEndSession={() => endCaptureSession()}
            sessionLabel={sessionId}
            capturePods={exportPodList}
            exportBusy={exportBusy}
            s3Export={s3Export}
            onDownloadPod={(p) => handleDownloadPod(p)}
            onDownloadMerged={() => handleDownloadMerged()}
            onOpenS3={() => handleOpenS3()}
          />
        </div>
      )}

      {showAI && sessionId && session && (
        <AIDiagnosticModal
          open={showAI}
          sessionId={sessionId}
          authSessionId={session.session_id}
          onClose={() => {
            setShowAI(false);
            setActiveSection(sessionId ? "flow" : "workspace");
          }}
        />
      )}
      </div>
    </AppShell>
  );
}

function ControllerTable({
  title,
  rows,
  selectedOwners,
  onToggle,
}: {
  title: string;
  rows: ControllerSummary[];
  selectedOwners: Record<string, boolean>;
  onToggle: (c: ControllerSummary) => void;
}) {
  if (rows.length === 0) return null;
  return (
    <table className="w-full text-sm border-t border-siem-border">
      <thead className="bg-siem-panel text-siem-muted">
        <tr>
          <th className="p-2 w-10" />
          <th className="p-2 text-left">{title}</th>
          <th className="p-2 text-left">Status</th>
          <th className="p-2 text-left">Ready</th>
        </tr>
      </thead>
      <tbody>
        {rows.map((c) => {
          const key = ownerKey(c.namespace, c.kind, c.name);
          return (
            <tr key={key} className="border-t border-siem-border hover:bg-siem-panel/50">
              <td className="p-2">
                <input type="checkbox" checked={!!selectedOwners[key]} onChange={() => onToggle(c)} />
              </td>
              <td className="p-2 font-mono">
                {c.kind}/{c.name}
              </td>
              <td className="p-2">
                <StatusBadge status={c.status} />
              </td>
              <td className="p-2 text-siem-muted">{c.ready || "—"}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function StatusBadge({ status }: { status: string }) {
  const color =
    status === "Running"
      ? "text-siem-ok bg-siem-ok/15 border border-siem-ok/30 px-2 py-0.5 rounded"
      : status === "Failed"
        ? "text-siem-err bg-siem-err/15 border border-siem-err/30 px-2 py-0.5 rounded"
        : "text-siem-warn bg-siem-warn/15 border border-siem-warn/30 px-2 py-0.5 rounded";
  return <span className={`text-xs font-medium ${color}`}>{status}</span>;
}
