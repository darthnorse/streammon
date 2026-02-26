CREATE TABLE maintenance_exclusions_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    library_item_id INTEGER NOT NULL UNIQUE,
    excluded_by TEXT NOT NULL,
    excluded_at DATETIME NOT NULL,
    FOREIGN KEY (library_item_id) REFERENCES library_items(id) ON DELETE CASCADE
);

-- Deduplicate: keep the earliest exclusion (lowest id) per library_item_id
-- so the original excluded_by and excluded_at are preserved.
INSERT INTO maintenance_exclusions_new (library_item_id, excluded_by, excluded_at)
SELECT library_item_id, excluded_by, excluded_at
FROM maintenance_exclusions
WHERE id IN (SELECT MIN(id) FROM maintenance_exclusions GROUP BY library_item_id);

DROP TABLE maintenance_exclusions;
ALTER TABLE maintenance_exclusions_new RENAME TO maintenance_exclusions;
