"use client";

import { useCallback } from "react";
import { Bar } from "@visx/shape";
import { Group } from "@visx/group";
import { scaleLinear, scaleBand } from "@visx/scale";
import { AxisBottom, AxisLeft } from "@visx/axis";
import { GridRows } from "@visx/grid";
import { ParentSize } from "@visx/responsive";
import { localPoint } from "@visx/event";
import { useTooltip, TooltipWithBounds, defaultStyles } from "@visx/tooltip";
import { theme } from "@/lib/theme";
import type { Event } from "@/lib/types";

interface Bucket {
  label: string;
  count: number;
}

function bucketEvents(events: Event[]): Bucket[] {
  if (!events.length) return [];

  const sorted = [...events].sort(
    (a, b) =>
      new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  );

  const earliest = new Date(sorted[0].created_at);
  const latest = new Date(sorted[sorted.length - 1].created_at);
  const rangeMs = latest.getTime() - earliest.getTime();

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
    buckets.set(key, (buckets.get(key) || 0) + 1);
  }

  const startKey = Math.floor(earliest.getTime() / bucketMs) * bucketMs;
  const endKey = Math.floor(latest.getTime() / bucketMs) * bucketMs;
  const result: Bucket[] = [];
  for (let k = startKey; k <= endKey; k += bucketMs) {
    const d = new Date(k);
    const label = d.toISOString();
    result.push({ label, count: buckets.get(k) || 0 });
  }

  if (result.length === 1) {
    result.unshift({
      label: new Date(result[0].label).getTime() - bucketMs
        ? new Date(
            new Date(result[0].label).getTime() - bucketMs,
          ).toISOString()
        : result[0].label,
      count: 0,
    });
  }

  return result;
}

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

function formatLabel(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
  });
}

const margin = { top: 8, right: 12, bottom: 32, left: 40 };

const tooltipStyles = {
  ...defaultStyles,
  backgroundColor: "rgba(0, 0, 0, 0.92)",
  border: `1px solid ${theme.colors.lightBlue}`,
  borderRadius: 6,
  padding: "6px 10px",
  fontSize: 12,
  fontFamily: theme.fonts.open,
  lineHeight: 1.5,
  boxShadow: "0 4px 20px rgba(0,0,0,0.8)",
  color: theme.colors.offWhite,
};

