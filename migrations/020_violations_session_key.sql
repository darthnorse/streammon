-- Add session_key to rule_violations for session-based deduplication
-- This prevents duplicate violations for the same stream session even if it runs > 15 minutes
ALTER TABLE rule_violations ADD COLUMN session_key TEXT DEFAULT '';

-- Index for efficient session-based deduplication queries
CREATE INDEX IF NOT EXISTS idx_violations_session_key ON rule_violations(rule_id, user_name, session_key);

-- Index for efficient time-based deduplication fallback (when session_key is empty)
CREATE INDEX IF NOT EXISTS idx_violations_time_dedup ON rule_violations(rule_id, user_name, occurred_at);

-- Unique constraint for session-based deduplication (only when session_key is non-empty)
-- This prevents race conditions where concurrent evaluations could create duplicate violations
CREATE UNIQUE INDEX IF NOT EXISTS idx_violations_unique_session ON rule_violations(rule_id, user_name, session_key) WHERE session_key != '';
