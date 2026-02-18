-- Remove duplicate watch_history rows caused by both the poller and Tautulli
-- recording the same viewing session with started_at within 60 seconds.
-- The NOT EXISTS clause ensures only "anchor" rows (no earlier duplicate)
-- can trigger deletions, preventing transitive chain deletions.
DELETE FROM watch_history WHERE id IN (
  SELECT w2.id
  FROM watch_history w1
  JOIN watch_history w2
    ON w1.server_id = w2.server_id
    AND w1.user_name = w2.user_name
    AND w1.title = w2.title
    AND w1.id < w2.id
    AND ABS(julianday(w1.started_at) - julianday(w2.started_at)) <= (60.0 / 86400.0)
  WHERE NOT EXISTS (
    SELECT 1 FROM watch_history w0
    WHERE w0.server_id = w1.server_id
      AND w0.user_name = w1.user_name
      AND w0.title = w1.title
      AND w0.id < w1.id
      AND ABS(julianday(w0.started_at) - julianday(w1.started_at)) <= (60.0 / 86400.0)
  )
);
