/** Shared IPv4/IPv6 parsing and public-address checks (offline geo, graph labels). */

export function ipToInt(ip: string): number {
  const [a, b, c, d] = ip.split(".").map((n) => Number(n));
  return ((a << 24) >>> 0) + ((b << 16) >>> 0) + ((c << 8) >>> 0) + (d >>> 0);
}

export function isPublicIPv4(ip: string): boolean {
  const parts = ip.split(".").map((n) => Number(n));
  if (parts.length !== 4 || parts.some((n) => Number.isNaN(n) || n < 0 || n > 255)) return false;
  const [a, b] = parts;
  if (a === 10 || a === 127 || a === 0) return false;
  if (a === 169 && b === 254) return false;
  if (a === 172 && b >= 16 && b <= 31) return false;
  if (a === 192 && b === 168) return false;
  if (a >= 224) return false;
  return true;
}

/** Expand IPv6 to 8 hextets (lowercase, no zone id). */
export function expandIPv6(ip: string): string[] | null {
  let s = ip.trim().toLowerCase().split("%")[0];
  if (!s.includes(":")) return null;

  const halves = s.split("::");
  if (halves.length > 2) return null;

  const parsePart = (part: string): string[] => {
    if (!part) return [];
    return part.split(":").map((h) => {
      if (!/^[0-9a-f]{1,4}$/.test(h)) return "";
      return h;
    });
  };

  let left = parsePart(halves[0] ?? "");
  let right = parsePart(halves[1] ?? "");
  if (left.some((h) => h === "") || right.some((h) => h === "")) return null;

  if (halves.length === 2) {
    const fill = 8 - left.length - right.length;
    if (fill < 1) return null;
    const mid = Array(fill).fill("0");
    const hextets = [...left, ...mid, ...right];
    if (hextets.length !== 8) return null;
    return hextets;
  }

  if (left.length !== 8) return null;
  return left;
}

export function parseIPv6BigInt(ip: string): bigint | null {
  const hextets = expandIPv6(ip);
  if (!hextets) return null;
  let n = 0n;
  for (const h of hextets) {
    n = (n << 16n) + BigInt(parseInt(h || "0", 16));
  }
  return n;
}

export function isPublicIPv6(ip: string): boolean {
  const n = parseIPv6BigInt(ip);
  if (n === null) return false;
  // ::/128 unspecified
  if (n === 0n) return false;
  // ::1/128 loopback
  if (n === 1n) return false;
  // fe80::/10 link-local
  if ((n >> 118n) === 0x3fan) return false;
  // fc00::/7 unique local (fc00:: – fdff::)
  if ((n >> 121n) === 0x7en) return false;
  // ff00::/8 multicast
  if ((n >> 120n) === 0xffn) return false;
  // 2001:db8::/32 documentation
  if ((n >> 96n) === 0x20010db8n) return false;
  return true;
}

export function isPublicIP(ip: string): boolean {
  if (ip.includes(":")) return isPublicIPv6(ip);
  return isPublicIPv4(ip);
}

export function normalizeIP(ip: string): string {
  if (ip.includes(":")) {
    const hextets = expandIPv6(ip);
    if (!hextets) return ip.trim().toLowerCase();
    return hextets.map((h) => parseInt(h || "0", 16).toString(16)).join(":");
  }
  return ip.trim();
}
