"use client";

import type { AuthConfigResponse } from "@/lib/api";
import type { TroubleshootEntry } from "@/lib/troubleshoot";

type Props = {
  authConfig: AuthConfigResponse | null;
  authLoading: boolean;
  loginError: string | null;
  trace: TroubleshootEntry[];
  openshiftLogin: boolean;
  kubeconfigLogin: boolean;
};

export function TroubleshootPanel({
  authConfig,
  authLoading,
  loginError,
  trace,
  openshiftLogin,
  kubeconfigLogin,
}: Props) {
  const runtime =
    typeof window !== "undefined"
      ? {
          origin: window.location.origin,
          apiBase: (window as Window & { __SPCG_API_BASE__?: string }).__SPCG_API_BASE__ || "(same-origin)",
          authMethods: (window as Window & { __SPCG_AUTH_METHODS__?: string }).__SPCG_AUTH_METHODS__ || "(unset)",
        }
      : null;

  return (
    <div className="mt-6 rounded-lg border border-amber-500/40 bg-amber-950/20 p-4 text-left">
      <p className="text-xs font-semibold text-amber-300 uppercase tracking-wide mb-2">Troubleshooting mode</p>
      <p className="text-xs text-siem-muted mb-3">
        Enabled via <code className="text-amber-200">SPCG_TROUBLESHOOT=true</code> on spcg-frontend. Check pod logs:{" "}
        <code className="text-amber-200">oc logs -f deployment/spcg-frontend -n pcap-frontend</code>
      </p>
      <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs font-mono mb-3">
        <dt className="text-siem-muted">openshift UI</dt>
        <dd>{String(openshiftLogin)}</dd>
        <dt className="text-siem-muted">kubeconfig UI</dt>
        <dd>{String(kubeconfigLogin)}</dd>
        <dt className="text-siem-muted">auth loading</dt>
        <dd>{String(authLoading)}</dd>
        {runtime && (
          <>
            <dt className="text-siem-muted">origin</dt>
            <dd className="break-all">{runtime.origin}</dd>
            <dt className="text-siem-muted">api base</dt>
            <dd className="break-all">{runtime.apiBase}</dd>
            <dt className="text-siem-muted">SPCG_AUTH_METHODS</dt>
            <dd>{runtime.authMethods}</dd>
          </>
        )}
      </dl>
      {loginError && (
        <pre className="text-xs text-siem-err whitespace-pre-wrap mb-3 max-h-32 overflow-auto">{loginError}</pre>
      )}
      <p className="text-xs text-siem-muted mb-1">/api/v1/auth/config response</p>
      <pre className="text-xs text-siem-text bg-black/30 p-2 rounded max-h-40 overflow-auto">
        {authConfig ? JSON.stringify(authConfig, null, 2) : authLoading ? "(loading…)" : "(empty)"}
      </pre>
      {trace.length > 0 && (
        <>
          <p className="text-xs text-siem-muted mt-3 mb-1">Client trace</p>
          <pre className="text-xs text-siem-text bg-black/30 p-2 rounded max-h-32 overflow-auto">
            {trace.map((e) => `${e.at} ${e.step}${e.detail ? `: ${e.detail}` : ""}`).join("\n")}
          </pre>
        </>
      )}
    </div>
  );
}
