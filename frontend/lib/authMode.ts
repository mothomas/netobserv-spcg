export function runtimeAuthMethods(): string[] {
  if (typeof window === "undefined") return [];
  const raw = (window as Window & { __SPCG_AUTH_METHODS__?: string }).__SPCG_AUTH_METHODS__;
  if (!raw) return [];
  return raw.split(",").map((m) => m.trim().toLowerCase()).filter(Boolean);
}

export function isOpenShiftAuthMode(methods?: string[] | null): boolean {
  if (methods?.includes("openshift")) return true;
  return runtimeAuthMethods().includes("openshift");
}

export function isKubeconfigAuthMode(methods?: string[] | null): boolean {
  if (methods?.includes("kubeconfig")) return true;
  return runtimeAuthMethods().includes("kubeconfig");
}
