import type { TimeBucket } from "@/lib/ai";

const BUCKET_MS = 100;
const MAX_BARS = 180;
const BAR_PX = 4;

export type BurstBar = {
  offsetMs: number;
  packets: number;
  bytes: number;
  /** Merged bucket width in ms (usually 100, larger when downsampled). */
  spanMs: number;
};

/** Sort, optionally densify, and downsample for a readable burst chart. */
export function prepareBurstSeries(
  buckets: TimeBucket[],
  durationSec?: number,
  maxBars = MAX_BARS
): BurstBar[] {
  if (!buckets.length) return [];

  const sorted = [...buckets]
    .map((b) => ({
      offsetMs: b.offset_ms ?? 0,
      packets: b.packets ?? 0,
      bytes: b.bytes ?? 0,
    }))
    .sort((a, b) => a.offsetMs - b.offsetMs);

  const lastOffset = sorted[sorted.length - 1].offsetMs;
  const spanMs =
    durationSec && durationSec > 0
      ? Math.ceil(durationSec * 1000)
      : lastOffset + BUCKET_MS;

  let series: BurstBar[] = sorted.map((b) => ({
    offsetMs: b.offsetMs,
    packets: b.packets,
    bytes: b.bytes,
    spanMs: BUCKET_MS,
  }));

  const slotCount = Math.max(1, Math.ceil(spanMs / BUCKET_MS));
  if (slotCount <= maxBars) {
    series = densify(series, lastOffset);
  }

  if (series.length > maxBars) {
    series = downsample(series, maxBars);
  }

  return series;
}

function densify(sorted: BurstBar[], lastOffset: number): BurstBar[] {
  const map = new Map(sorted.map((b) => [b.offsetMs, b]));
  const out: BurstBar[] = [];
  for (let ms = 0; ms <= lastOffset; ms += BUCKET_MS) {
    const hit = map.get(ms);
    out.push(
      hit ?? {
        offsetMs: ms,
        packets: 0,
        bytes: 0,
        spanMs: BUCKET_MS,
      }
    );
  }
  return out;
}

function downsample(series: BurstBar[], maxBars: number): BurstBar[] {
  const sorted = [...series].sort((a, b) => a.offsetMs - b.offsetMs);
  const start = sorted[0].offsetMs;
  const end = sorted[sorted.length - 1].offsetMs;
  const span = Math.max(BUCKET_MS, end - start + BUCKET_MS);
  const binMs = Math.max(BUCKET_MS, Math.ceil(span / maxBars / BUCKET_MS) * BUCKET_MS);
  const bins = new Map<number, BurstBar>();

  for (const b of sorted) {
    const idx = Math.floor((b.offsetMs - start) / binMs);
    const key = start + idx * binMs;
    const cur = bins.get(key);
    if (!cur) {
      bins.set(key, {
        offsetMs: key,
        packets: b.packets,
        bytes: b.bytes,
        spanMs: binMs,
      });
    } else {
      cur.packets += b.packets;
      cur.bytes += b.bytes;
    }
  }

  return [...bins.values()].sort((a, b) => a.offsetMs - b.offsetMs);
}

export function burstChartWidthPx(barCount: number): number {
  return Math.max(barCount * BAR_PX + barCount, 120);
}

export function formatBurstMbps(bytes: number, spanMs: number): string {
  if (spanMs <= 0 || bytes === 0) return "0 Mbps";
  const mbps = (bytes * 8) / (spanMs / 1000) / 1e6;
  if (mbps < 0.01) return "<0.01 Mbps";
  return `${mbps.toFixed(2)} Mbps`;
}

export { BAR_PX, BUCKET_MS };
