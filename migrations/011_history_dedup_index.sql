-- Index for Tautulli import deduplication queries
CREATE INDEX IF NOT EXISTS idx_watch_history_dedup
ON watch_history(server_id, user_name, title, started_at);
