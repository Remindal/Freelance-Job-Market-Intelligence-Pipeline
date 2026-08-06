package filter

import (
	"fmt"
	"time"

	"upwork-scout/internal/domain"
)

// ClientFilter 客户质量粗筛：在 LLM 精筛之前执行，字段为 nil（未解析到）时不淘汰。
type ClientFilter struct {
	StaleDays int // 发布超过该天数的好单必已满员，不值得投
}

func NewClientFilter(staleDays int) *ClientFilter {
	if staleDays <= 0 {
		staleDays = 2
	}
	return &ClientFilter{StaleDays: staleDays}
}

// Accept 命中任一规则返回 false 与原因。
func (c *ClientFilter) Accept(j domain.Job, now time.Time) (bool, string) {
	// a) 未验证支付 + 零花费的新客户
	if j.PaymentVerified != nil && !*j.PaymentVerified &&
		j.ClientSpentUSD != nil && *j.ClientSpentUSD == 0 {
		return false, "客户未验证支付且历史零花费"
	}
	// b) 超期死帖
	if j.PostedAt != nil && now.Sub(*j.PostedAt) > time.Duration(c.StaleDays)*24*time.Hour {
		return false, fmt.Sprintf("发布已超过 %d 天", c.StaleDays)
	}
	return true, ""
}

// EvaluateActivity 详情页活性复核硬杀规则，返回是否死帖与原因。
// 字段为 nil（未解析到）时不杀，保守放行。
func EvaluateActivity(lastViewedAt *time.Time, interviewing, invitesSent *int, now time.Time) (bool, string) {
	// 客户超过 3 天没看帖：在别处已谈妥
	if lastViewedAt != nil && now.Sub(*lastViewedAt) > 3*24*time.Hour {
		return true, "客户超过 3 天未查看（死帖）"
	}
	// 面试名单已满
	if interviewing != nil && *interviewing >= 5 {
		return true, fmt.Sprintf("面试中人数 %d ≥ 5（名单已满）", *interviewing)
	}
	// 受邀人洽谈中，发帖只是走流程
	if invitesSent != nil && *invitesSent >= 1 &&
		interviewing != nil && *interviewing >= 1 &&
		lastViewedAt != nil && now.Sub(*lastViewedAt) > 24*time.Hour {
		return true, "已发邀请且洽谈中，客户超 1 天未查看"
	}
	return false, ""
}
