#!/usr/bin/env node
/**
 * Build frontend/public/ip-country-map.json from DB-IP Lite (CC BY 4.0).
 * Source: https://github.com/sapics/ip-location-db
 *
 * Usage:
 *   node scripts/build-ip-country-map.mjs
 *   node scripts/build-ip-country-map.mjs --input-v4 a.csv --input-v6 b.csv
 */

import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.join(__dirname, "..");
const OUT = path.join(ROOT, "frontend/public/ip-country-map.json");
const LICENSE_OUT = path.join(ROOT, "frontend/public/DBIP-LICENSE");
const URL_V4 =
  "https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-country/dbip-country-ipv4-num.csv";
const URL_V6 =
  "https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-country/dbip-country-ipv6-num.csv";

function parseArgs() {
  const v4Idx = process.argv.indexOf("--input-v4");
  const v6Idx = process.argv.indexOf("--input-v6");
  const legacyIdx = process.argv.indexOf("--input");
  return {
    inputV4: v4Idx >= 0 ? process.argv[v4Idx + 1] : legacyIdx >= 0 ? process.argv[legacyIdx + 1] : null,
    inputV6: v6Idx >= 0 ? process.argv[v6Idx + 1] : null,
  };
}

async function downloadCsv(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`download failed: ${res.status} ${url}`);
  return res.text();
}

function parseV4Csv(text) {
  const ranges = [];
  for (const line of text.split("\n")) {
    const row = line.trim();
    if (!row || row.startsWith("#")) continue;
    const parts = row.split(",");
    if (parts.length < 3) continue;
    const start = Number(parts[0]);
    const end = Number(parts[1]);
    const cc = parts[2].trim().toUpperCase();
    if (!Number.isFinite(start) || !Number.isFinite(end) || !/^[A-Z]{2}$/.test(cc)) continue;
    ranges.push([start, end, cc]);
  }
  ranges.sort((a, b) => a[0] - b[0]);
  return ranges;
}

function parseV6Csv(text) {
  const ranges = [];
  for (const line of text.split("\n")) {
    const row = line.trim();
    if (!row || row.startsWith("#")) continue;
    const parts = row.split(",");
    if (parts.length < 3) continue;
    const start = parts[0].trim();
    const end = parts[1].trim();
    const cc = parts[2].trim().toUpperCase();
    if (!/^\d+$/.test(start) || !/^\d+$/.test(end) || !/^[A-Z]{2}$/.test(cc)) continue;
    ranges.push([start, end, cc]);
  }
  ranges.sort((a, b) => {
    const as = BigInt(a[0]);
    const bs = BigInt(b[0]);
    return as < bs ? -1 : as > bs ? 1 : 0;
  });
  return ranges;
}

function mergeV4Ranges(ranges) {
  if (ranges.length === 0) return [];
  const out = [];
  let [s, e, cc] = ranges[0];
  for (let i = 1; i < ranges.length; i++) {
    const [ns, ne, ncc] = ranges[i];
    if (ncc === cc && ns === e + 1) {
      e = ne;
      continue;
    }
    out.push([s, e, cc]);
    [s, e, cc] = [ns, ne, ncc];
  }
  out.push([s, e, cc]);
  return out;
}

function mergeV6Ranges(ranges) {
  if (ranges.length === 0) return [];
  const out = [];
  let [s, e, cc] = ranges[0];
  for (let i = 1; i < ranges.length; i++) {
    const [ns, ne, ncc] = ranges[i];
    if (ncc === cc && BigInt(ns) === BigInt(e) + 1n) {
      e = ne;
      continue;
    }
    out.push([s, e, cc]);
    [s, e, cc] = [ns, ne, ncc];
  }
  out.push([s, e, cc]);
  return out;
}

async function main() {
  const { inputV4, inputV6 } = parseArgs();

  let csvV4;
  if (inputV4) {
    csvV4 = fs.readFileSync(path.resolve(inputV4), "utf8");
  } else {
    console.log("Downloading DB-IP Lite IPv4 numeric ranges…");
    csvV4 = await downloadCsv(URL_V4);
  }

  let csvV6;
  if (inputV6) {
    csvV6 = fs.readFileSync(path.resolve(inputV6), "utf8");
  } else {
    console.log("Downloading DB-IP Lite IPv6 numeric ranges…");
    csvV6 = await downloadCsv(URL_V6);
  }

  const raw4 = parseV4Csv(csvV4);
  const merged4 = mergeV4Ranges(raw4);
  console.log(`IPv4: ${raw4.length} rows → ${merged4.length} ranges`);

  const raw6 = parseV6Csv(csvV6);
  const merged6 = mergeV6Ranges(raw6);
  console.log(`IPv6: ${raw6.length} rows → ${merged6.length} ranges`);

  const payload = {
    v: 3,
    source: "dbip-country-ipv4-num + dbip-country-ipv6-num (DB-IP Lite, CC BY 4.0)",
    ranges4: merged4,
    ranges6: merged6,
  };

  fs.mkdirSync(path.dirname(OUT), { recursive: true });
  fs.writeFileSync(OUT, JSON.stringify(payload));
  const bytes = fs.statSync(OUT).size;
  console.log(`Wrote ${OUT} (${(bytes / 1024 / 1024).toFixed(2)} MiB)`);

  if (!fs.existsSync(LICENSE_OUT)) {
    const lic = await fetch(
      "https://cdn.jsdelivr.net/npm/@ip-location-db/dbip-country/DBIP-LICENSE"
    );
    if (lic.ok) {
      fs.writeFileSync(LICENSE_OUT, await lic.text());
      console.log(`Wrote ${LICENSE_OUT}`);
    }
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
