import { echarts, ReactEChartsCore } from "../lib/echarts";
import type { Stats } from "../api/types";

export function DailyNewChart({ stats }: { stats: Stats }) {
  const option: echarts.EChartsCoreOption = {
    tooltip: { trigger: "axis" },
    grid: { left: 40, right: 16, top: 24, bottom: 28 },
    xAxis: {
      type: "category",
      data: (stats.daily_new ?? []).map((d) => d.date.slice(5)),
      axisLabel: { color: "#78716c", fontSize: 11 },
    },
    yAxis: {
      type: "value",
      minInterval: 1,
      axisLabel: { color: "#78716c", fontSize: 11 },
      splitLine: { lineStyle: { color: "#f0efed" } },
    },
    series: [
      {
        name: "每日新增",
        type: "line",
        smooth: true,
        symbolSize: 5,
        data: (stats.daily_new ?? []).map((d) => d.count),
        lineStyle: { color: "#1a7f5a", width: 2 },
        itemStyle: { color: "#1a7f5a" },
        areaStyle: { color: "rgba(26,127,90,0.12)" },
      },
    ],
  };
  return (
    <ReactEChartsCore
      echarts={echarts}
      option={option}
      style={{ height: 240 }}
      notMerge
    />
  );
}
