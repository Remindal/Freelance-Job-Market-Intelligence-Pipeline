package domain

import "time"

type Status string

const (
	StatusNew      Status = "new"
	StatusRejected Status = "rejected"
	StatusWant     Status = "want"
	StatusProposed Status = "proposed"
	StatusIgnored  Status = "ignored"
	StatusStale    Status = "stale" // 死帖：详情页活性复核命中硬杀规则
)

// Valid 判断状态值是否合法，供 web 层校验表单输入。
func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusRejected, StatusWant, StatusProposed, StatusIgnored, StatusStale:
		return true
	}
	return false
}

type Job struct {
	ID          int64      `json:"id"`
	URL         string     `json:"url"` // 唯一约束，去重指纹
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Budget      string     `json:"budget"` // 原文，如 "$500" / "$25-50/hr"，不做强解析
	Skills      []string   `json:"skills"` // 单子上挂的技能标签
	Score       int        `json:"score"`  // LLM 打分 0-100，未打分=0
	Reason      string     `json:"reason"` // LLM 中文理由
	Tags        []string   `json:"tags"`   // LLM 打的短标签，旧数据为空数组
	Status      Status     `json:"status"`
	FetchedAt   time.Time  `json:"fetched_at"`
	ScoredAt    *time.Time `json:"scored_at"`

	// 客户质量信号（搜索页 Nuxt 载荷提取，nil=未知）
	PaymentVerified *bool      `json:"payment_verified"`
	ClientSpentUSD  *float64   `json:"client_spent_usd"`
	ClientRating    *float64   `json:"client_rating"`
	PostedAt        *time.Time `json:"posted_at"`
	ProposalsBucket string     `json:"proposals_bucket"`

	// 详情页活性字段（仅高分单复核时填充，nil=未核验）
	LastViewedAt *time.Time `json:"last_viewed_at"`
	Interviewing *int       `json:"interviewing"`
	InvitesSent  *int       `json:"invites_sent"`
}
