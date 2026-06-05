/** Derive spcg-api Route origin from UI Route host (e.g. spcg.apps.example.com → spcg-api.apps.example.com). */
export function deriveApiBaseFromUiHost(host: string, protocol = "https:"): string {
  const h = (host || "").split(":")[0].trim().toLowerCase();
  if (!h) return "";
  const proto = protocol.endsWith(":") ? protocol : `${protocol}:`;
  if (h.startsWith("spcg-api.")) return `${proto}//${h}`;
  if (h.startsWith("spcg.")) return `${proto}//spcg-api.${h.slice("spcg.".length)}`;
  const dot = h.indexOf(".");
  if (dot > 0 && h.slice(0, dot) === "spcg") return `${proto}//spcg-api${h.slice(dot)}`;
  return "";
}

export function envPublicApiBase(): string {
  return (process.env.SPCG_PUBLIC_API_BASE || process.env.NEXT_PUBLIC_SPCG_API_BASE || "")
    .trim()
    .replace(/\/$/, "");
}

export function resolvePublicApiBase(host?: string, protocol?: string): string {
  const fromEnv = envPublicApiBase();
  if (fromEnv) return fromEnv;
  if (host) return deriveApiBaseFromUiHost(host, protocol || "https:");
  return "";
}
