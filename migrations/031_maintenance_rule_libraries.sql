-- Add media_type to maintenance_rules
ALTER TABLE maintenance_rules ADD COLUMN media_type TEXT NOT NULL DEFAULT '';

-- Populate media_type from existing rules based on criterion_type
UPDATE maintenance_rules
SET media_type = 'movie'
WHERE criterion_type = 'unwatched_movie';

UPDATE maintenance_rules
SET media_type = 'episode'
WHERE criterion_type = 'unwatched_tv_none';

UPDATE maintenance_rules
SET media_type = COALESCE(
    (SELECT li.media_type FROM library_items li
     WHERE li.server_id = maintenance_rules.server_id
       AND li.library_id = maintenance_rules.library_id
     LIMIT 1),
    'movie'
)
WHERE criterion_type IN ('low_resolution', 'large_files');

-- Junction table: one rule can apply to many server/library pairs
CREATE TABLE IF NOT EXISTS maintenance_rule_libraries (
    rule_id INTEGER NOT NULL REFERENCES maintenance_rules(id) ON DELETE CASCADE,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    library_id TEXT NOT NULL,
    PRIMARY KEY (rule_id, server_id, library_id)
);

CREATE INDEX IF NOT EXISTS idx_mrl_server_library ON maintenance_rule_libraries(server_id, library_id);

-- Migrate existing rule-library associations into the junction table
INSERT INTO maintenance_rule_libraries (rule_id, server_id, library_id)
SELECT id, server_id, library_id FROM maintenance_rules
WHERE server_id > 0 AND library_id != '';

-- Drop the index on old columns before removing them
DROP INDEX IF EXISTS idx_maintenance_rules_server_library;

-- Remove old direct server_id/library_id columns from maintenance_rules
-- (SQLite 3.35.0+ supports ALTER TABLE DROP COLUMN)
ALTER TABLE maintenance_rules DROP COLUMN server_id;
ALTER TABLE maintenance_rules DROP COLUMN library_id;
