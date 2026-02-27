ALTER TABLE rule_violations ADD COLUMN action_taken TEXT NOT NULL DEFAULT '';

CREATE TABLE rule_exemptions (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id    INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    user_name  TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE(rule_id, user_name)
);
