-- Add in_flight_at for stale IN_FLIGHT recovery (existing databases).
ALTER TABLE sync_queue ADD COLUMN in_flight_at INTEGER;
