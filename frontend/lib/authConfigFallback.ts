import { resolvePublicApiBase } from "./publicApiBase";

export type AuthConfigRequestContext = { host?: string; proto?: string };

/** Server-side auth/config fallback from pod env (no portal egress required). */
export function serverAuthMethods(): string[] {
  const raw = (process.env.SPCG_AUTH_METHODS || "").trim();
  if (!raw) return [];
  return raw.split(",").map((m) => m.trim().toLowerCase()).filter(Boolean);
}

export type AuthConfigBody = {
  methods: string[];
  public_api_base?: string;
  openshift?: { authorize_path: string; authorize_url?: string; error?: string };
};

export function buildAuthConfigBody(detail?: string, req?: AuthConfigRequestContext): AuthConfigBody | null {
  const methods = serverAuthMethods();
  if (!methods.length) return null;
  const publicBase = resolvePublicApiBase(req?.host, req?.proto);
  const body: AuthConfigBody = { methods, ...(publicBase ? { public_api_base: publicBase } : {}) };
  if (methods.includes("openshift")) {
    // Same-origin authorize (frontend proxies to portal); avoids wrong derived spcg-api host.
    body.openshift = {
      authorize_path: "/api/v1/auth/openshift/authorize",
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
