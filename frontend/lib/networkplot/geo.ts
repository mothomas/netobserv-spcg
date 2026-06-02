type CountryMapFile = {
  exact?: Record<string, string>;
  cidr?: Array<{ range: string; cc: string }>;
};

export type IPGeo = {
  countryCode: string;
  flagEmoji: string;
  flagSvg: string;
};

const geoCache = new Map<string, IPGeo>();
let countryMap: CountryMapFile | null = null;

function isPublicIPv4(ip: string): boolean {
  const parts = ip.split(".").map((n) => Number(n));
  if (parts.length !== 4 || parts.some((n) => Number.isNaN(n) || n < 0 || n > 255)) return false;
  const [a, b] = parts;
  if (a === 10) return false;
  if (a === 127) return false;
  if (a === 0) return false;
  if (a === 169 && b === 254) return false;
  if (a === 172 && b >= 16 && b <= 31) return false;
  if (a === 192 && b === 168) return false;
  if (a >= 224) return false;
  return true;
}

function toFlagEmoji(cc: string): string {
  if (!/^[A-Z]{2}$/.test(cc)) return "🌐";
  const offset = 127397;
  return String.fromCodePoint(cc.charCodeAt(0) + offset, cc.charCodeAt(1) + offset);
}

function ccColor(cc: string): string {
  const palette = ["#3b82f6", "#14b8a6", "#8b5cf6", "#f59e0b", "#22c55e", "#ef4444"];
  let hash = 0;
  for (const ch of cc) hash = (hash * 31 + ch.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

function flagSvg(cc: string): string {
  const bg = ccColor(cc);
  return `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="16" viewBox="0 0 26 16">
      <rect width="26" height="16" rx="8" fill="${bg}" />
      <text x="13" y="11" text-anchor="middle" font-size="8" font-family="Inter,Arial" fill="#fff">${cc}</text>
    </svg>`
  )}`;
}

function ipToInt(ip: string): number {
  const [a, b, c, d] = ip.split(".").map((n) => Number(n));
  return ((a << 24) >>> 0) + ((b << 16) >>> 0) + ((c << 8) >>> 0) + (d >>> 0);
}

function cidrContains(ip: string, cidr: string): boolean {
  const [base, maskLenRaw] = cidr.split("/");
  const maskLen = Number(maskLenRaw);
  if (!base || Number.isNaN(maskLen)) return false;
  const ipNum = ipToInt(ip);
  const baseNum = ipToInt(base);
  const mask = maskLen === 0 ? 0 : ((0xffffffff << (32 - maskLen)) >>> 0);
  return (ipNum & mask) === (baseNum & mask);
}

async function loadCountryMap(): Promise<CountryMapFile> {
  if (countryMap) return countryMap;
  try {
    const res = await fetch("/ip-country-map.json");
    if (!res.ok) throw new Error("map fetch failed");
    countryMap = (await res.json()) as CountryMapFile;
  } catch {
    countryMap = { exact: {}, cidr: [] };
  }
  return countryMap;
}

function fromCountryCode(cc: string): IPGeo {
  const code = cc.toUpperCase();
  return { countryCode: code, flagEmoji: toFlagEmoji(code), flagSvg: flagSvg(code) };
}

async function lookupFromMap(ip: string): Promise<IPGeo | null> {
  const map = await loadCountryMap();
  const cc = map.exact?.[ip];
  if (cc) return fromCountryCode(cc);
  for (const item of map.cidr || []) {
    if (cidrContains(ip, item.range)) return fromCountryCode(item.cc);
  }
  return null;
}

export async function resolveIPGeo(ip: string): Promise<IPGeo | null> {
  if (!ip || !isPublicIPv4(ip)) return null;
  if (geoCache.has(ip)) return geoCache.get(ip)!;
  const mapped = await lookupFromMap(ip);
  if (mapped) {
    geoCache.set(ip, mapped);
    return mapped;
  }
  try {
    const res = await fetch(`https://ipwho.is/${encodeURIComponent(ip)}`);
    if (!res.ok) throw new Error("geo lookup failed");
    const body = (await res.json()) as { success?: boolean; country_code?: string };
    const geo = fromCountryCode(body.success ? (body.country_code || "ZZ") : "ZZ");
    geoCache.set(ip, geo);
    return geo;
  } catch {
    const fallback = fromCountryCode("ZZ");
    geoCache.set(ip, fallback);
    return fallback;
  }
}
