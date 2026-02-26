"use client";

import { useCallback } from "react";
import { AreaClosed, LinePath, Bar } from "@visx/shape";
import { scaleLinear, scaleTime } from "@visx/scale";
import { LinearGradient } from "@visx/gradient";
import { ParentSize } from "@visx/responsive";
import { curveMonotoneX } from "@visx/curve";
import { localPoint } from "@visx/event";
import { bisector } from "d3-array";
import { useTooltip, TooltipWithBounds, defaultStyles } from "@visx/tooltip";
import { theme } from "@/lib/theme";
import type { Event } from "@/lib/types";

interface Bucket {
  time: Date;
  sats: number;
}

function bucketEvents(events: Event[]): Bucket[] {
  if (!events.length) return [];

  const sorted = [...events]
    .filter((e) => e.status === "success")
    .sort(
      (a, b) =>
        new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
    );

  if (!sorted.length) return [];

  const earliest = new Date(sorted[0].created_at);
  const latest = new Date(sorted[sorted.length - 1].created_at);
  const rangeMs = latest.getTime() - earliest.getTime();

  // Pick bucket size based on range: 5min, 1hr, or 1day.
  let bucketMs: number;
  if (rangeMs < 3600000) {
    bucketMs = 300000; // 5 minutes
  } else if (rangeMs < 2 * 86400000) {
    bucketMs = 3600000; // 1 hour
  } else {
    bucketMs = 86400000; // 1 day
  }

  const buckets = new Map<number, number>();
  for (const evt of sorted) {
    const t = new Date(evt.created_at).getTime();
    const key = Math.floor(t / bucketMs) * bucketMs;
    buckets.set(key, (buckets.get(key) || 0) + evt.amount_sat);
  }

  const startKey = Math.floor(earliest.getTime() / bucketMs) * bucketMs;
  const endKey = Math.floor(latest.getTime() / bucketMs) * bucketMs;
  const result: Bucket[] = [];
  for (let k = startKey; k <= endKey; k += bucketMs) {
    result.push({ time: new Date(k), sats: buckets.get(k) || 0 });
  }

  // Ensure at least 2 points for the chart to render.
  if (result.length === 1) {
    result.unshift({
      time: new Date(result[0].time.getTime() - bucketMs),
      sats: 0,
    });
  }

  return result;
}

const bisectDate = bisector<Bucket, Date>((d) => d.time).left;

function formatCompact(value: number): string {
  if (value >= 1_000_000) {
    const v = value / 1_000_000;
    return v % 1 === 0 ? `${v}M` : `${v.toFixed(1)}M`;
  }
  if (value >= 1_000) {
    const v = value / 1_000;
    return v % 1 === 0 ? `${v}k` : `${v.toFixed(1)}k`;
  }
  return String(value);
}

const LEFT_PAD = 50;

