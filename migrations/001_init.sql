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
    scored_at   DATETIME
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_score ON jobs(status, score DESC);
