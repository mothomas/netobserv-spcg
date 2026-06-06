"use client";

import type { ReactNode } from "react";
import { useMemo } from "react";
import type { ControllerSummary, NamespaceWorkloads, PodDetail } from "@/lib/api";
import type { TraceEndpoint } from "@/lib/trace";

type Side = "source" | "destination";

type Props = {
  side: Side;
  endpoint: TraceEndpoint;
  onChange: (ep: TraceEndpoint) => void;
  workloadGroups: NamespaceWorkloads[];
  namespaces: string[];
  disabled?: boolean;
};

export function TraceEndpointPanel({
  side,
  endpoint,
  onChange,
  workloadGroups,
  namespaces,
  disabled,
}: Props) {
  const title = side === "source" ? "Source" : "Destination";
  const accent = side === "source" ? "border-siem-accent/50" : "border-siem-ok/40";

  const nsOptions = useMemo(() => {
    const set = new Set(namespaces);
    workloadGroups.forEach((g) => set.add(g.namespace));
    return Array.from(set).sort();
  }, [namespaces, workloadGroups]);

  const group = workloadGroups.find((g) => g.namespace === endpoint.namespace);

  const setMode = (mode: "ip" | "namespace") => {
    if (mode === "ip") {
      onChange({
        mode: "ip",
        ip: side === "destination" ? "external" : "",
        external: side === "destination",
      });
      return;
    }
    onChange({
      mode: "namespace",
      namespace: endpoint.namespace || nsOptions[0] || "",
      type: endpoint.type || "owner",
    });
  };

  return (
    <section
      className={`siem-card p-4 flex flex-col gap-4 border-t-4 ${accent}`}
      aria-label={`${title} endpoint`}
    >
      <div>
        <h3 className="text-sm font-semibold text-siem-text">{title}</h3>
        <p className="text-xs text-siem-muted mt-0.5">
          {side === "source"
            ? "Where traffic originates (pod, workload, or IP)"
            : "Where traffic is headed (pod, workload, or IP)"}
        </p>
      </div>

      <div className="flex rounded-siem border border-siem-border overflow-hidden text-xs">
        <ModeButton active={endpoint.mode === "namespace"} onClick={() => setMode("namespace")} disabled={disabled}>
          Namespace
        </ModeButton>
        <ModeButton active={endpoint.mode === "ip"} onClick={() => setMode("ip")} disabled={disabled}>
          IP
        </ModeButton>
      </div>

      {endpoint.mode === "ip" ? (
        <IpFields side={side} endpoint={endpoint} onChange={onChange} disabled={disabled} />
      ) : (
        <NamespaceFields
          endpoint={endpoint}
          onChange={onChange}
          nsOptions={nsOptions}
          group={group}
          disabled={disabled}
        />
      )}
    </section>
  );
}

function ModeButton({
  active,
  onClick,
  disabled,
  children,
}: {
  active: boolean;
  onClick: () => void;
  disabled?: boolean;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      className={`flex-1 px-3 py-2 ${active ? "fluent-nav-active" : "fluent-nav-idle hover:bg-siem-panel/60"}`}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function IpFields({
  side,
  endpoint,
  onChange,
  disabled,
}: {
  side: Side;
  endpoint: TraceEndpoint;
  onChange: (ep: TraceEndpoint) => void;
  disabled?: boolean;
}) {
  const isExternal = endpoint.ip === "external" || (!endpoint.ip && side === "destination");
  return (
    <div className="space-y-3">
      {side === "destination" && (
        <label className="flex items-center gap-2 text-sm text-siem-muted">
          <input
            type="checkbox"
            checked={isExternal}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...endpoint,
                ip: e.target.checked ? "external" : "",
                external: e.target.checked,
              })
            }
          />
          External destination (egress path)
        </label>
      )}
      {!isExternal && (
        <label className="block text-xs text-siem-muted">
          IP address
          <input
            className="siem-input w-full mt-1 font-mono text-sm"
            placeholder={side === "source" ? "10.0.0.5" : "203.0.113.10"}
            value={endpoint.ip || ""}
            disabled={disabled}
            onChange={(e) =>
              onChange({
                ...endpoint,
                ip: e.target.value.trim(),
                external: side === "destination" ? endpoint.external : false,
              })
            }
          />
        </label>
      )}
      {side === "destination" && !isExternal && (
        <label className="flex items-center gap-2 text-xs text-siem-muted">
          <input
            type="checkbox"
            checked={!!endpoint.external}
            disabled={disabled}
            onChange={(e) => onChange({ ...endpoint, external: e.target.checked })}
          />
          Treat as external (outside cluster)
        </label>
      )}
    </div>
  );
}

