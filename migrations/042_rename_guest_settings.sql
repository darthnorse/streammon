-- Rename existing settings keys to guest.* prefix
UPDATE settings SET key = 'guest.access_enabled' WHERE key = 'auth.guest_access';
UPDATE settings SET key = 'guest.visible_trust_score' WHERE key = 'users.trust_score_visible';
UPDATE settings SET key = 'guest.show_discover' WHERE key = 'display.show_discover';
UPDATE settings SET key = 'guest.store_plex_tokens' WHERE key = 'overseerr.store_plex_tokens';
