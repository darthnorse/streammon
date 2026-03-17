-- Fix users with empty provider who have watch history.
-- Migration 050 only targeted provider='oidc'. This catches users with provider=''
-- (created by the poller or avatar sync before the server type was passed through).

UPDATE users
SET provider = (
    SELECT s.type FROM watch_history wh
    JOIN servers s ON wh.server_id = s.id
    WHERE wh.user_name = users.name
    ORDER BY wh.started_at DESC LIMIT 1
),
    updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE provider = ''
  AND (password_hash = '' OR password_hash IS NULL)
  AND EXISTS (
    SELECT 1 FROM watch_history wh WHERE wh.user_name = users.name
  );
