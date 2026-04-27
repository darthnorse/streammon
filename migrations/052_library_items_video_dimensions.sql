-- Encoded video frame dimensions in pixels, populated by the next library sync.
-- Used by the Low Resolution maintenance rule when the global setting
-- maintenance.resolution_width_aware is true. Width-aware classification
-- handles cropped widescreen content (1280x688 still classifies as 720p) and
-- 21:9 sources (1920x800 still classifies as 1080p) correctly.
ALTER TABLE library_items ADD COLUMN video_width INTEGER NOT NULL DEFAULT 0;
ALTER TABLE library_items ADD COLUMN video_height INTEGER NOT NULL DEFAULT 0;
