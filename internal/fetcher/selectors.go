package fetcher

import (
	"regexp"
	"strings"
	"time"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/Remindal/scout/internal/domain"
)

// 目标站搜索页 DOM 选择器集中管理：站点改 DOM 时只动这一个文件。
// 每个字段给一组候选选择器，按顺序取第一个命中。

// CardSelectorCSS 卡片主选择器（字符串形式，供 WaitForSelector 使用）。
const CardSelectorCSS = `article[data-test="JobTile"]`

var (
	selCards = mustCompileAll(
		CardSelectorCSS,
		`[data-test="JobTile"]`,
		`article`,
	)
	selTitleLink = mustCompileAll(
		`a[data-test="job-tile-title-link"]`,
		`h2 a[href*="/jobs/"]`,
		`h3 a[href*="/jobs/"]`,
		`h4 a[href*="/jobs/"]`,
		`a[href*="/jobs/~"]`,
	)
	selDescription = mustCompileAll(
		`[data-test="job-description-text"]`,
		`[class*="job-tile-description"]`,
		`[data-test="UpCLineClamp"]`,
	)
	selBudget = mustCompileAll(
		`[data-test="budget"]`,
		`[data-test="job-type-label"]`,
	)
	selJobType = mustCompileAll(
		`[data-test="job-type-label"]`,
	)
	selSkills = mustCompileAll(
		`[data-test="token"]`,
		`[data-test="attrs"] span`,
	)
	selPosted = mustCompileAll(
		`[data-test="job-pubilshed-date"]`, // 站点官方属性名就是这个拼写（pubilshed 是上游拼写错误）
		`[data-test="job-pub-date"]`,
	)
	// 人机验证特征：Cloudflare / PerimeterX 拦截页
	selChallenge = mustCompileAll(
		`.px-captcha`,
		`#px-captcha`,
		`[id*="cf-chl"]`,
		`#challenge-form`,
	)
)

func mustCompileAll(selectors ...string) []cascadia.Selector {
	out := make([]cascadia.Selector, len(selectors))
	for i, s := range selectors {
		out[i] = cascadia.MustCompile(s)
	}
	return out
}

// queryFirst 按候选选择器顺序返回第一个命中的节点。
func queryFirst(n *html.Node, sels []cascadia.Selector) *html.Node {
	for _, sel := range sels {
		if m := cascadia.Query(n, sel); m != nil {
			return m
		}
	}
	return nil
}

func queryAllNodes(n *html.Node, sels []cascadia.Selector) []*html.Node {
	for _, sel := range sels {
		if m := cascadia.QueryAll(n, sel); len(m) > 0 {
			return m
		}
	}
	return nil
}

