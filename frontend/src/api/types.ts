// 与 internal/desktop/app.go 数据契约一一对应，底层类型来自 wails 生成物
import { desktop, domain, store } from "../wailsjs/go/models";

export type Job = domain.Job;
export type JobDetail = desktop.JobDetail;
export type JobListResult = desktop.JobListResult;
export type Stats = store.Stats;
export type AnalysisReport = desktop.AnalysisReport;

export interface JobsFilterInput {
  status: string;
  min_score: number;
  keyword: string;
  tag: string;
  page: number;
  page_size: number;
  sort: string;
}

export const DEFAULT_FILTER: JobsFilterInput = {
  status: "",
  min_score: 0,
  keyword: "",
  tag: "",
  page: 1,
  page_size: 20,
  sort: "score_desc",
};
