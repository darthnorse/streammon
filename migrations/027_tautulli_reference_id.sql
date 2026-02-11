ALTER TABLE watch_history ADD COLUMN tautulli_reference_id INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN enriched INTEGER DEFAULT 0;
CREATE INDEX idx_watch_history_unenriched ON watch_history (server_id) WHERE tautulli_reference_id > 0 AND enriched = 0;
