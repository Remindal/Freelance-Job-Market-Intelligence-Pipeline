package filter

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Remindal/scout/internal/domain"
	"github.com/Remindal/scout/internal/llm"
)

// LLMChatter 抽象 LLM 能力，便于测试时替换为假实现。
type LLMChatter interface {
	Chat(ctx context.Context, system, user string) (string, error)
}

// ScoreResult LLM 打分结果。Tags 为短标签（如 go/webhook/需速学:stripe/否决:营销岗）。
type ScoreResult struct {
	Score  int
	Reason string
	Tags   []string
}

// Scorer LLM 精筛打分器，信号量限制并发，任何失败都不中断 pipeline。
type Scorer struct {
	client  LLMChatter
	profile string
	sem     chan struct{}
	logger  *slog.Logger
}

func NewScorer(client LLMChatter, profile string, maxConcurrent int, logger *slog.Logger) *Scorer {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	return &Scorer{
		client:  client,
		profile: profile,
		sem:     make(chan struct{}, maxConcurrent),
		logger:  logger,
	}
}

// Score 对单子打分，失败时返回 score=0 与兜底理由，永不返回 error 中断调用方。
func (s *Scorer) Score(ctx context.Context, j domain.Job) ScoreResult {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	user := llm.BuildScorePrompt(j)
	system := llm.ScoreSystemPrompt(s.profile)

	// JSON 解析失败重试 1 次
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		raw, err := s.client.Chat(ctx, system, user)
		if err != nil {
			lastErr = err
			continue
		}
		res, err := ParseScoreJSON(raw)
		if err != nil {
			lastErr = err
			continue
		}
		return res
	}

	s.logger.Warn("llm scoring failed, fallback to score=0",
		"url", j.URL, "err", lastErr)
	return ScoreResult{Score: 0, Reason: "评分失败"}
}

// ScoreBatch 并发打分一批单子，并发度由内部信号量控制；返回与输入等长的结果切片。
// onItem 非 nil 时每完成一单回调一次（done, total），用于进度展示。
func (s *Scorer) ScoreBatch(ctx context.Context, jobs []domain.Job, onItem func(done, total int)) []ScoreResult {
	results := make([]ScoreResult, len(jobs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j domain.Job) {
			defer wg.Done()
			results[i] = s.Score(ctx, j)
			if onItem != nil {
				mu.Lock()
				done++
				onItem(done, len(jobs))
				mu.Unlock()
			}
		}(i, j)
	}
	wg.Wait()
	return results
}

// codeFenceRe 兜底去掉模型偶发输出的 ```json 代码围栏。
var codeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// ParseScoreJSON 严格按 JSON 解析模型输出并做容错（围栏剥离、分数截断），禁止正则抠数字。
func ParseScoreJSON(raw string) (ScoreResult, error) {
	raw = strings.TrimSpace(raw)
	if m := codeFenceRe.FindStringSubmatch(raw); m != nil {
		raw = strings.TrimSpace(m[1])
	}

	var out struct {
		Score  int      `json:"score"`
		Reason string   `json:"reason"`
		Tags   []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return ScoreResult{}, fmt.Errorf("parse score json: %w", err)
	}
	if out.Score < 0 {
		out.Score = 0
	}
	if out.Score > 100 {
		out.Score = 100
	}
	if out.Tags == nil {
		out.Tags = []string{}
	}
	return ScoreResult{Score: out.Score, Reason: strings.TrimSpace(out.Reason), Tags: out.Tags}, nil
}

// NowUTC 供 pipeline 记录 scored_at，独立成函数便于测试替换。
var NowUTC = func() time.Time { return time.Now().UTC() }
