import { useNavigate } from "react-router-dom";
import { Card } from "./ui/card";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { ScoreBadge } from "./ScoreBadge";
import { StatusActions } from "./StatusActions";
import { statusLabel, timeAgo } from "../lib/utils";
import type { Job } from "../api/types";

const statusCls: Record<string, string> = {
  new: "bg-[#e3f2ea] text-primary",
  want: "bg-[#dcfce7] text-[#15803d]",
  proposed: "bg-[#fef3c7] text-warning",
  ignored: "bg-neutral-100 text-neutral-500",
  rejected: "bg-neutral-100 text-neutral-400",
  stale: "bg-[#f3ece7] text-[#8a7a6d]",
};

export function JobCard({ job }: { job: Job }) {
  const navigate = useNavigate();
  return (
    <Card className="p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <ScoreBadge score={job.score} />
            <h3 className="truncate text-sm font-semibold text-neutral-800">
              {job.title}
            </h3>
          </div>
          <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-neutral-500">
            {job.budget && <span className="tnum">💰 {job.budget}</span>}
            {job.skills && job.skills.length > 0 && (
              <span className="truncate">🏷 {job.skills.join(", ")}</span>
            )}
            <span>🕐 {timeAgo(String(job.fetched_at))}</span>
            <Badge className={statusCls[job.status] ?? statusCls.new}>
              {statusLabel(job.status)}
            </Badge>
          </div>
          {job.reason && (
            <p className="mt-2 line-clamp-2 text-xs leading-5 text-neutral-600">
              “{job.reason}”
            </p>
          )}
          {job.tags && job.tags.length > 0 && (
            <div className="mt-1.5 flex flex-wrap gap-1">
              {job.tags.map((t) => (
                <Badge key={t} className="bg-[#eef2f1] text-neutral-600">
                  {t}
                </Badge>
              ))}
            </div>
          )}
        </div>
      </div>
      <div className="mt-3 flex items-center gap-1.5">
        <Button size="sm" onClick={() => navigate(`/jobs/${job.id}`)}>
          查看详情
        </Button>
        <StatusActions id={job.id} current={job.status} />
      </div>
    </Card>
  );
}
