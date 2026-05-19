-- Add partial index for cross-server item-level history queries
-- Indexes (server_id, grandparent_item_id) where grandparent_item_id is not empty
-- Used by cross-server show/season/episode history aggregation queries
CREATE INDEX IF NOT EXISTS idx_watch_history_server_grandparent
    ON watch_history(server_id, grandparent_item_id)
    WHERE grandparent_item_id != '';
