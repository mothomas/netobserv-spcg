import type { LayerScope } from "@/lib/sections";

type Props = {
  layer: LayerScope;
  detail?: string;
  compact?: boolean;
};

export function LayerScopeBanner({ layer, detail, compact }: Props) {
  if (compact) {
    return (
      <p className="text-xs text-siem-muted border border-siem-border rounded-siem px-3 py-2 bg-siem-panel/30">
        <span className="font-medium text-siem-text">{layer.shortLabel}</span>
        <span className="mx-2 opacity-40">·</span>
        {layer.boundary}
        {detail ? (
          <>
            <span className="mx-2 opacity-40">·</span>
            <span className="font-mono">{detail}</span>
          </>
        ) : null}
      </p>
    );
  }

  return (
    <div className="siem-card border border-siem-border px-4 py-3 bg-siem-panel/20 space-y-1">
      <p className="text-[10px] uppercase tracking-wide text-siem-muted">Layer scope</p>
      <p className="text-sm font-medium text-siem-text">{layer.title}</p>
      <p className="text-xs text-siem-muted">
        <span className="text-siem-text/80">Boundary:</span> {layer.boundary}
      </p>
      <p className="text-xs text-siem-muted">{layer.purpose}</p>
      <p className="text-xs text-siem-muted">
        <span className="text-siem-text/80">Selection:</span> {layer.selection}
      </p>
      {detail ? (
        <p className="text-xs font-mono text-siem-accent pt-1 border-t border-siem-border mt-2">{detail}</p>
      ) : null}
    </div>
  );
}