function Chart({
  data,
  width,
  height,
}: {
  data: Bucket[];
  width: number;
  height: number;
}) {
  const {
    showTooltip,
    hideTooltip,
    tooltipData,
    tooltipLeft = 0,
    tooltipTop = 0,
    tooltipOpen,
  } = useTooltip<Bucket>();

  const padding = 2;
  const chartLeft = LEFT_PAD;
  const xScale = scaleTime({
    domain: [data[0].time, data[data.length - 1].time],
    range: [chartLeft + padding, width - padding],
  });

  const maxSats = Math.max(...data.map((d) => d.sats), 1);
  const yScale = scaleLinear<number>({
    domain: [0, maxSats],
    range: [height - padding, padding],
    nice: true,
  });

  const niceMax = yScale.domain()[1];

  const getX = (d: Bucket) => xScale(d.time);
  const getY = (d: Bucket) => yScale(d.sats);

  const handleTooltip = useCallback(
    (
      event:
        | React.TouchEvent<SVGRectElement>
        | React.MouseEvent<SVGRectElement>,
    ) => {
      const point = localPoint(event);
      if (!point) return;

      const x0 = xScale.invert(point.x);
      const idx = bisectDate(data, x0, 1);
      const d0 = data[idx - 1];
      const d1 = data[idx];
      let d = d0;
      if (d1 && d1.time) {
        d =
          x0.getTime() - d0.time.getTime() > d1.time.getTime() - x0.getTime()
            ? d1
            : d0;
      }

      showTooltip({
        tooltipData: d,
        tooltipLeft: xScale(d.time),
        tooltipTop: yScale(d.sats),
      });
    },
    [xScale, yScale, data, showTooltip],
  );

  if (data.length < 2) {
    return (
      <svg width={width} height={height}>
        <text
          x={width / 2}
          y={height / 2}
          textAnchor="middle"
          fill="#848a99"
          fontSize={13}
          fontFamily="Open Sans, sans-serif"
        >
          Not enough data for chart
        </text>
      </svg>
    );
  }

  return (
    <div style={{ position: "relative" }}>
      <svg width={width} height={height}>
        <LinearGradient
          id="spending-gradient"
          from="#3B82F6"
          to="#3B82F6"
          fromOpacity={0.25}
          toOpacity={0.0}
        />
        <text
          x={chartLeft - 6}
          y={yScale(niceMax)}
          textAnchor="end"
          dominantBaseline="hanging"
          fill={theme.colors.gray}
          fontSize={10}
          fontFamily={theme.fonts.open}
        >
          {formatCompact(niceMax)}
        </text>
        <text
          x={chartLeft - 6}
          y={height - padding}
          textAnchor="end"
          dominantBaseline="auto"
          fill={theme.colors.gray}
          fontSize={10}
          fontFamily={theme.fonts.open}
        >
          0
        </text>
        <AreaClosed
          data={data}
          x={getX}
          y={getY}
          yScale={yScale}
          fill="url(#spending-gradient)"
          curve={curveMonotoneX}
        />
        <LinePath
          data={data}
          x={getX}
          y={getY}
          stroke="#3B82F6"
          strokeWidth={1.5}
          strokeOpacity={0.8}
          curve={curveMonotoneX}
        />
        <Bar
          x={chartLeft}
          y={padding}
          width={width - chartLeft - padding}
          height={height - padding * 2}
          fill="transparent"
          onTouchStart={handleTooltip}
          onTouchMove={handleTooltip}
          onMouseMove={handleTooltip}
          onMouseLeave={hideTooltip}
        />
        {tooltipOpen && tooltipData && (
          <>
            <line
              x1={tooltipLeft}
              x2={tooltipLeft}
              y1={padding}
              y2={height - padding}
              stroke={theme.colors.lightBlue}
              strokeWidth={1}
              strokeDasharray="3,3"
              pointerEvents="none"
            />
            <circle
              cx={tooltipLeft}
              cy={tooltipTop}
              r={4}
              fill="#3B82F6"
              stroke={theme.colors.lightNavy}
              strokeWidth={2}
              pointerEvents="none"
            />
          </>
        )}
      </svg>
      {tooltipOpen && tooltipData && (
        <TooltipWithBounds
          top={tooltipTop - 8}
          left={tooltipLeft + 12}
          style={{
            ...defaultStyles,
            backgroundColor: "rgba(0, 0, 0, 0.92)",
            border: `1px solid ${theme.colors.lightBlue}`,
            borderRadius: 6,
            padding: "6px 10px",
            fontSize: 12,
            fontFamily: theme.fonts.open,
            lineHeight: 1.5,
            boxShadow: "0 4px 20px rgba(0,0,0,0.8)",
            pointerEvents: "none",
            zIndex: 9999,
            whiteSpace: "nowrap",
          }}
        >
          <div
            style={{ color: theme.colors.gray, fontSize: 11, marginBottom: 2 }}
          >
            {tooltipData.time.toLocaleDateString(undefined, {
              month: "short",
              day: "numeric",
              hour: "numeric",
              minute: "2-digit",
            })}
          </div>
          <div style={{ color: theme.colors.gold, fontWeight: 600 }}>
            {tooltipData.sats.toLocaleString()} sats
          </div>
        </TooltipWithBounds>
      )}
    </div>
  );
}

interface Props {
  events: Event[];
}

export default function SpendingChart({ events }: Props) {
  const data = bucketEvents(events);

  return (
    <div style={{ width: "100%", height: 100 }}>
      <ParentSize>
        {({ width, height }) =>
          width > 0 && height > 0 && data.length >= 2 ? (
            <Chart data={data} width={width} height={height} />
          ) : null
        }
      </ParentSize>
    </div>
  );
}
