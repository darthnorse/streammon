-- Fix users incorrectly marked as 'oidc' who actually use plex/emby/jellyfin.
-- Migration 021 set DEFAULT 'oidc' for the provider column, so pre-existing users
-- and users created via avatar sync/imports got the wrong provider value.

-- Step 1: Fix users who have a provider token (strongest signal).
-- Keep provider_id intact since it was set correctly during the original login.
UPDATE users
SET provider = pt.provider,
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
FROM provider_tokens pt
WHERE users.id = pt.user_id
  AND users.provider = 'oidc'
  AND pt.provider IN ('plex', 'emby', 'jellyfin');

-- Step 2: Fix remaining 'oidc' users based on which server type they stream from.
-- Uses the most recent watch history entry to determine the server type.
UPDATE users
SET provider = (
    SELECT s.type FROM watch_history wh
    JOIN servers s ON wh.server_id = s.id
    WHERE wh.user_name = users.name
    ORDER BY wh.started_at DESC LIMIT 1
),
    provider_id = '',
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE provider = 'oidc'
  AND (password_hash = '' OR password_hash IS NULL)
  AND EXISTS (
    SELECT 1 FROM watch_history wh WHERE wh.user_name = users.name
  );

-- Step 3: Clear any remaining 'oidc' users with no tokens, no password, and no history.
UPDATE users
SET provider = '',
    provider_id = '',
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE provider = 'oidc'
  AND provider_id = ''
  AND (password_hash = '' OR password_hash IS NULL);
