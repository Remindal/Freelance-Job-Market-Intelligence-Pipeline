package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Remindal/scout/internal/domain"
)

// schema 与 migrations/001_init.sql 保持一致，启动时自动执行建表。
const schema = `
CREATE TABLE IF NOT EXISTS jobs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    url         TEXT NOT NULL UNIQUE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    budget      TEXT DEFAULT '',
    skills      TEXT DEFAULT '[]',
    score       INTEGER DEFAULT 0,
    reason      TEXT DEFAULT '',
    tags        TEXT DEFAULT '[]',
    status      TEXT NOT NULL DEFAULT 'new',
    fetched_at  DATETIME NOT NULL,
    scored_at   DATETIME,
    payment_verified  INTEGER,
    client_spent_usd  REAL,
    client_rating     REAL,
    posted_at         TEXT,
    proposals_bucket  TEXT DEFAULT '',
    last_viewed_at    TEXT,
    interviewing      INTEGER,
    invites_sent      INTEGER
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_score ON jobs(status, score DESC);
`

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLite(path string) (*SQLiteStore, error) {
	// _pragma 配置 busy_timeout 避免 web 查询与写入并发时的 database is locked
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite 单机写入串行化，限制单连接避免写冲突
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// newColumns 增量列（IF NOT EXISTS 建表不会给已有表加列）。
var newColumns = []string{
	`tags TEXT DEFAULT '[]'`,
	`payment_verified INTEGER`,
	`client_spent_usd REAL`,
	`client_rating REAL`,
	`posted_at TEXT`,
	`proposals_bucket TEXT DEFAULT ''`,
	`last_viewed_at TEXT`,
	`interviewing INTEGER`,
	`invites_sent INTEGER`,
}

// migrate 对老库做增量列添加。
func migrate(db *sql.DB) error {
	existing := map[string]bool{}
	rows, err := db.Query(`PRAGMA table_info(jobs)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		existing[name] = true
	}
	for _, ddl := range newColumns {
		name := strings.SplitN(strings.TrimSpace(ddl), " ", 2)[0]
		if existing[name] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE jobs ADD COLUMN ` + ddl); err != nil {
			return fmt.Errorf("add column %s: %w", name, err)
		}
	}
	return nil
}

// jobColumns 查询列清单（与 scanJob 顺序一致）。
const jobColumns = `id, url, title, description, budget, skills, score, reason, tags, status, fetched_at, scored_at,
	payment_verified, client_spent_usd, client_rating, posted_at, proposals_bucket, last_viewed_at, interviewing, invites_sent`

func nullBool(p *bool) any {
	if p == nil {
		return nil
	}
	if *p {
		return 1
	}
	return 0
}

