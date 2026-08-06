import { cn } from "../lib/utils";
import { Input } from "./ui/input";
import type { JobsFilterInput } from "../api/types";

const tabs: { value: string; label: string }[] = [
  { value: "", label: "全部" },
  { value: "new", label: "新单" },
  { value: "want", label: "想投" },
  { value: "proposed", label: "已投" },
  { value: "ignored", label: "忽略" },
];

export function FilterBar({
  filter,
  onChange,
}: {
  filter: JobsFilterInput;
  onChange: (f: JobsFilterInput) => void;
}) {
  const set = (patch: Partial<JobsFilterInput>) =>
    onChange({ ...filter, page: 1, ...patch });

  return (
    <div className="flex flex-wrap items-center gap-3">
      <div className="flex rounded-md border border-neutral-200 bg-white p-0.5">
        {tabs.map((t) => (
          <button
            key={t.value}
            type="button"
            onClick={() => set({ status: t.value })}
            className={cn(
              "rounded px-3 py-1 text-sm transition-colors",
              filter.status === t.value
                ? "bg-primary text-white"
                : "text-neutral-600 hover:bg-neutral-100",
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      <label className="flex items-center gap-1.5 text-sm text-neutral-600">
        分数≥
        <Input
          type="number"
          min={0}
          max={100}
          className="w-20 tnum"
          value={filter.min_score || ""}
          placeholder="0"
          onChange={(e) => set({ min_score: Number(e.target.value) || 0 })}
        />
      </label>

      <Input
        className="w-52"
        placeholder="🔍 搜索标题/描述"
        value={filter.keyword}
        onChange={(e) => set({ keyword: e.target.value })}
      />

      <Input
        className="w-36"
        placeholder="🏷 标签"
        value={filter.tag}
        onChange={(e) => set({ tag: e.target.value })}
      />

      <select
        className="h-9 rounded-md border border-neutral-300 bg-white px-2 text-sm text-neutral-700 outline-none focus:border-primary"
        value={filter.sort}
        onChange={(e) => set({ sort: e.target.value })}
      >
        <option value="score_desc">分数从高到低</option>
        <option value="score_asc">分数从低到高</option>
        <option value="fetched_desc">最新抓取</option>
      </select>
    </div>
  );
}
