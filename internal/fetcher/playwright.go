package fetcher

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/Remindal/scout/internal/domain"
)

// PWFetcher 通过 CDP 接管用户已运行的真实 Chrome 抓取搜索页。
// 不 launch 任何浏览器实例——目标站有完善的反爬保护，
// 复用本人浏览会话做只读、低频访问是最稳妥的方式。
type PWFetcher struct {
	feeds        []Feed
	cdpEndpoint  string
	pagesPerFeed int           // 翻页安全上限
	windowDays   int           // 时间窗口：遇到早于该天数的单即停止翻页（0=不看时间，抓满上限）
	logger       *slog.Logger
	dumpDir      string        // 环境变量 SCOUT_DEBUG_DUMP 开启后保存页面 HTML，便于排查选择器
	onFeedDone   func(feed string, index, total, jobs int)
}

// SetOnFeedDone 实现 ProgressReporter，逐源回报进度。
func (f *PWFetcher) SetOnFeedDone(fn func(feed string, index, total, jobs int)) {
	f.onFeedDone = fn
}

func NewPlaywright(feeds []Feed, cdpEndpoint string, pagesPerFeed, windowDays int, logger *slog.Logger) *PWFetcher {
	if cdpEndpoint == "" {
		cdpEndpoint = "http://127.0.0.1:9222"
	}
	if pagesPerFeed <= 0 {
		pagesPerFeed = 1
	}
	f := &PWFetcher{
		feeds:        feeds,
		cdpEndpoint:  cdpEndpoint,
		pagesPerFeed: pagesPerFeed,
		windowDays:   windowDays,
		logger:       logger,
	}
	if dir := os.Getenv("SCOUT_DEBUG_DUMP"); dir != "" {
		f.dumpDir = dir
	}
	return f
}

func (f *PWFetcher) Name() string { return "playwright-cdp" }

// Fetch 连接已运行的 Chrome，在其现有 context 里逐 feed 开页抓取，抓完只关 page。
// 连接失败直接报错并给出启动指引，不重试、不自行启动浏览器。
func (f *PWFetcher) Fetch(ctx context.Context) ([]domain.Job, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright driver: %w", err)
	}
	defer pw.Stop()

	browser, err := pw.Chromium.ConnectOverCDP(f.cdpEndpoint)
	if err != nil {
		return nil, fmt.Errorf(
			"无法连接 Chrome（%s）：请用带 --remote-debugging-port=9222 的快捷方式启动 Chrome 后重试: %w",
			f.cdpEndpoint, err)
	}
	// 注意：禁止 browser.Close()，只断开驱动连接（pw.Stop），浏览器本体是用户的

	contexts := browser.Contexts()
	if len(contexts) == 0 {
		return nil, errors.New("已连接 Chrome 但没有可用 context（浏览器是否刚启动？）")
	}
	bctx := contexts[0]

	var jobs []domain.Job
	var errs []error
	for i, feed := range f.feeds {
		if i > 0 {
			if err := sleepJitter(ctx, 5*time.Second, 10*time.Second); err != nil {
				return jobs, err
			}
		}
		got, err := f.fetchFeed(ctx, bctx, feed)
		if err != nil {
			f.logger.Warn("feed fetch failed, continue with others",
				"feed", feed.Name, "err", err)
			errs = append(errs, fmt.Errorf("feed %s: %w", feed.Name, err))
			if f.onFeedDone != nil {
				f.onFeedDone(feed.Name, i+1, len(f.feeds), 0)
			}
			continue
		}
		jobs = append(jobs, got...)
		f.logger.Info("feed fetched", "feed", feed.Name, "jobs", len(got))
		if f.onFeedDone != nil {
			f.onFeedDone(feed.Name, i+1, len(f.feeds), len(got))
		}
	}
	return jobs, errors.Join(errs...)
}

// fetchFeed 按配置的页数逐页抓取，页间随机短歇。
func (f *PWFetcher) fetchFeed(ctx context.Context, bctx playwright.BrowserContext, feed Feed) ([]domain.Job, error) {
	page, err := bctx.NewPage()
	if err != nil {
		return nil, err
	}
	defer page.Close()

	var jobs []domain.Job
	seen := map[string]bool{}
	for p := 1; p <= f.pagesPerFeed; p++ {
		if err := ctx.Err(); err != nil {
			return jobs, err
		}
		if p > 1 {
			// 页间 3-6s 随机短歇
			select {
			case <-ctx.Done():
				return jobs, ctx.Err()
			case <-time.After(3*time.Second + time.Duration(rand.Int63n(int64(3*time.Second)))):
			}
		}
		got, err := f.fetchPage(page, feedPageURL(feed.URL, p))
		if err != nil {
			// 第一页失败整源算失败；后续页失败保留已抓到的，记日志即可
			if p == 1 {
				return nil, err
			}
			f.logger.Warn("page fetch failed, keep previous pages", "feed", feed.Name, "page", p, "err", err)
			break
		}
		for _, j := range got {
			if !seen[j.URL] {
				seen[j.URL] = true
				jobs = append(jobs, j)
			}
		}
		// 时间窗口早停：本页最旧的单已超出窗口，再翻页无意义（更旧的单会被客户粗筛淘汰）
		if f.windowDays > 0 && len(got) > 0 {
			if oldest := oldestPosted(got); oldest != nil &&
				time.Since(*oldest) > time.Duration(f.windowDays)*24*time.Hour {
				break
			}
		}
	}
	if len(jobs) == 0 {
		return nil, errors.New("页面已渲染但未提取到任何卡片（选择器可能已失效）")
	}
	return jobs, nil
}

