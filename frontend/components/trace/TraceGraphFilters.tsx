"use client";

import type { TraceGraphFilter } from "@/lib/traceGraphFilter";

type Props = {
  value: TraceGraphFilter;
  stats?: { focused_nodes?: number; logical_nodes?: number; physical_nodes?: number; pruned_nodes?: number };
  onChange: (next: TraceGraphFilter) => void;
};

export function TraceGraphFilters({ value, stats, onChange }: Props) {
  return (
    <div className="flex flex-wrap items-center gap-3 text-xs">
      <FilterToggle
        label="Focus path"
        checked={value.focusPath}
        hint={stats?.focused_nodes != null ? `${stats.focused_nodes} hops` : undefined}
        onChange={(focusPath) => onChange({ ...value, focusPath })}
      />
      <FilterToggle
        label="Logical"
        checked={value.logical}
        hint={stats?.logical_nodes != null ? String(stats.logical_nodes) : undefined}
        onChange={(logical) => onChange({ ...value, logical })}
      />
      <FilterToggle
        label="Physical"
        checked={value.physical}
        hint={stats?.physical_nodes != null ? String(stats.physical_nodes) : undefined}
        onChange={(physical) => onChange({ ...value, physical })}
      />
      {stats?.pruned_nodes != null && stats.pruned_nodes > 0 && (
        <span className="text-siem-muted font-mono">{stats.pruned_nodes} context nodes hidden</span>
      )}
    </div>
  );
}

function FilterToggle({
  label,
  checked,
  hint,
  onChange,
}: {
  label: string;
  checked: boolean;
  hint?: string;
  onChange: (v: boolean) => void;
}) {
  return (
    <label className="flex items-center gap-2 px-2 py-1 rounded-md border border-siem-border bg-siem-panel/40 cursor-pointer">
      <input type="checkbox" checked={checked} onChange={(e) => onChange(e.target.checked)} />
      <span className="text-siem-text">{label}</span>
      {hint && <span className="text-siem-muted font-mono">{hint}</span>}
    </label>
  );
}
