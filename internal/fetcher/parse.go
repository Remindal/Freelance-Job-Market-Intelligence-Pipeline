package fetcher

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 本文件：客户质量/活性字段的纯解析函数，全部离线可测。

// spentRe 匹配 "$23K total spent"、"$3.1K spent"、"$0" 等金额。
var spentRe = regexp.MustCompile(`\$([\d,.]+)\s*([KkMm])?`)

// ParseMoney 解析 "$23K"→23000、"$3.1K"→3100、"$0"→0、"$1.5M"→1500000。失败返回 nil。
func ParseMoney(text string) *float64 {
	m := spentRe.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	n, err := strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	if err != nil {
		return nil
	}
	switch strings.ToUpper(m[2]) {
	case "K":
		n *= 1000
	case "M":
		n *= 1000000
	}
	return &n
}

// ParseProposalsBucket 把 "Less than 5"/"5 to 10"/"50+" 映射为枚举，未知返回空串。
func ParseProposalsBucket(text string) string {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimPrefix(t, "proposals:")
	t = strings.TrimSpace(t)
	switch {
	case strings.Contains(t, "less than 5"):
		return "fewer_than_5"
	case strings.Contains(t, "5 to 10"):
		return "5_to_10"
	case strings.Contains(t, "10 to 15"):
		return "10_to_15"
	case strings.Contains(t, "15 to 20"):
		return "15_to_20"
	case strings.Contains(t, "20 to 50"):
		return "20_to_50"
	case strings.Contains(t, "50+"):
		return "50_plus"
	}
	return ""
}

var relativeRe = regexp.MustCompile(`(?i)(\d+|a|an)\s+(minute|hour|day|week|month)s?\s+ago`)

// ParseRelativeTime 解析 "Posted 2 hours ago" / "Posted yesterday" / "last week" 等为时间点。
// now 作为基准传入便于测试。解析失败返回 nil。
func ParseRelativeTime(text string, now time.Time) *time.Time {
	t := strings.ToLower(strings.TrimSpace(text))
	t = strings.TrimPrefix(t, "posted")
	t = strings.TrimSpace(t)

	switch t {
	case "yesterday":
		r := now.Add(-24 * time.Hour)
		return &r
	case "last week":
		r := now.AddDate(0, 0, -7)
		return &r
	case "last month":
		r := now.AddDate(0, -1, 0)
		return &r
	case "just now", "today":
		return &now
	}

	m := relativeRe.FindStringSubmatch(t)
	if m == nil {
		return nil
	}
	n := 1
	if m[1] != "a" && m[1] != "an" {
		var err error
		if n, err = strconv.Atoi(m[1]); err != nil {
			return nil
		}
	}
	var d time.Duration
	switch m[2] {
	case "minute":
		d = time.Duration(n) * time.Minute
	case "hour":
		d = time.Duration(n) * time.Hour
	case "day":
		d = time.Duration(n) * 24 * time.Hour
	case "week":
		d = time.Duration(n) * 7 * 24 * time.Hour
	case "month":
		d = time.Duration(n) * 30 * 24 * time.Hour
	}
	r := now.Add(-d)
	return &r
}

// ClientSignals 搜索页 Nuxt 载荷里的客户字段（browser 端 evaluate 产出）。
type ClientSignals struct {
	CID           string   `json:"cid"`
	Verified      *bool    `json:"verified"`
	Spent         *float64 `json:"spent"`
	Feedback      *float64 `json:"feedback"` // 客户评分
	ProposalsTier string   `json:"proposalsTier"`
	PublishedOn   string   `json:"publishedOn"`
}

// ParseClientSignals 解析 browser evaluate 返回的 JSON 数组，按单子 ID 索引。
func ParseClientSignals(jsonStr string) (map[string]ClientSignals, error) {
	var list []ClientSignals
	if err := json.Unmarshal([]byte(jsonStr), &list); err != nil {
		return nil, err
	}
	out := make(map[string]ClientSignals, len(list))
	for _, s := range list {
		id := strings.TrimPrefix(s.CID, "~")
		if id != "" {
			out[id] = s
		}
	}
	return out, nil
}

// ratingRe 匹配 sr-only 文本 "Rating is 4.9 out of 5"。
var ratingRe = regexp.MustCompile(`Rating is ([\d.]+) out of 5`)

// DetailInfo 详情页解析结果。
type DetailInfo struct {
	Spent        *float64
	Rating       *float64
	LastViewedAt *time.Time
	Interviewing *int
	InvitesSent  *int
	Proposals    string
}

// activityTitleRe 定位活性条目标题 <span class="title">Interviewing:</span>。
var activityTitleRe = regexp.MustCompile(`(?s)<span class="title"[^>]*>\s*([^<:]+):?\s*</span>`)

var valueOpenRe = regexp.MustCompile(`<(?:div|span) class="value"[^>]*>`)

// activityValue 取 title 之后第一个 value 元素的文本（Proposals 行中间隔着弹层标记，不能靠紧邻匹配）。
func activityValue(html string, titleEnd int) string {
	loc := valueOpenRe.FindStringIndex(html[titleEnd:])
	if loc == nil {
		return ""
	}
	start := titleEnd + loc[1]
	rest := html[start:]
	if end := strings.Index(rest, "</"); end >= 0 {
		return strings.TrimSpace(rest[:end])
	}
	return ""
}

// ParseDetailPage 从详情页 HTML 解析客户与活性字段，缺失记 nil 不报错。
func ParseDetailPage(pageHTML string, now time.Time) DetailInfo {
	var info DetailInfo

	if i := strings.Index(pageHTML, `data-qa="client-spend"`); i >= 0 {
		seg := pageHTML[i:min(i+500, len(pageHTML))]
		info.Spent = ParseMoney(seg)
	}
	if m := ratingRe.FindStringSubmatch(pageHTML); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			info.Rating = &v
		}
	}

	for _, m := range activityTitleRe.FindAllStringSubmatchIndex(pageHTML, -1) {
		title := strings.TrimSpace(pageHTML[m[2]:m[3]])
		value := activityValue(pageHTML, m[1])
		switch {
		case strings.HasPrefix(title, "Interviewing"):
			info.Interviewing = parseIntPtr(value)
		case strings.HasPrefix(title, "Invites sent"):
			info.InvitesSent = parseIntPtr(value)
		case strings.HasPrefix(title, "Last viewed"):
			info.LastViewedAt = ParseRelativeTime(value, now)
		case strings.HasPrefix(title, "Proposals"):
			info.Proposals = ParseProposalsBucket(value)
		}
	}
	return info
}

func parseIntPtr(s string) *int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &n
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
