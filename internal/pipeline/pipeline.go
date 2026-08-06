package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"upwork-scout/internal/domain"
	"upwork-scout/internal/fetcher"
	"upwork-scout/internal/filter"
	"upwork-scout/internal/notify"
	"upwork-scout/internal/store"
)

// ErrRoundInProgress 上一轮尚未结束且已有排队请求时返回此错误。
var ErrRoundInProgress = errors.New("上一轮进行中")

// ErrQueued 上一轮进行中，本次请求已排队（本轮结束后自动再跑一轮）。
var ErrQueued = errors.New("已排队")

// RoundResult 一轮流水线的统计结果，供 UI 反馈。
type RoundResult struct {
	Fetched  int `json:"fetched"`
	New      int `json:"new"`
	Dup      int `json:"dup"`
	Rejected int `json:"rejected"`
}

// Progress 流水线进度事件，前端订阅后可视化。
type Progress struct {
	Stage      string `json:"stage"`       // fetch_start | feed_done | score | done
	Feed       string `json:"feed"`        // feed_done 时的源名
	FeedIndex  int    `json:"feed_index"`  // 当前第几个源（从 1 计）
	FeedTotal  int    `json:"feed_total"`  // 源总数
	FeedJobs   int    `json:"feed_jobs"`   // 该源抓到的条数
	ScoreDone  int    `json:"score_done"`  // score 阶段已完成数
	ScoreTotal int    `json:"score_total"` // score 阶段总数
	New        int    `json:"new"`         // done 时的新单数
	Fetched    int    `json:"fetched"`     // done 时的抓取总数
}

// Pipeline 唯一编排者：fetch → dedupe → 粗筛 → 精筛打分 → 入库 → 通知。
type Pipeline struct {
	fetcher      fetcher.Fetcher
	store        store.Store
	rules        *filter.Rules
	clientFilter *filter.ClientFilter
	scorer       *filter.Scorer // 可为 nil（未配置 LLM 时跳过分）
	notifiers    []notify.Notifier
	threshold    int
	logger       *slog.Logger

	mu         sync.Mutex   // 防重入：cron 与手动触发共用同一把锁
	queued     atomic.Bool  // 单深度排队位：抓取中再来请求则排队
	onChange   func()       // 每轮结束后的回调（桌面端用于推送刷新事件）
	onProgress func(Progress) // 进度事件回调
	last       atomic.Value // 最近一次 Progress，供晚挂载的前端拉取当前状态
	cancel     atomic.Value // 当前轮次的 cancel 函数（取消抓取用）
	lastDetail atomic.Value // 上次详情页请求时间（限速 ≥5s/次）
}

func New(
	f fetcher.Fetcher,
	st store.Store,
	rules *filter.Rules,
	clientFilter *filter.ClientFilter,
	scorer *filter.Scorer,
	notifiers []notify.Notifier,
	threshold int,
	logger *slog.Logger,
) *Pipeline {
	return &Pipeline{
		fetcher:      f,
		store:        st,
		rules:        rules,
		clientFilter: clientFilter,
		scorer:       scorer,
		notifiers:    notifiers,
		threshold:    threshold,
		logger:       logger,
	}
}

// SetOnChange 注册轮次结束回调。
func (p *Pipeline) SetOnChange(fn func()) { p.onChange = fn }

// SetOnProgress 注册进度事件回调，并接线 fetcher 的逐源进度（若支持）。
func (p *Pipeline) SetOnProgress(fn func(Progress)) {
	p.onProgress = fn
	if pr, ok := p.fetcher.(fetcher.ProgressReporter); ok {
		pr.SetOnFeedDone(func(feed string, index, total, jobs int) {
			p.emit(Progress{Stage: "feed_done", Feed: feed, FeedIndex: index, FeedTotal: total, FeedJobs: jobs})
		})
	}
}

func (p *Pipeline) emit(prog Progress) {
	p.last.Store(prog)
	if p.onProgress != nil {
		p.onProgress(prog)
	}
}

// IsRunning 报告当前是否有轮次在执行。
func (p *Pipeline) IsRunning() bool {
	if p.mu.TryLock() {
		p.mu.Unlock()
		return false
	}
	return true
}

