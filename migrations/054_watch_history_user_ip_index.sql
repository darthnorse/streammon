-- Composite index for user+IP lookups (household auto-learn, distinct-IP/device queries)
-- Avoids residual filtering on ip_address after seeking on user_name alone
CREATE INDEX IF NOT EXISTS idx_watch_history_user_ip ON watch_history(user_name, ip_address);
