"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { EdgePaintState, ProbeAttachInterface, ProbeEvent } from "@/lib/trace";
import { fetchProbeInterfaces, fireTraceProbe, streamTraceProbeEvents } from "@/lib/trace";

type Props = {
  authSessionId: string;
  traceId: string;
  onEdgeStates: (states: Record<string, EdgePaintState>) => void;
  onProbingChange?: (active: boolean) => void;
};

export function TraceProbePanel({ authSessionId, traceId, onEdgeStates, onProbingChange }: Props) {
  const [interfaces, setInterfaces] = useState<ProbeAttachInterface[]>([]);
  const [iface, setIface] = useState("default");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [paintToken, setPaintToken] = useState<string | null>(null);
  const [simulate, setSimulate] = useState(true);
  const edgeStatesRef = useRef<Record<string, EdgePaintState>>({});

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchProbeInterfaces(authSessionId, traceId)
      .then((list) => {
        if (cancelled) return;
        setInterfaces(list.length ? list : [{ name: "default", primary: true }]);
        setIface(list[0]?.name ?? "default");
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

  const applyEvent = useCallback(
    (ev: ProbeEvent) => {
      if (ev.type === "probe_started") {
        edgeStatesRef.current = {};
        onEdgeStates({});
        setStatus(ev.message ?? "Probe started");
        onProbingChange?.(true);
        return;
      }
      if (ev.type === "edge_update" && ev.edge_id && ev.state) {
        edgeStatesRef.current = { ...edgeStatesRef.current, [ev.edge_id]: ev.state };
        onEdgeStates({ ...edgeStatesRef.current });
        setStatus(`Hop ${ev.seq ?? ""}: ${ev.hook ?? ev.edge_id}`.trim());
        return;
      }
      if (ev.type === "snapshot" && ev.edge_states) {
        edgeStatesRef.current = { ...ev.edge_states };
        onEdgeStates({ ...edgeStatesRef.current });
        return;
      }
      if (ev.type === "probe_finished") {
        setStatus(ev.message ?? "Probe complete");
        onProbingChange?.(false);
        return;
      }
      if (ev.type === "error") {
        setError(ev.message ?? "Probe error");
        onProbingChange?.(false);
      }
    },
    [onEdgeStates, onProbingChange]
  );

  const fireProbe = async () => {
    setBusy(true);
    setError(null);
    setStatus(null);
    onProbingChange?.(true);
    const abort = new AbortController();
    try {
      const resp = await fireTraceProbe(authSessionId, traceId, iface, simulate);
      setPaintToken(resp.paint_token);
      setStatus(`Firing ${resp.mode} probe · ${resp.primary_edges} hops · ${resp.paint_token}`);
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

  return (
    <div className="rounded-siem border border-siem-border bg-siem-panel/40 p-4 space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-semibold text-siem-text">Probe paint</h3>
          <p className="text-xs text-siem-muted mt-1 max-w-xl">
            Fire a marked probe from a pod interface and paint primary hops green as observations correlate.
            Simulate mode walks the predicted path without cluster exec.
          </p>
        </div>
        {paintToken && (
          <span className="text-[10px] font-mono px-2 py-1 rounded-md border border-siem-border text-siem-muted">
            {paintToken}
          </span>
        )}
      </div>

      <div className="flex flex-wrap items-end gap-3">
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
          <input
            type="checkbox"
            checked={simulate}
            disabled={busy}
            onChange={(e) => setSimulate(e.target.checked)}
          />
          Simulate paint
        </label>

        <button
          type="button"
          className="siem-btn-primary"
          disabled={loading || busy}
          onClick={() => fireProbe().catch(() => undefined)}
        >
          {busy ? "Painting path…" : "Fire probe"}
        </button>
      </div>

      {status && <p className="text-xs text-siem-ok">{status}</p>}
      {error && <p className="text-xs text-siem-err">{error}</p>}
    </div>
  );
}
