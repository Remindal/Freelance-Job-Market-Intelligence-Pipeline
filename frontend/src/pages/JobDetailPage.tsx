import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, ExternalLink } from "lucide-react";

import { useJob } from "../api/hooks";
import { OpenInBrowser } from "../wailsjs/go/desktop/App";
import { ScoreBadge } from "../components/ScoreBadge";
import { VerdictTag } from "../components/VerdictTag";
import { DimensionRadar } from "../components/DimensionRadar";
import { StatusActions } from "../components/StatusActions";
import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import { Skeleton } from "../components/ui/skeleton";
import { formatDateTime, statusLabel, PROPOSALS_LABELS } from "../lib/utils";

export default function JobDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const jobId = Number(id) || 0;
  const { data, isLoading, isError, error } = useJob(jobId);

  if (isLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-6">
        <Skeleton className="mb-3 h-8 w-40" />
        <Skeleton className="h-96" />
      </div>
    );
  }
  if (isError || !data) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-6">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="h-4 w-4" /> 返回
        </Button>
        <Card className="mt-4 p-10 text-center text-sm text-danger">
          加载失败：{String(error)}
        </Card>
      </div>
    );
  }

  const analysis = data.analysis;

  return (
    <div className="mx-auto max-w-6xl px-4 pb-10">
      <div className="py-4">
        <Button variant="ghost" size="sm" onClick={() => navigate(-1)}>
          <ArrowLeft className="h-4 w-4" /> 返回
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-5">
        {/* 左侧 60%：单子原文 */}
        <div className="lg:col-span-3">
          <Card>
            <CardHeader>
              <h1 className="text-lg font-bold leading-7 text-neutral-800">
                {data.title}
              </h1>
              <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-neutral-500">
                {data.budget && <span className="tnum">💰 {data.budget}</span>}
                <span>抓取：{formatDateTime(String(data.fetched_at))}</span>
                <Badge className="bg-neutral-100 text-neutral-500">
                  {statusLabel(data.status)}
                </Badge>
              </div>
              {(data.payment_verified != null ||
                data.client_spent_usd != null ||
                data.client_rating != null ||
                data.proposals_bucket) && (
                <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-neutral-500">
                  {data.payment_verified != null && (
                    <span>{data.payment_verified ? "✅ 支付已验证" : "⚠️ 支付未验证"}</span>
                  )}
                  {data.client_spent_usd != null && (
                    <span className="tnum">
                      客户花费 ${Math.round(data.client_spent_usd).toLocaleString()}
                    </span>
                  )}
                  {data.client_rating != null && (
                    <span className="tnum">评分 {data.client_rating.toFixed(1)}</span>
                  )}
                  {data.proposals_bucket && (
                    <span>{PROPOSALS_LABELS[data.proposals_bucket] ?? data.proposals_bucket}</span>
                  )}
                </div>
              )}
            </CardHeader>
            <CardContent>
              {data.skills && data.skills.length > 0 && (
                <div className="mb-3 flex flex-wrap gap-1.5">
                  {data.skills.map((s) => (
                    <Badge key={s} className="bg-primary-light text-primary">
                      {s}
                    </Badge>
                  ))}
                </div>
              )}
              <Button
                variant="outline"
                size="sm"
                className="mb-4"
                onClick={() => OpenInBrowser(data.url)}
              >
                <ExternalLink className="h-3.5 w-3.5" /> 打开原帖
              </Button>
              <div className="whitespace-pre-wrap text-sm leading-7 text-neutral-700">
                {data.description}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* 右侧 40%：AI 分析 */}
        <div className="flex flex-col gap-3 lg:col-span-2">
          <Card>
            <CardHeader>
              <CardTitle>AI 评估</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="flex items-center gap-3">
                <span className="tnum text-4xl font-bold text-primary">
                  {analysis ? analysis.overall : data.score}
                </span>
                {analysis ? (
                  <VerdictTag verdict={analysis.verdict} />
                ) : (
                  <ScoreBadge score={data.score} />
                )}
              </div>
              {data.reason && (
                <p className="mt-3 rounded-md border-l-2 border-primary/40 bg-primary-light/50 px-3 py-2 text-xs leading-6 text-neutral-700">
                  {data.reason}
                </p>
              )}
            </CardContent>
          </Card>

          {analysis ? (
            <>
              <Card>
                <CardHeader>
                  <CardTitle>五维评分</CardTitle>
                </CardHeader>
                <CardContent>
                  <DimensionRadar analysis={analysis} />
                  <div className="mt-2 flex flex-col gap-2">
                    {Object.entries(analysis.dimensions ?? {}).map(
                      ([name, dim]) => (
                        <details key={name} className="text-xs">
                          <summary className="cursor-pointer text-neutral-700">
                            <span className="tnum font-semibold">{dim.score}</span>
                            {" · "}
                            {name}
                          </summary>
                          <p className="mt-1 pl-4 leading-5 text-neutral-500">
                            {dim.analysis}
                          </p>
                        </details>
                      ),
                    )}
                  </div>
                </CardContent>
              </Card>

              {analysis.pitch_angle && (
                <Card className="border-primary/30 bg-primary-light/40">
                  <CardHeader>
                    <CardTitle className="text-primary">切入角度</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <p className="text-xs leading-6 text-neutral-700">
                      {analysis.pitch_angle}
                    </p>
                  </CardContent>
                </Card>
              )}

              {analysis.risks && analysis.risks.length > 0 && (
                <Card className="border-danger/20">
                  <CardHeader>
                    <CardTitle className="text-danger">风险点</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ul className="list-disc pl-4 text-xs leading-6 text-danger/90">
                      {analysis.risks.map((r, i) => (
                        <li key={i}>{r}</li>
                      ))}
                    </ul>
                  </CardContent>
                </Card>
              )}
            </>
          ) : (
            <Card className="p-6 text-center text-xs text-neutral-400">
              深度分析报告暂未生成（五维评估将在后续版本上线）
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>标记状态</CardTitle>
            </CardHeader>
            <CardContent>
              <StatusActions id={data.id} current={data.status} size="md" />
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
