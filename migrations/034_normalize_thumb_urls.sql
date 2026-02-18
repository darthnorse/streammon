-- Strip leading slashes from thumb_url to standardize format.
-- Some code paths stored paths like "/library/metadata/..." instead of
-- "library/metadata/..." which caused double-slash URLs in the frontend.
-- Use LTRIM to strip all leading slashes, not just one.
UPDATE watch_history SET thumb_url = LTRIM(thumb_url, '/') WHERE thumb_url LIKE '/%';
UPDATE library_items SET thumb_url = LTRIM(thumb_url, '/') WHERE thumb_url LIKE '/%';
