-- Add index on ip_address for faster joins with ip_geo_cache
CREATE INDEX IF NOT EXISTS idx_watch_history_ip_address ON watch_history(ip_address);
