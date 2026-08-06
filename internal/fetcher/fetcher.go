package fetcher

import (
	"context"
	"math/rand"
	"time"

	"upwork-scout/internal/domain"
)

type Fetcher interface {
	Name() string
	Fetch(ctx context.Context) ([]domain.Job, error)
}

// ProgressReporter 可选接口：支持逐源进度回报的 fetcher 实现它。
type ProgressReporter interface {
	SetOnFeedDone(func(feed string, index, total, jobs int))
}

// DetailFetcher 可选接口：支持取详情页的 fetcher 实现它（用于高分单活性复核）。
type DetailFetcher interface {
	FetchDetailHTML(ctx context.Context, jobURL string) (string, error)
}

// Feed 一个订阅源（一组关键词搜索页）。
type Feed struct {
	Name string
	URL  string
}

// sleepJitter 在 [min, max] 间随机睡眠，模拟人类节奏，可被 ctx 取消。
func sleepJitter(ctx context.Context, min, max time.Duration) error {
	d := min + time.Duration(rand.Int63n(int64(max-min)))
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
