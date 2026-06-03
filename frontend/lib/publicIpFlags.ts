import { toFlagEmoji } from "@/lib/sigmaGraphStyle";
import { isPublicIP } from "@/lib/ipCountryLookup";
import {
  loadIpCountryMap,
  lookupCountryFromMap,
  type CountryMapFile,
} from "@/lib/ipCountryLookup";

export type PublicIpFlagPair = {
  ip: string;
  countryCode: string;
  flag: string;
};

export { loadIpCountryMap, lookupCountryFromMap, type CountryMapFile };

export function pairFromCountry(ip: string, countryCode: string): PublicIpFlagPair {
  const cc = countryCode.toUpperCase();
  return { ip, countryCode: cc, flag: toFlagEmoji(cc) };
}

/** Build sorted public-IP → flag pairs from graph node labels/ids plus the static map. */
export function compilePublicIpFlagPairs(
  ips: string[],
  map: CountryMapFile,
  resolved: Record<string, string> = {}
): PublicIpFlagPair[] {
  const unique = [...new Set(ips.filter(isPublicIP))].sort();
  return unique.map((ip) => {
    const cc = resolved[ip] || lookupCountryFromMap(ip, map) || "ZZ";
    return pairFromCountry(ip, cc);
  });
}