function Chart({
  data,
  width,
  height,
  color,
}: {
  data: Bucket[];
  width: number;
  height: number;
  color: string;
}) {
  const {
    showTooltip,
    hideTooltip,
    tooltipData,
    tooltipLeft = 0,
    tooltipTop = 0,
    tooltipOpen,
  } = useTooltip<Bucket>();

  const xMax = width - margin.left - margin.right;
  const yMax = height - margin.top - margin.bottom;

  const xScale = scaleBand<string>({
    domain: data.map((d) => d.label),
    range: [0, xMax],
    padding: 0.25,
  });

  const maxCount = Math.max(...data.map((d) => d.count), 1);

  const yScale = scaleLinear<number>({
    domain: [0, maxCount],
    range: [yMax, 0],
    nice: true,
  });

  const handleTooltip = useCallback(
    (
      event:
        | React.TouchEvent<SVGRectElement>
        | React.MouseEvent<SVGRectElement>,
    ) => {
      const point = localPoint(event);
      if (!point) return;

      const x = point.x - margin.left;
      const step = xScale.step();
      const idx = Math.min(
        Math.max(0, Math.floor(x / step)),
        data.length - 1,
      );
      const d = data[idx];
      if (!d) return;

      const barCenterX =
        (xScale(d.label) ?? 0) + xScale.bandwidth() / 2 + margin.left;

      showTooltip({
        tooltipData: d,
        tooltipLeft: barCenterX,
        tooltipTop: yScale(d.count) + margin.top,
      });
    },
    [xScale, yScale, data, showTooltip],
  );

  const tickValues = (() => {
    const len = data.length;
    if (len <= 6) return data.map((d) => d.label);
    const maxTicks = Math.max(3, Math.min(6, Math.floor(xMax / 70)));
    const step = Math.ceil(len / maxTicks);
    const values: string[] = [];
    for (let i = 0; i < len; i += step) {
      values.push(data[i].label);
    }
    if (values[values.length - 1] !== data[len - 1].label) {
      values.push(data[len - 1].label);
    }
    return values;
  })();

  return (
    <div style={{ position: "relative" }}>
      <svg width={width} height={height}>
        <Group left={margin.left} top={margin.top}>
          <GridRows
            scale={yScale}
            width={xMax}
            stroke={theme.colors.blue}
            strokeOpacity={0.5}
            strokeDasharray="2,4"
            numTicks={4}
          />
          {data.map((d) => {
            const barWidth = xScale.bandwidth();
            const barHeight = yMax - (yScale(d.count) ?? 0);
            const barX = xScale(d.label) ?? 0;
            const barY = yMax - barHeight;
            const isHovered = tooltipOpen && tooltipData?.label === d.label;
            return (
              <Bar
                key={d.label}
                x={barX}
                y={barY}
                width={barWidth}
                height={Math.max(barHeight, d.count > 0 ? 2 : 0)}
                fill={color}
                rx={2}
                opacity={isHovered ? 1 : 0.85}
              />
            );
          })}
          <Bar
            x={0}
            y={0}
            width={xMax}
            height={yMax}
            fill="transparent"
            onTouchStart={handleTooltip}
            onTouchMove={handleTooltip}
            onMouseMove={handleTooltip}
            onMouseLeave={hideTooltip}
            style={{ cursor: "pointer" }}
          />
          <AxisLeft
            scale={yScale}
            stroke={theme.colors.blue}
            tickStroke="transparent"
            numTicks={4}
            tickFormat={(v) => formatCompact(v as number)}
            tickLabelProps={{
              fill: theme.colors.gray,
              fontSize: 10,
              fontFamily: theme.fonts.open,
              textAnchor: "end",
              dy: "0.33em",
            }}
            hideAxisLine
          />
          <AxisBottom
            scale={xScale}
            top={yMax}
            stroke={theme.colors.blue}
            tickStroke="transparent"
            tickValues={tickValues}
            tickFormat={(v) => formatLabel(v as string)}
            tickLabelProps={{
              fill: theme.colors.gray,
              fontSize: 10,
              fontFamily: theme.fonts.open,
              textAnchor: "middle",
            }}
          />
        </Group>
        {tooltipOpen && tooltipData && (
          <>
            <line
              x1={tooltipLeft}
              x2={tooltipLeft}
              y1={margin.top}
              y2={height - margin.bottom}
              stroke={theme.colors.lightBlue}
              strokeWidth={1}
              strokeDasharray="3,3"
              pointerEvents="none"
            />
            <circle
              cx={tooltipLeft}
              cy={tooltipTop}
              r={3}
              fill={color}
              stroke={theme.colors.lightNavy}
              strokeWidth={2}
              pointerEvents="none"
            />
          </>
        )}
      </svg>
      {tooltipOpen && tooltipData && (
        <TooltipWithBounds
          left={tooltipLeft}
          top={tooltipTop - 12}
          style={tooltipStyles}
        >
          <div
            style={{ color: theme.colors.gray, fontSize: 11, marginBottom: 2 }}
          >
            {formatLabel(tooltipData.label)}
          </div>
          <div style={{ fontWeight: 600 }}>
            {tooltipData.count.toLocaleString()} payments
          </div>
        </TooltipWithBounds>
      )}
    </div>
  );
}

interface Props {
  events: Event[];
  height?: number;
  color?: string;
}

export default function PaymentVolumeChart({
  events,
  height = 200,
  color = theme.colors.purple,
}: Props) {
  const data = bucketEvents(events);

  if (!data.length) {
    return (
      <div
        style={{
          height,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: theme.colors.gray,
          fontFamily: theme.fonts.open,
          fontSize: 14,
        }}
      >
        No payment data yet.
      </div>
    );
  }

  return (
    <div style={{ height }}>
      <ParentSize>
        {({ width, height: h }) =>
          width > 0 && h > 0 ? (
            <Chart data={data} width={width} height={h} color={color} />
          ) : null
        }
      </ParentSize>
    </div>
  );
}
