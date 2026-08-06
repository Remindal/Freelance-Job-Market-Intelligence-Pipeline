// Package desktop Wails 绑定层：前端通过自动生成的 TS 绑定直接调用这些方法，
// 前后端之间无 HTTP。App 只做参数校验与转发，业务全在 store / pipeline。
package desktop

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"upwork-scout/internal/domain"
	"upwork-scout/internal/pipeline"
	"upwork-scout/internal/scheduler"
	"upwork-scout/internal/store"
)

// 数据契约：与前端 src/api/types.ts 一一对应。

type ListFilter struct {
	Status   string `json:"status"` // "" = 全部
	MinScore int    `json:"min_score"`
	Keyword  string `json:"keyword"`
	Tag      string `json:"tag"` // 按 LLM 标签过滤
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Sort     string `json:"sort"` // 白名单: score_desc | score_asc | fetched_desc
}

type JobListResult struct {
	Items []domain.Job `json:"items"`
	Total int          `json:"total"`
}

type Dimension struct {
	Score    int    `json:"score"`
	Analysis string `json:"analysis"`
}

type AnalysisReport struct {
	Overall    int                  `json:"overall"`
	Verdict    string               `json:"verdict"` // 强烈推荐|可投|观望|不推荐
	Dimensions map[string]Dimension `json:"dimensions"`
	PitchAngle string               `json:"pitch_angle"`
	Risks      []string             `json:"risks"`
}

type JobDetail struct {
	domain.Job
	Analysis *AnalysisReport `json:"analysis"` // 可空，前端空态处理
}

type App struct {
	ctx           context.Context // Wails 运行时上下文，Startup 注入
	store         store.Store
	pipeline      *pipeline.Pipeline
	scheduler     *scheduler.Scheduler
	highScoreFrom int // 「高分待决策」阈值，取 notify.threshold
	logger        *slog.Logger
}

func NewApp(st store.Store, pl *pipeline.Pipeline, sched *scheduler.Scheduler, highScoreFrom int, logger *slog.Logger) *App {
	return &App{
		store:         st,
		pipeline:      pl,
		scheduler:     sched,
		highScoreFrom: highScoreFrom,
		logger:        logger,
	}
}

// Context 暴露 Wails 运行时上下文（托盘菜单动作用）。
func (a *App) Context() context.Context { return a.ctx }

// Startup Wails 启动钩子：保存运行时上下文，后台启动调度器，并把 pipeline 轮次回调接到前端事件。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.pipeline.SetOnChange(func() {
		runtime.EventsEmit(ctx, "jobs:changed")
	})
	a.pipeline.SetOnProgress(func(prog pipeline.Progress) {
		runtime.EventsEmit(ctx, "jobs:progress", prog)
	})
	go func() {
		if err := a.scheduler.Start(ctx); err != nil {
			a.logger.Error("scheduler start failed", "err", err)
		}
	}()
}

func (a *App) Shutdown(ctx context.Context) {
	<-a.scheduler.Stop().Done()
}

var validSorts = map[string]bool{
	"":             true,
	"score_desc":   true,
	"score_asc":    true,
	"fetched_desc": true,
}

func (a *App) ListJobs(filter ListFilter) (JobListResult, error) {
	if !validSorts[filter.Sort] {
		return JobListResult{}, fmt.Errorf("非法排序字段: %q", filter.Sort)
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 20
	}

	jobs, total, err := a.store.List(a.ctx, store.ListFilter{
		Status:   domain.Status(filter.Status),
		MinScore: filter.MinScore,
		Keyword:  filter.Keyword,
		Tag:      filter.Tag,
		Sort:     filter.Sort,
		Limit:    filter.PageSize,
		Offset:   (filter.Page - 1) * filter.PageSize,
	})
	if err != nil {
		return JobListResult{}, err
	}
	if jobs == nil {
		jobs = []domain.Job{}
	}
	return JobListResult{Items: jobs, Total: total}, nil
}

func (a *App) GetJob(id int64) (JobDetail, error) {
	j, err := a.store.Get(a.ctx, id)
	if err != nil {
		return JobDetail{}, err
	}
	// 五维分析报告生成器尚未实现（M7），契约先留口，前端做空态
	return JobDetail{Job: *j, Analysis: nil}, nil
}

func (a *App) UpdateStatus(id int64, status string) (domain.Job, error) {
	st := domain.Status(status)
	if !st.Valid() {
		return domain.Job{}, fmt.Errorf("非法状态: %q", status)
	}
	if err := a.store.UpdateStatus(a.ctx, id, st); err != nil {
		return domain.Job{}, err
	}
	j, err := a.store.Get(a.ctx, id)
	if err != nil {
		return domain.Job{}, err
	}
	// 事件驱动刷新：前端监听 jobs:changed 后自动 refetch
	runtime.EventsEmit(a.ctx, "jobs:changed")
	return *j, nil
}

func (a *App) GetStats() (store.Stats, error) {
	stats, err := a.store.Stats(a.ctx, a.highScoreFrom)
	if err != nil {
		return store.Stats{}, err
	}
	return *stats, nil
}

// RunResult 「立即抓取」的三态反馈：成功N条 / 成功0条 / 失败+原因。
type RunResult struct {
	Success bool   `json:"success"`
	Fetched int    `json:"fetched"`
	New     int    `json:"new"`
	Message string `json:"message"`
}

// RunNow 托盘/按钮「立即抓取」，与 cron 共用 pipeline 内同一把防重入锁。
func (a *App) RunNow() (RunResult, error) {
	res, err := a.pipeline.RunOnce(a.ctx)
	if errors.Is(err, pipeline.ErrQueued) {
		return RunResult{Success: true, Message: "本轮抓取进行中，已排队，结束后自动再抓一轮"}, nil
	}
	if errors.Is(err, pipeline.ErrRoundInProgress) {
		return RunResult{}, fmt.Errorf("上一轮抓取仍在进行中，且已有排队请求")
	}
	out := RunResult{Fetched: res.Fetched, New: res.New}
	switch {
	case err != nil && res.Fetched == 0:
		out.Success = false
		out.Message = "抓取失败: " + err.Error()
	case err != nil:
		out.Success = true
		out.Message = fmt.Sprintf("部分源失败，本轮新单 %d 条", res.New)
	case res.New > 0:
		out.Success = true
		out.Message = fmt.Sprintf("抓取成功：%d 条新单", res.New)
	default:
		out.Success = true
		out.Message = "抓取完成：无新单"
	}
	return out, nil
}

// ProgressView 当前进度快照：Active 表示是否有轮次在执行。
type ProgressView struct {
	Active bool `json:"active"`
	pipeline.Progress
}

// GetProgress 供前端挂载/刷新时拉取当前进度（事件错过可补）。
func (a *App) GetProgress() (ProgressView, error) {
	return ProgressView{Active: a.pipeline.IsRunning(), Progress: a.pipeline.LastProgress()}, nil
}

// CancelFetch 取消进行中的抓取（下一个检查点生效）。
func (a *App) CancelFetch() (string, error) {
	if !a.pipeline.CancelRound() {
		return "当前没有进行中的抓取", nil
	}
	return "已请求取消，当前步骤结束后停止", nil
}

// LogInfo 供托盘菜单动作记录日志。
func (a *App) LogInfo(msg string) { a.logger.Info(msg) }

// OpenInBrowser 用系统默认浏览器打开外链（桌面窗口内不允许开外链）。
func (a *App) OpenInBrowser(url string) error {
	if url == "" {
		return errors.New("empty url")
	}
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}
