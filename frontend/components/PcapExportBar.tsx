"use client";

export type CapturePodRef = {
  uid: string;
  name: string;
  namespace: string;
};

type Props = {
  pods: CapturePodRef[];
  busy?: boolean;
  onDownloadPod: (pod: CapturePodRef) => void;
  onDownloadMerged: () => void;
};

export function PcapExportBar({ pods, busy, onDownloadPod, onDownloadMerged }: Props) {
  if (pods.length === 0) return null;

  return (
    <div className="px-5 py-3 border-b border-siem-border bg-siem-bg/80 flex flex-wrap items-center gap-2">
      <span className="siem-label mr-1">PCAP export</span>
      {pods.map((p) => (
        <button
          key={p.uid || `${p.namespace}/${p.name}`}
          type="button"
          className="siem-btn-ghost text-xs"
          disabled={busy || !p.uid}
          title={p.uid ? `Download ${p.namespace}/${p.name}` : "Pod UID unavailable"}
          onClick={() => onDownloadPod(p)}
        >
          {p.name}
        </button>
      ))}
      <button
        type="button"
        className="ml-auto px-5 py-2 rounded-2xl text-sm font-semibold text-white bg-cyan-500 hover:bg-cyan-400 shadow-[0_8px_22px_rgba(34,211,238,0.35)] transition"
        disabled={busy}
        title="Merge all pod captures into one file"
        onClick={onDownloadMerged}
      >
        Merge all
      </button>
    </div>
  );
}
