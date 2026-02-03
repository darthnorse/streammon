-- Add season and episode number columns to watch_history
ALTER TABLE watch_history ADD COLUMN season_number INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN episode_number INTEGER DEFAULT 0;
