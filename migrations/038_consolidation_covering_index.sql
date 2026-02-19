DROP INDEX IF EXISTS idx_watch_history_consolidate;
CREATE INDEX idx_watch_history_consolidate
ON watch_history (server_id, user_name, title, stopped_at, started_at);
