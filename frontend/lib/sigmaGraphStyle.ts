import { isPublicIP } from "@/lib/ipAddress";

export { isPublicIPv4, isPublicIPv6, isPublicIP } from "@/lib/ipAddress";

export function hashColor(seed: string, palette: string[]): string {
  let hash = 0;
  for (const ch of seed) hash = (hash * 33 + ch.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

export function edgeColor(edgeKey: string): string {
  let hash = 0;
  for (const ch of edgeKey) hash = (hash * 33 + ch.charCodeAt(0)) | 0;
  const hue = Math.abs(hash) % 360;
  return `hsl(${hue} 78% 58%)`;
}

export function podColor(nodeId: string): string {
  return hashColor(nodeId, POD_PALETTE);
}

export function ipCircleColor(ipOrId: string): string {
  return hashColor(ipOrId, IP_PALETTE);
}

const POD_PALETTE = ["#2875E2", "#479ef5", "#6ccb5f", "#a78bfa", "#f472b6", "#22d3ee", "#fbbf24", "#fb7185"];
const IP_PALETTE = ["#64748b", "#6366f1", "#14b8a6", "#eab308", "#f97316", "#ec4899", "#0ea5e9", "#84cc16"];

export function extractPublicIp(node: { id: string; label?: string }): string | null {
  const label = (node.label || "").trim();
  if (isPublicIP(label)) return label;
  if (node.id.startsWith("ext/")) {
    const ip = node.id.slice(4);
    if (isPublicIP(ip)) return ip;
  }
  return null;
}

export function extractIpLabel(node: { id: string; label?: string }): string {
  const label = (node.label || "").trim();
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(label) || label.includes(":")) return label;
  return node.id;
}

export function isPodNode(node: { id: string; label?: string; type?: string; tracked?: boolean }): boolean {
  const kind = (node.type || "").toLowerCase();
  if (kind === "pod" || node.tracked) return true;
  if (kind === "deployment" || kind === "daemonset" || kind === "statefulset" || kind === "replicaset") {
    return true;
  }
  const label = node.label || "";
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(label)) return false;
  if (/^[a-z0-9][a-z0-9-]{2,}$/i.test(label) && label.includes("-")) return true;
  return false;
}

export function computeNodeDegrees(graph: { nodes: { id: string }[]; edges?: { source: string; target: string }[] }): Map<string, number> {
  const degrees = new Map<string, number>();
  for (const n of graph.nodes) degrees.set(n.id, 0);
  for (const e of graph.edges ?? []) {
    degrees.set(e.source, (degrees.get(e.source) ?? 0) + 1);
    degrees.set(e.target, (degrees.get(e.target) ?? 0) + 1);
  }
  return degrees;
}

export function scaledNodeSize(base: number, degree: number, maxDegree: number): number {
  if (maxDegree <= 0) return base;
  const ratio = degree / maxDegree;
  const hubBoost = degree >= Math.max(3, maxDegree * 0.5) ? 4 : 0;
  return Math.max(9, base + ratio * 10 + hubBoost);
}

export function toFlagEmoji(countryCode: string): string {
  const cc = countryCode.toUpperCase();
  if (!/^[A-Z]{2}$/.test(cc) || cc === "ZZ") return "🌐";
  return String.fromCodePoint(cc.charCodeAt(0) + 127397, cc.charCodeAt(1) + 127397);
}

/** Kubernetes pod icon (draw.io mxgraph.kubernetes.icon2 pod shape). */
export function podNodeImageDataUrl(accent = "#2875E2"): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 50 48">
    <polygon fill="${accent}" stroke="#ffffff" stroke-width="1.4" stroke-linejoin="round"
      points="0.25,30.24 5,9.6 45,9.6 25,0 49.75,30.24 36,47.52 25,48 14,47.52"/>
    <rect x="14" y="16" width="22" height="14" rx="2.2" fill="#ffffff" fill-opacity="0.96"/>
    <rect x="17" y="19" width="6" height="5" rx="1" fill="${accent}"/>
    <rect x="27" y="19" width="6" height="5" rx="1" fill="${accent}"/>
    <rect x="22.5" y="31" width="5" height="2.5" rx="0.8" fill="#ffffff" fill-opacity="0.9"/>
  </svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

/** Public endpoint node — flag fills the glyph; IP shown as node label only. */
export function flagNodeImageDataUrl(countryCode: string, emoji?: string): string {
  const flag = emoji || toFlagEmoji(countryCode);
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64">
    <circle cx="32" cy="32" r="30" fill="#0f172a" stroke="#475569" stroke-width="1.5"/>
    <text x="32" y="42" text-anchor="middle" font-size="34">${flag}</text>
  </svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

export const UI_BUILD_TAG = "small-20260610";