// oldestPosted 返回一页单子里最旧的发布时间；全部未知返回 nil（继续翻页，保守多抓）。
func oldestPosted(jobs []domain.Job) *time.Time {
	var oldest *time.Time
	for i := range jobs {
		if jobs[i].PostedAt == nil {
			continue
		}
		if oldest == nil || jobs[i].PostedAt.Before(*oldest) {
			oldest = jobs[i].PostedAt
		}
	}
	return oldest
}

// feedPageURL 给搜索 URL 追加/覆盖 page 参数。
func feedPageURL(raw string, page int) string {
	if page <= 1 {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}

func (f *PWFetcher) fetchPage(page playwright.Page, pageURL string) ([]domain.Job, error) {
	if _, err := page.Goto(pageURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(45000),
	}); err != nil {
		return nil, fmt.Errorf("goto: %w", err)
	}

	// 真人浏览器偶尔会碰到验证页：等它自动过（不刷新不打断，实测自动通过最长约 2.5 分钟），
	// 超时 4 分钟跳过该 feed
	title, _ := page.Title()
	content, _ := page.Content()
	if IsChallengePage(title, content) {
		f.logger.Info("challenge page detected, waiting for auto-pass", "url", pageURL)
		deadline := time.Now().Add(240 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(5 * time.Second)
			title, _ = page.Title()
			content, _ = page.Content()
			if !IsChallengePage(title, content) {
				break
			}
		}
		if IsChallengePage(title, content) {
			f.dumpPage("challenge", content)
			return nil, errors.New("触发人机验证且等待超时，本轮跳过该源")
		}
		f.logger.Info("challenge passed", "url", pageURL)
	}

	// 等卡片渲染出来 + 短缓冲让异步渲染稳定，禁止固定长时间 sleep
	if _, err := page.WaitForSelector(CardSelectorCSS, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(20000),
	}); err != nil {
		content, _ := page.Content()
		f.dumpPage("no-cards", content)
		return nil, fmt.Errorf("等待列表渲染超时: %w", err)
	}
	time.Sleep(1500 * time.Millisecond)

	content, err := page.Content()
	if err != nil {
		return nil, err
	}
	f.dumpPage("page", content)

	// origin 从 feed URL 推导，保证代码不硬编码站点域名
	origin := ""
	if u, err := url.Parse(pageURL); err == nil {
		origin = u.Scheme + "://" + u.Host
	}
	jobs := ExtractJobsFromHTML(content, time.Now().UTC(), origin)
	if len(jobs) == 0 {
		return nil, errors.New("页面已渲染但未提取到任何卡片（选择器可能已失效）")
	}

	// 从 Nuxt 载荷补客户信号（零新增请求），失败不影哂主流程
	if v, err := page.Evaluate(clientSignalsJS); err == nil {
		if s, ok := v.(string); ok {
			if signals, err := ParseClientSignals(s); err == nil {
				MergeClientSignals(jobs, signals)
			}
		}
	} else {
		f.logger.Warn("client signals extract failed, continue without", "err", err)
	}
	return jobs, nil
}

// FetchDetailHTML 取单子详情页 HTML（复用同一 CDP 连接的一个新页签）。
func (f *PWFetcher) FetchDetailHTML(ctx context.Context, jobURL string) (string, error) {
	pw, err := playwright.Run()
	if err != nil {
		return "", err
	}
	defer pw.Stop()
	browser, err := pw.Chromium.ConnectOverCDP(f.cdpEndpoint)
	if err != nil {
		return "", fmt.Errorf("connect chrome: %w", err)
	}
	contexts := browser.Contexts()
	if len(contexts) == 0 {
		return "", errors.New("no browser context")
	}
	page, err := contexts[0].NewPage()
	if err != nil {
		return "", err
	}
	defer page.Close()
	if _, err := page.Goto(jobURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return "", err
	}
	// 滚动触发懒加载（活性/客户区块在下方）
	for i := 0; i < 5; i++ {
		page.Mouse().Wheel(0, 800)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(600 * time.Millisecond):
		}
	}
	time.Sleep(1500 * time.Millisecond)
	return page.Content()
}

// dumpPage 调试用：保存页面 HTML 到 dumpDir。
func (f *PWFetcher) dumpPage(tag, content string) {
	if f.dumpDir == "" {
		return
	}
	if err := os.MkdirAll(f.dumpDir, 0o755); err != nil {
		return
	}
	path := filepath.Join(f.dumpDir, fmt.Sprintf("dump_%s_%d.html", tag, time.Now().Unix()))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.logger.Warn("dump page failed", "err", err)
	}
}