function NamespaceFields({
  endpoint,
  onChange,
  nsOptions,
  group,
  disabled,
}: {
  endpoint: TraceEndpoint;
  onChange: (ep: TraceEndpoint) => void;
  nsOptions: string[];
  group?: NamespaceWorkloads;
  disabled?: boolean;
}) {
  return (
    <div className="space-y-3">
      <label className="block text-xs text-siem-muted">
        Namespace
        <select
          className="siem-input w-full mt-1 text-sm"
          value={endpoint.namespace || ""}
          disabled={disabled}
          onChange={(e) =>
            onChange({
              ...endpoint,
              namespace: e.target.value,
              pod_name: "",
              owner_kind: "",
              owner_name: "",
            })
          }
        >
          <option value="">Select namespace…</option>
          {nsOptions.map((ns) => (
            <option key={ns} value={ns}>
              {ns}
            </option>
          ))}
        </select>
      </label>

      <div className="flex rounded-siem border border-siem-border overflow-hidden text-xs">
        <ModeButton
          active={endpoint.type !== "pod"}
          disabled={disabled || !endpoint.namespace}
          onClick={() => onChange({ ...endpoint, type: "owner", pod_name: "" })}
        >
          Workload
        </ModeButton>
        <ModeButton
          active={endpoint.type === "pod"}
          disabled={disabled || !endpoint.namespace}
          onClick={() => onChange({ ...endpoint, type: "pod", owner_kind: "", owner_name: "" })}
        >
          Pod
        </ModeButton>
      </div>

      {endpoint.type === "pod" ? (
        <PodPicker
          pods={group?.pods ?? []}
          value={endpoint.pod_name || ""}
          disabled={disabled}
          onSelect={(p) =>
            onChange({
              ...endpoint,
              pod_name: p.name,
              pod_uid: p.uid,
            })
          }
        />
      ) : (
        <WorkloadPicker
          group={group}
          kind={endpoint.owner_kind || ""}
          name={endpoint.owner_name || ""}
          disabled={disabled}
          onSelect={(kind, name, label_selector) =>
            onChange({
              ...endpoint,
              owner_kind: kind,
              owner_name: name,
              label_selector,
            })
          }
        />
      )}
    </div>
  );
}

function PodPicker({
  pods,
  value,
  onSelect,
  disabled,
}: {
  pods: PodDetail[];
  value: string;
  onSelect: (p: PodDetail) => void;
  disabled?: boolean;
}) {
  if (!pods.length) {
    return <p className="text-xs text-siem-muted">No pods in this namespace.</p>;
  }
  return (
    <label className="block text-xs text-siem-muted">
      Pod
      <select
        className="siem-input w-full mt-1 text-sm font-mono"
        value={value}
        disabled={disabled}
        onChange={(e) => {
          const p = pods.find((x) => x.name === e.target.value);
          if (p) onSelect(p);
        }}
      >
        <option value="">Select pod…</option>
        {pods.map((p) => (
          <option key={p.uid} value={p.name}>
            {p.name} {p.pod_ip ? `· ${p.pod_ip}` : ""}
          </option>
        ))}
      </select>
    </label>
  );
}

function WorkloadPicker({
  group,
  kind,
  name,
  onSelect,
  disabled,
}: {
  group?: NamespaceWorkloads;
  kind: string;
  name: string;
  onSelect: (kind: string, name: string, label_selector?: string) => void;
  disabled?: boolean;
}) {
  const rows: ControllerSummary[] = [
    ...(group?.deployments ?? []).map((r) => ({ ...r, kind: "Deployment" })),
    ...(group?.statefulsets ?? []).map((r) => ({ ...r, kind: "StatefulSet" })),
    ...(group?.daemonsets ?? []).map((r) => ({ ...r, kind: "DaemonSet" })),
  ];
  const selected = rows.find((r) => r.kind === kind && r.name === name);
  const key = selected ? `${selected.kind}/${selected.name}` : "";

  if (!group) {
    return <p className="text-xs text-siem-muted">Select a namespace first.</p>;
  }
  if (!rows.length) {
    return <p className="text-xs text-siem-muted">No Deployments, StatefulSets, or DaemonSets found.</p>;
  }
  return (
    <label className="block text-xs text-siem-muted">
      Workload
      <select
        className="siem-input w-full mt-1 text-sm"
        value={key}
        disabled={disabled}
        onChange={(e) => {
          const row = rows.find((r) => `${r.kind}/${r.name}` === e.target.value);
          if (row) onSelect(row.kind, row.name, row.label_selector);
        }}
      >
        <option value="">Select Deployment / DS / SS…</option>
        {rows.map((r) => (
          <option key={`${r.kind}/${r.name}`} value={`${r.kind}/${r.name}`}>
            {r.kind}/{r.name}
          </option>
        ))}
      </select>
    </label>
  );
}

export function TraceEndpointSelector({
  source,
  destination,
  onSourceChange,
  onDestChange,
  workloadGroups,
  namespaces,
  disabled,
}: {
  source: TraceEndpoint;
  destination: TraceEndpoint;
  onSourceChange: (ep: TraceEndpoint) => void;
  onDestChange: (ep: TraceEndpoint) => void;
  workloadGroups: NamespaceWorkloads[];
  namespaces: string[];
  disabled?: boolean;
}) {
  return (
    <div className="grid lg:grid-cols-[1fr_auto_1fr] gap-4 items-start">
      <TraceEndpointPanel
        side="source"
        endpoint={source}
        onChange={onSourceChange}
        workloadGroups={workloadGroups}
        namespaces={namespaces}
        disabled={disabled}
      />
      <div className="hidden lg:flex items-center justify-center text-siem-muted text-2xl pt-20 px-2" aria-hidden>
        →
      </div>
      <TraceEndpointPanel
        side="destination"
        endpoint={destination}
        onChange={onDestChange}
        workloadGroups={workloadGroups}
        namespaces={namespaces}
        disabled={disabled}
      />
    </div>
  );
}
