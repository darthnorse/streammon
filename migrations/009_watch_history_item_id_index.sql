-- Add index on item_id for future queries filtering by item
CREATE INDEX IF NOT EXISTS idx_watch_history_item_id ON watch_history(item_id);
