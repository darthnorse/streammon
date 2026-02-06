-- Audit log for maintenance candidate deletions
CREATE TABLE IF NOT EXISTS maintenance_delete_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    item_id TEXT NOT NULL,
    title TEXT NOT NULL,
    media_type TEXT NOT NULL,
    file_size INTEGER DEFAULT 0,
    deleted_by TEXT NOT NULL,
    deleted_at DATETIME NOT NULL,
    server_deleted INTEGER DEFAULT 0,
    error_message TEXT DEFAULT '',
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_delete_log_server ON maintenance_delete_log(server_id);
CREATE INDEX IF NOT EXISTS idx_delete_log_deleted_at ON maintenance_delete_log(deleted_at);
CREATE INDEX IF NOT EXISTS idx_delete_log_deleted_by ON maintenance_delete_log(deleted_by);
