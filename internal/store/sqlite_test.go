package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Remindal/scout/internal/domain"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func sampleJob(url string) domain.Job {
	return domain.Job{
		URL:         url,
		Title:       "Golang backend developer",
		Description: "Build a REST API in Go",
		Budget:      "$500",
		Skills:      []string{"Go", "MySQL"},
		Status:      domain.StatusNew,
		FetchedAt:   time.Now().UTC(),
	}
}

func TestInsertIfNewDedupesByURL(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	isNew, err := s.InsertIfNew(ctx, sampleJob("https://example.com/jobs/1"))
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if !isNew {
		t.Fatal("first insert should be new")
	}

	dup := sampleJob("https://example.com/jobs/1")
	dup.Title = "changed title"
	isNew, err = s.InsertIfNew(ctx, dup)
	if err != nil {
		t.Fatalf("dup insert: %v", err)
	}
	if isNew {
		t.Fatal("duplicate url must not be treated as new")
	}

	jobs, total, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(jobs) != 1 {
		t.Fatalf("expected 1 job, got len=%d total=%d", len(jobs), total)
	}
	if jobs[0].Title != "Golang backend developer" {
		t.Fatalf("duplicate insert must not overwrite, got title %q", jobs[0].Title)
	}
}

func TestRejectedJobsArePersisted(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	j := sampleJob("https://example.com/jobs/rejected-1")
	j.Status = domain.StatusRejected
	if _, err := s.InsertIfNew(ctx, j); err != nil {
		t.Fatalf("insert: %v", err)
	}

	jobs, total, err := s.List(ctx, ListFilter{Status: domain.StatusRejected})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(jobs) != 1 {
		t.Fatalf("expected rejected job to be persisted, got len=%d total=%d", len(jobs), total)
	}
}

func TestUpdateScoreAndStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.InsertIfNew(ctx, sampleJob("https://example.com/jobs/2")); err != nil {
		t.Fatalf("insert: %v", err)
	}
	stored, err := s.GetByURL(ctx, "https://example.com/jobs/2")
	if err != nil {
		t.Fatalf("get by url: %v", err)
	}
	id := stored.ID

	scoredAt := time.Now().UTC()
	if err := s.UpdateScore(ctx, id, 87, "匹配度很高", []string{"go", "api"}, scoredAt); err != nil {
		t.Fatalf("update score: %v", err)
	}
	if err := s.UpdateStatus(ctx, id, domain.StatusWant); err != nil {
		t.Fatalf("update status: %v", err)
	}

	got, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Score != 87 || got.Reason != "匹配度很高" {
		t.Fatalf("score/reason mismatch: %+v", got)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Fatalf("tags mismatch: %v", got.Tags)
	}
	if got.Status != domain.StatusWant {
		t.Fatalf("status mismatch: %s", got.Status)
	}
	if got.ScoredAt == nil || !got.ScoredAt.Equal(scoredAt.Truncate(time.Second)) {
		t.Fatalf("scored_at mismatch: %v", got.ScoredAt)
	}
	if len(got.Skills) != 2 || got.Skills[0] != "Go" {
		t.Fatalf("skills mismatch: %v", got.Skills)
	}
}

