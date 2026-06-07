"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { EdgePaintState, ProbeAttachInterface, ProbeEvent } from "@/lib/trace";
import {
  fetchProbeInterfaces,
  fetchTraceStatus,
  fireTraceProbe,
  startTraceCapture,
  streamTraceProbeEvents,
} from "@/lib/trace";

type Props = {
  authSessionId: string;
  traceId: string;
  onEdgeStates: (states: Record<string, EdgePaintState>) => void;
  onProbingChange?: (active: boolean) => void;
  onVerifiedChange?: (verified: number, total: number) => void;
};

export function TraceProbePanel({
  authSessionId,
  traceId,
  onEdgeStates,
  onProbingChange,
  onVerifiedChange,
}: Props) {
  const [interfaces, setInterfaces] = useState<ProbeAttachInterface[]>([]);
  const [iface, setIface] = useState("default");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [paintToken, setPaintToken] = useState<string | null>(null);
  const [mode, setMode] = useState<string>("simulate");
  const [simulate, setSimulate] = useState(true);
  const [demoDrop, setDemoDrop] = useState(false);
  const [captureActive, setCaptureActive] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [verified, setVerified] = useState(0);
  const [total, setTotal] = useState(0);
  const [dropReason, setDropReason] = useState<string | null>(null);
  const edgeStatesRef = useRef<Record<string, EdgePaintState>>({});

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    Promise.all([
      fetchProbeInterfaces(authSessionId, traceId),
      fetchTraceStatus(authSessionId, traceId).catch(() => ({ capture_active: false })),
    ])
      .then(([list, st]) => {
        if (cancelled) return;
        setInterfaces(list.length ? list : [{ name: "default", primary: true }]);
        setIface(list[0]?.name ?? "default");
        setCaptureActive(!!st.capture_active);
      })
      .catch((e) => {
        if (!cancelled) {
          setInterfaces([{ name: "default", primary: true }]);
          setError(e instanceof Error ? e.message : String(e));
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [authSessionId, traceId]);

  const syncProgress = useCallback(
    (v: number, t: number) => {
      setVerified(v);
      setTotal(t);
      onVerifiedChange?.(v, t);
    },
    [onVerifiedChange]
  );

  const applyEvent = useCallback(
    (ev: ProbeEvent) => {
      if (ev.type === "probe_started") {
        edgeStatesRef.current = {};
        onEdgeStates({});
        setDropReason(null);
        setStatus(ev.message ?? "Verification started");
        onProbingChange?.(true);
        if (ev.total != null) syncProgress(ev.verified ?? 0, ev.total);
        return;
      }
      if (ev.type === "edge_update" && ev.edge_id && ev.state) {
        edgeStatesRef.current = { ...edgeStatesRef.current, [ev.edge_id]: ev.state };
        onEdgeStates({ ...edgeStatesRef.current });
        if (ev.drop_reason) setDropReason(ev.drop_reason);
        if (ev.total != null) syncProgress(ev.verified ?? 0, ev.total);
        setStatus(
          ev.state === "DROPPED_RED"
            ? `Blocked at hop ${ev.seq ?? ""}: ${ev.drop_reason ?? ev.edge_id}`
            : `Verified hop ${ev.seq ?? ""} · ${ev.hook ?? ev.edge_id}`
        );
        return;
      }
      if (ev.type === "snapshot" && ev.edge_states) {
        edgeStatesRef.current = { ...ev.edge_states };
        onEdgeStates({ ...edgeStatesRef.current });
        return;
      }
      if (ev.type === "probe_finished") {
        setStatus(ev.message ?? "Path verified");
        if (ev.total != null) syncProgress(ev.verified ?? verified, ev.total);
        onProbingChange?.(false);
        return;
      }
      if (ev.type === "error") {
        setError(ev.message ?? "Verification error");
        onProbingChange?.(false);
      }
    },
    [onEdgeStates, onProbingChange, syncProgress, verified]
  );

  const verifyPath = async () => {
    setBusy(true);
    setError(null);
    setStatus(null);
    setDropReason(null);
    onProbingChange?.(true);
    const abort = new AbortController();
    try {
      if (!simulate && !captureActive) {
        setStatus("Starting trace capture for live correlation…");
        const cap = await startTraceCapture(authSessionId, traceId);
        setCaptureActive(cap.capture_active);
      }
      const resp = await fireTraceProbe(authSessionId, traceId, {
        interface: iface,
        simulate,
        demoDrop,
      });
      setPaintToken(resp.paint_token);
      setMode(resp.mode);
      setTotal(resp.primary_edges);
      syncProgress(0, resp.primary_edges);
      setStatus(
        resp.capture_linked
          ? `Correlating ${resp.paint_token} with live capture (${resp.primary_edges} hops)`
          : `Walking ${resp.primary_edges} predicted hops · ${resp.paint_token} (${resp.mode})`
      );
      await streamTraceProbeEvents(authSessionId, traceId, applyEvent, abort.signal);
    } catch (e) {
      if ((e as Error).name !== "AbortError") {
        setError(e instanceof Error ? e.message : String(e));
      }
      onProbingChange?.(false);
    } finally {
      setBusy(false);
    }
  };

  const progressPct = total > 0 ? Math.min(100, Math.round((verified / total) * 100)) : 0;

  return (
    <div className="rounded-siem border border-siem-accent/40 bg-siem-panel/50 p-5 space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-1 min-w-[240px] flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-semibold text-siem-text">Verify path</h3>
            <span className="text-[10px] px-2 py-0.5 rounded-md border border-siem-border text-siem-muted uppercase tracking-wide">
              {busy ? "running" : mode}
            </span>
            {captureActive && !simulate && (
              <span className="text-[10px] px-2 py-0.5 rounded-md text-siem-ok border border-siem-ok/30">
                capture linked
              </span>
            )}
          </div>
          <p className="text-xs text-siem-muted max-w-2xl">
            Mark a test packet from the source pod, correlate observations onto primary hops, and paint the path map
            green — or red when policy blocks egress.
          </p>
        </div>
        <div className="flex flex-col items-end gap-2 shrink-0">
          {paintToken && (
            <span className="text-[10px] font-mono px-2 py-1 rounded-md border border-siem-border bg-siem-panel text-siem-muted">
              {paintToken}
            </span>
          )}
          <button
            type="button"
            className="siem-btn-primary text-sm px-6 py-2.5 min-w-[160px]"
            disabled={loading || busy}
            onClick={() => verifyPath().catch(() => undefined)}
          >
            {busy ? "Verifying…" : "Verify path"}
          </button>
        </div>
      </div>

      {(busy || verified > 0) && total > 0 && (
        <div className="space-y-1">
          <div className="flex justify-between text-[10px] text-siem-muted uppercase tracking-wide">
            <span>Verified hops</span>
            <span>
              {verified}/{total}
            </span>
          </div>
          <div className="h-1.5 rounded-full bg-siem-border overflow-hidden">
            <div
              className={`h-full transition-all duration-500 ${dropReason ? "bg-siem-err" : "bg-siem-ok"}`}
              style={{ width: `${progressPct}%` }}
            />
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center gap-4 text-xs">
        <label className="flex items-center gap-2 text-siem-muted">
          <input type="checkbox" checked={demoDrop} disabled={busy} onChange={(e) => setDemoDrop(e.target.checked)} />
          Demo policy block (red final hop)
        </label>
        <button
          type="button"
          className="text-siem-accent hover:underline"
          onClick={() => setShowAdvanced((v) => !v)}
        >
          {showAdvanced ? "Hide options" : "Interface & mode"}
        </button>
      </div>

      {showAdvanced && (
        <div className="flex flex-wrap items-end gap-3 pt-1 border-t border-siem-border/60">
          <label className="text-xs text-siem-muted flex flex-col gap-1">
            Egress interface
            <select
              className="siem-input text-sm min-w-[160px]"
              value={iface}
              disabled={loading || busy}
              onChange={(e) => setIface(e.target.value)}
            >
              {interfaces.map((i) => (
                <option key={i.name} value={i.name}>
                  {i.name}
                  {i.primary ? " (primary)" : i.cni ? ` (${i.cni})` : ""}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 text-xs text-siem-muted pb-2">
            <input type="checkbox" checked={simulate} disabled={busy} onChange={(e) => setSimulate(e.target.checked)} />
            Simulate (cinematic walk — no cluster exec)
          </label>
        </div>
      )}

      {status && !error && <p className="text-xs text-siem-ok">{status}</p>}
      {dropReason && <p className="text-xs text-siem-err">{dropReason}</p>}
      {error && <p className="text-xs text-siem-err">{error}</p>}
    </div>
  );
}
