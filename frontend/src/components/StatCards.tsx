import { Card, CardContent, CardHeader, CardTitle } from "./ui/card";
import { Skeleton } from "./ui/skeleton";
import type { Stats } from "../api/types";

const items: { key: keyof Stats; label: string }[] = [
  { key: "today_new", label: "今日新单" },
  { key: "high_score_pending", label: "高分待决策" },
  { key: "want_count", label: "想投" },
  { key: "proposed_count", label: "已投" },
];

export function StatCards({ stats, loading }: { stats?: Stats; loading: boolean }) {
  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      {items.map((it) => (
        <Card key={it.key}>
          <CardHeader>
            <CardTitle>{it.label}</CardTitle>
          </CardHeader>
          <CardContent>
            {loading || !stats ? (
              <Skeleton className="h-8 w-16" />
            ) : (
              <div className="tnum text-2xl font-bold text-neutral-800">
                {stats[it.key] as number}
              </div>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
