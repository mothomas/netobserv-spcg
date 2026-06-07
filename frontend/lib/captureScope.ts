import type { CaptureSelection, PodDetail } from "@/lib/api";

export function buildCaptureScopeLabel(selections: CaptureSelection[], pods: PodDetail[]): string {
  if (selections.length === 0) return "No workload selected";
  const parts: string[] = [];
  const ownerSel = selections.filter((s) => s.type === "owner");
  const podSel = selections.filter((s) => s.type === "pod");
  for (const o of ownerSel) {
    parts.push(`${o.namespace}/${o.owner_kind}/${o.owner_name}`);
  }
  for (const p of podSel) {
    parts.push(`${p.namespace}/${p.pod_name}`);
  }
  if (parts.length <= 3) return parts.join(", ");
  return `${parts.slice(0, 2).join(", ")} +${parts.length - 2} more`;
}

export function captureScopeSummary(
  namespaces: string[],
  selections: CaptureSelection[],
  resolvedPodCount: number
): string {
  const ns = namespaces.length === 1 ? namespaces[0] : `${namespaces.length} namespaces`;
  const kind =
    selections.some((s) => s.type === "owner") && selections.some((s) => s.type === "pod")
      ? "mixed pod & workload"
      : selections.some((s) => s.type === "owner")
        ? "workload"
        : "pod";
  return `${ns} · ${kind} scope · ${resolvedPodCount || "—"} pod(s) resolved`;
}

export function selectionPodIdsFromState(
  selectedPods: Record<string, boolean>,
  podList: PodDetail[]
): string[] {
  return podList.filter((p) => selectedPods[p.uid]).map((p) => `${p.namespace}/${p.name}`);
}
