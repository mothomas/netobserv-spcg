import type { ForceLink, ForceNode } from "./convert";

export type PlacedNode = ForceNode & { x: number; y: number };
export type PlacedLink = ForceLink & {
  flag: string;
  countryCode: string;
};

export type SvgLayout = {
  width: number;
  height: number;
  nodes: PlacedNode[];
  links: PlacedLink[];
};

/** Deterministic radial layout — always produces on-screen coordinates. */
export function layoutRadialGraph(
  nodes: ForceNode[],
  links: Array<ForceLink & { flag?: string; countryCode?: string }>,
  width: number,
  height: number
): SvgLayout {
  const w = Math.max(width, 320);
  const h = Math.max(height, 240);
  const cx = w / 2;
  const cy = h / 2;
  const radius = Math.min(w, h) * 0.34;

  const tracked = nodes.filter((n) => n.tracked);
  const other = nodes.filter((n) => !n.tracked);
  const ordered = [...tracked, ...other];

  const placedNodes: PlacedNode[] = ordered.map((n, idx) => {
    const angle = (idx / Math.max(ordered.length, 1)) * Math.PI * 2 - Math.PI / 2;
    const inner = n.tracked && tracked.length <= 3 ? 0.55 : 1;
    return {
      ...n,
      x: cx + Math.cos(angle) * radius * inner,
      y: cy + Math.sin(angle) * radius * inner,
    };
  });

  const placedLinks: PlacedLink[] = links.map((l) => ({
    ...l,
    flag: l.flag || "",
    countryCode: l.countryCode || "",
  }));

  return { width: w, height: h, nodes: placedNodes, links: placedLinks };
}

export function edgeBezierPath(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
  curvature: number
): string {
  const mx = (x1 + x2) / 2;
  const my = (y1 + y2) / 2;
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.hypot(dx, dy) || 1;
  const nx = -dy / len;
  const ny = dx / len;
  const bend = curvature * 36;
  return `M ${x1} ${y1} Q ${mx + nx * bend} ${my + ny * bend} ${x2} ${y2}`;
}
