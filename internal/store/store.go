package store

import (
	"context"
	"time"

	"github.com/Remindal/scout/internal/domain"
)

// ListFilter 列表查询条件，零值字段表示不限制。
// Sort 只允许白名单值：score_desc | score_asc | fetched_desc（默认 score_desc）。
type ListFilter struct {
	Status   domain.Status `json:"status"` // 为空表示全部状态
	MinScore int           `json:"min_score"`
	Keyword  string        `json:"keyword"` // 标题/描述模糊匹配
	Tag      string        `json:"tag"`    // 按 LLM 标签精确过滤
	Sort     string        `json:"sort"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
}

type DailyCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type ScoreBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type Stats struct {
	TodayNew          int            `json:"today_new"`
	HighScorePending  int            `json:"high_score_pending"`
	WantCount         int            `json:"want_count"`
	ProposedCount     int            `json:"proposed_count"`
	DailyNew          []DailyCount   `json:"daily_new"`
	ScoreDistribution []ScoreBucket  `json:"score_distribution"`
	StatusCounts      map[string]int `json:"status_counts"`
}

type Store interface {
	// InsertIfNew 以 url 为指纹去重插入，返回是否为新单。
	InsertIfNew(ctx context.Context, j domain.Job) (isNew bool, err error)
	UpdateScore(ctx context.Context, id int64, score int, reason string, tags []string, scoredAt time.Time) error
	// UpdateClientInfo 回写客户/活性字段（nil 字段不覆盖已有值）。
	UpdateClientInfo(ctx context.Context, id int64, j domain.Job) error
	UpdateStatus(ctx context.Context, id int64, status domain.Status) error
	Get(ctx context.Context, id int64) (*domain.Job, error)
	GetByURL(ctx context.Context, url string) (*domain.Job, error)
	// List 返回当前页数据与符合条件的总条数（分页在前端展示，LIMIT/OFFSET 在此层执行）。
	List(ctx context.Context, f ListFilter) ([]domain.Job, int, error)
	// Stats 仪表盘统计，highScoreThreshold 为「高分待决策」的分数下限。
	Stats(ctx context.Context, highScoreThreshold int) (*Stats, error)
	// DeleteOlderThan 删除抓取时间早于 cutoff 的单子（想投/已投的用户标记单除外），返回删除条数。
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
	Close() error
}
