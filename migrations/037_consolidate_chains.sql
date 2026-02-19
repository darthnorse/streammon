-- Migration 036 only handled direct pairs (A->B). This handles full chains
-- (A->B->C->...) and overlapping sessions (negative gaps) using a recursive CTE.

-- Step 1: Build chain membership via recursive CTE.
-- Each row is mapped to its chain's anchor (the earliest entry with no predecessor).
-- Handles both gaps (0..1800s) and overlaps (negative gaps where sessions overlap).
CREATE TEMP TABLE consolidate_chains AS
WITH RECURSIVE chains(id, anchor_id, server_id, user_name, title, started_at, stopped_at) AS (
  -- Base: anchors — entries with no predecessor within 30min (including overlaps)
  SELECT w.id, w.id, w.server_id, w.user_name, w.title, w.started_at, w.stopped_at
  FROM watch_history w
  WHERE NOT EXISTS (
    SELECT 1 FROM watch_history pred
    WHERE pred.server_id = w.server_id
      AND pred.user_name = w.user_name
      AND pred.title = w.title
      AND pred.started_at < w.started_at
      AND (julianday(w.started_at) - julianday(pred.stopped_at)) * 86400 <= 1800
  )
  AND EXISTS (
    SELECT 1 FROM watch_history succ
    WHERE succ.server_id = w.server_id
      AND succ.user_name = w.user_name
      AND succ.title = w.title
      AND succ.started_at > w.started_at
      AND (julianday(succ.started_at) - julianday(w.stopped_at)) * 86400 <= 1800
  )

  UNION ALL

  -- Recursive: find the immediate next entry in the chain.
  -- "Immediate" = no other qualifying entry starts between chains.stopped_at and succ.started_at.
  SELECT succ.id, chains.anchor_id, succ.server_id, succ.user_name, succ.title,
         succ.started_at,
         -- Carry forward the MAX stopped_at so the next iteration checks the correct window
         CASE WHEN succ.stopped_at > chains.stopped_at THEN succ.stopped_at ELSE chains.stopped_at END
  FROM watch_history succ
  JOIN chains ON succ.server_id = chains.server_id
    AND succ.user_name = chains.user_name
    AND succ.title = chains.title
    AND succ.started_at > chains.started_at
    AND (julianday(succ.started_at) - julianday(chains.stopped_at)) * 86400 <= 1800
  WHERE NOT EXISTS (
    SELECT 1 FROM watch_history mid
    WHERE mid.server_id = chains.server_id
      AND mid.user_name = chains.user_name
      AND mid.title = chains.title
      AND mid.started_at > chains.started_at
      AND mid.started_at < succ.started_at
      AND (julianday(mid.started_at) - julianday(chains.stopped_at)) * 86400 <= 1800
  )
)
SELECT id, anchor_id FROM chains;

-- Step 2: Merge all non-anchor entries into their anchors
UPDATE watch_history SET
  stopped_at = COALESCE((
    SELECT MAX(w.stopped_at) FROM watch_history w
    JOIN consolidate_chains cc ON cc.id = w.id
    WHERE cc.anchor_id = watch_history.id
  ), stopped_at),
  watched_ms = watched_ms + COALESCE((
    SELECT SUM(w.watched_ms) FROM watch_history w
    JOIN consolidate_chains cc ON cc.id = w.id
    WHERE cc.anchor_id = watch_history.id AND cc.id != cc.anchor_id
  ), 0),
  paused_ms = paused_ms + COALESCE((
    SELECT SUM(w.paused_ms) FROM watch_history w
    JOIN consolidate_chains cc ON cc.id = w.id
    WHERE cc.anchor_id = watch_history.id AND cc.id != cc.anchor_id
  ), 0),
  duration_ms = MAX(duration_ms, COALESCE((
    SELECT MAX(w.duration_ms) FROM watch_history w
    JOIN consolidate_chains cc ON cc.id = w.id
    WHERE cc.anchor_id = watch_history.id AND cc.id != cc.anchor_id
  ), 0))
WHERE id IN (SELECT DISTINCT anchor_id FROM consolidate_chains);

-- Recalculate watched flag for updated anchors
UPDATE watch_history SET
  watched = CASE WHEN duration_ms > 0 AND watched_ms * 100.0 / duration_ms >= 85 THEN 1 ELSE watched END
WHERE id IN (SELECT DISTINCT anchor_id FROM consolidate_chains);

-- Step 3: Delete absorbed entries
DELETE FROM watch_history WHERE id IN (
  SELECT id FROM consolidate_chains WHERE id != anchor_id
);

DROP TABLE consolidate_chains;
