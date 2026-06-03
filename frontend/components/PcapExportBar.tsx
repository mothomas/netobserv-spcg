"use client";

import type { S3ExportInfo } from "@/lib/api";

export type CapturePodRef = {
  uid: string;
  name: string;
  namespace: string;
};

type Props = {
  pods: CapturePodRef[];
  busy?: boolean;
  s3Export?: S3ExportInfo | null;
  onDownloadPod: (pod: CapturePodRef) => void;
  onDownloadMerged: () => void;
  onOpenS3?: () => void;
};

export function PcapExportBar({ pods, busy, s3Export, onDownloadPod, onDownloadMerged, onOpenS3 }: Props) {
  if (pods.length === 0 && !s3Export?.enabled) return null;

  const s3Mode = !!s3Export?.enabled;
  const s3Ready = s3Mode && s3Export?.upload_done && !!s3Export.object_url;

  return (
    <div className="px-5 py-3 border-b border-siem-border bg-siem-bg/80 flex flex-wrap items-center gap-2">
      <span className="siem-label mr-1">{s3Mode ? "S3 PCAP export" : "PCAP export"}</span>

      {!s3Mode &&
        pods.map((p) => (
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

      {s3Mode && (
        <span className="text-xs text-siem-muted font-mono truncate max-w-[420px]" title={s3Export?.object_key}>
          {s3Export?.bucket}/{s3Export?.object_key}
          {typeof s3Export?.bytes === "number" ? ` · ${formatBytes(s3Export.bytes)}` : ""}
        </span>
      )}

      {s3Mode ? (
        <button
          type="button"
          className="ml-auto px-4 py-2 rounded-full text-sm font-semibold text-white shadow-[0_8px_22px_rgba(37,99,235,0.35)] transition fluent-accent-btn disabled:opacity-50"
          disabled={busy || !s3Ready}
          title={s3Ready ? "Open merged PCAP in S3" : "Available after capture stops and upload completes"}
          onClick={() => (onOpenS3 ? onOpenS3() : onDownloadMerged())}
        >
          {s3Ready ? "Open in S3" : "Uploading…"}
        </button>
      ) : (
        <button
          type="button"
          className="ml-auto px-4 py-2 rounded-full text-sm font-semibold text-white shadow-[0_8px_22px_rgba(37,99,235,0.35)] transition fluent-accent-btn"
          disabled={busy}
          title="Merge all pod captures into one file"
          onClick={onDownloadMerged}
        >
          Merge all
        </button>
      )}
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}
