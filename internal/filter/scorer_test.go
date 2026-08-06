package filter

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	"upwork-scout/internal/domain"
)

func TestParseScoreJSON(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		want    ScoreResult
		wantErr bool
	}{
		{"plain", `{"score": 85, "reason": "匹配", "tags": ["go", "webhook"]}`, ScoreResult{85, "匹配", []string{"go", "webhook"}}, false},
		{"no tags field", `{"score": 85, "reason": "匹配"}`, ScoreResult{85, "匹配", []string{}}, false},
		{"fenced", "```json\n{\"score\": 72, \"reason\": \"还行\", \"tags\": [\"需速学:stripe\"]}\n```", ScoreResult{72, "还行", []string{"需速学:stripe"}}, false},
		{"clamp high", `{"score": 150, "reason": "x"}`, ScoreResult{100, "x", []string{}}, false},
		{"clamp low", `{"score": -5, "reason": "x"}`, ScoreResult{0, "x", []string{}}, false},
		{"garbage", "score is about 80 I think", ScoreResult{}, true},
		{"empty", "", ScoreResult{}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseScoreJSON(c.raw)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && !scoreResultEqual(got, c.want) {
				t.Fatalf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func scoreResultEqual(a, b ScoreResult) bool {
	if a.Score != b.Score || a.Reason != b.Reason || len(a.Tags) != len(b.Tags) {
		return false
	}
	for i := range a.Tags {
		if a.Tags[i] != b.Tags[i] {
			return false
		}
	}
	return true
}

type fakeLLM struct {
	responses []string
	errs      []error
	calls     atomic.Int32
}

func (f *fakeLLM) Chat(ctx context.Context, system, user string) (string, error) {
	n := int(f.calls.Add(1)) - 1
	if n < len(f.errs) && f.errs[n] != nil {
		return "", f.errs[n]
	}
	if n < len(f.responses) {
		return f.responses[n], nil
	}
	return "", errors.New("no more fake responses")
}

func newTestScorer(client LLMChatter) *Scorer {
	return NewScorer(client, "测试画像", 3, slog.Default())
}

var testJob = domain.Job{URL: "https://example.com/j/1", Title: "Go backend", Description: "build api"}

func TestScorerParsesValidResponse(t *testing.T) {
	s := newTestScorer(&fakeLLM{responses: []string{`{"score": 88, "reason": "高度匹配", "tags": ["go", "api"]}`}})
	got := s.Score(context.Background(), testJob)
	if got.Score != 88 || got.Reason != "高度匹配" || len(got.Tags) != 2 || got.Tags[0] != "go" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestScorerRetriesOnceOnBadJSON(t *testing.T) {
	fake := &fakeLLM{responses: []string{"not json", `{"score": 60, "reason": "重试成功"}`}}
	s := newTestScorer(fake)
	got := s.Score(context.Background(), testJob)
	if got.Score != 60 {
		t.Fatalf("expected retry to succeed, got %+v", got)
	}
	if fake.calls.Load() != 2 {
		t.Fatalf("expected exactly 2 calls, got %d", fake.calls.Load())
	}
}

func TestScorerNeverFailsPipeline(t *testing.T) {
	fake := &fakeLLM{errs: []error{errors.New("boom"), errors.New("boom again")}}
	s := newTestScorer(fake)
	got := s.Score(context.Background(), testJob)
	if got.Score != 0 || got.Reason != "评分失败" {
		t.Fatalf("expected fallback score=0, got %+v", got)
	}
	if fake.calls.Load() != 2 {
		t.Fatalf("expected 1 retry (2 calls total), got %d", fake.calls.Load())
	}
}

func TestScorerBatchMatchesInputOrder(t *testing.T) {
	fake := &fakeLLM{responses: []string{
		`{"score": 10, "reason": "a"}`,
		`{"score": 20, "reason": "b"}`,
		`{"score": 30, "reason": "c"}`,
	}}
	s := newTestScorer(fake)
	jobs := []domain.Job{{URL: "1"}, {URL: "2"}, {URL: "3"}}
	var progressCalls int
	results := s.ScoreBatch(context.Background(), jobs, func(done, total int) {
		progressCalls++
		if total != 3 || done < 1 || done > 3 {
			t.Errorf("bad progress args: %d/%d", done, total)
		}
	})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if progressCalls != 3 {
		t.Fatalf("expected 3 progress callbacks, got %d", progressCalls)
	}
	for _, r := range results {
		if r.Score == 0 && r.Reason != "评分失败" {
			t.Fatalf("unexpected zero score: %+v", results)
		}
	}
}
