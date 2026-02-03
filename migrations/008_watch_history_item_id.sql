-- Add item_id column to track media server's item identifier for clickable stats
ALTER TABLE watch_history ADD COLUMN item_id TEXT DEFAULT '';
