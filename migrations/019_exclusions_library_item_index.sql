-- Index on library_item_id for efficient CASCADE deletes when library items are removed
CREATE INDEX IF NOT EXISTS idx_exclusions_library_item ON maintenance_exclusions(library_item_id);
