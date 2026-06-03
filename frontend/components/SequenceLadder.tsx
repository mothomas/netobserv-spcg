"use client";

import type { SequenceStep } from "@/lib/ai";

type Props = {
  steps: SequenceStep[];
  srcLabel: string;
  dstLabel: string;
  proto?: string;
  ports?: string;
};

const PHASE_TONE: Record<string, string> = {
  start: "text-cyan-300 border-cyan-400/40 bg-cyan-500/10",
  reply: "text-emerald-300 border-emerald-400/40 bg-emerald-500/10",
  data: "text-siem-text border-siem-border bg-siem-card",
  close: "text-rose-300 border-rose-400/40 bg-rose-500/10",
};

export function SequenceLadder({ steps, srcLabel, dstLabel, proto, ports }: Props) {
  if (!steps.length) {
    return (
      <p className="text-sm text-siem-muted py-6 text-center px-4">
        No packet sequence captured for this link yet. Keep capture running or select a busier edge.
      </p>
    );
  }

  return (
    <div className="flow-seq-panel">
      <div className="flow-seq-participants">
        <Participant title="Source" label={srcLabel} align="left" />
        <div className="flow-seq-link-meta">
          {proto && <span className="flow-seq-proto">{proto}</span>}
          {ports && ports !== "—" && <span className="flow-seq-ports">{ports}</span>}
          <span className="flow-seq-hint">{steps.length} message{steps.length === 1 ? "" : "s"}</span>
        </div>
        <Participant title="Destination" label={dstLabel} align="right" />
      </div>

      <ol className="flow-seq-list" aria-label="Flow message sequence">
        {steps.map((step, idx) => {
          const forward = (step.direction || step.lane) !== "reverse";
          const phase = step.phase || inferPhase(step);
          const tone = PHASE_TONE[phase] ?? PHASE_TONE.data;
          return (
            <li key={`${step.at_us ?? step.rel_us}_${idx}`} className="flow-seq-row">
              <span className="flow-seq-num">{idx + 1}</span>
              <div className={`flow-seq-msg ${tone}`}>
                <span className="flow-seq-phase">{phaseLabel(phase)}</span>
                <span className="flow-seq-label">{step.label}</span>
              </div>
              <div className={`flow-seq-arrow ${forward ? "is-forward" : "is-reverse"}`} aria-hidden>
                <span className="flow-seq-arrow-line" />
                <span className="flow-seq-arrow-head">{forward ? "▶" : "◀"}</span>
                <span className="flow-seq-arrow-from">{forward ? srcLabel : dstLabel}</span>
                <span className="flow-seq-arrow-to">{forward ? dstLabel : srcLabel}</span>
              </div>
              <span className="flow-seq-time">+{formatUs(step.rel_us)}</span>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function Participant({ title, label, align }: { title: string; label: string; align: "left" | "right" }) {
  return (
    <div className={`flow-seq-participant ${align === "right" ? "text-right" : ""}`}>
      <div className="flow-seq-participant-title">{title}</div>
      <div className="flow-seq-participant-label" title={label}>
        {label}
      </div>
    </div>
  );
}

function phaseLabel(phase: string): string {
  switch (phase) {
    case "start":
      return "Start";
    case "reply":
      return "Reply";
    case "close":
      return "Close";
    default:
      return "Data";
  }
}

function inferPhase(step: SequenceStep): string {
  const flags = new Set((step.flags ?? []).map((f) => f.toUpperCase()));
  if (flags.has("RST") || flags.has("FIN")) return "close";
  if (flags.has("SYN") && !flags.has("ACK")) return "start";
  if (flags.has("SYN") && flags.has("ACK")) return "reply";
  if (flags.has("PSH")) return "data";
  if (flags.has("ACK")) return "reply";
  return "data";
}

function formatUs(us: number): string {
  if (us >= 1_000_000) return `${(us / 1_000_000).toFixed(2)}s`;
  if (us >= 1_000) return `${(us / 1_000).toFixed(1)}ms`;
  return `${us}µs`;
}
