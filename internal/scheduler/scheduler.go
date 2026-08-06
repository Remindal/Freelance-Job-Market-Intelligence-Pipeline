package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Remindal/scout/internal/pipeline"
)

// Scheduler 封装 robfig/cron，注册 pipeline 周期执行。
// 防重入由 pipeline.RunOnce 内的互斥锁保证（与手动触发共用同一把锁）。
type Scheduler struct {
	cron     *cron.Cron
	run      func(ctx context.Context) error
	interval time.Duration
	logger   *slog.Logger
}

func New(interval time.Duration, run func(ctx context.Context) error, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		cron:     cron.New(),
		run:      run,
		interval: interval,
		logger:   logger,
	}
}

// Start 启动调度，并立即先跑一轮（不等第一个周期）。
func (s *Scheduler) Start(ctx context.Context) error {
	spec := fmt.Sprintf("@every %s", s.interval)
	if _, err := s.cron.AddFunc(spec, func() { s.guardedRun(ctx) }); err != nil {
		return fmt.Errorf("register cron job: %w", err)
	}
	s.cron.Start()
	s.logger.Info("scheduler started", "interval", s.interval)

	go s.guardedRun(ctx)
	return nil
}

func (s *Scheduler) guardedRun(ctx context.Context) {
	if err := s.run(ctx); err != nil {
		if errors.Is(err, pipeline.ErrRoundInProgress) {
			s.logger.Warn("previous round still running, skip this tick")
		} else {
			s.logger.Error("pipeline round failed", "err", err)
		}
	}
}

// Stop 停止调度，返回的 ctx 在正在执行的任务结束后关闭。
func (s *Scheduler) Stop() context.Context {
	return s.cron.Stop()
}
