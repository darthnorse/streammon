CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    email TEXT DEFAULT '',
    role TEXT DEFAULT 'viewer',
    thumb_url TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    url TEXT NOT NULL,
    api_key TEXT NOT NULL,
    enabled INTEGER DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS watch_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL REFERENCES servers(id),
    user_name TEXT NOT NULL,
    media_type TEXT NOT NULL,
    title TEXT NOT NULL,
    parent_title TEXT DEFAULT '',
    grandparent_title TEXT DEFAULT '',
    year INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    watched_ms INTEGER DEFAULT 0,
    player TEXT DEFAULT '',
    platform TEXT DEFAULT '',
    ip_address TEXT DEFAULT '',
    started_at DATETIME NOT NULL,
    stopped_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS ip_geo_cache (
    ip TEXT PRIMARY KEY,
    lat REAL DEFAULT 0,
    lng REAL DEFAULT 0,
    city TEXT DEFAULT '',
    country TEXT DEFAULT '',
    cached_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_watch_history_user ON watch_history(user_name);
CREATE INDEX IF NOT EXISTS idx_watch_history_started ON watch_history(started_at);
CREATE INDEX IF NOT EXISTS idx_watch_history_server ON watch_history(server_id);
