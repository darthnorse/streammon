CREATE INDEX IF NOT EXISTS idx_watch_history_consolidate
ON watch_history (server_id, user_name, title, stopped_at);

-- Build a mapping of (anchor_id -> absorbed_id) for direct pairs only.
-- Each anchor is the earliest row in a potential chain (no predecessor within 30min).
-- Each absorbed_id is the FIRST successor within 30min of the anchor's stopped_at.
-- This guarantees we only delete rows whose data was actually merged.
CREATE TEMP TABLE consolidate_pairs AS
SELECT w1.id AS anchor_id, (
  SELECT w2.id FROM watch_history w2
  WHERE w2.server_id = w1.server_id
    AND w2.user_name = w1.user_name
    AND w2.title = w1.title
    AND w2.id > w1.id
    AND (julianday(w2.started_at) - julianday(w1.stopped_at)) * 86400 BETWEEN 0 AND 1800
  ORDER BY w2.started_at ASC LIMIT 1
) AS absorbed_id
FROM watch_history w1
WHERE NOT EXISTS (
  SELECT 1 FROM watch_history w0
  WHERE w0.server_id = w1.server_id
    AND w0.user_name = w1.user_name
    AND w0.title = w1.title
    AND w0.id < w1.id
    AND (julianday(w1.started_at) - julianday(w0.stopped_at)) * 86400 BETWEEN 0 AND 1800
)
AND EXISTS (
  SELECT 1 FROM watch_history w2
  WHERE w2.server_id = w1.server_id
    AND w2.user_name = w1.user_name
    AND w2.title = w1.title
    AND w2.id > w1.id
    AND (julianday(w2.started_at) - julianday(w1.stopped_at)) * 86400 BETWEEN 0 AND 1800
);

-- Merge absorbed row data into each anchor
UPDATE watch_history SET
  stopped_at = (SELECT a.stopped_at FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id),
  watched_ms = watched_ms + (SELECT a.watched_ms FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id),
  paused_ms = paused_ms + (SELECT a.paused_ms FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id),
  duration_ms = MAX(duration_ms, (SELECT a.duration_ms FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id)),
  watched = CASE
    WHEN duration_ms > 0
      AND (watched_ms + (SELECT a.watched_ms FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id)) * 100.0
        / MAX(duration_ms, (SELECT a.duration_ms FROM watch_history a JOIN consolidate_pairs cp ON cp.absorbed_id = a.id WHERE cp.anchor_id = watch_history.id)) >= 85
    THEN 1 ELSE watched END
WHERE id IN (SELECT anchor_id FROM consolidate_pairs);

-- Delete only the specifically absorbed rows
DELETE FROM watch_history WHERE id IN (SELECT absorbed_id FROM consolidate_pairs);

DROP TABLE consolidate_pairs;
