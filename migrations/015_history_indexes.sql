-- Indexes for rule evaluator queries
CREATE INDEX IF NOT EXISTS idx_history_user_time
    ON watch_history(user_name, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_history_device
    ON watch_history(user_name, player, platform, started_at DESC);

-- Simpler index for device existence checks (HasDeviceBeenUsed)
CREATE INDEX IF NOT EXISTS idx_history_device_lookup
    ON watch_history(user_name, player, platform);