func TestListFilterSortPaginate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seed := []struct {
		url    string
		title  string
		score  int
		status domain.Status
	}{
		{"https://example.com/a", "Go crawler for ecommerce", 90, domain.StatusNew},
		{"https://example.com/b", "Python django website", 40, domain.StatusNew},
		{"https://example.com/c", "Go microservice api", 70, domain.StatusWant},
	}
	for _, item := range seed {
		j := sampleJob(item.url)
		j.Title = item.title
		j.Status = item.status
		if _, err := s.InsertIfNew(ctx, j); err != nil {
			t.Fatalf("insert %s: %v", item.url, err)
		}
		stored, _ := s.GetByURL(ctx, item.url)
		if err := s.UpdateScore(ctx, stored.ID, item.score, "", nil, time.Now().UTC()); err != nil {
			t.Fatalf("update score: %v", err)
		}
	}

	// 默认 score 降序 + total
	jobs, total, err := s.List(ctx, ListFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(jobs) != 3 || jobs[0].Score != 90 || jobs[2].Score != 40 {
		t.Fatalf("default sort broken: total=%d jobs=%+v", total, jobs)
	}

	// 升序白名单
	asc, _, err := s.List(ctx, ListFilter{Sort: "score_asc"})
	if err != nil {
		t.Fatalf("list asc: %v", err)
	}
	if asc[0].Score != 40 {
		t.Fatalf("score_asc broken: %+v", asc)
	}

	// 关键词过滤
	kw, total, err := s.List(ctx, ListFilter{Keyword: "crawler"})
	if err != nil {
		t.Fatalf("list keyword: %v", err)
	}
	if total != 1 || kw[0].Title != "Go crawler for ecommerce" {
		t.Fatalf("keyword filter broken: total=%d jobs=%+v", total, kw)
	}

	// 状态 + 最低分组合
	combo, total, err := s.List(ctx, ListFilter{Status: domain.StatusWant, MinScore: 60})
	if err != nil {
		t.Fatalf("list combo: %v", err)
	}
	if total != 1 || combo[0].Title != "Go microservice api" {
		t.Fatalf("combo filter broken: total=%d jobs=%+v", total, combo)
	}

	// 分页
	page2, total, err := s.List(ctx, ListFilter{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list paged: %v", err)
	}
	if total != 3 || len(page2) != 1 || page2[0].Score != 40 {
		t.Fatalf("pagination broken: total=%d jobs=%+v", total, page2)
	}

	// tag 过滤：给 a 单打上标签，验证精确匹配
	storedA, _ := s.GetByURL(ctx, "https://example.com/a")
	if err := s.UpdateScore(ctx, storedA.ID, 90, "", []string{"go", "webhook"}, time.Now().UTC()); err != nil {
		t.Fatalf("update tags: %v", err)
	}
	tagged, total, err := s.List(ctx, ListFilter{Tag: "webhook"})
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if total != 1 || tagged[0].URL != "https://example.com/a" {
		t.Fatalf("tag filter broken: total=%d jobs=%+v", total, tagged)
	}
	// 子串不得误命中（web 不过滤出 webhook）
	tagged, total, err = s.List(ctx, ListFilter{Tag: "web"})
	if err != nil {
		t.Fatalf("list by tag: %v", err)
	}
	if total != 0 {
		t.Fatalf("tag filter must be exact, got total=%d jobs=%+v", total, tagged)
	}
}

func TestStats(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	seed := []struct {
		url    string
		score  int
		status domain.Status
		daysAgo int
	}{
		{"https://example.com/s1", 95, domain.StatusNew, 0},
		{"https://example.com/s2", 80, domain.StatusNew, 0},
		{"https://example.com/s3", 50, domain.StatusWant, 1},
		{"https://example.com/s4", 30, domain.StatusProposed, 20}, // 超出 14 天窗口
	}
	for _, item := range seed {
		j := sampleJob(item.url)
		j.Status = item.status
		j.FetchedAt = time.Now().UTC().AddDate(0, 0, -item.daysAgo)
		if _, err := s.InsertIfNew(ctx, j); err != nil {
			t.Fatalf("insert: %v", err)
		}
		stored, _ := s.GetByURL(ctx, item.url)
		if err := s.UpdateScore(ctx, stored.ID, item.score, "", nil, time.Now().UTC()); err != nil {
			t.Fatalf("update score: %v", err)
		}
	}

	stats, err := s.Stats(ctx, 70)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats.TodayNew != 2 {
		t.Errorf("today_new = %d, want 2", stats.TodayNew)
	}
	if stats.HighScorePending != 2 {
		t.Errorf("high_score_pending = %d, want 2", stats.HighScorePending)
	}
	if stats.WantCount != 1 || stats.ProposedCount != 1 {
		t.Errorf("want/proposed = %d/%d, want 1/1", stats.WantCount, stats.ProposedCount)
	}
	if stats.StatusCounts[string(domain.StatusNew)] != 2 {
		t.Errorf("status_counts[new] = %d, want 2", stats.StatusCounts[string(domain.StatusNew)])
	}
	if len(stats.DailyNew) != 14 {
		t.Fatalf("daily_new len = %d, want 14", len(stats.DailyNew))
	}
	// 14 天窗口内共 3 单（20 天前的不计入）
	sum := 0
	for _, d := range stats.DailyNew {
		sum += d.Count
	}
	if sum != 3 {
		t.Errorf("daily_new sum = %d, want 3", sum)
	}
	if len(stats.ScoreDistribution) != 4 {
		t.Fatalf("score_distribution len = %d, want 4", len(stats.ScoreDistribution))
	}
	// 90+ 桶应只有 s1
	if stats.ScoreDistribution[3].Count != 1 {
		t.Errorf("90+ bucket = %d, want 1", stats.ScoreDistribution[3].Count)
	}
}
