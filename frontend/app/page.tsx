"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AIDiagnosticModal } from "@/components/AIDiagnosticModal";
import { ObservabilityWorkbench } from "@/components/ObservabilityWorkbench";
import { fetchAIContext, type CaptureSummary, type FlowTopology } from "@/lib/ai";
import { emptyTopology } from "@/lib/topology";
import {
  fetchNamespaces,
  fetchWorkloads,
  ownerKey,
  ownerLabel,
  podUnderOwner,
  type CaptureSelection,
  type ControllerSummary,
  type NamespaceRow,
  type NamespaceWorkloads,
  type PodDetail,
  loginWithKubeconfig,
  loginWithToken,
  logout,
  type AuthMode,
  type LoginResponse,
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
  const [authMode, setAuthMode] = useState<AuthMode>("kubeconfig");
  const [token, setToken] = useState("");
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
  const [topology, setTopology] = useState<FlowTopology | null>(null);
  const [captureSummary, setCaptureSummary] = useState<CaptureSummary | null>(null);
  const [flowsLoading, setFlowsLoading] = useState(false);
  const [capturePods, setCapturePods] = useState<{ name: string; namespace: string; uid?: string }[]>([]);
  const [trackedPodIds, setTrackedPodIds] = useState<string[]>([]);
  const abortRef = useRef<AbortController | null>(null);

  const [loginError, setLoginError] = useState<string | null>(null);
  const [captureError, setCaptureError] = useState<string | null>(null);

  const login = useCallback(async () => {
    setLoginError(null);
    try {
      const auth =
        authMode === "kubeconfig"
          ? await loginWithKubeconfig(kubeconfigText)
          : await loginWithToken(token);
      setSession(auth);
      const ns = await fetchNamespaces(auth.session_id);
      setNamespaces(ns);
      setLoggedIn(true);
    } catch (e) {
      setLoginError(e instanceof Error ? e.message : String(e));
      throw e;
    }
  }, [authMode, token, kubeconfigText]);

  const handleLogout = useCallback(async () => {
    if (session?.session_id) {
      await logout(session.session_id).catch(() => undefined);
    }
    setSession(null);
    setLoggedIn(false);
    setWorkspaceReady(false);
    setKubeconfigText("");
    setToken("");
  }, [session]);

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
  }, [session, selectedNamespaces]);

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

  const startCapture = async () => {
    if (!hasSelection || !session) return;
    setCaptureError(null);
    setCapturing(true);
    setMetrics({});
    const pods = selectedPodList.map((p) => ({ namespace: p.namespace, name: p.name, uid: p.uid }));
    setCapturePods(pods);
    setTrackedPodIds(selectionPodIds);
    abortRef.current?.abort();
    abortRef.current = new AbortController();

    let res: Response;
    try {
      res = await fetch("/api/v1/capture/stream", {
        method: "POST",
        headers: authHeaders(session.session_id),
        body: JSON.stringify({
          namespaces: selectedNamespaces,
          selections: captureSelections,
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
        if (eventLine?.includes("session")) {
          try {
            const meta = JSON.parse(data);
            setSessionId(meta.session_id || data.trim());
          } catch {
            setSessionId(data.trim());
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
  };

  const stopCapture = () => {
    abortRef.current?.abort();
    setCapturing(false);
  };

  const loadFlowTopology = useCallback(async () => {
    if (!session?.session_id || !sessionId) return;
    setFlowsLoading(true);
    try {
      const ctx = await fetchAIContext(session.session_id, sessionId, 400);
      setTopology(ctx.topology ?? emptyTopology());
      setCaptureSummary(ctx.capture_summary ?? null);
      setTrackedPodIds(ctx.tracked_pod_ids?.length ? ctx.tracked_pod_ids : selectionPodIds);
    } catch {
      setTopology(emptyTopology());
      setCaptureSummary(null);
    } finally {
      setFlowsLoading(false);
    }
  }, [session, sessionId, selectionPodIds]);

  useEffect(() => {
    if (sessionId && !capturing && session?.session_id) {
      loadFlowTopology();
    }
  }, [sessionId, capturing, session?.session_id, loadFlowTopology]);

  useEffect(() => {
    if (!sessionId || !capturing || !session?.session_id) return;
    const id = window.setInterval(() => loadFlowTopology(), 4000);
    return () => window.clearInterval(id);
  }, [sessionId, capturing, session?.session_id, loadFlowTopology]);

  const downloadPod = (podUid: string) => {
    if (!sessionId) return;
    window.open(`/api/v1/capture/download/${sessionId}?pod_uid=${podUid}`, "_blank");
  };

  const mergePcap = () => {
    if (!sessionId) return;
    window.open(`/api/v1/capture/merge/${sessionId}`, "_blank");
  };

  if (!loggedIn) {
    return (
      <main className="min-h-screen flex items-center justify-center p-6 bg-gradient-to-b from-slate-50 to-white">
        <div className="w-full max-w-lg bg-white rounded-2xl border border-slate-200 p-8 shadow-lg">
          <div className="flex items-center gap-3 mb-6">
            <span className="h-10 w-10 rounded-xl bg-blue-600 text-white flex items-center justify-center font-bold text-sm">
              SPCG
            </span>
            <div>
              <h1 className="text-xl font-semibold text-slate-900">Packet observability</h1>
              <p className="text-sm text-slate-500">Kubernetes · NetObserv eBPF</p>
            </div>
          </div>
          <p className="text-slate-600 text-sm mb-4">
            Connect with kubeconfig or bearer token. Credentials remain in browser memory only.
          </p>
          <div className="flex gap-2 mb-4">
            <button
              className={`flex-1 py-2 rounded-lg text-sm font-medium ${
                authMode === "kubeconfig" ? "bg-blue-600 text-white" : "border border-slate-300 text-slate-700"
              }`}
              onClick={() => setAuthMode("kubeconfig")}
            >
              Kubeconfig
            </button>
            <button
              className={`flex-1 py-2 rounded-lg text-sm font-medium ${
                authMode === "token" ? "bg-blue-600 text-white" : "border border-slate-300 text-slate-700"
              }`}
              onClick={() => setAuthMode("token")}
            >
              Bearer token
            </button>
          </div>
          {authMode === "kubeconfig" ? (
            <>
              <input
                type="file"
                accept=".yaml,.yml,.config"
                className="w-full text-sm mb-2 text-slate-600"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (!f) return;
                  const reader = new FileReader();
                  reader.onload = () => setKubeconfigText(String(reader.result || ""));
                  reader.readAsText(f);
                }}
              />
              <textarea
                className="w-full h-40 rounded-lg border border-slate-300 p-3 font-mono text-xs text-slate-800"
                placeholder="Paste kubeconfig YAML"
                value={kubeconfigText}
                onChange={(e) => setKubeconfigText(e.target.value)}
              />
            </>
          ) : (
            <textarea
              className="w-full h-28 rounded-lg border border-slate-300 p-3 font-mono text-sm text-slate-800"
              placeholder="Bearer token"
              value={token}
              onChange={(e) => setToken(e.target.value.trim())}
            />
          )}
          {loginError && <p className="mt-3 text-sm text-red-600 whitespace-pre-wrap">{loginError}</p>}
          <button
            className="mt-4 w-full py-2.5 rounded-lg bg-blue-600 hover:bg-blue-700 text-white font-medium"
            onClick={() => login().catch(() => undefined)}
          >
            Sign in
          </button>
        </div>
      </main>
    );
  }

  if (!workspaceReady) {
    return (
      <main className="p-8 max-w-4xl mx-auto min-h-screen bg-slate-50">
        <h1 className="text-xl font-semibold text-slate-900 mb-2">Namespaces</h1>
        <p className="text-slate-500 text-sm mb-6">Select namespaces for capture and topology view.</p>
        <div className="grid gap-2 mb-6">
          {namespaces.map((n) => (
            <label
              key={n.name}
              className="flex items-center gap-3 px-4 py-3 rounded-xl bg-white border border-slate-200 cursor-pointer hover:border-blue-300"
            >
              <input type="checkbox" checked={!!selectedNs[n.name]} onChange={() => toggleNs(n.name)} />
              <span className="font-mono text-slate-800">{n.name}</span>
              <span className="text-slate-500 text-sm ml-auto">{n.status}</span>
            </label>
          ))}
        </div>
        <button
          className="px-4 py-2 rounded-lg bg-blue-600 text-white font-medium disabled:opacity-40"
          disabled={selectedNamespaces.length === 0}
          onClick={() => enterWorkspace().catch((e) => alert(String(e)))}
        >
          Continue ({selectedNamespaces.length})
        </button>
      </main>
    );
  }

  return (
    <main className="min-h-screen bg-slate-50 p-6 max-w-[1400px] mx-auto space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4 bg-white rounded-2xl border border-slate-200 px-5 py-4 shadow-sm">
        <div>
          <h1 className="text-xl font-semibold text-slate-900">Observability workspace</h1>
          <p className="text-slate-500 text-sm font-mono mt-0.5">
            {session?.cluster ? `${session.cluster} · ` : ""}
            {selectedNamespaces.join(", ")}
          </p>
          <button className="text-xs text-slate-500 hover:text-slate-800 mt-1" onClick={() => handleLogout()}>
            Sign out
          </button>
        </div>
        <div className="flex gap-2">
          {!capturing ? (
            <button
              className="px-4 py-2 rounded-lg bg-emerald-600 text-white font-medium hover:bg-emerald-700 disabled:opacity-40"
              onClick={startCapture}
              disabled={!hasSelection}
            >
              Start capture
            </button>
          ) : (
            <button className="px-4 py-2 rounded-lg bg-red-600 text-white font-medium" onClick={stopCapture}>
              Stop capture
            </button>
          )}
        </div>
      </header>

      {workloadGroups.map((g) => (
        <section key={g.namespace} className="bg-white rounded-2xl border border-slate-200 overflow-hidden shadow-sm">
          <div className="px-4 py-2 bg-slate-50 text-sm font-mono text-slate-700 border-b border-slate-100">
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

          <div className="px-4 py-2 text-xs text-slate-500 border-t border-slate-100">Pods</div>
          <table className="w-full text-sm">
            <thead className="bg-slate-50 text-slate-500">
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
                <tr key={p.uid} className="border-t border-slate-100">
                  <td className="p-3">
                    <input type="checkbox" checked={!!selectedPods[p.uid]} onChange={() => togglePod(p.uid)} />
                  </td>
                  <td className="p-3 font-mono">{p.name}</td>
                  <td className="p-3 text-slate-600">{ownerLabel(p)}</td>
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
        <p className="text-xs text-slate-600 bg-blue-50 border border-blue-100 rounded-lg px-3 py-2">
          Watching {capturePods.length} pod(s):{" "}
          {capturePods.map((p) => `${p.namespace}/${p.name}`).join(", ")}
          {capturing ? " · restarts update sensor filters automatically" : ""}
        </p>
      )}

      {captureError && (
        <p className="text-sm text-red-800 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
          Capture error: {captureError}
        </p>
      )}

      {(capturing || sessionId) && (
        <section className="space-y-3">
          <h2 className="text-sm font-semibold text-slate-700 uppercase tracking-wide">Live capture</h2>
          {Object.entries(metrics).map(([pod, m]) => (
            <div key={pod} className="bg-white rounded-xl border border-slate-200 p-4 shadow-sm">
              <div className="flex justify-between text-sm mb-2">
                <span className="font-mono text-slate-800">{pod}</span>
                <span className="text-slate-500">
                  {m.packetsPerSec} pkt/s · {m.cumulativeBytes} bytes
                </span>
              </div>
              <pre className="font-mono text-xs bg-slate-50 rounded-lg p-3 max-h-32 overflow-auto text-emerald-700 border border-slate-100">
                {m.lines.join("\n") || "Awaiting sensor packets…"}
              </pre>
            </div>
          ))}
        </section>
      )}

      {sessionId && (
        <>
          {!capturing && (
          <div className="flex flex-wrap gap-2 items-center">
            {selectedPodList.map((p) => (
              <button
                key={p.uid}
                className="px-3 py-1.5 rounded-lg border border-slate-300 text-sm text-slate-700 hover:bg-white"
                onClick={() => downloadPod(p.uid)}
              >
                PCAP {p.name}
              </button>
            ))}
            <button
              className="px-3 py-1.5 rounded-lg border border-slate-300 text-sm"
              onClick={mergePcap}
            >
              Merge PCAP
            </button>
          </div>
          )}
          <ObservabilityWorkbench
            topology={topology}
            captureSummary={captureSummary}
            trackedPodIds={trackedPodIds.length ? trackedPodIds : selectionPodIds}
            loading={flowsLoading}
            onRefresh={() => loadFlowTopology()}
            onOpenAnalyst={() => setShowAI(true)}
            sessionLabel={sessionId}
          />
        </>
      )}

      {showAI && sessionId && session && (
        <AIDiagnosticModal
          open={showAI}
          sessionId={sessionId}
          authSessionId={session.session_id}
          onClose={() => setShowAI(false)}
        />
      )}
    </main>
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
    <table className="w-full text-sm border-t border-slate-100">
      <thead className="bg-slate-50 text-slate-500">
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
            <tr key={key} className="border-t border-slate-100">
              <td className="p-2">
                <input type="checkbox" checked={!!selectedOwners[key]} onChange={() => onToggle(c)} />
              </td>
              <td className="p-2 font-mono">
                {c.kind}/{c.name}
              </td>
              <td className="p-2">
                <StatusBadge status={c.status} />
              </td>
              <td className="p-2 text-slate-500">{c.ready || "—"}</td>
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
      ? "text-emerald-700 bg-emerald-50 px-2 py-0.5 rounded"
      : status === "Failed"
        ? "text-red-700 bg-red-50 px-2 py-0.5 rounded"
        : "text-amber-700 bg-amber-50 px-2 py-0.5 rounded";
  return <span className={`text-xs font-medium ${color}`}>{status}</span>;
}
