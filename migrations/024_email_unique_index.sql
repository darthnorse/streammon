-- Enforce email uniqueness at the database layer.
-- Replaces the non-unique partial index from migration 021.
DROP INDEX IF EXISTS idx_users_email;
CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE email != '';
