"use client";

import { useMemo, useState } from "react";
import { testS3Capture, type S3CaptureConfig } from "@/lib/api";

export type CaptureTierLimits = {
  max_concurrent_sessions: number;
  max_pods_per_session: number;
  max_capture_duration: string;
  max_capture_bytes: number;
  s3_offload_required: boolean;
  active_streams?: number;
};

type Props = {
  authSessionId: string;
  tierLimits?: CaptureTierLimits | null;
  value: S3CaptureConfig;
  onChange: (next: S3CaptureConfig) => void;
  tested: boolean;
  onTested: (ok: boolean) => void;
  disabled?: boolean;
};

const emptyConfig: S3CaptureConfig = {
  enabled: false,
  endpoint: "",
  region: "",
  bucket: "",
  prefix: "",
  access_key_id: "",
  secret_access_key: "",
  session_token: "",
  force_path_style: false,
  proxy_url: "",
};

export function S3CapturePanel({ authSessionId, tierLimits, value, onChange, tested, onTested, disabled }: Props) {
  const [testing, setTesting] = useState(false);
  const [testError, setTestError] = useState<string | null>(null);

  const cfg = value ?? emptyConfig;
  const s3Required = !!tierLimits?.s3_offload_required;
  const s3Active = cfg.enabled || s3Required;

  const canTest = useMemo(() => {
    if (!s3Active) return false;
    return cfg.bucket.trim() !== "" && cfg.access_key_id.trim() !== "" && cfg.secret_access_key.trim() !== "";
  }, [cfg, s3Active]);

  const update = (patch: Partial<S3CaptureConfig>) => {
    onTested(false);
    onChange({ ...cfg, ...patch });
  };

  const runTest = async () => {
    if (!canTest) return;
    setTesting(true);
    setTestError(null);
    try {
      await testS3Capture(authSessionId, { ...cfg, enabled: true });
      onTested(true);
    } catch (e) {
      onTested(false);
      setTestError(e instanceof Error ? e.message : String(e));
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="siem-card p-4 space-y-3">
      {tierLimits && (
        <p className="fluent-tier-policy">
          Tier policy: up to {tierLimits.max_pods_per_session} pods · {tierLimits.max_concurrent_sessions} concurrent
          streams · RAM captures ≤ {formatBytes(tierLimits.max_capture_bytes)} / {tierLimits.max_capture_duration}
          {tierLimits.s3_offload_required ? " · S3 streaming required" : " · S3 optional for large PCAP"}
          {typeof tierLimits.active_streams === "number" && tierLimits.active_streams > 0
            ? ` · ${tierLimits.active_streams} stream(s) active`
            : ""}
        </p>
      )}
      <label className="flex items-center gap-3 cursor-pointer">
        <input
          type="checkbox"
          checked={s3Active}
          disabled={disabled || s3Required}
          onChange={(e) => update({ enabled: e.target.checked })}
        />
        <span className="text-sm text-siem-text font-medium">
          Stream PCAP to S3 (no pod storage)
          {s3Required ? " — required by deployment tier" : ""}
        </span>
      </label>

      {s3Active && (
        <>
          <p className="text-xs text-siem-muted">
            Frames stream directly to your bucket. Stats and topology stay in the portal; credentials are kept in memory
            for this session only. Test the connection before starting capture.
          </p>
          <div className="grid gap-3 sm:grid-cols-2">
            <label className="block text-xs siem-label">
              Endpoint URL
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                placeholder="https://s3.amazonaws.com or MinIO URL"
                value={cfg.endpoint}
                disabled={disabled}
                onChange={(e) => update({ endpoint: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label">
              Region
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                placeholder="us-east-1"
                value={cfg.region}
                disabled={disabled}
                onChange={(e) => update({ region: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label">
              Bucket
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                value={cfg.bucket}
                disabled={disabled}
                onChange={(e) => update({ bucket: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label">
              Prefix (optional)
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                placeholder="captures/tenant-a"
                value={cfg.prefix}
                disabled={disabled}
                onChange={(e) => update({ prefix: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label">
              Access key ID
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                autoComplete="off"
                value={cfg.access_key_id}
                disabled={disabled}
                onChange={(e) => update({ access_key_id: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label">
              Secret access key
              <input
                type="password"
                className="siem-input mt-1 w-full font-mono text-xs"
                autoComplete="off"
                value={cfg.secret_access_key}
                disabled={disabled}
                onChange={(e) => update({ secret_access_key: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label sm:col-span-2">
              Session token (optional)
              <input
                type="password"
                className="siem-input mt-1 w-full font-mono text-xs"
                autoComplete="off"
                value={cfg.session_token ?? ""}
                disabled={disabled}
                onChange={(e) => update({ session_token: e.target.value })}
              />
            </label>
            <label className="block text-xs siem-label sm:col-span-2">
              HTTP proxy (optional)
              <input
                className="siem-input mt-1 w-full font-mono text-xs"
                placeholder="http://proxy:8080"
                value={cfg.proxy_url ?? ""}
                disabled={disabled}
                onChange={(e) => update({ proxy_url: e.target.value })}
              />
            </label>
          </div>
          <label className="flex items-center gap-2 text-xs text-siem-muted">
            <input
              type="checkbox"
              checked={!!cfg.force_path_style}
              disabled={disabled}
              onChange={(e) => update({ force_path_style: e.target.checked })}
            />
            Path-style URLs (MinIO / custom endpoint)
          </label>
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              className="siem-btn-ghost text-xs"
              disabled={disabled || testing || !canTest}
              onClick={() => runTest()}
            >
              {testing ? "Testing…" : "Test S3 connection"}
            </button>
            {tested && !testError && (
              <span className="text-xs text-emerald-400 font-medium">Connection verified</span>
            )}
            {!tested && s3Active && (
              <span className="text-xs text-amber-400">Test required before capture</span>
            )}
          </div>
          {testError && <p className="text-xs text-rose-400">{testError}</p>}
        </>
      )}
    </div>
  );
}

function formatBytes(n: number): string {
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KiB`;
  return `${Math.round(n / (1024 * 1024))} MiB`;
}
