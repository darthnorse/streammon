-- Composite index for efficient play-count lookups per (server_id, item_id)
CREATE INDEX IF NOT EXISTS idx_watch_history_server_item ON watch_history(server_id, item_id);
