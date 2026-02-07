-- Per-rule exclusions for maintenance candidates
CREATE TABLE IF NOT EXISTS maintenance_exclusions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL,
    library_item_id INTEGER NOT NULL,
    excluded_by TEXT NOT NULL,
    excluded_at DATETIME NOT NULL,
    FOREIGN KEY (rule_id) REFERENCES maintenance_rules(id) ON DELETE CASCADE,
    FOREIGN KEY (library_item_id) REFERENCES library_items(id) ON DELETE CASCADE,
    UNIQUE(rule_id, library_item_id)
);

CREATE INDEX IF NOT EXISTS idx_exclusions_rule ON maintenance_exclusions(rule_id);
