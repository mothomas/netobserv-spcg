/** Styling aligned with networkplot-openshift/networkplot/html.py */

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

function svgUri(svg: string): string {
  return "data:image/svg+xml," + encodeURIComponent(svg);
}

function svgUriText(fill: string, stroke: string, text: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64"><rect x="4" y="4" width="56" height="56" rx="12" fill="${fill}" stroke="${stroke}" stroke-width="3"/><text x="32" y="38" text-anchor="middle" font-size="20" font-family="sans-serif" fill="${stroke}">${text}</text></svg>`;
  return svgUri(svg);
}

const K8S_ICON_SVGS: Record<string, string> = {
  pod: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 50 48"><path fill="#326ce5" d="M25 0 2 8.5v15L25 48l23-24.5V8.5z"/><path fill="#fff" d="M25 6.5 8 13v10.5L25 42l17-18.5V13z"/></svg>`,
  svc: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><circle cx="32" cy="32" r="24" fill="#10b981"/><circle cx="32" cy="32" r="10" fill="#fff"/></svg>`,
  ing: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect x="8" y="20" width="48" height="24" rx="6" fill="#22c55e"/><path fill="#fff" d="M28 32h16l-6-6v12z"/></svg>`,
  node: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect x="10" y="14" width="44" height="36" rx="4" fill="#64748b"/><rect x="16" y="20" width="32" height="8" rx="2" fill="#fff"/><rect x="16" y="32" width="32" height="8" rx="2" fill="#cbd5e1"/></svg>`,
  netpol: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><path fill="#94a3b8" d="M32 6 8 18v14l24 26 24-26V18z"/><path fill="#fff" d="M32 16 16 24v8l16 18 16-18v-8z"/></svg>`,
  pv: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><ellipse cx="32" cy="20" rx="20" ry="8" fill="#6366f1"/><rect x="12" y="20" width="40" height="24" fill="#818cf8"/><ellipse cx="32" cy="44" rx="20" ry="8" fill="#6366f1"/></svg>`,
};

const FALLBACK_SVGS: Record<string, string> = {
  egressip: svgUriText("#fff3cd", "#fd7e14", "E"),
  egressservice: svgUriText("#ede9fe", "#7c3aed", "S"),
  nad: svgUriText("#e7d4ff", "#6f42c1", "N"),
  "loadbalancer-external": svgUriText("#9be7a2", "#1e7e34", "LB"),
  external: svgUriText("#f8d7da", "#dc3544", "X"),
  generic: svgUriText("#e9ecef", "#6c757d", "?"),
};

const ICON_FILE_TO_KEY: Record<string, string> = {
  "pod.svg": "pod",
  "svc.svg": "svc",
  "ing.svg": "ing",
  "node.svg": "node",
  "netpol.svg": "netpol",
  "pv.svg": "pv",
};

export function iconUrl(ntype: string): string {
  const file = ICON_MAP[ntype];
  if (file) {
    const key = ICON_FILE_TO_KEY[file];
    if (key && K8S_ICON_SVGS[key]) return svgUri(K8S_ICON_SVGS[key]);
  }
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
