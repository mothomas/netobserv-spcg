/** Styling aligned with networkplot-openshift/networkplot/html.py */

export const K8S_ICON_BASE =
  "https://raw.githubusercontent.com/kubernetes/community/master/icons/svg/resources/unlabeled/";

export const ICON_MAP: Record<string, string> = {
  pod: "pod.svg",
  "service-clusterip": "svc.svg",
  "service-loadbalancer": "svc.svg",
  "service-other": "svc.svg",
  service: "svc.svg",
  ingress: "ing.svg",
  route: "ing.svg",
  node: "node.svg",
  networkpolicy: "netpol.svg",
  pvc: "pv.svg",
};

export type NodeStyle = { bg: string; accent: string; border: string; text: string };

export const STYLE_BY_TYPE: Record<string, NodeStyle> = {
  pod: { bg: "#ffffff", accent: "#dbeafe", border: "#3b82f6", text: "#1e3a8a" },
  "service-clusterip": { bg: "#ffffff", accent: "#d1fae5", border: "#10b981", text: "#065f46" },
  "service-loadbalancer": { bg: "#ffffff", accent: "#bbf7d0", border: "#059669", text: "#064e3b" },
  "service-other": { bg: "#ffffff", accent: "#f1f5f9", border: "#94a3b8", text: "#64748b" },
  service: { bg: "#ffffff", accent: "#d1fae5", border: "#10b981", text: "#065f46" },
  ingress: { bg: "#ffffff", accent: "#dcfce7", border: "#22c55e", text: "#14532d" },
  route: { bg: "#ffffff", accent: "#dcfce7", border: "#16a34a", text: "#14532d" },
  egressip: { bg: "#ffffff", accent: "#ffedd5", border: "#f97316", text: "#7c2d12" },
  egressservice: { bg: "#ffffff", accent: "#ede9fe", border: "#7c3aed", text: "#5b21b6" },
  node: { bg: "#ffffff", accent: "#f1f5f9", border: "#64748b", text: "#334155" },
  nad: { bg: "#ffffff", accent: "#ede9fe", border: "#8b5cf6", text: "#4c1d95" },
  "external-client": { bg: "#ffffff", accent: "#ffe4e6", border: "#f43f5e", text: "#881337" },
  "loadbalancer-external": { bg: "#ffffff", accent: "#ccfbf1", border: "#14b8a6", text: "#115e59" },
  external: { bg: "#ffffff", accent: "#fee2e2", border: "#ef4444", text: "#991b1b" },
  networkpolicy: { bg: "#ffffff", accent: "#f1f5f9", border: "#94a3b8", text: "#475569" },
  generic: { bg: "#ffffff", accent: "#f8fafc", border: "#cbd5e1", text: "#475569" },
};

export const RANK_BY_TYPE: Record<string, number> = {
  "external-client": 0,
  ingress: 1,
  route: 1,
  "service-clusterip": 2,
  "service-loadbalancer": 2,
  service: 2,
  "loadbalancer-external": 2,
  pod: 3,
  node: 4,
  external: 4,
  generic: 5,
  "service-other": 6,
};

function svgUri(fill: string, stroke: string, text: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect x="4" y="4" width="56" height="56" rx="12" fill="${fill}" stroke="${stroke}" stroke-width="3"/><text x="32" y="38" text-anchor="middle" font-size="20" font-family="sans-serif" fill="${stroke}">${text}</text></svg>`;
  return "data:image/svg+xml," + encodeURIComponent(svg);
}

const FALLBACK_SVGS: Record<string, string> = {
  egressip: svgUri("#fff3cd", "#fd7e14", "E"),
  egressservice: svgUri("#ede9fe", "#7c3aed", "S"),
  nad: svgUri("#e7d4ff", "#6f42c1", "N"),
  "loadbalancer-external": svgUri("#9be7a2", "#1e7e34", "LB"),
  external: svgUri("#f8d7da", "#dc3544", "X"),
  generic: svgUri("#e9ecef", "#6c757d", "?"),
};

export function iconUrl(ntype: string): string {
  if (ntype in ICON_MAP) return K8S_ICON_BASE + ICON_MAP[ntype];
  return FALLBACK_SVGS[ntype] ?? FALLBACK_SVGS.generic;
}

export function inferNodeType(kind?: string, label?: string): string {
  const k = (kind || "").toLowerCase();
  if (k === "pod") return "pod";
  if (k === "service") return "service-clusterip";
  if (k === "deployment" || k === "daemonset" || k === "statefulset") return "pod";
  const low = (label || "").toLowerCase();
  if (low.includes("dns") || low.startsWith("kube-")) return "service-clusterip";
  if (low.includes("external")) return "external";
  return "generic";
}
