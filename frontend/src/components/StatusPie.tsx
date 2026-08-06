import { echarts, ReactEChartsCore } from "../lib/echarts";
import { statusLabel } from "../lib/utils";
import type { Stats } from "../api/types";

// rejected 灰色弱化
const statusColors: Record<string, string> = {
  new: "#6495b1",
  want: "#1a7f5a",
  proposed: "#b45309",
  ignored: "#9ca3af",
  rejected: "#d6d3d1",
};

export function StatusPie({ stats }: { stats: Stats }) {
  const counts = stats.status_counts ?? {};
  const data = Object.entries(counts).map(([k, v]) => ({
    name: statusLabel(k),
    value: v,
    itemStyle: { color: statusColors[k] ?? "#9ca3af" },
  }));

  const option: echarts.EChartsCoreOption = {
    tooltip: { trigger: "item" },
    legend: { bottom: 0, textStyle: { color: "#57534e", fontSize: 11 } },
    series: [
      {
        type: "pie",
        radius: ["45%", "70%"],
        center: ["50%", "45%"],
        avoidLabelOverlap: true,
        label: { show: false },
        data,
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
