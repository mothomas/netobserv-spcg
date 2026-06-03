import { toFlagEmoji } from "@/lib/sigmaGraphStyle";
import {
  ipToInt,
  isPublicIP,
  isPublicIPv4,
  normalizeIP,
  parseIPv6BigInt,
} from "@/lib/ipAddress";

export { isPublicIPv4, isPublicIP } from "@/lib/ipAddress";

/** Legacy v1 map (exact + cidr strings). */
export type LegacyCountryMap = {
  exact?: Record<string, string>;
  cidr?: Array<{ range: string; cc: string }>;
};

/** v2: IPv4 only. v3: IPv4 + IPv6 (decimal string bounds for v6). */
export type CompactCountryMapV2 = {
  v: 2;
  ranges: Array<[number, number, string]>;
};

export type CompactCountryMapV3 = {
  v: 3;
  ranges4: Array<[number, number, string]>;
  ranges6: Array<[string, string, string]>;
};

export type CompactCountryMap = CompactCountryMapV2 | CompactCountryMapV3;

export type CountryMapFile = LegacyCountryMap | CompactCountryMap;

export function isCompactV3(map: CountryMapFile): map is CompactCountryMapV3 {
  return (map as CompactCountryMapV3).v === 3;
}

export function isCompactV2(map: CountryMapFile): map is CompactCountryMapV2 {
  return (map as CompactCountryMapV2).v === 2 && Array.isArray((map as CompactCountryMapV2).ranges);
}

export function cidrContains(ip: string, cidr: string): boolean {
  const [base, maskLenRaw] = cidr.split("/");
  const maskLen = Number(maskLenRaw);
  if (!base || Number.isNaN(maskLen)) return false;
  const mask = maskLen === 0 ? 0 : ((0xffffffff << (32 - maskLen)) >>> 0);
  return (ipToInt(ip) & mask) === (ipToInt(base) & mask);
}

function lookupLegacy(ip: string, map: LegacyCountryMap): string | null {
  const exact = map.exact?.[ip];
  if (exact) return exact.toUpperCase();
  for (const item of map.cidr ?? []) {
    if (cidrContains(ip, item.range)) return item.cc.toUpperCase();
  }
  return null;
}

function lookupCompactV4(ipNum: number, ranges: Array<[number, number, string]>): string | null {
  let lo = 0;
  let hi = ranges.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const [start, end, cc] = ranges[mid];
    if (ipNum < start) hi = mid - 1;
    else if (ipNum > end) lo = mid + 1;
    else return cc;
  }
  return null;
}

function lookupCompactV6(ipNum: bigint, ranges: Array<[string, string, string]>): string | null {
  let lo = 0;
  let hi = ranges.length - 1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    const [start, end, cc] = ranges[mid];
    const s = BigInt(start);
    const e = BigInt(end);
    if (ipNum < s) hi = mid - 1;
    else if (ipNum > e) lo = mid + 1;
    else return cc;
  }
  return null;
}

export function lookupCountryFromMap(ip: string, map: CountryMapFile): string | null {
  const norm = normalizeIP(ip);
  if (isCompactV3(map)) {
    if (norm.includes(":")) {
      const n = parseIPv6BigInt(norm);
      if (n === null) return null;
      return lookupCompactV6(n, map.ranges6);
    }
    return lookupCompactV4(ipToInt(norm), map.ranges4);
  }
  if (isCompactV2(map)) {
    if (norm.includes(":")) return null;
    return lookupCompactV4(ipToInt(norm), map.ranges);
  }
  return lookupLegacy(norm, map);
}

let mapCache: CountryMapFile | null = null;
let mapPromise: Promise<CountryMapFile> | null = null;

export async function loadIpCountryMap(): Promise<CountryMapFile> {
  if (mapCache) return mapCache;
  if (!mapPromise) {
    mapPromise = (async () => {
      try {
        const res = await fetch("/ip-country-map.json");
        if (!res.ok) throw new Error("map fetch failed");
        mapCache = (await res.json()) as CountryMapFile;
      } catch {
        mapCache = { v: 3, ranges4: [], ranges6: [] };
      }
      return mapCache;
    })();
  }
  return mapPromise;
}

export type IPGeo = {
  countryCode: string;
  flagEmoji: string;
  flagSvg: string;
};

const geoCache = new Map<string, IPGeo>();

function ccColor(cc: string): string {
  const palette = ["#3b82f6", "#14b8a6", "#8b5cf6", "#f59e0b", "#22c55e", "#ef4444"];
  let hash = 0;
  for (const ch of cc) hash = (hash * 31 + ch.charCodeAt(0)) | 0;
  return palette[Math.abs(hash) % palette.length];
}

export function flagSvg(cc: string): string {
  const code = cc.toUpperCase();
  const bg = ccColor(code);
  return `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="16" viewBox="0 0 26 16">
      <rect width="26" height="16" rx="8" fill="${bg}" />
      <text x="13" y="11" text-anchor="middle" font-size="8" font-family="Inter,Arial" fill="#fff">${code}</text>
    </svg>`
  )}`;
}

export function geoFromCountryCode(cc: string): IPGeo {
  const code = cc.toUpperCase();
  return { countryCode: code, flagEmoji: toFlagEmoji(code), flagSvg: flagSvg(code) };
}

export async function resolveIPGeo(ip: string): Promise<IPGeo | null> {
  if (!ip || !isPublicIP(ip)) return null;
  const key = normalizeIP(ip);
  if (geoCache.has(key)) return geoCache.get(key)!;
  const map = await loadIpCountryMap();
  const cc = lookupCountryFromMap(key, map) ?? "ZZ";
  const geo = geoFromCountryCode(cc);
  geoCache.set(key, geo);
  return geo;
}
