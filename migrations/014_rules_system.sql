-- Rules definitions
CREATE TABLE IF NOT EXISTS rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    config TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rules_type ON rules(type);
CREATE INDEX IF NOT EXISTS idx_rules_enabled ON rules(enabled);

-- Rule violations (kept forever for reporting)
CREATE TABLE IF NOT EXISTS rule_violations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    user_name TEXT NOT NULL,
    severity TEXT NOT NULL,
    message TEXT NOT NULL,
    details TEXT DEFAULT '{}',
    confidence_score REAL DEFAULT 0,
    occurred_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_violations_user ON rule_violations(user_name);
CREATE INDEX IF NOT EXISTS idx_violations_rule ON rule_violations(rule_id);
CREATE INDEX IF NOT EXISTS idx_violations_occurred ON rule_violations(occurred_at);
CREATE INDEX IF NOT EXISTS idx_violations_severity ON rule_violations(severity);

-- Household locations (trusted locations per user)
CREATE TABLE IF NOT EXISTS household_locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_name TEXT NOT NULL,
    ip_address TEXT DEFAULT '',
    city TEXT DEFAULT '',
    country TEXT DEFAULT '',
    latitude REAL DEFAULT 0,
    longitude REAL DEFAULT 0,
    auto_learned INTEGER DEFAULT 0,
    trusted INTEGER DEFAULT 1,
    session_count INTEGER DEFAULT 1,
    first_seen DATETIME NOT NULL,
    last_seen DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_name, ip_address, city, country)
);

CREATE INDEX IF NOT EXISTS idx_household_user ON household_locations(user_name);
CREATE INDEX IF NOT EXISTS idx_household_trusted ON household_locations(trusted);

-- User trust scores (0-100)
CREATE TABLE IF NOT EXISTS user_trust_scores (
    user_name TEXT PRIMARY KEY,
    score INTEGER DEFAULT 100,
    violation_count INTEGER DEFAULT 0,
    last_violation_at DATETIME,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Notification channel settings
CREATE TABLE IF NOT EXISTS notification_channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    config TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notif_type ON notification_channels(channel_type);
CREATE INDEX IF NOT EXISTS idx_notif_enabled ON notification_channels(enabled);

-- Rule-to-notification channel mapping (per-rule notifications)
CREATE TABLE IF NOT EXISTS rule_notifications (
    rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    PRIMARY KEY (rule_id, channel_id)
);

-- Library maintenance tasks (pending deletions requiring confirmation)
CREATE TABLE IF NOT EXISTS maintenance_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    action_type TEXT NOT NULL,
    item_id TEXT NOT NULL,
    title TEXT NOT NULL,
    details TEXT DEFAULT '{}',
    status TEXT DEFAULT 'pending',
    confirmed_by TEXT DEFAULT '',
    confirmed_at DATETIME,
    executed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_maintenance_status ON maintenance_tasks(status);
CREATE INDEX IF NOT EXISTS idx_maintenance_rule ON maintenance_tasks(rule_id);
