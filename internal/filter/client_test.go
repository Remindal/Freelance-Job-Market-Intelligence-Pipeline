package filter

import (
	"testing"
	"time"

	"github.com/Remindal/scout/internal/domain"
)

func boolPtr(b bool) *bool          { return &b }
func floatPtr(f float64) *float64   { return &f }
func intPtr(i int) *int             { return &i }
func timePtr(t time.Time) *time.Time { return &t }

var now = time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

func TestClientFilterRejectsUnverifiedZeroSpent(t *testing.T) {
	cf := NewClientFilter(2)
	// 巴基斯坦 $0 未验证单：拒
	ok, reason := cf.Accept(domain.Job{
		PaymentVerified: boolPtr(false),
		ClientSpentUSD:  floatPtr(0),
	}, now)
	if ok {
		t.Fatal("unverified + zero spent should be rejected")
	}
	if reason == "" {
		t.Fatal("should carry reason")
	}
}

func TestClientFilterPassesVerifiedSpender(t *testing.T) {
	cf := NewClientFilter(2)
	// Cloudflare 单：verified + $3.1K → 通过
	ok, _ := cf.Accept(domain.Job{
		PaymentVerified: boolPtr(true),
		ClientSpentUSD:  floatPtr(3100),
	}, now)
	if !ok {
		t.Fatal("verified + $3.1K should pass")
	}
}

func TestClientFilterStaleDays(t *testing.T) {
	cf := NewClientFilter(2)
	stale := domain.Job{PostedAt: timePtr(now.Add(-72 * time.Hour))}
	if ok, _ := cf.Accept(stale, now); ok {
		t.Fatal("posted 3 days ago with stale_days=2 should be rejected")
	}
	fresh := domain.Job{PostedAt: timePtr(now.Add(-20 * time.Hour))}
	if ok, _ := cf.Accept(fresh, now); !ok {
		t.Fatal("posted 20h ago should pass")
	}
}

func TestClientFilterNilFieldsPass(t *testing.T) {
	cf := NewClientFilter(2)
	// 全部字段未知（解析失败）→ 不误杀
	if ok, _ := cf.Accept(domain.Job{}, now); !ok {
		t.Fatal("nil fields should pass (conservative)")
	}
	// 未验证但花费未知 → 不拒（证据不足）
	if ok, _ := cf.Accept(domain.Job{PaymentVerified: boolPtr(false)}, now); !ok {
		t.Fatal("unverified but unknown spent should pass")
	}
}

func TestEvaluateActivity(t *testing.T) {
	// FHIR 案例：last week 未查看 → 死帖
	stale, reason := EvaluateActivity(timePtr(now.Add(-7*24*time.Hour)), intPtr(0), intPtr(0), now)
	if !stale || reason == "" {
		t.Fatal("last viewed a week ago should be stale")
	}
	// 面试满员
	stale, _ = EvaluateActivity(nil, intPtr(5), nil, now)
	if !stale {
		t.Fatal("interviewing >= 5 should be stale")
	}
	// 受邀洽谈中 + 超 1 天未查看
	stale, _ = EvaluateActivity(timePtr(now.Add(-26*time.Hour)), intPtr(2), intPtr(3), now)
	if !stale {
		t.Fatal("invites+interviewing+silent should be stale")
	}
	// 活跃帖：全字段正常 → 不杀
	stale, _ = EvaluateActivity(timePtr(now.Add(-3*time.Hour)), intPtr(1), intPtr(0), now)
	if stale {
		t.Fatal("active post should not be stale")
	}
	// 全 nil（未核验）→ 不杀
	stale, _ = EvaluateActivity(nil, nil, nil, now)
	if stale {
		t.Fatal("nil fields should not be stale")
	}
}
