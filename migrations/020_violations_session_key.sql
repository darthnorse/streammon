-- Add session_key to rule_violations for session-based deduplication
-- This prevents duplicate violations for the same stream session even if it runs > 15 minutes
ALTER TABLE rule_violations ADD COLUMN session_key TEXT DEFAULT '';

-- Index for efficient session-based deduplication queries
CREATE INDEX IF NOT EXISTS idx_violations_session_key ON rule_violations(rule_id, user_name, session_key);
