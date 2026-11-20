-- Media-server usernames merged into a user. The poller, importers and user
-- sync resolve an alias to its user so a merge keeps applying to new plays.
CREATE TABLE user_aliases (
    alias TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_user_aliases_user_id ON user_aliases(user_id);