// textContent 取节点全部文本并压缩空白。
func textContent(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
			b.WriteByte(' ')
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// clientSignalsJS 在页面内遍历 window.__NUXT__，找到单子数组并提取客户字段（零新增请求）。
const clientSignalsJS = `(() => {
  const out = [];
  const seen = new Set();
  function walk(o, depth) {
    if (!o || typeof o !== 'object' || seen.has(o) || depth > 14) return;
    seen.add(o);
    if (Array.isArray(o)) {
      if (o.length && o[0] && typeof o[0] === 'object' && o[0].ciphertext && o[0].title) {
        for (const j of o) {
          out.push({
            cid: j.ciphertext,
            verified: j.client?.isPaymentVerified ?? null,
            spent: j.client?.totalSpent ?? null,
            feedback: j.client?.totalFeedback ?? null,
            proposalsTier: j.proposalsTier ?? null,
            publishedOn: j.publishedOn ?? null,
          });
        }
        return;
      }
      for (const v of o) walk(v, depth + 1);
      return;
    }
    for (const v of Object.values(o)) walk(v, depth + 1);
  }
  walk(window.__NUXT__, 0);
  return JSON.stringify(out);
})()`

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// jobIDRe 提取链接中的单子 ID（~01xxxx 形式）。
var jobIDRe = regexp.MustCompile(`~([0-9a-zA-Z]+)`)

// normalizeURL 规范化单子链接：去 query/fragment，并收敛为 <origin>/jobs/~id 短链。
// 搜索页会对关键词高亮，同一单在不同关键词源下 slug 不同（甚至带 span 标记残留），
// 只有 ID 部分是稳定的，去重指纹必须用短链。origin 由调用方从 feed URL 推导。
func normalizeURL(href, origin string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if i := strings.IndexAny(href, "?#"); i >= 0 {
		href = href[:i]
	}
	if m := jobIDRe.FindString(href); m != "" {
		return origin + "/jobs/" + m
	}
	if strings.HasPrefix(href, "/") {
		href = origin + href
	}
	return href
}

var (
	budgetAmountRe = regexp.MustCompile(`\$[\d,]+(?:\.\d+)?(?:\s*-\s*\$?[\d,]+(?:\.\d+)?)?`)
	hourlyRe       = regexp.MustCompile(`(?i)hourly|/hr`)
)

// extractBudget 从预算/类型标签文本中提取预算原文。
// 固定价返回 "$800"，时薪返回 "$25-50/hr"（供粗筛区分，时薪不做预算下限淘汰）。
func extractBudget(card *html.Node) string {
	var text string
	if n := queryFirst(card, selBudget); n != nil {
		text = textContent(n)
	}
	if t := queryFirst(card, selJobType); t != nil {
		text += " " + textContent(t)
	}
	amount := budgetAmountRe.FindString(text)
	if amount == "" {
		return ""
	}
	amount = strings.ReplaceAll(amount, " ", "")
	if hourlyRe.MatchString(text) && !strings.HasSuffix(amount, "/hr") {
		amount += "/hr"
	}
	return amount
}

// challengeTitles 验证页标题特征（小写匹配）。
var challengeTitles = []string{"security check", "just a moment", "challenge", "请完成安全验证"}

// IsChallengePage 检测 Cloudflare / PerimeterX 人机验证页特征。
func IsChallengePage(pageTitle, pageHTML string) bool {
	t := strings.ToLower(pageTitle)
	for _, pat := range challengeTitles {
		if strings.Contains(t, pat) {
			return true
		}
	}
	// Cloudflare managed challenge 的内联脚本特征
	if strings.Contains(pageHTML, "cdn-cgi/challenge-platform") {
		return true
	}
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return false
	}
	return queryFirst(doc, selChallenge) != nil
}

// ExtractJobsFromHTML 从搜索页完整 HTML 提取单子卡片，origin 为站点源（如 https://example.com）。
// 单字段缺失留空不丢整卡；只有 title+url 齐全才保留（url 是去重指纹，缺失无法入库）。
func ExtractJobsFromHTML(pageHTML string, fetchedAt time.Time, origin string) []domain.Job {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil
	}
	var cards []*html.Node
	for _, sel := range selCards {
		if cards = cascadia.QueryAll(doc, sel); len(cards) > 0 {
			break
		}
	}

	var jobs []domain.Job
	seen := map[string]bool{}
	for _, card := range cards {
		link := queryFirst(card, selTitleLink)
		if link == nil {
			continue
		}
		title := textContent(link)
		url := normalizeURL(attr(link, "href"), origin)
		if title == "" || url == "" || seen[url] {
			continue
		}
		seen[url] = true

		j := domain.Job{
			URL:       url,
			Title:     title,
			Budget:    extractBudget(card),
			FetchedAt: fetchedAt,
		}
		if n := queryFirst(card, selDescription); n != nil {
			j.Description = textContent(n)
		}
		for _, tok := range queryAllNodes(card, selSkills) {
			if s := textContent(tok); s != "" {
				j.Skills = append(j.Skills, s)
			}
		}
		// 卡片上的 "Posted x ago" 作为发布时间的兑底来源（主来源是 Nuxt 载荷的 publishedOn）
		if n := queryFirst(card, selPosted); n != nil {
			j.PostedAt = ParseRelativeTime(textContent(n), fetchedAt)
		}
		jobs = append(jobs, j)
	}
	return jobs
}

// MergeClientSignals 把 Nuxt 载荷提取的客户信号按单子 ID 合并进抓取结果。
func MergeClientSignals(jobs []domain.Job, signals map[string]ClientSignals) {
	for i := range jobs {
		id := jobs[i].URL[strings.LastIndex(jobs[i].URL, "~")+1:]
		s, ok := signals[id]
		if !ok {
			continue
		}
		jobs[i].PaymentVerified = s.Verified
		if s.Spent != nil {
			jobs[i].ClientSpentUSD = s.Spent
		}
		if s.Feedback != nil {
			jobs[i].ClientRating = s.Feedback
		}
		if b := ParseProposalsBucket(s.ProposalsTier); b != "" {
			jobs[i].ProposalsBucket = b
		}
		if s.PublishedOn != "" {
			if t, err := time.Parse(time.RFC3339, s.PublishedOn); err == nil {
				t = t.UTC()
				jobs[i].PostedAt = &t
			}
		}
	}
}
