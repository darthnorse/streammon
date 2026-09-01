-- Retire Plex token storage. Requests are attributed to Overseerr / Seerr via
-- the X-API-User header and email matching, so no code reads these tokens any
-- more. They are long-lived Plex account credentials, so drop them outright
-- rather than leave them at rest for a feature that no longer exists.
DROP TABLE IF EXISTS provider_tokens;
DELETE FROM settings WHERE key = 'guest.store_plex_tokens';
