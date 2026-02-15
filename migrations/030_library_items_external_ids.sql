ALTER TABLE library_items ADD COLUMN tmdb_id TEXT DEFAULT '';
ALTER TABLE library_items ADD COLUMN tvdb_id TEXT DEFAULT '';
ALTER TABLE library_items ADD COLUMN imdb_id TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_library_items_tmdb ON library_items(tmdb_id) WHERE tmdb_id != '';
CREATE INDEX IF NOT EXISTS idx_library_items_tvdb ON library_items(tvdb_id) WHERE tvdb_id != '';
CREATE INDEX IF NOT EXISTS idx_library_items_imdb ON library_items(imdb_id) WHERE imdb_id != '';
