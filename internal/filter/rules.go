package filter

import (
	"regexp"
	"strconv"
	"strings"

	"upwork-scout/internal/domain"
)

type keyword struct {
	text string
	re   *regexp.Regexp
}

// Rules 规则粗筛：关键词/预算下限，全部规则来自配置。
type Rules struct {
	include      []keyword
	exclude      []keyword
	MinBudgetUSD int
}

func NewRules(include, exclude []string, minBudget int) *Rules {
	return &Rules{
		include:      compileKeywords(include),
		exclude:      compileKeywords(exclude),
		MinBudgetUSD: minBudget,
	}
}

// compileKeywords 编译为词边界匹配，避免 "go" 误命中 "logo" 这类子串。
func compileKeywords(keywords []string) []keyword {
	var out []keyword
	for _, kw := range keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" {
			continue
		}
		out = append(out, keyword{
			text: kw,
			re:   regexp.MustCompile(`(?:^|[^a-z0-9])` + regexp.QuoteMeta(kw) + `(?:[^a-z0-9]|$)`),
		})
	}
	return out
}

// Accept 判断单子是否通过粗筛，不通过时返回淘汰原因（用于复盘）。
func (r *Rules) Accept(j domain.Job) (bool, string) {
	// 首尾补空格，保证边界关键词也能命中
	text := " " + strings.ToLower(j.Title+" "+j.Description) + " "

	for _, kw := range r.exclude {
		if kw.re.MatchString(text) {
			return false, "命中排除关键词: " + kw.text
		}
	}

	if len(r.include) > 0 {
		matched := false
		for _, kw := range r.include {
			if kw.re.MatchString(text) {
				matched = true
				break
			}
		}
		if !matched {
			return false, "未命中任一关注关键词"
		}
	}

	if r.MinBudgetUSD > 0 {
		if amount, ok := ParseFixedBudgetUSD(j.Budget); ok && amount < r.MinBudgetUSD {
			return false, "预算低于下限"
		}
	}
	return true, ""
}

// fixedBudgetRe 匹配 "$500"、"$1,200" 这类固定价，不匹配 "$25-50/hr" 时薪。
var fixedBudgetRe = regexp.MustCompile(`^\$\s*([0-9][0-9,]*)\s*$`)

// ParseFixedBudgetUSD 解析固定价预算文本，时薪或无法解析时返回 ok=false（不做预算淘汰）。
func ParseFixedBudgetUSD(budget string) (int, bool) {
	m := fixedBudgetRe.FindStringSubmatch(strings.TrimSpace(budget))
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	if err != nil {
		return 0, false
	}
	return n, true
}
