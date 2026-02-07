-- Multi-provider authentication support
-- Adds local password auth, Plex OAuth, and provider tracking

-- Add provider tracking and credentials to users
ALTER TABLE users ADD COLUMN provider TEXT NOT NULL DEFAULT 'oidc';
ALTER TABLE users ADD COLUMN provider_id TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN password_hash TEXT NOT NULL DEFAULT '';

-- Index for efficient provider lookups during login
CREATE INDEX IF NOT EXISTS idx_users_provider ON users(provider, provider_id) WHERE provider_id != '';
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email) WHERE email != '';

-- Auth configuration (Plex guest access, etc.)
-- Uses existing settings table pattern

-- Migrate existing OIDC users: set provider_id to email for account linking
UPDATE users SET provider_id = email WHERE email != '' AND provider = 'oidc';

-- Add last_used_at to sessions for activity tracking
ALTER TABLE sessions ADD COLUMN last_used_at DATETIME DEFAULT CURRENT_TIMESTAMP;
