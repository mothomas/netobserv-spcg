"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import {
  AIProvider,
  AIContextResponse,
  AIVerifyResponse,
  PROVIDER_META,
  fetchAIContext,
  sendAIChat,
  verifyAI,
} from "@/lib/ai";

type ChatLine = { role: "user" | "assistant" | "system"; content: string };

type Props = {
  open: boolean;
  sessionId: string;
  authSessionId: string;
  onClose: () => void;
};

type Panel = "chat" | "context" | "settings";

type Status = "unknown" | "ok" | "fail" | "checking";

export function AIDiagnosticModal({ open, sessionId, authSessionId, onClose }: Props) {
  const [panel, setPanel] = useState<Panel>("chat");
  const [ctx, setCtx] = useState<AIContextResponse | null>(null);
  const [chat, setChat] = useState<ChatLine[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);

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
      setCaptureStatus("ok");
      setChat([
        {
          role: "system",
          content:
            "Capture context loaded. Packet JSONL and graph summary are scrubbed before any public LLM call; replies are restored with real IPs in this chat only.",
        },
      ]);
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
      setAuthStatus("unknown");
      setCaptureStatus("unknown");
      setLlmStatus("unknown");
      setPanel("chat");
      loadContext();
    }
  }, [open, sessionId]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [chat, loading]);

  const sendMessage = async () => {
    const userMsg = input.trim();
    if (!userMsg) return;
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

  return (
    <div className="ai-chat-shell fixed inset-0 z-50 flex bg-siem-bg text-siem-text">
      <aside className="ai-chat-rail w-72 shrink-0 border-r border-siem-border bg-siem-panel flex flex-col">
        <div className="px-4 py-4 border-b border-siem-border">
          <div className="flex items-center gap-2">
            <span className="inline-flex h-9 w-9 items-center justify-center rounded-full bg-siem-accent/20 text-siem-accentHi text-sm font-bold">
              AI
            </span>
            <div>
              <h1 className="text-sm font-semibold">Network analyst</h1>
              <p className="text-[10px] text-siem-muted font-mono">{sessionId.slice(0, 14)}…</p>
            </div>
          </div>
        </div>

        <nav className="p-2 space-y-1">
          <RailButton active={panel === "chat"} onClick={() => setPanel("chat")}>
            Chat
          </RailButton>
          <RailButton active={panel === "context"} onClick={() => setPanel("context")}>
            Scrubbed context
          </RailButton>
          <RailButton active={panel === "settings"} onClick={() => setPanel("settings")}>
            Connection
          </RailButton>
        </nav>

        <div className="px-4 py-3 space-y-2 text-xs border-t border-siem-border mt-auto">
          <StatusRow label="Session" status={authStatus} />
          <StatusRow label="Capture" status={captureStatus} detail={verifyDetail?.capture_events ? `${verifyDetail.capture_events} events` : undefined} />
          <StatusRow label="LLM" status={llmStatus} />
          <p className="text-[10px] text-siem-muted pt-1">
            Public LLMs receive scrubbed tokens only. This chat shows restored IPs in assistant replies.
          </p>
        </div>
      </aside>

      <div className="flex-1 flex flex-col min-w-0">
        <header className="shrink-0 px-5 py-3 border-b border-siem-border flex items-center justify-between gap-3 bg-siem-panel/60">
          <div>
            <h2 className="text-sm font-semibold">
              {panel === "chat" ? "Analyst chat" : panel === "context" ? "Scrubbed capture context" : "LLM connection"}
            </h2>
            <p className="text-xs text-siem-muted">
              Independent from flow graph · Neo4j topology included in scrubbed graph summary
            </p>
          </div>
          <div className="flex gap-2">
            <button type="button" className="siem-btn-ghost text-xs" onClick={() => runVerify().catch(() => undefined)}>
              Verify
            </button>
            <button type="button" className="siem-btn-ghost text-xs" onClick={onClose}>
              Close
            </button>
          </div>
        </header>

        <div className="flex-1 min-h-0 overflow-hidden">
          {panel === "chat" && (
            <div className="h-full flex flex-col">
              <div className="flex-1 overflow-auto px-5 py-4 space-y-3">
                {chat.length === 0 && !loading && (
                  <p className="text-sm text-siem-muted text-center py-16 max-w-md mx-auto">
                    Ask about drops, DNS failures, pod paths, or latency. Your message is scrubbed upstream; the assistant reply is restored here.
                  </p>
                )}
                {chat.map((m, i) => (
                  <ChatBubble key={i} role={m.role} content={m.content} />
                ))}
                {loading && (
                  <div className="flex gap-2 items-center text-xs text-siem-muted px-2">
                    <span className="ai-chat-typing" />
                    Analyst thinking…
                  </div>
                )}
                <div ref={chatEndRef} />
              </div>
              <div className="shrink-0 border-t border-siem-border p-4 bg-siem-panel/50">
                <div className="flex gap-2 max-w-4xl mx-auto">
                  <textarea
                    className="flex-1 siem-input min-h-[44px] max-h-32 resize-y text-sm"
                    placeholder="Ask about this capture…"
                    rows={1}
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && !e.shiftKey && !loading && apiKey) {
                        e.preventDefault();
                        sendMessage();
                      }
                    }}
                  />
                  <button
                    type="button"
                    className="siem-btn-primary shrink-0 disabled:opacity-40 px-5"
                    disabled={loading || !apiKey || captureStatus === "fail" || !input.trim()}
                    onClick={() => sendMessage()}
                  >
                    Send
                  </button>
                </div>
                {!apiKey && <p className="text-[10px] text-amber-400 mt-2 max-w-4xl mx-auto">Add an API key under Connection to chat.</p>}
              </div>
            </div>
          )}

          {panel === "context" && (
            <div className="h-full overflow-auto p-5 space-y-4 max-w-4xl">
              <div className="grid grid-cols-3 gap-3">
                <MetricCard label="Events" value={String(ctx?.event_count ?? 0)} />
                <MetricCard label="JSONL lines" value={String(ctx?.jsonl_lines ?? 0)} />
                <MetricCard
                  label="Capture bytes"
                  value={verifyDetail?.capture_bytes != null ? formatBytes(verifyDetail.capture_bytes) : "—"}
                />
              </div>
              {ctx?.graph_context && (
                <div>
                  <h3 className="siem-label mb-2">Graph summary (scrubbed, sent to LLM)</h3>
                  <pre className="text-xs font-mono border border-siem-border rounded-md p-3 overflow-auto max-h-48 text-siem-muted bg-siem-bg whitespace-pre-wrap">
                    {ctx.graph_context}
                  </pre>
                </div>
              )}
              <div>
                <h3 className="siem-label mb-2">JSONL preview (scrubbed)</h3>
                <pre className="text-xs font-mono border border-siem-border rounded-md p-4 overflow-auto max-h-[50vh] text-siem-muted bg-siem-bg">
                  {ctx?.jsonl_preview || "Reload to load scrubbed JSONL."}
                </pre>
              </div>
            </div>
          )}

          {panel === "settings" && (
            <div className="h-full overflow-auto p-5 max-w-2xl space-y-4">
              <Field label="Provider">
                <select className="siem-input" value={provider} onChange={(e) => onProviderChange(e.target.value as AIProvider)}>
                  {(Object.keys(PROVIDER_META) as AIProvider[]).map((p) => (
                    <option key={p} value={p}>
                      {PROVIDER_META[p].label}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Model">
                <input className="siem-input font-mono" value={model} onChange={(e) => setModel(e.target.value)} />
              </Field>
              <Field label="API endpoint">
                <input className="siem-input text-xs font-mono" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
              </Field>
              <Field label="API key (session memory only)">
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
                <input className="siem-input" placeholder="http://proxy:8080" value={proxy} onChange={(e) => setProxy(e.target.value)} />
              </Field>
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={testLlmOnVerify} onChange={(e) => setTestLlmOnVerify(e.target.checked)} />
                Test LLM on verify
              </label>
              <button type="button" className="siem-btn-primary" onClick={() => runVerify().catch(() => undefined)}>
                Verify session & connection
              </button>
            </div>
          )}
        </div>

        {error && (
          <div className="shrink-0 mx-5 mb-3 px-4 py-2 rounded-md bg-siem-err/10 border border-siem-err/30 text-sm text-siem-err">
            {error}
          </div>
        )}

        <footer className="shrink-0 flex justify-between items-center px-5 py-2 border-t border-siem-border text-xs text-siem-muted bg-siem-panel/40">
          <span>Scrubbed upstream · restored in chat · wiped on flush</span>
          <div className="flex gap-2">
            <button type="button" className="siem-btn-ghost text-xs" onClick={() => loadContext()} disabled={loading}>
              Reload context
            </button>
            <button type="button" className="siem-btn-ghost text-xs text-siem-err border-siem-err/40" onClick={() => flush()}>
              Flush & close
            </button>
          </div>
        </footer>
      </div>
    </div>
  );
}

function RailButton({ active, onClick, children }: { active?: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`w-full text-left px-3 py-2 rounded-siem text-sm transition ${
        active ? "fluent-nav-active" : "fluent-nav-idle hover:bg-siem-bg/50"
      }`}
    >
      {children}
    </button>
  );
}

