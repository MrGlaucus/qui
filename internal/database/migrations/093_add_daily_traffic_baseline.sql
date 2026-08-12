-- Baseline snapshot for daily traffic rows.
--
-- Each calendar day's row records the cumulative qBittorrent counters captured
-- at the first sample of that day (the day boundary baseline). The values shown
-- follow the row's data_source: session deltas use baseline_session_*, while
-- alltime/restart fallbacks use baseline_alltime_*.
ALTER TABLE instance_daily_traffic ADD COLUMN baseline_session_uploaded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance_daily_traffic ADD COLUMN baseline_session_downloaded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance_daily_traffic ADD COLUMN baseline_alltime_uploaded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance_daily_traffic ADD COLUMN baseline_alltime_downloaded INTEGER NOT NULL DEFAULT 0;
ALTER TABLE instance_daily_traffic ADD COLUMN baseline_at TIMESTAMP;
