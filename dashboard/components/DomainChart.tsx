"use client";

import { Group } from "@visx/group";
import { Bar } from "@visx/shape";
import { scaleLinear, scaleBand } from "@visx/scale";
import { AxisLeft, AxisBottom } from "@visx/axis";
import { GridColumns } from "@visx/grid";
import { ParentSize } from "@visx/responsive";
import { LinearGradient } from "@visx/gradient";
import { useTooltip, TooltipWithBounds, defaultStyles } from "@visx/tooltip";
import { theme } from "@/lib/theme";
import type { DomainSpending } from "@/lib/types";

const margin = { top: 10, right: 40, bottom: 30, left: 120 };

interface Props {
  data: DomainSpending[];
}

function Chart({
  data,
  width,
  height,
}: Props & { width: number; height: number }) {
  const {
    showTooltip,
    hideTooltip,
    tooltipData,
    tooltipLeft,
    tooltipTop,
    tooltipOpen,
  } = useTooltip<DomainSpending>();

  const xMax = width - margin.left - margin.right;
  const yMax = height - margin.top - margin.bottom;

  const xScale = scaleLinear<number>({
    domain: [0, Math.max(...data.map((d) => d.total_sat), 1)],
    range: [0, xMax],
    nice: true,
  });

  const yScale = scaleBand<string>({
    domain: data.map((d) => d.domain),
    range: [0, yMax],
    padding: 0.35,
  });

  return (
    <div style={{ position: "relative" }}>
      <svg width={width} height={height}>
        <LinearGradient id="domain-bar-gradient" from="#5D5FEF" to="#3B82F6" />
        <Group left={margin.left} top={margin.top}>
          <GridColumns
            scale={xScale}
            height={yMax}
            stroke="#384770"
            strokeOpacity={0.5}
            strokeDasharray="2,4"
            numTicks={5}
          />
          {data.map((d) => {
            const barWidth = xScale(d.total_sat);
            const barHeight = yScale.bandwidth();
            const barY = yScale(d.domain) ?? 0;
            return (
              <Bar
                key={d.domain}
                x={0}
                y={barY}
                width={barWidth}
                height={barHeight}
                fill="url(#domain-bar-gradient)"
                rx={3}
                onMouseMove={(e) => {
                  const svg = (e.target as SVGElement).ownerSVGElement;
                  if (!svg) return;
                  const point = svg.createSVGPoint();
                  point.x = e.clientX;
                  point.y = e.clientY;
                  const svgPoint = point.matrixTransform(
                    svg.getScreenCTM()?.inverse(),
                  );
                  showTooltip({
                    tooltipData: d,
                    tooltipLeft: svgPoint.x,
                    tooltipTop: svgPoint.y - 10,
                  });
                }}
                onMouseLeave={hideTooltip}
                style={{ cursor: "pointer" }}
              />
            );
          })}
          <AxisLeft
            scale={yScale}
            stroke="#384770"
            tickStroke="transparent"
            tickLabelProps={{
              fill: "#B9BDC5",
              fontSize: 12,
              fontFamily: "Open Sans, sans-serif",
              textAnchor: "end",
              dy: "0.33em",
            }}
            hideAxisLine
          />
          <AxisBottom
            scale={xScale}
            top={yMax}
            stroke="#384770"
            tickStroke="#384770"
            numTicks={5}
            tickLabelProps={{
              fill: "#848a99",
              fontSize: 11,
              fontFamily: "Open Sans, sans-serif",
              textAnchor: "middle",
            }}
            label="sats"
            labelProps={{
              fill: "#848a99",
              fontSize: 11,
              fontFamily: "Open Sans, sans-serif",
              textAnchor: "middle",
            }}
          />
        </Group>
      </svg>
      {tooltipOpen && tooltipData && (
        <TooltipWithBounds
          left={tooltipLeft}
          top={tooltipTop}
          style={{
            ...defaultStyles,
            backgroundColor: "rgba(0, 0, 0, 0.92)",
            border: `1px solid ${theme.colors.lightBlue}`,
            color: theme.colors.offWhite,
            fontFamily: theme.fonts.open,
            fontSize: 13,
            padding: "8px 12px",
            borderRadius: 6,
            boxShadow: "0 4px 20px rgba(0,0,0,0.8)",
            pointerEvents: "none",
            zIndex: 9999,
            whiteSpace: "nowrap",
          }}
        >
          <div style={{ marginBottom: 4 }}>
            <strong>{tooltipData.domain}</strong>
          </div>
          <span style={{ color: theme.colors.gold, fontWeight: 600 }}>
            {tooltipData.total_sat.toLocaleString()}
          </span>{" "}
          sats ({tooltipData.payment_count} payments)
        </TooltipWithBounds>
      )}
    </div>
  );
}

export default function DomainChart({ data }: Props) {
  if (!data.length) {
    return (
      <div
        style={{
          height: 200,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "#848a99",
          fontFamily: "Open Sans, sans-serif",
          fontSize: 14,
        }}
      >
        No spending data yet.
      </div>
    );
  }

  return (
    <div style={{ height: 200, position: "relative" }}>
      <ParentSize>
        {({ width, height }) =>
          width > 0 && height > 0 ? (
            <Chart data={data} width={width} height={height} />
          ) : null
        }
      </ParentSize>
    </div>
  );
}
