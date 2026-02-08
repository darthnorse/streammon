-- Add unique constraint on provider+provider_id to prevent duplicate linking
-- This ensures each external identity can only be linked to one user

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_provider_unique
ON users(provider, provider_id) WHERE provider_id != '';