func nullFloat(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullTime(p *time.Time) any {
	if p == nil {
		return nil
	}
	return p.UTC().Format(time.RFC3339)
}

func (s *SQLiteStore) InsertIfNew(ctx context.Context, j domain.Job) (bool, error) {
	skills, err := json.Marshal(j.Skills)
	if err != nil {
		return false, fmt.Errorf("marshal skills: %w", err)
	}
	status := j.Status
	if status == "" {
		status = domain.StatusNew
	}
	fetchedAt := j.FetchedAt
	if fetchedAt.IsZero() {
		fetchedAt = time.Now().UTC()
	}
	// 时间统一以 RFC3339 字符串存库（UTC），避免依赖驱动的时间类型转换
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO jobs (url, title, description, budget, skills, status, fetched_at,
		 payment_verified, client_spent_usd, client_rating, posted_at, proposals_bucket)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.URL, j.Title, j.Description, j.Budget, string(skills), string(status),
		fetchedAt.UTC().Format(time.RFC3339),
		nullBool(j.PaymentVerified), nullFloat(j.ClientSpentUSD), nullFloat(j.ClientRating),
		nullTime(j.PostedAt), j.ProposalsBucket)
	if err != nil {
		return false, fmt.Errorf("insert job: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

// UpdateClientInfo 详情页复核后回写客户/活性字段。
func (s *SQLiteStore) UpdateClientInfo(ctx context.Context, id int64, j domain.Job) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET payment_verified = COALESCE(?, payment_verified),
		 client_spent_usd = COALESCE(?, client_spent_usd),
		 client_rating = COALESCE(?, client_rating),
		 proposals_bucket = COALESCE(NULLIF(?, ''), proposals_bucket),
		 last_viewed_at = COALESCE(?, last_viewed_at),
		 interviewing = COALESCE(?, interviewing),
		 invites_sent = COALESCE(?, invites_sent)
		 WHERE id = ?`,
		nullBool(j.PaymentVerified), nullFloat(j.ClientSpentUSD), nullFloat(j.ClientRating),
		j.ProposalsBucket, nullTime(j.LastViewedAt), nullInt(j.Interviewing), nullInt(j.InvitesSent), id)
	if err != nil {
		return fmt.Errorf("update client info: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateScore(ctx context.Context, id int64, score int, reason string, tags []string, scoredAt time.Time) error {
	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE jobs SET score = ?, reason = ?, tags = ?, scored_at = ? WHERE id = ?`,
		score, reason, string(tagsJSON), scoredAt.UTC().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update score: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UpdateStatus(ctx context.Context, id int64, status domain.Status) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = ? WHERE id = ?`, string(status), id)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, id int64) (*domain.Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE id = ?`, id)
	j, err := scanJob(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job %d: %w", id, err)
	}
	if err != nil {
		return nil, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

func (s *SQLiteStore) GetByURL(ctx context.Context, url string) (*domain.Job, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+jobColumns+` FROM jobs WHERE url = ?`, url)
	j, err := scanJob(row)
	if err != nil {
		return nil, fmt.Errorf("get job by url: %w", err)
	}
	return j, nil
}

func (s *SQLiteStore) List(ctx context.Context, f ListFilter) ([]domain.Job, int, error) {
	where, args := buildWhere(f)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count jobs: %w", err)
	}

	// sort 白名单映射为固定 SQL 片段，绝不拼接外部输入
	var orderBy string
	switch f.Sort {
	case "score_asc":
		orderBy = ` ORDER BY score ASC, fetched_at DESC`
	case "fetched_desc":
		orderBy = ` ORDER BY fetched_at DESC`
	default:
		orderBy = ` ORDER BY score DESC, fetched_at DESC`
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT ` + jobColumns + ` FROM jobs` +
		where + orderBy + ` LIMIT ? OFFSET ?`
	args = append(args, limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, *j)
	}
	return jobs, total, rows.Err()
}

// buildWhere 拼装 WHERE 子句与参数，全部走占位符。
func buildWhere(f ListFilter) (string, []any) {
	var conds []string
	var args []any
	if f.Status != "" {
		conds = append(conds, `status = ?`)
		args = append(args, string(f.Status))
	}
	if f.MinScore > 0 {
		conds = append(conds, `score >= ?`)
		args = append(args, f.MinScore)
	}
	if f.Keyword != "" {
		conds = append(conds, `(title LIKE ? OR description LIKE ?)`)
		kw := "%" + f.Keyword + "%"
		args = append(args, kw, kw)
	}
	if f.Tag != "" {
		// tags 是 JSON 数组文本，用带引号的子串匹配实现精确 tag 过滤
		conds = append(conds, `tags LIKE ?`)
		args = append(args, `%"`+f.Tag+`"%`)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return ` WHERE ` + strings.Join(conds, ` AND `), args
}

func (s *SQLiteStore) Stats(ctx context.Context, highScoreThreshold int) (*Stats, error) {
	stats := &Stats{StatusCounts: map[string]int{}}

	// 今日新单：fetched_at 为 RFC3339 UTC 文本，字符串比较即可
	todayStart := time.Now().UTC().Truncate(24 * time.Hour).Format(time.RFC3339)
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM jobs WHERE fetched_at >= ?`, todayStart).Scan(&stats.TodayNew); err != nil {
		return nil, fmt.Errorf("stats today_new: %w", err)
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM jobs WHERE status = ? AND score >= ?`,
		string(domain.StatusNew), highScoreThreshold).Scan(&stats.HighScorePending); err != nil {
		return nil, fmt.Errorf("stats high_score_pending: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `SELECT status, COUNT(*) FROM jobs GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("stats status_counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("stats status scan: %w", err)
		}
		stats.StatusCounts[status] = count
		switch domain.Status(status) {
		case domain.StatusWant:
			stats.WantCount = count
		case domain.StatusProposed:
			stats.ProposedCount = count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 近 14 天每日新增（含无数据的日期补零）
	since := time.Now().UTC().AddDate(0, 0, -13).Truncate(24*time.Hour).Format(time.RFC3339)
	dailyRows, err := s.db.QueryContext(ctx,
		`SELECT substr(fetched_at, 1, 10) AS d, COUNT(*) FROM jobs WHERE fetched_at >= ? GROUP BY d`, since)
	if err != nil {
		return nil, fmt.Errorf("stats daily_new: %w", err)
	}
	byDate := map[string]int{}
	for dailyRows.Next() {
		var d string
		var c int
		if err := dailyRows.Scan(&d, &c); err != nil {
			dailyRows.Close()
			return nil, fmt.Errorf("stats daily scan: %w", err)
		}
		byDate[d] = c
	}
	dailyRows.Close()
	for i := 13; i >= 0; i-- {
		day := time.Now().UTC().AddDate(0, 0, -i).Format("2006-01-02")
		stats.DailyNew = append(stats.DailyNew, DailyCount{Date: day, Count: byDate[day]})
	}

	// 分数分布桶，与前端 ScoreBadge 分档一致
	var low, mid, high, top int
	var scanErr error
	scan := func(dest *int, query string, args ...any) {
		if scanErr != nil {
			return
		}
		scanErr = s.db.QueryRowContext(ctx, query, args...).Scan(dest)
	}
	scan(&low, `SELECT COUNT(*) FROM jobs WHERE scored_at IS NOT NULL AND score < 40`)
	scan(&mid, `SELECT COUNT(*) FROM jobs WHERE scored_at IS NOT NULL AND score >= 40 AND score < 70`)
	scan(&high, `SELECT COUNT(*) FROM jobs WHERE scored_at IS NOT NULL AND score >= 70 AND score < 90`)
	scan(&top, `SELECT COUNT(*) FROM jobs WHERE scored_at IS NOT NULL AND score >= 90`)
	if scanErr != nil {
		return nil, fmt.Errorf("stats score_distribution: %w", scanErr)
	}
	stats.ScoreDistribution = []ScoreBucket{
		{Label: "<40", Count: low},
		{Label: "40-69", Count: mid},
		{Label: "70-89", Count: high},
		{Label: "90+", Count: top},
	}
	return stats, nil
}

func (s *SQLiteStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	// 用户标记过（想投/已投）的单永久保留，只清理新单/已淘汰/忽略/死帖
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM jobs WHERE fetched_at < ? AND status NOT IN (?, ?)`,
		cutoff.UTC().Format(time.RFC3339), string(domain.StatusWant), string(domain.StatusProposed))
	if err != nil {
		return 0, fmt.Errorf("delete old jobs: %w", err)
	}
	return res.RowsAffected()
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// scanner 抽象 *sql.Row 与 *sql.Rows 共有的 Scan 方法。
type scanner interface {
	Scan(dest ...any) error
}

func scanJob(s scanner) (*domain.Job, error) {
	var j domain.Job
	var skills, tags string
	var status string
	var fetchedAt string
	var scoredAt sql.NullString
	var paymentVerified sql.NullInt64
	var spent, rating sql.NullFloat64
	var postedAt, lastViewedAt sql.NullString
	var interviewing, invitesSent sql.NullInt64
	if err := s.Scan(&j.ID, &j.URL, &j.Title, &j.Description, &j.Budget,
		&skills, &j.Score, &j.Reason, &tags, &status, &fetchedAt, &scoredAt,
		&paymentVerified, &spent, &rating, &postedAt, &j.ProposalsBucket,
		&lastViewedAt, &interviewing, &invitesSent); err != nil {
		return nil, err
	}
	if skills != "" {
		// 历史脏数据容错：反序列化失败按空技能处理
		_ = json.Unmarshal([]byte(skills), &j.Skills)
	}
	// 旧数据无 tags 字段时按空数组处理
	j.Tags = []string{}
	if tags != "" {
		_ = json.Unmarshal([]byte(tags), &j.Tags)
	}
	j.Status = domain.Status(status)
	if t, err := time.Parse(time.RFC3339, fetchedAt); err == nil {
		j.FetchedAt = t.UTC()
	}
	if scoredAt.Valid {
		if t, err := time.Parse(time.RFC3339, scoredAt.String); err == nil {
			t = t.UTC()
			j.ScoredAt = &t
		}
	}
	if paymentVerified.Valid {
		v := paymentVerified.Int64 == 1
		j.PaymentVerified = &v
	}
	if spent.Valid {
		j.ClientSpentUSD = &spent.Float64
	}
	if rating.Valid {
		j.ClientRating = &rating.Float64
	}
	if postedAt.Valid {
		if t, err := time.Parse(time.RFC3339, postedAt.String); err == nil {
			t = t.UTC()
			j.PostedAt = &t
		}
	}
	if lastViewedAt.Valid {
		if t, err := time.Parse(time.RFC3339, lastViewedAt.String); err == nil {
			t = t.UTC()
			j.LastViewedAt = &t
		}
	}
	if interviewing.Valid {
		v := int(interviewing.Int64)
		j.Interviewing = &v
	}
	if invitesSent.Valid {
		v := int(invitesSent.Int64)
		j.InvitesSent = &v
	}
	return &j, nil
}
