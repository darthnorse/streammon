-- Schedules the VACUUM that 056 needed. Databases upgraded before this marker
-- existed applied 056 without reclaiming, so their token pages are still
-- readable. Store.Migrate clears this row once the reclaim succeeds.
INSERT OR REPLACE INTO settings (key, value) VALUES ('db.pending_vacuum', '1');
