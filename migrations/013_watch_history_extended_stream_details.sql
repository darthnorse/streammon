-- Extended stream details for quality analytics
ALTER TABLE watch_history ADD COLUMN video_codec TEXT DEFAULT '';
ALTER TABLE watch_history ADD COLUMN audio_codec TEXT DEFAULT '';
ALTER TABLE watch_history ADD COLUMN audio_channels INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN bandwidth INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN video_decision TEXT DEFAULT '';
ALTER TABLE watch_history ADD COLUMN audio_decision TEXT DEFAULT '';
ALTER TABLE watch_history ADD COLUMN transcode_hw_decode INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN transcode_hw_encode INTEGER DEFAULT 0;
ALTER TABLE watch_history ADD COLUMN dynamic_range TEXT DEFAULT '';

-- Indexes for common distribution queries
CREATE INDEX idx_watch_history_video_codec ON watch_history(video_codec);
CREATE INDEX idx_watch_history_dynamic_range ON watch_history(dynamic_range);
