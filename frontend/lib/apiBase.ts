/** Public API origin (OpenShift spcg-api Route). Empty = same-origin /api via Next proxy. */
export function publicApiBase(): string {
  if (typeof window !== "undefined") {
    const w = window as Window & { __SPCG_API_BASE__?: string };
    if (w.__SPCG_API_BASE__) return w.__SPCG_API_BASE__.replace(/\/$/, "");
  }
  const env = process.env.SPCG_PUBLIC_API_BASE || process.env.NEXT_PUBLIC_SPCG_API_BASE || "";
  return env.replace(/\/$/, "");
}

export function apiUrl(path: string): string {
  const p = path.startsWith("/") ? path : `/${path}`;
  const base = publicApiBase();
  return base ? `${base}${p}` : p;
}
