-- Overseerr attribution now uses the X-API-User header, so nothing reads these
-- Plex account tokens. Store.Migrate VACUUMs afterwards (vacuumAfter in
-- migrate.go) because DROP TABLE leaves the rows readable in the freelist.
-- Comments here must not contain a semicolon -- the runner splits on them.
DROP TABLE IF EXISTS provider_tokens;
DELETE FROM settings WHERE key = 'guest.store_plex_tokens';