function ChatBubble({ role, content }: { role: ChatLine["role"]; content: string }) {
  if (role === "system") {
    return (
      <div className="mx-auto max-w-lg text-center text-xs text-siem-muted border border-siem-border rounded-full px-4 py-2 bg-siem-panel/50">
        {content}
      </div>
    );
  }
  const isUser = role === "user";
  return (
    <div className={`flex ${isUser ? "justify-end" : "justify-start"}`}>
      <div
        className={`max-w-[min(720px,85%)] rounded-2xl px-4 py-3 text-sm ${
          isUser
            ? "bg-siem-accent/15 border border-siem-accent/25 rounded-br-md"
            : "bg-siem-card border border-siem-border rounded-bl-md"
        }`}
      >
        <div className="text-[10px] uppercase tracking-wide text-siem-muted mb-1">{isUser ? "You" : "Analyst"}</div>
        <div className="whitespace-pre-wrap text-siem-text leading-relaxed">{content}</div>
      </div>
    </div>
  );
}

function StatusRow({ label, status, detail }: { label: string; status: Status; detail?: string }) {
  const tone =
    status === "ok"
      ? "text-siem-ok"
      : status === "fail"
        ? "text-siem-err"
        : status === "checking"
          ? "text-siem-accentHi"
          : "text-siem-muted";
  return (
    <div className="flex justify-between gap-2">
      <span className="text-siem-muted">{label}</span>
      <span className={`font-medium capitalize ${tone}`}>
        {status === "checking" ? "…" : status}
        {detail ? ` · ${detail}` : ""}
      </span>
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
