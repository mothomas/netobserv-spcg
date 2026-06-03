"use client";

import { useMemo, useState } from "react";
import type { TimeBucket } from "@/lib/ai";
import {
  BAR_PX,
  burstChartWidthPx,
  formatBurstMbps,
  prepareBurstSeries,
  type BurstBar,
} from "@/lib/burstChart";

type Props = {
  buckets: TimeBucket[];
  durationSec?: number;
  peakBucketPackets?: number;
};

const CHART_HEIGHT_PX = 112;

export function TrafficBurstChart({ buckets, durationSec, peakBucketPackets }: Props) {
  const [hover, setHover] = useState<BurstBar | null>(null);

  const series = useMemo(
    () => prepareBurstSeries(buckets, durationSec),
    [buckets, durationSec]
  );

  const maxPackets = useMemo(
    () => Math.max(1, ...series.map((b) => b.packets), peakBucketPackets ?? 0),
    [series, peakBucketPackets]
  );

  const totalPackets = useMemo(() => series.reduce((n, b) => n + b.packets, 0), [series]);

  if (!series.length || totalPackets === 0) {
    return (
      <div className="h-28 flex items-center justify-center text-xs text-siem-muted border border-dashed border-siem-border rounded-md">
        No packet bursts in this capture window yet
      </div>
    );
  }

  const widthPx = burstChartWidthPx(series.length);
  const binLabel =
    series.length > 0 && series[0].spanMs > 100
      ? `${series[0].spanMs}ms buckets`
      : "100ms buckets";

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap justify-between items-baseline gap-2">
        <span className="siem-label">Traffic burst ({binLabel})</span>
        <div className="text-[10px] text-siem-muted font-mono text-right">
          {durationSec != null && durationSec > 0 && <span>{durationSec.toFixed(1)}s · </span>}
          {hover ? (
            <span className="text-siem-text">
              {hover.offsetMs}ms · {hover.packets} pkts · {formatBurstMbps(hover.bytes, hover.spanMs)}
            </span>
          ) : (
            <span>peak {maxPackets} pkts/bin</span>
          )}
        </div>
      </div>

      <div className="burst-chart-track">
        <div
          className="flex items-end gap-px px-1 py-2"
          style={{ width: widthPx, height: CHART_HEIGHT_PX }}
          role="img"
          aria-label="Traffic burst histogram"
        >
          {series.map((b) => {
            const barPx =
              b.packets > 0
                ? Math.max(2, Math.round((b.packets / maxPackets) * (CHART_HEIGHT_PX - 8)))
                : 0;
            const isPeak = b.packets > 0 && b.packets >= maxPackets;
            const barClass = isPeak
              ? "burst-bar burst-bar-peak"
              : b.packets > 0
                ? "burst-bar"
                : "burst-bar burst-bar-empty";
            return (
              <div
                key={`${b.offsetMs}-${b.spanMs}`}
                className="flex flex-col justify-end shrink-0"
                style={{ width: BAR_PX, height: CHART_HEIGHT_PX - 8 }}
                onMouseEnter={() => setHover(b)}
                onMouseLeave={() => setHover(null)}
              >
                <div
                  className={barClass}
                  style={{ height: barPx }}
                  title={`${b.offsetMs}ms (+${b.spanMs}ms): ${b.packets} packets`}
                />
              </div>
            );
          })}
        </div>
      </div>

      <div className="flex justify-between text-[10px] text-siem-muted font-mono px-1">
        <span>0s</span>
        <span>
          {durationSec && durationSec > 0
            ? `${durationSec.toFixed(1)}s`
            : `${((series[series.length - 1].offsetMs + series[series.length - 1].spanMs) / 1000).toFixed(1)}s`}
        </span>
      </div>
    </div>
  );
}
