ALTER TABLE watch_history ADD COLUMN video_resolution TEXT DEFAULT '';
ALTER TABLE watch_history ADD COLUMN transcode_decision TEXT DEFAULT '';
CREATE INDEX idx_watch_history_resolution ON watch_history(video_resolution);
CREATE INDEX idx_watch_history_transcode ON watch_history(transcode_decision);
