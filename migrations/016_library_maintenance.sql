-- Library items cached from media servers
CREATE TABLE IF NOT EXISTS library_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    library_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    media_type TEXT NOT NULL,
    title TEXT NOT NULL,
    year INTEGER DEFAULT 0,
    added_at DATETIME NOT NULL,
    video_resolution TEXT DEFAULT '',
    file_size INTEGER DEFAULT 0,
    episode_count INTEGER DEFAULT 0,
    thumb_url TEXT DEFAULT '',
    synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(server_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_library_items_server ON library_items(server_id);
CREATE INDEX IF NOT EXISTS idx_library_items_server_library ON library_items(server_id, library_id);
CREATE INDEX IF NOT EXISTS idx_library_items_media_type ON library_items(media_type);
CREATE INDEX IF NOT EXISTS idx_library_items_added_at ON library_items(added_at);

-- User-defined maintenance rules
CREATE TABLE IF NOT EXISTS maintenance_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    library_id TEXT NOT NULL,
    name TEXT NOT NULL,
    criterion_type TEXT NOT NULL,
    parameters TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_maintenance_rules_server_library ON maintenance_rules(server_id, library_id);

-- Pre-computed candidates flagged by rules
CREATE TABLE IF NOT EXISTS maintenance_candidates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES maintenance_rules(id) ON DELETE CASCADE,
    library_item_id INTEGER NOT NULL REFERENCES library_items(id) ON DELETE CASCADE,
    reason TEXT NOT NULL,
    computed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(rule_id, library_item_id)
);

CREATE INDEX IF NOT EXISTS idx_maintenance_candidates_rule ON maintenance_candidates(rule_id);
CREATE INDEX IF NOT EXISTS idx_maintenance_candidates_library_item ON maintenance_candidates(library_item_id);
