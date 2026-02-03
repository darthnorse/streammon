-- Add thumb_url column to watch_history for stats thumbnail display
ALTER TABLE watch_history ADD COLUMN thumb_url TEXT DEFAULT '';
