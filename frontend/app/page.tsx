"use client";

import { useCallback, useMemo, useRef, useState } from "react";
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
  const [aiSummary, setAiSummary] = useState("");
  const [aiProxy, setAiProxy] = useState("");
  const [aiTarget, setAiTarget] = useState<"ollama" | "bedrock" | "gateway">("ollama");
  const [aiEndpoint, setAiEndpoint] = useState("http://127.0.0.1:11434/api/generate");
  const [aiBearer, setAiBearer] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  const login = useCallback(async () => {
    const auth =
      authMode === "kubeconfig"
        ? await loginWithKubeconfig(kubeconfigText)
        : await loginWithToken(token);
    setSession(auth);
    const ns = await fetchNamespaces(auth.session_id);
    setNamespaces(ns);
    setLoggedIn(true);
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
    if (!hasSelection) return;
    setCapturing(true);
    setMetrics({});
    abortRef.current?.abort();
    abortRef.current = new AbortController();

    if (!session) return;
    const res = await fetch("/api/v1/capture/stream", {
      method: "POST",
      headers: authHeaders(session.session_id),
      body: JSON.stringify({
        namespaces: selectedNamespaces,
        selections: captureSelections,
      }),
      signal: abortRef.current.signal,
    });

    const reader = res.body?.getReader();
    if (!reader) return;
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
        if (eventLine?.includes("session")) {
          try {
            const meta = JSON.parse(data);
            setSessionId(meta.session_id || data.trim());
          } catch {
            setSessionId(data.trim());
          }
        }
        if (eventLine?.includes("chunk")) {
          const ev = JSON.parse(data) as ChunkEvent;
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

  const downloadPod = (podUid: string) => {
    if (!sessionId) return;
    window.open(`/api/v1/capture/download/${sessionId}?pod_uid=${podUid}`, "_blank");
  };

  const mergePcap = () => {
    if (!sessionId) return;
    window.open(`/api/v1/capture/merge/${sessionId}`, "_blank");
  };

  const runAI = async () => {
    if (!sessionId) return;
    const res = await fetch("/api/v1/ai/triage", {
      method: "POST",
      headers: authHeaders(session!.session_id),
      body: JSON.stringify({
        session_id: sessionId,
        proxy_url: aiProxy,
        target_type: aiTarget,
        api_endpoint: aiEndpoint,
        bearer_token: aiBearer,
      }),
    });
    const json = await res.json();
    setAiSummary(json.summary || JSON.stringify(json));
  };

  const flushAI = async () => {
    if (!sessionId) return;
    await fetch("/api/v1/ai/triage", {
      method: "POST",
      headers: authHeaders(session!.session_id),
      body: JSON.stringify({ session_id: sessionId, flush_session: true }),
    });
    setAiBearer("");
    setShowAI(false);
  };

  if (!loggedIn) {
    return (
      <main className="min-h-screen flex items-center justify-center p-6">
        <div className="w-full max-w-lg bg-spcg-panel rounded-xl border border-slate-700 p-8 shadow-xl">
          <h1 className="text-2xl font-semibold mb-2">SPCG Login</h1>
          <p className="text-slate-400 text-sm mb-4">
            Works with vanilla Kubernetes and OpenShift. Credentials stay in memory only for this browser session.
          </p>
          <div className="flex gap-2 mb-4">
            <button
              className={`flex-1 py-2 rounded-lg text-sm ${authMode === "kubeconfig" ? "bg-spcg-accent" : "border border-slate-600"}`}
              onClick={() => setAuthMode("kubeconfig")}
            >
              Kubeconfig file
            </button>
            <button
              className={`flex-1 py-2 rounded-lg text-sm ${authMode === "token" ? "bg-spcg-accent" : "border border-slate-600"}`}
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
                className="w-full text-sm mb-2"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (!f) return;
                  const reader = new FileReader();
                  reader.onload = () => setKubeconfigText(String(reader.result || ""));
                  reader.readAsText(f);
                }}
              />
              <textarea
                className="w-full h-40 rounded-lg bg-spcg-bg border border-slate-600 p-3 font-mono text-xs"
                placeholder="Or paste kubeconfig YAML"
                value={kubeconfigText}
                onChange={(e) => setKubeconfigText(e.target.value)}
              />
            </>
          ) : (
            <textarea
              className="w-full h-28 rounded-lg bg-spcg-bg border border-slate-600 p-3 font-mono text-sm"
              placeholder="Bearer token (OpenShift OAuth or K8s service account token)"
              value={token}
              onChange={(e) => setToken(e.target.value.trim())}
            />
          )}
          <button
            className="mt-4 w-full py-2 rounded-lg bg-spcg-accent hover:opacity-90 font-medium"
            onClick={() => login().catch((e) => alert(String(e)))}
          >
            Authenticate
          </button>
        </div>
      </main>
    );
  }

  if (!workspaceReady) {
    return (
      <main className="p-8 max-w-4xl mx-auto">
        <h1 className="text-xl font-semibold mb-2">Select namespace(s)</h1>
        <p className="text-slate-400 text-sm mb-6">Choose one or more namespaces you can access.</p>
        <div className="grid gap-2 mb-6">
          {namespaces.map((n) => (
            <label
              key={n.name}
              className="flex items-center gap-3 px-4 py-3 rounded-lg bg-spcg-panel border border-slate-700 cursor-pointer"
            >
              <input type="checkbox" checked={!!selectedNs[n.name]} onChange={() => toggleNs(n.name)} />
              <span className="font-mono">{n.name}</span>
              <span className="text-slate-400 text-sm ml-auto">{n.status}</span>
            </label>
          ))}
        </div>
        <button
          className="px-4 py-2 rounded-lg bg-spcg-accent disabled:opacity-40"
          disabled={selectedNamespaces.length === 0}
          onClick={() => enterWorkspace().catch((e) => alert(String(e)))}
        >
          Open workspace ({selectedNamespaces.length} selected)
        </button>
      </main>
    );
  }

  return (
    <main className="p-6 max-w-6xl mx-auto space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold">Workspace</h1>
          <p className="text-slate-400 text-sm font-mono">
            {session?.cluster ? `${session.cluster} · ` : ""}
            {selectedNamespaces.join(", ")}
          </p>
          <button className="text-xs text-slate-500 underline" onClick={() => handleLogout()}>
            Sign out (wipe credentials)
          </button>
        </div>
        <div className="flex gap-2">
          {!capturing ? (
            <button className="px-4 py-2 rounded-lg bg-spcg-accent" onClick={startCapture} disabled={!hasSelection}>
              Initiate Stream Capture
            </button>
          ) : (
            <button className="px-4 py-2 rounded-lg bg-spcg-err" onClick={stopCapture}>
              End Capture
            </button>
          )}
        </div>
      </header>

      {workloadGroups.map((g) => (
        <section key={g.namespace} className="bg-spcg-panel rounded-xl border border-slate-700 overflow-hidden">
          <div className="px-4 py-2 bg-spcg-bg text-sm font-mono text-slate-300">Namespace: {g.namespace}</div>

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

          <div className="px-4 py-2 text-xs text-slate-400 border-t border-slate-800">Pods (optional per-pod override)</div>
          <table className="w-full text-sm">
            <thead className="bg-spcg-bg/80 text-slate-400">
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
                <tr key={p.uid} className="border-t border-slate-800">
                  <td className="p-3">
                    <input type="checkbox" checked={!!selectedPods[p.uid]} onChange={() => togglePod(p.uid)} />
                  </td>
                  <td className="p-3 font-mono">{p.name}</td>
                  <td className="p-3 text-slate-300">{ownerLabel(p)}</td>
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

      {(capturing || sessionId) && (
        <section className="space-y-4">
          <h2 className="text-lg font-medium">Live capture (netobserv eBPF)</h2>
          {Object.entries(metrics).map(([pod, m]) => (
            <div key={pod} className="bg-spcg-panel rounded-xl border border-slate-700 p-4">
              <div className="flex justify-between text-sm mb-2">
                <span className="font-mono">{pod}</span>
                <span>
                  {m.packetsPerSec} B/s window · {m.cumulativeBytes} bytes
                </span>
              </div>
              <pre className="font-mono text-xs bg-spcg-bg rounded-lg p-3 max-h-40 overflow-auto text-emerald-400">
                {m.lines.join("\n") || "awaiting packets from sensors…"}
              </pre>
            </div>
          ))}
        </section>
      )}

      {sessionId && !capturing && (
        <section className="bg-spcg-panel rounded-xl border border-slate-700 p-6 space-y-4">
          <h2 className="text-lg font-medium">Post-capture workbench</h2>
          <p className="text-sm text-slate-400">Session: {sessionId}</p>
          <div className="flex flex-wrap gap-3">
            {selectedPodList.map((p) => (
              <button key={p.uid} className="px-3 py-2 rounded-lg border border-slate-600 text-sm" onClick={() => downloadPod(p.uid)}>
                Download {p.namespace}/{p.name}
              </button>
            ))}
            <button className="px-3 py-2 rounded-lg bg-spcg-accent text-sm" onClick={mergePcap}>
              Merge PCAP
            </button>
            <button className="px-3 py-2 rounded-lg border border-spcg-accent text-sm" onClick={() => setShowAI(true)}>
              AI Diagnostic Triage
            </button>
          </div>
        </section>
      )}

      {showAI && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center p-4 z-50">
          <div className="bg-spcg-panel max-w-lg w-full rounded-xl border border-slate-600 p-6 space-y-4">
            <h3 className="text-lg font-semibold">AI Triage (zero-retention)</h3>
            <input className="w-full rounded bg-spcg-bg border border-slate-600 p-2 text-sm" placeholder="Proxy URL" value={aiProxy} onChange={(e) => setAiProxy(e.target.value)} />
            <select className="w-full rounded bg-spcg-bg border border-slate-600 p-2 text-sm" value={aiTarget} onChange={(e) => setAiTarget(e.target.value as typeof aiTarget)}>
              <option value="ollama">Local Ollama</option>
              <option value="bedrock">Private Bedrock</option>
              <option value="gateway">Dedicated API Gateway</option>
            </select>
            <input className="w-full rounded bg-spcg-bg border border-slate-600 p-2 text-sm" placeholder="API endpoint" value={aiEndpoint} onChange={(e) => setAiEndpoint(e.target.value)} />
            <input className="w-full rounded bg-spcg-bg border border-slate-600 p-2 text-sm" placeholder="Bearer token (session only)" type="password" value={aiBearer} onChange={(e) => setAiBearer(e.target.value)} />
            <div className="flex gap-2">
              <button className="flex-1 py-2 rounded-lg bg-spcg-accent" onClick={() => runAI().catch((e) => alert(String(e)))}>
                Run triage
              </button>
              <button className="px-4 py-2 rounded-lg border border-slate-600" onClick={() => flushAI()}>
                Flush creds
              </button>
            </div>
            {aiSummary && <pre className="text-xs bg-spcg-bg p-3 rounded-lg max-h-48 overflow-auto whitespace-pre-wrap">{aiSummary}</pre>}
            <button className="text-sm text-slate-400" onClick={() => setShowAI(false)}>
              Close
            </button>
          </div>
        </div>
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
    <table className="w-full text-sm border-t border-slate-800">
      <thead className="bg-spcg-bg/60 text-slate-400">
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
            <tr key={key} className="border-t border-slate-800/80">
              <td className="p-2">
                <input type="checkbox" checked={!!selectedOwners[key]} onChange={() => onToggle(c)} />
              </td>
              <td className="p-2 font-mono">
                {c.kind}/{c.name}
              </td>
              <td className="p-2">
                <StatusBadge status={c.status} />
              </td>
              <td className="p-2 text-slate-400">{c.ready || "—"}</td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function StatusBadge({ status }: { status: string }) {
  const color = status === "Running" ? "text-spcg-ok" : status === "Failed" ? "text-spcg-err" : "text-spcg-warn";
  return <span className={color}>{status}</span>;
}
