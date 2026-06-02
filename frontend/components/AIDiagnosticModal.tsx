"use client";

import { useCallback, useEffect, useState } from "react";
import {
  AIProvider,
  AIContextResponse,
  AIVerifyResponse,
  FlowTopology,
  PROVIDER_META,
  fetchAIContext,
  sendAIChat,
  verifyAI,
} from "@/lib/ai";
import { emptyTopology, mergeTopology } from "@/lib/topology";
import { ObservabilityWorkbench } from "./ObservabilityWorkbench";

type ChatLine = { role: "user" | "assistant"; content: string };

type Props = {
  open: boolean;
  sessionId: string;
  authSessionId: string;
  onClose: () => void;
};

type Tab = "observe" | "chat" | "data" | "settings";

type Status = "unknown" | "ok" | "fail" | "checking";

function StatusPill({ label, status, detail }: { label: string; status: Status; detail?: string }) {
  const colors: Record<Status, string> = {
    unknown: "bg-siem-bg text-siem-muted border-siem-border",
    ok: "bg-siem-ok/15 text-siem-ok border-siem-ok/30",
    fail: "bg-siem-err/15 text-siem-err border-siem-err/30",
    checking: "bg-siem-accent/15 text-siem-accentHi border-siem-accent/30 animate-pulse",
  };
  return (
    <div className={`flex flex-col gap-0.5 px-3 py-2 rounded-lg border min-w-[120px] ${colors[status]}`}>
      <span className="text-[10px] uppercase tracking-wider font-medium">{label}</span>
      <span className="text-sm font-semibold capitalize">{status === "checking" ? "Checking…" : status}</span>
      {detail && (
        <span className="text-[10px] opacity-80 truncate max-w-[160px]" title={detail}>
          {detail}
        </span>
      )}
    </div>
  );
}

