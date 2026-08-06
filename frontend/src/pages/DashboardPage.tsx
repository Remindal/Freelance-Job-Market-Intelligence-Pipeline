import { useState } from "react";
import { RefreshCw, ChevronDown, ChevronUp } from "lucide-react";

import { useJobs, useRunNow, useStats } from "../api/hooks";
import { DEFAULT_FILTER, type JobsFilterInput } from "../api/types";
import { StatCards } from "../components/StatCards";
import { FetchProgress } from "../components/FetchProgress";
import { FilterBar } from "../components/FilterBar";
import { JobCard } from "../components/JobCard";
import { DailyNewChart } from "../components/DailyNewChart";
import { StatusPie } from "../components/StatusPie";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Skeleton } from "../components/ui/skeleton";

export default function DashboardPage() {
  const [filter, setFilter] = useState<JobsFilterInput>(DEFAULT_FILTER);
  const [chartsOpen, setChartsOpen] = useState(true);

  const jobs = useJobs(filter);
  const stats = useStats();
  const runNow = useRunNow();

  const total = jobs.data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / filter.page_size));

  return (
    <div className="mx-auto max-w-6xl px-4 pb-10">
      {/* 标题栏 */}
      <header className="flex items-center justify-between py-4">
        <div className="flex items-center gap-2.5">
          <h1 className="text-lg font-bold text-neutral-800">Scout</h1>
          <span className="flex items-center gap-1 text-xs text-primary">
            <span className="inline-block h-2 w-2 rounded-full bg-primary" />
            运行中
          </span>
        </div>
        <Button
          variant="outline"
          size="md"
          disabled={runNow.isPending}
          onClick={() => runNow.mutate()}
        >
          <RefreshCw
            className={runNow.isPending ? "h-4 w-4 animate-spin" : "h-4 w-4"}
          />
          {runNow.isPending ? "抓取中…" : "立即抓取"}
        </Button>
      </header>

      <FetchProgress />

      <StatCards stats={stats.data} loading={stats.isLoading} />

      {/* 图表区（可折叠） */}
      <div className="mt-3">
        <button
          type="button"
          className="mb-2 flex items-center gap-1 text-xs text-neutral-500 hover:text-neutral-700"
          onClick={() => setChartsOpen(!chartsOpen)}
        >
          {chartsOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
          图表
        </button>
        {chartsOpen && (
          <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
            <Card>
              <CardHeader>
                <CardTitle>每日新增（近 14 天）</CardTitle>
              </CardHeader>
              <CardContent>
                {stats.data ? (
                  <DailyNewChart stats={stats.data} />
                ) : (
                  <Skeleton className="h-[240px]" />
                )}
              </CardContent>
            </Card>
            <Card>
              <CardHeader>
                <CardTitle>状态占比</CardTitle>
              </CardHeader>
              <CardContent>
                {stats.data ? (
                  <StatusPie stats={stats.data} />
                ) : (
                  <Skeleton className="h-[240px]" />
                )}
              </CardContent>
            </Card>
          </div>
        )}
      </div>

      {/* 筛选 */}
      <div className="mt-4">
        <FilterBar filter={filter} onChange={setFilter} />
      </div>

      {/* 列表 */}
      <div className="mt-3 flex flex-col gap-2.5">
        {jobs.isLoading ? (
          <>
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
          </>
        ) : jobs.isError ? (
          <Card className="p-8 text-center text-sm text-danger">
            加载失败：{String(jobs.error)}
          </Card>
        ) : jobs.data && jobs.data.items.length > 0 ? (
          jobs.data.items.map((j) => <JobCard key={j.id} job={j} />)
        ) : (
          <Card className="p-10 text-center text-sm text-neutral-400">
            暂无数据，等待下一轮抓取（每 15 分钟）
          </Card>
        )}
      </div>

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="mt-4 flex items-center justify-center gap-3 text-sm text-neutral-600">
          <Button
            variant="outline"
            size="sm"
            disabled={filter.page <= 1}
            onClick={() => setFilter({ ...filter, page: filter.page - 1 })}
          >
            ← 上一页
          </Button>
          <span className="tnum">
            {filter.page} / {totalPages}（共 {total} 条）
          </span>
          <Button
            variant="outline"
            size="sm"
            disabled={filter.page >= totalPages}
            onClick={() => setFilter({ ...filter, page: filter.page + 1 })}
          >
            下一页 →
          </Button>
        </div>
      )}
    </div>
  );
}
