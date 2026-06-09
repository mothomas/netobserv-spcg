"use client";

import type { PathOption } from "@/lib/trace";

type Props = {
  options: PathOption[];
  selected: string[];
  onToggle: (id: string) => void;
  onClear: () => void;
};

function statusBadge(status: string) {
  if (status === "discovered") {
    return (
      <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-ok border border-siem-ok/30">discovered</span>
    );
  }
  if (status === "out_of_scope") {
    return (
      <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-warn border border-siem-warn/30">out of scope</span>
    );
  }
  return (
    <span className="text-[10px] px-1.5 py-0.5 rounded-md text-siem-muted border border-siem-border">{status}</span>
  );
}

function PathGroup({
  title,
  accentClass,
  items,
  selected,
  onToggle,
}: {
  title: string;
  accentClass: string;
  items: PathOption[];
  selected: string[];
  onToggle: (id: string) => void;
}) {
  if (!items.length) return null;
  return (
    <div className="space-y-2">
      <p className={`text-xs font-semibold uppercase tracking-wide ${accentClass}`}>
        {title} ({items.length})
      </p>
      <div className="space-y-1.5">
        {items.map((p) => {
          const active = selected.includes(p.id);
          return (
            <button
              key={p.id}
              type="button"
              onClick={() => onToggle(p.id)}
              className={`w-full text-left rounded-siem border px-3 py-2 transition-colors ${
                active
                  ? "border-siem-accent bg-siem-accent/10"
                  : "border-siem-border bg-siem-panel/40 hover:border-siem-border-hi"
              }`}
            >
              <div className="flex items-start justify-between gap-2">
                <span className="text-sm text-siem-text font-medium leading-snug">{p.label}</span>
                {statusBadge(p.status)}
              </div>
              <p className="text-[11px] text-siem-muted mt-1 font-mono">{p.mechanism}</p>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function TracePathOptionList({ options, selected, onToggle, onClear }: Props) {
  const ingress = options.filter((p) => p.direction === "ingress");
  const egress = options.filter((p) => p.direction === "egress");
  const host = options.filter((p) => p.direction === "host" || p.direction === "context");

  if (!options.length) {
    return <p className="text-sm text-siem-muted">No path options discovered for this trace.</p>;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-xs text-siem-muted">Highlight ingress and egress independently</p>
        {selected.length > 0 && (
          <button type="button" className="siem-btn-ghost text-xs py-1 px-2" onClick={onClear}>
            Clear
          </button>
        )}
      </div>
      <PathGroup title="Ingress" accentClass="text-siem-accent" items={ingress} selected={selected} onToggle={onToggle} />
      <PathGroup title="Egress" accentClass="text-siem-ok" items={egress} selected={selected} onToggle={onToggle} />
      {host.length > 0 && (
        <PathGroup title="Host / context" accentClass="text-siem-muted" items={host} selected={selected} onToggle={onToggle} />
      )}
    </div>
  );
}