// LastProgress 返回最近一次进度事件。
func (p *Pipeline) LastProgress() Progress {
	if v := p.last.Load(); v != nil {
		return v.(Progress)
	}
	return Progress{}
}

// RunOnce 带防重入与 panic 兜底的一轮执行，供 cron 与手动触发共用。
// 已有轮次在执行时：第一次重复请求排队（ErrQueued），之后返回 ErrRoundInProgress。
// 本轮结束后若有排队请求则紧接着再跑一轮。
func (p *Pipeline) RunOnce(ctx context.Context) (res RoundResult, err error) {
	if !p.mu.TryLock() {
		if p.queued.CompareAndSwap(false, true) {
			return res, ErrQueued
		}
		return res, ErrRoundInProgress
	}
	defer p.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("pipeline panicked, recovered", "panic", r)
			err = fmt.Errorf("pipeline panic: %v", r)
		}
	}()
	roundCtx, cancelFn := context.WithCancel(ctx)
	p.cancel.Store(cancelFn)
	for {
		res = p.Run(roundCtx)
		if p.onChange != nil {
			p.onChange()
		}
		if !p.queued.CompareAndSwap(true, false) {
			return res, nil
		}
		p.logger.Info("queued round requested, run again")
	}
}

// Run 执行一轮完整流水线。任何单点失败只记日志，整轮绝不 panic。
func (p *Pipeline) Run(ctx context.Context) RoundResult {
	log := p.logger.With("fetcher", p.fetcher.Name())
	log.Info("pipeline round started")

	p.emit(Progress{Stage: "fetch_start"})
	jobs, fetchErr := p.fetcher.Fetch(ctx)
	if fetchErr != nil {
		// 部分 feed 失败时 err 非空但 jobs 仍有数据，继续处理
		log.Warn("fetch completed with errors", "err", fetchErr)
	}
	log.Info("fetch done", "fetched", len(jobs))

	var toScore []domain.Job
	var newCount, dupCount, rejectedCount int

	for _, j := range jobs {
		ok, reason := p.rules.Accept(j)
		if !ok {
			j.Status = domain.StatusRejected
			j.Reason = "粗筛淘汰: " + reason
		} else if p.clientFilter != nil {
			// 客户质量粗筛：未验证+零花费 / 超期死帖，不进 LLM
			if ok, reason := p.clientFilter.Accept(j, time.Now().UTC()); !ok {
				j.Status = domain.StatusRejected
				j.Reason = "客户粗筛淘汰: " + reason
			}
		}
		if j.Status != domain.StatusRejected {
			j.Status = domain.StatusNew
		}

		isNew, err := p.store.InsertIfNew(ctx, j)
		if err != nil {
			log.Error("insert job failed, skip", "url", j.URL, "err", err)
			continue
		}
		if !isNew {
			dupCount++
			continue
		}
		newCount++
		if j.Status == domain.StatusRejected {
			rejectedCount++
			continue
		}
		// 新单且通过粗筛才进入打分队列，去重先于 LLM 调用
		toScore = append(toScore, j)
	}

	if len(toScore) > 0 && p.scorer != nil {
		p.scoreAndSave(ctx, toScore, log)
	} else if len(toScore) > 0 {
		log.Warn("llm not configured, skip scoring", "pending", len(toScore))
	}

	log.Info("pipeline round finished",
		"fetched", len(jobs), "new", newCount, "dup", dupCount, "rejected", rejectedCount)
	p.emit(Progress{Stage: "done", New: newCount, Fetched: len(jobs)})
	return RoundResult{Fetched: len(jobs), New: newCount, Dup: dupCount, Rejected: rejectedCount}
}

