-- Add machine_id to servers for secure Plex server verification
-- The machine_id (clientIdentifier) uniquely identifies a Plex server
-- and cannot be spoofed like the display name

ALTER TABLE servers ADD COLUMN machine_id TEXT DEFAULT '';

-- Index for efficient machine_id lookups
CREATE INDEX IF NOT EXISTS idx_servers_machine_id ON servers(machine_id) WHERE machine_id != '';
