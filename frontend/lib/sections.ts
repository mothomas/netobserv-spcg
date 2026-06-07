/** Primary app shell sections — each maps to a distinct observability layer. */
export type AppSection = "workspace" | "capture" | "trace" | "apptraffic" | "flow" | "ai";

export type LayerScope = {
  id: AppSection;
  title: string;
  shortLabel: string;
  boundary: string;
  purpose: string;
  selection: string;
};

export const LAYER_SCOPES: Record<Exclude<AppSection, "workspace" | "ai">, LayerScope> = {
  capture: {
    id: "capture",
    title: "Capture",
    shortLabel: "Capture",
    boundary: "Selected namespaces (tenant RBAC boundary)",
    purpose: "Live PCAP from eBPF sensors on chosen pods or controllers (Deployment, StatefulSet, DaemonSet).",
    selection: "Pod-level or workload-level within scoped namespaces — not cross-namespace unless both are in your scope list.",
  },
  trace: {
    id: "trace",
    title: "Packet Trace",
    shortLabel: "Packet Trace",
    boundary: "Cross-namespace and external endpoints allowed",
    purpose: "Map an application source to a destination through infrastructure (Service, Ingress, Node, CNI, policy hops).",
    selection: "IP or namespace/workload on each side — source ↔ destination, independent of capture checkbox selection.",
  },
  apptraffic: {
    id: "apptraffic",
    title: "Application network",
    shortLabel: "App network",
    boundary: "Same as active capture selection (namespace / workload / pod)",
    purpose: "How selected workloads talk on the wire: ingress into pods, replies, DNS/TLS, and in-scope service calls.",
    selection: "Requires a running capture from the Capture layer — stays inside your workload boundary, unlike Packet Trace.",
  },
  flow: {
    id: "flow",
    title: "Flow graph",
    shortLabel: "Flow graph",
    boundary: "Active capture session",
    purpose: "L3/L4 topology, sequence ladders, PCAP export, and drop diagnosis from the same capture scope.",
    selection: "Populated after Capture starts; shares the capture session boundary.",
  },
};

export function syncAppUrl(section: AppSection, traceId?: string | null): void {
  if (typeof window === "undefined") return;
  const url = new URL(window.location.href);
  if (section === "workspace") {
    url.searchParams.delete("section");
    url.searchParams.delete("trace_id");
  } else if (section === "capture") {
    url.searchParams.set("section", "capture");
    url.searchParams.delete("trace_id");
  } else {
    url.searchParams.set("section", section);
    if (section === "trace" && traceId) {
      url.searchParams.set("trace_id", traceId);
    } else {
      url.searchParams.delete("trace_id");
    }
  }
  const qs = url.searchParams.toString();
  window.history.replaceState({}, "", qs ? `${url.pathname}?${qs}` : url.pathname);
}
