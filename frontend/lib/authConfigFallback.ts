/** Server-side auth/config fallback from pod env (no portal egress required). */
export function serverAuthMethods(): string[] {
  const raw = (process.env.SPCG_AUTH_METHODS || "").trim();
  if (!raw) return [];
  return raw.split(",").map((m) => m.trim().toLowerCase()).filter(Boolean);
}

export type AuthConfigBody = {
  methods: string[];
  openshift?: { authorize_path: string; authorize_url?: string; error?: string };
};

export function buildAuthConfigBody(detail?: string): AuthConfigBody | null {
  const methods = serverAuthMethods();
  if (!methods.length) return null;
  const body: AuthConfigBody = { methods };
  if (methods.includes("openshift")) {
    const publicBase = (process.env.SPCG_PUBLIC_API_BASE || process.env.NEXT_PUBLIC_SPCG_API_BASE || "")
      .trim()
      .replace(/\/$/, "");
    body.openshift = {
      authorize_path: "/api/v1/auth/openshift/authorize",
      ...(publicBase ? { authorize_url: `${publicBase}/api/v1/auth/openshift/authorize` } : {}),
      ...(detail ? { error: detail } : {}),
    };
  }
  return body;
}

export function apiProxyDisabled(): boolean {
  const v = (process.env.SPCG_DISABLE_API_PROXY || "").trim().toLowerCase();
  return v === "true" || v === "1";
}

export function portalBase(): string {
  return (
    process.env.SPCG_API_URL ||
    "http://spcg-ui-portal.spcg-control.svc.cluster.local:80"
  ).replace(/\/$/, "");
}
