-- Add grandparent_item_id column to store series ID for TV episodes
-- For movies, this will be empty. For TV episodes, this stores the series ID.
ALTER TABLE watch_history ADD COLUMN grandparent_item_id TEXT DEFAULT '';
