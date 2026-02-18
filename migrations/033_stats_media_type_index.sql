CREATE INDEX IF NOT EXISTS idx_watch_history_media_started ON watch_history(media_type, started_at DESC);
