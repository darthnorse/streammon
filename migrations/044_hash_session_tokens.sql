-- Session tokens are now stored as SHA-256 hashes instead of plaintext.
-- Existing sessions cannot be rehashed (we don't have the raw tokens),
-- so delete them all. Users will need to log in again.
DELETE FROM sessions;
