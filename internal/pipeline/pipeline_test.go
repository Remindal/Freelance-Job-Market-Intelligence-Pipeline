package pipeline

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"upwork-scout/internal/domain"
	"upwork-scout/internal/filter"
	"upwork-scout/internal/store"
)

// slowFetcher 拉取阻塞在 gate 上，用于构造「上一轮进行中」的场景。
type slowFetcher struct {
	gate  chan struct{}
	calls atomic.Int32
}

func (f *slowFetcher) Name() string { return "fake" }
func (f *slowFetcher) Fetch(ctx context.Context) ([]domain.Job, error) {
	f.calls.Add(1)
	<-f.gate
	return nil, nil
}

type fakeStore struct{}

func (fakeStore) InsertIfNew(ctx context.Context, j domain.Job) (bool, error) { return false, nil }
func (fakeStore) UpdateScore(ctx context.Context, id int64, score int, reason string, tags []string, at time.Time) error {
	return nil
}
func (fakeStore) UpdateStatus(ctx context.Context, id int64, s domain.Status) error { return nil }
func (fakeStore) UpdateClientInfo(ctx context.Context, id int64, j domain.Job) error { return nil }
func (fakeStore) Get(ctx context.Context, id int64) (*domain.Job, error) {
	return nil, errors.New("not found")
}
func (fakeStore) GetByURL(ctx context.Context, url string) (*domain.Job, error) {
	return nil, errors.New("not found")
}
func (fakeStore) List(ctx context.Context, f store.ListFilter) ([]domain.Job, int, error) {
	return nil, 0, nil
}
func (fakeStore) Stats(ctx context.Context, th int) (*store.Stats, error) { return &store.Stats{}, nil }
func (fakeStore) Close() error                                            { return nil }

func TestRunOnceQueuesSecondRequest(t *testing.T) {
	fetcher := &slowFetcher{gate: make(chan struct{})}
	rules := filter.NewRules(nil, nil, 0)
	p := New(fetcher, fakeStore{}, rules, nil, nil, nil, 70, slog.Default())

	// 第一轮：卡在 fetch 上
	firstDone := make(chan error, 1)
	go func() {
		_, err := p.RunOnce(context.Background())
		firstDone <- err
	}()
	// 等第一轮拿到锁进入 fetch
	for fetcher.calls.Load() == 0 {
		time.Sleep(10 * time.Millisecond)
	}

	// 第二轮请求：应返回 ErrQueued
	if _, err := p.RunOnce(context.Background()); !errors.Is(err, ErrQueued) {
		t.Fatalf("expected ErrQueued, got %v", err)
	}
	// 第三个请求：已有排队，应返回 ErrRoundInProgress
	if _, err := p.RunOnce(context.Background()); !errors.Is(err, ErrRoundInProgress) {
		t.Fatalf("expected ErrRoundInProgress, got %v", err)
	}

	// 放行两轮 fetch，第一轮结束后应自动跑排队的第二轮
	close(fetcher.gate)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first round err: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first round did not finish")
	}
	// 等排队轮跑完
	deadline := time.Now().Add(5 * time.Second)
	for fetcher.calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if fetcher.calls.Load() != 2 {
		t.Fatalf("queued round did not run, fetch calls = %d", fetcher.calls.Load())
	}
}
