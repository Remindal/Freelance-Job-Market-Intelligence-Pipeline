import { echarts, ReactEChartsCore } from "../lib/echarts";
import type { AnalysisReport } from "../api/types";

export function DimensionRadar({ analysis }: { analysis: AnalysisReport }) {
  const dims = analysis.dimensions ?? {};
  const names = Object.keys(dims);

  const option: echarts.EChartsCoreOption = {
    radar: {
      indicator: names.map((n) => ({ name: n, max: 100 })),
      radius: "65%",
      axisName: { color: "#57534e", fontSize: 11 },
      splitLine: { lineStyle: { color: "#e7e5e4" } },
      splitArea: { areaStyle: { color: ["#ffffff", "#faf9f7"] } },
    },
    series: [
      {
        type: "radar",
        data: [
          {
            value: names.map((n) => dims[n].score),
            name: "维度评分",
            lineStyle: { color: "#1a7f5a" },
            itemStyle: { color: "#1a7f5a" },
            areaStyle: { color: "rgba(26,127,90,0.15)" },
          },
        ],
      },
    ],
  };
  return (
    <ReactEChartsCore
      echarts={echarts}
      option={option}
      style={{ height: 260 }}
      notMerge
    />
  );
}