export function AIDiagnosticModal({ open, sessionId, authSessionId, onClose }: Props) {
  const [tab, setTab] = useState<Tab>("observe");
  const [ctx, setCtx] = useState<AIContextResponse | null>(null);
  const [topology, setTopology] = useState<FlowTopology | null>(null);
  const [trackedPodIds, setTrackedPodIds] = useState<string[]>([]);
  const [chat, setChat] = useState<ChatLine[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [provider, setProvider] = useState<AIProvider>("gemini");
  const [model, setModel] = useState(PROVIDER_META.gemini.defaultModel);
  const [endpoint, setEndpoint] = useState(PROVIDER_META.gemini.defaultEndpoint);
  const [apiKey, setApiKey] = useState("");
  const [proxy, setProxy] = useState("");

  const [authStatus, setAuthStatus] = useState<Status>("unknown");
  const [captureStatus, setCaptureStatus] = useState<Status>("unknown");
  const [llmStatus, setLlmStatus] = useState<Status>("unknown");
  const [verifyDetail, setVerifyDetail] = useState<AIVerifyResponse | null>(null);
  const [testLlmOnVerify, setTestLlmOnVerify] = useState(true);

  const onProviderChange = (p: AIProvider) => {
    setProvider(p);
    setModel(PROVIDER_META[p].defaultModel);
    setEndpoint(PROVIDER_META[p].defaultEndpoint);
    setLlmStatus("unknown");
  };

  const runVerify = useCallback(async () => {
    setError(null);
    setAuthStatus("checking");
    setCaptureStatus("checking");
    setLlmStatus(testLlmOnVerify ? "checking" : "unknown");
    try {
      const v = await verifyAI(authSessionId, sessionId, {
        provider,
        model,
        api_endpoint: endpoint,
        api_key: apiKey,
        proxy_url: proxy,
        test_llm: testLlmOnVerify && !!apiKey,
      });
      setVerifyDetail(v);
      setAuthStatus(v.auth_ok ? "ok" : "fail");
      setCaptureStatus(v.capture_ok ? "ok" : "fail");
      if (testLlmOnVerify && apiKey) {
        setLlmStatus(v.llm_ok ? "ok" : "fail");
      } else {
        setLlmStatus("unknown");
      }
      if (!v.capture_ok && v.capture_error) setError(v.capture_error);
      else if (testLlmOnVerify && !v.llm_ok && v.llm_error) setError(v.llm_error);
      else if (!v.auth_ok && v.auth_error) setError(v.auth_error);
      return v;
    } catch (e) {
      setAuthStatus("fail");
      setCaptureStatus("fail");
      setLlmStatus("fail");
      setError(e instanceof Error ? e.message : String(e));
      return null;
    }
  }, [authSessionId, sessionId, provider, model, endpoint, apiKey, proxy, testLlmOnVerify]);

  const loadContext = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const v = await runVerify();
      if (v && !v.capture_ok) return;
      const c = await fetchAIContext(authSessionId, sessionId, 400);
      setCtx(c);
      setTopology((prev) => mergeTopology(prev, c.topology));
      setTrackedPodIds(c.tracked_pod_ids ?? []);
      setCaptureStatus("ok");
    } catch (e) {
      setCaptureStatus("fail");
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [authSessionId, sessionId, runVerify]);

  useEffect(() => {
    if (open && sessionId) {
      setChat([]);
      setVerifyDetail(null);
      setTopology(emptyTopology());
      setAuthStatus("unknown");
      setCaptureStatus("unknown");
      setLlmStatus("unknown");
      loadContext();
    }
  }, [open, sessionId]); // eslint-disable-line react-hooks/exhaustive-deps

  const sendMessage = async () => {
    const userMsg = input.trim() || "Summarize capture: traffic patterns, drops, and remediation.";
    setInput("");
    setChat((c) => [...c, { role: "user", content: userMsg }]);
    setLoading(true);
    setError(null);
    try {
      const v = await runVerify();
      if (v && testLlmOnVerify && !v.ready) return;
      const res = await sendAIChat(authSessionId, sessionId, {
        message: userMsg,
        provider,
        model,
        api_endpoint: endpoint,
        api_key: apiKey,
        proxy_url: proxy,
      });
      if (res.reply) {
        setChat((c) => [...c, { role: "assistant", content: res.reply }]);
        setLlmStatus("ok");
      }
      if (res.topology) setTopology((prev) => mergeTopology(prev, res.topology));
      if (res.flow_graph) {
        setCtx((prev) => (prev ? { ...prev, flow_graph: res.flow_graph! } : prev));
      }
    } catch (e) {
      setLlmStatus("fail");
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  };

  const flush = async () => {
    await sendAIChat(authSessionId, sessionId, { provider, model, flush_session: true });
    setApiKey("");
    setChat([]);
    onClose();
  };

  if (!open) return null;

  const tabs: { id: Tab; label: string }[] = [
    { id: "observe", label: "Observability" },
    { id: "chat", label: "Analyst chat" },
    { id: "data", label: "Scrubbed data" },
    { id: "settings", label: "Connection" },
  ];

  return (
    <div className="fixed inset-0 z-50 flex flex-col bg-siem-bg text-siem-text">
      <header className="shrink-0 px-6 py-4 border-b border-siem-border bg-siem-panel">
        <div className="flex items-start justify-between gap-4 max-w-[1600px] mx-auto w-full">
          <div>
            <div className="flex items-center gap-3">
              <span className="inline-flex h-9 w-9 items-center justify-center rounded-md bg-siem-accent text-white text-sm font-bold">
                AI
              </span>
              <div>
                <h1 className="text-lg font-semibold tracking-tight">Network analyst — full view</h1>
                <p className="text-sm text-siem-muted">
                  Scrubbed JSONL only · credentials in session memory · capture {sessionId.slice(0, 12)}…
                </p>
              </div>
            </div>
            <div className="mt-4 flex flex-wrap gap-2">
              <StatusPill
                label="SPCG session"
                status={authStatus}
                detail={verifyDetail?.auth_ok ? "Authenticated" : verifyDetail?.auth_error}
              />
              <StatusPill
                label="Capture buffer"
                status={captureStatus}
                detail={
                  verifyDetail?.capture_ok
                    ? `${verifyDetail.capture_events ?? 0} events`
                    : undefined
                }
              />
              <StatusPill
                label="LLM"
                status={llmStatus}
                detail={testLlmOnVerify ? PROVIDER_META[provider].label : "Not tested"}
              />
              <button
                type="button"
                className="ml-auto siem-btn-ghost"
                onClick={() => runVerify().catch(() => undefined)}
              >
                Verify connection
              </button>
            </div>
          </div>
          <button
            type="button"
            className="shrink-0 siem-btn-ghost"
            onClick={onClose}
          >
            Close
          </button>
        </div>

        <nav className="flex gap-1 mt-4 max-w-[1600px] mx-auto w-full border-b border-siem-border -mb-px">
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              className={`px-4 py-2.5 text-sm font-medium border-b-2 -mb-px transition ${
                tab === t.id
                  ? "border-siem-accent text-siem-accentHi"
                  : "border-transparent text-siem-muted hover:text-siem-text"
              }`}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </nav>
      </header>

      <div className="flex-1 overflow-auto min-h-0 bg-siem-bg">
        <div className="max-w-[1600px] mx-auto w-full h-full p-4">
          {tab === "observe" && (
            <ObservabilityWorkbench
              topology={topology}
              captureSummary={ctx?.capture_summary ?? null}
              trackedPodIds={trackedPodIds}
              loading={loading}
              onRefresh={() => loadContext()}
              sessionLabel={sessionId}
            />
          )}

          {tab === "chat" && (
            <div className="flex flex-col h-[min(70vh,720px)] siem-card overflow-hidden">
              <div className="flex-1 overflow-auto p-6 space-y-4">
                {chat.length === 0 && (
                  <p className="text-sm text-siem-muted text-center py-12">
                    Configure connection, verify capture, then ask about drops, DNS, or pod paths.
                  </p>
                )}
                {chat.map((m, i) => (
                  <div
                    key={i}
                    className={`max-w-[85%] rounded-md px-4 py-3 text-sm ${
                      m.role === "user"
                        ? "ml-auto bg-siem-accent/15 border border-siem-accent/30"
                        : "bg-siem-panel border border-siem-border"
                    }`}
                  >
                    <div className="siem-label mb-1">{m.role}</div>
                    <div className="whitespace-pre-wrap text-siem-text">{m.content}</div>
                  </div>
                ))}
              </div>
              <div className="p-4 border-t border-siem-border flex gap-3 bg-siem-panel">
                <input
                  className="flex-1 siem-input"
                  placeholder="Ask about this capture…"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && !loading && apiKey && sendMessage()}
                />
                <button
                  type="button"
                  className="siem-btn-primary disabled:opacity-40"
                  disabled={loading || !apiKey || captureStatus === "fail"}
                  onClick={() => sendMessage()}
                >
                  Send
                </button>
              </div>
            </div>
          )}

          {tab === "data" && (
            <div className="space-y-4 siem-card p-6">
              <div className="grid grid-cols-3 gap-3">
                <MetricCard label="Events" value={String(ctx?.event_count ?? 0)} />
                <MetricCard label="JSONL lines" value={String(ctx?.jsonl_lines ?? 0)} />
                <MetricCard
                  label="Capture bytes"
                  value={
                    verifyDetail?.capture_bytes != null
                      ? formatBytes(verifyDetail.capture_bytes)
                      : "—"
                  }
                />
              </div>
              <pre className="text-xs font-mono border border-siem-border rounded-md p-4 overflow-auto max-h-[60vh] text-siem-muted bg-siem-bg">
                {ctx?.jsonl_preview || "Reload context to load scrubbed JSONL."}
              </pre>
            </div>
          )}

          {tab === "settings" && (
            <div className="grid md:grid-cols-2 gap-6 siem-card p-6">
              <div className="space-y-4">
                <h3 className="text-sm font-semibold text-siem-text">LLM provider</h3>
                <Field label="Provider">
                  <select
                    className="siem-input"
                    value={provider}
                    onChange={(e) => onProviderChange(e.target.value as AIProvider)}
                  >
                    {(Object.keys(PROVIDER_META) as AIProvider[]).map((p) => (
                      <option key={p} value={p}>
                        {PROVIDER_META[p].label}
                      </option>
                    ))}
                  </select>
                </Field>
                <Field label="Model">
                  <input
                    className="siem-input font-mono"
                    value={model}
                    onChange={(e) => setModel(e.target.value)}
                  />
                </Field>
                <Field label="API endpoint">
                  <input
                    className="siem-input text-xs font-mono"
                    value={endpoint}
                    onChange={(e) => setEndpoint(e.target.value)}
                  />
                </Field>
                <Field label="API key (session only)">
                  <input
                    type="password"
                    className="siem-input"
                    value={apiKey}
                    onChange={(e) => {
                      setApiKey(e.target.value);
                      setLlmStatus("unknown");
                    }}
                  />
                </Field>
                <Field label="HTTP proxy">
                  <input
                    className="siem-input"
                    placeholder="http://proxy:8080"
                    value={proxy}
                    onChange={(e) => setProxy(e.target.value)}
                  />
                </Field>
              </div>
              <div className="siem-panel p-4 space-y-3">
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={testLlmOnVerify}
                    onChange={(e) => setTestLlmOnVerify(e.target.checked)}
                  />
                  Test LLM on verify
                </label>
                <button
                  type="button"
                  className="w-full siem-btn-primary"
                  onClick={() => runVerify().catch(() => undefined)}
                >
                  Verify session & connection
                </button>
                <p className="text-xs text-siem-muted">
                  Cursor uses the Cloud Agents API (no-repo) with your key from Dashboard → Integrations.
                  Azure / Copilot: use OpenAI-compatible with your gateway URL.
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {error && (
        <div className="shrink-0 mx-6 mb-2 px-4 py-2 rounded-md bg-siem-err/10 border border-siem-err/30 text-sm text-siem-err max-w-[1600px] w-full self-center">
          {error}
        </div>
      )}

      <footer className="shrink-0 flex justify-between items-center px-6 py-3 border-t border-siem-border bg-siem-panel text-xs text-siem-muted">
        <span>Zero retention · wiped on flush or sign-out</span>
        <div className="flex gap-2">
          <button
            type="button"
            className="siem-btn-ghost"
            onClick={() => loadContext()}
            disabled={loading}
          >
            Reload context
          </button>
          <button
            type="button"
            className="siem-btn-ghost text-siem-err border-siem-err/40"
            onClick={() => flush()}
          >
            Flush & close
          </button>
        </div>
      </footer>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="siem-label">{label}</span>
      <div className="mt-1">{children}</div>
    </label>
  );
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="siem-card px-4 py-3">
      <div className="siem-label">{label}</div>
      <div className="text-lg font-semibold text-siem-text">{value}</div>
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