func (p *Pipeline) scoreAndSave(ctx context.Context, jobs []domain.Job, log *slog.Logger) {
	results := p.scorer.ScoreBatch(ctx, jobs, func(done, total int) {
		p.emit(Progress{Stage: "score", ScoreDone: done, ScoreTotal: total})
	})

	// 按 url 找回入库后的 id，用于更新分数与生成面板链接
	for i, j := range jobs {
		stored, err := p.store.GetByURL(ctx, j.URL)
		if err != nil {
			log.Error("lookup stored job failed", "url", j.URL, "err", err)
			continue
		}
		res := results[i]
		if err := p.store.UpdateScore(ctx, stored.ID, res.Score, res.Reason, res.Tags, filter.NowUTC()); err != nil {
			log.Error("update score failed", "id", stored.ID, "err", err)
			continue
		}
		stored.Score = res.Score
		stored.Reason = res.Reason
		stored.Tags = res.Tags

		if res.Score >= p.threshold {
			// 详情页活性复核：死帖不推送，解析失败保守推送但标注未核验
			stale, staleReason, verified := p.checkActivity(ctx, stored, log)
			if stale {
				if err := p.store.UpdateStatus(ctx, stored.ID, domain.StatusStale); err != nil {
					log.Error("mark stale failed", "id", stored.ID, "err", err)
				}
				log.Info("job marked stale, skip notify", "id", stored.ID, "reason", staleReason)
				continue
			}
			if !verified {
				stored.Reason += "（活性未核验）"
			}
			p.notifyAll(ctx, *stored, log)
		}
	}
}

// checkActivity 拉详情页复核活性。verified=false 表示未能完成核验（保守放行）。
func (p *Pipeline) checkActivity(ctx context.Context, j *domain.Job, log *slog.Logger) (stale bool, reason string, verified bool) {
	df, ok := p.fetcher.(fetcher.DetailFetcher)
	if !ok {
		return false, "", false
	}
	// 限速：距上次详情请求至少 5s
	if last := p.lastDetail.Load(); last != nil {
		if wait := 5*time.Second - time.Since(last.(time.Time)); wait > 0 {
			select {
			case <-ctx.Done():
				return false, "", false
			case <-time.After(wait):
			}
		}
	}

	var html string
	var err error
	for attempt := 0; attempt < 2; attempt++ { // 失败重试一次
		p.lastDetail.Store(time.Now())
		html, err = df.FetchDetailHTML(ctx, j.URL)
		if err == nil {
			break
		}
		log.Warn("detail fetch failed, retry once", "id", j.ID, "err", err)
	}
	if err != nil {
		return false, "", false
	}

	info := fetcher.ParseDetailPage(html, time.Now().UTC())
	if info.Spent == nil && info.Interviewing == nil && info.LastViewedAt == nil {
		return false, "", false // 页面没解析出任何活性字段，按未核验处理
	}

	updated := *j
	if info.Spent != nil {
		updated.ClientSpentUSD = info.Spent
	}
	if info.Rating != nil {
		updated.ClientRating = info.Rating
	}
	if info.Proposals != "" {
		updated.ProposalsBucket = info.Proposals
	}
	updated.LastViewedAt = info.LastViewedAt
	updated.Interviewing = info.Interviewing
	updated.InvitesSent = info.InvitesSent
	if err := p.store.UpdateClientInfo(ctx, j.ID, updated); err != nil {
		log.Warn("save client info failed", "id", j.ID, "err", err)
	}

	stale, reason = filter.EvaluateActivity(info.LastViewedAt, info.Interviewing, info.InvitesSent, time.Now().UTC())
	return stale, reason, true
}

// CancelRound 请求取消当前进行中的轮次（在下一个检查点生效）。
func (p *Pipeline) CancelRound() bool {
	if !p.IsRunning() {
		return false
	}
	if c := p.cancel.Load(); c != nil {
		if fn, ok := c.(context.CancelFunc); ok && fn != nil {
			fn()
			return true
		}
	}
	return false
}

func (p *Pipeline) notifyAll(ctx context.Context, j domain.Job, log *slog.Logger) {
	for _, n := range p.notifiers {
		if err := n.Notify(ctx, j); err != nil {
			// 推送失败只记日志不中断
			log.Error("notify failed", "notifier", n.Name(), "id", j.ID, "err", err)
		} else {
			log.Info("notified", "notifier", n.Name(), "id", j.ID, "score", j.Score)
		}
	}
}
