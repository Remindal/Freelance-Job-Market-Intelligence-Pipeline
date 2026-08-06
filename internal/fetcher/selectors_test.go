package fetcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func loadFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("tests", "fixtures", "search_page.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return string(data)
}

func TestExtractJobsFromHTML(t *testing.T) {
	jobs := ExtractJobsFromHTML(loadFixture(t), time.Now().UTC())
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d: %+v", len(jobs), jobs)
	}

	fixed := jobs[0]
	if fixed.Title != "Golang Backend Developer for API Platform" {
		t.Errorf("title mismatch: %q", fixed.Title)
	}
	// url 必须规范化：去 query 参数，收敛为 /jobs/~id 短链
	if fixed.URL != "https://www.upwork.com/jobs/~01abc123def456" {
		t.Errorf("url not normalized: %q", fixed.URL)
	}
	if fixed.Budget != "$800" {
		t.Errorf("fixed budget mismatch: %q", fixed.Budget)
	}
	if fixed.Description == "" {
		t.Error("description should be extracted")
	}
	if len(fixed.Skills) != 3 || fixed.Skills[0] != "Go" {
		t.Errorf("skills mismatch: %v", fixed.Skills)
	}

	hourly := jobs[1]
	if hourly.Budget != "$25-$50/hr" {
		t.Errorf("hourly budget mismatch: %q", hourly.Budget)
	}

	// 缺预算/技能字段的卡片：字段留空但整卡保留
	bare := jobs[2]
	if bare.Budget != "" || len(bare.Skills) != 0 {
		t.Errorf("missing fields should stay empty, got budget=%q skills=%v", bare.Budget, bare.Skills)
	}
	if bare.URL != "https://www.upwork.com/jobs/~03zzz999" {
		t.Errorf("fragment should be stripped: %q", bare.URL)
	}
}

func TestExtractJobsDedupesSameURL(t *testing.T) {
	// 同一单在不同关键词源下 slug 不同（高亮标记残留），但 ID 相同，必须去重
	html := `<html><body>
	<article data-test="JobTile"><h3><a data-test="job-tile-title-link" href="/jobs/A_~01x?a=1">Same Job</a></h3></article>
	<article data-test="JobTile"><h3><a data-test="job-tile-title-link" href="/jobs/span-class-highlight-A-span_~01x?b=2">Same Job</a></h3></article>
	</body></html>`
	jobs := ExtractJobsFromHTML(html, time.Now().UTC())
	if len(jobs) != 1 {
		t.Fatalf("same normalized url should dedupe within page, got %d", len(jobs))
	}
}

func TestIsChallengePage(t *testing.T) {
	if !IsChallengePage("Security check", "<html></html>") {
		t.Error("title containing 'Security check' should be detected")
	}
	if !IsChallengePage("Upwork", `<html><body><div class="px-captcha">verify</div></body></html>`) {
		t.Error("px-captcha element should be detected")
	}
	if IsChallengePage("Golang jobs - Upwork", loadFixture(t)) {
		t.Error("normal search page must not be flagged as challenge")
	}
}
