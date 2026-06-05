import { envPublicApiBase } from "./publicApiBase";

/** Public API origin. Empty = same-origin /api via spcg-frontend in-cluster proxy (secure layout default). */
export function publicApiBase(): string {
  if (typeof window !== "undefined") {
    const w = window as Window & { __SPCG_API_BASE__?: string };
    if (w.__SPCG_API_BASE__) return w.__SPCG_API_BASE__.replace(/\/$/, "");
    return "";
  }
  return envPublicApiBase();
}

export function apiUrl(path: string): string {
  const p = path.startsWith("/") ? path : `/${path}`;
  const base = publicApiBase();
  return base ? `${base}${p}` : p;
}
