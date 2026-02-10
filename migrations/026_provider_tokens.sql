CREATE TABLE IF NOT EXISTS provider_tokens (
    user_id    INTEGER NOT NULL,
    provider   TEXT    NOT NULL,
    token      TEXT    NOT NULL,
    updated_at TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    PRIMARY KEY (user_id, provider),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
