function authMethodsFromMeta(): string {
  if (typeof document === "undefined") return "";
  return document.querySelector('meta[name="spcg-auth-methods"]')?.getAttribute("content")?.trim() || "";
}

export function runtimeAuthMethods(): string[] {
  if (typeof window === "undefined") return [];
  const raw =
    (window as Window & { __SPCG_AUTH_METHODS__?: string }).__SPCG_AUTH_METHODS__ || authMethodsFromMeta();
  if (!raw) return [];
  return raw.split(",").map((m) => m.trim().toLowerCase()).filter(Boolean);
}

/** Prefer portal /auth/config; fall back to SPCG_AUTH_METHODS injected at runtime. */
export function effectiveAuthMethods(apiMethods?: string[] | null): string[] {
  if (apiMethods?.length) return apiMethods;
  return runtimeAuthMethods();
}

export function isOpenShiftAuthMode(methods?: string[] | null): boolean {
  return effectiveAuthMethods(methods).includes("openshift");
}

export function isKubeconfigAuthMode(methods?: string[] | null): boolean {
  return effectiveAuthMethods(methods).includes("kubeconfig");
}
