-- Add indexes to optimize stats queries with time filtering
CREATE INDEX IF NOT EXISTS idx_history_movie_time ON watch_history(title, year, started_at);
CREATE INDEX IF NOT EXISTS idx_history_tv_time ON watch_history(grandparent_title, started_at);
