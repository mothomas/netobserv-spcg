import type { EdgeDetail, SequenceStep, TopologyEdge } from "@/lib/ai";

function reverseEdgeId(from: string, to: string): string {
  return `${from}->${to}`;
}

function cloneWithDirection(steps: SequenceStep[], direction: "forward" | "reverse"): SequenceStep[] {
  return steps.map((s) => ({ ...s, direction, lane: direction }));
}

/** Merge forward + reply legs for the selected directed link only. */
export function conversationSteps(
  edge: TopologyEdge,
  edgeDetails?: Record<string, EdgeDetail> | null
): SequenceStep[] {
  if (!edgeDetails) return [];
  const fwd = edgeDetails[edge.id]?.sequence ?? [];
  const rev = edgeDetails[reverseEdgeId(edge.to, edge.from)]?.sequence ?? [];
  if (fwd.length === 0 && rev.length === 0) return [];

  const merged = [...cloneWithDirection(fwd, "forward"), ...cloneWithDirection(rev, "reverse")];

  merged.sort((a, b) => {
    const atA = a.at_us ?? a.rel_us;
    const atB = b.at_us ?? b.rel_us;
    return atA - atB || a.rel_us - b.rel_us;
  });

  const capped = merged.slice(0, 48);
  const base = capped.find((s) => s.at_us && s.at_us > 0)?.at_us ?? 0;
  if (base > 0) {
    return capped.map((s) => ({
      ...s,
      rel_us: s.at_us && s.at_us > 0 ? s.at_us - base : s.rel_us,
    }));
  }
  return capped;
}

export function endpointLabel(node?: { pod?: string; owner_name?: string; label?: string; namespace?: string }, fallback = "—"): string {
  if (!node) return fallback;
  const entity = node.pod || node.owner_name || node.label || fallback;
  const ns = node.namespace ? `${node.namespace}/` : "";
  return `${ns}${entity}`;
}
