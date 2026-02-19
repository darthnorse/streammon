ALTER TABLE watch_history ADD COLUMN session_count INTEGER DEFAULT 1;

CREATE TABLE watch_sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    history_id INTEGER NOT NULL REFERENCES watch_history(id) ON DELETE CASCADE,
    duration_ms INTEGER DEFAULT 0,
    watched_ms INTEGER DEFAULT 0,
    paused_ms INTEGER DEFAULT 0,
    player TEXT DEFAULT '',
    platform TEXT DEFAULT '',
    ip_address TEXT DEFAULT '',
    started_at DATETIME NOT NULL,
    stopped_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO watch_sessions (history_id, duration_ms, watched_ms, paused_ms,
    player, platform, ip_address, started_at, stopped_at, created_at)
SELECT id, duration_ms, watched_ms, paused_ms,
    player, platform, ip_address, started_at, stopped_at, created_at
FROM watch_history;

CREATE INDEX idx_watch_sessions_history_id ON watch_sessions(history_id);
