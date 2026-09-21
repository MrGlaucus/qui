-- Persistent per-tracker traffic totals and per-torrent counter checkpoints.
CREATE TABLE IF NOT EXISTS tracker_daily_traffic (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id   INTEGER NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    tracker_key   TEXT    NOT NULL,
    date          TEXT    NOT NULL,
    uploaded      INTEGER NOT NULL DEFAULT 0,
    downloaded    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tracker_daily_instance_key_date
    ON tracker_daily_traffic(instance_id, tracker_key, date);

CREATE TABLE IF NOT EXISTS tracker_torrent_checkpoints (
    instance_id   INTEGER NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    torrent_hash  TEXT    NOT NULL,
    tracker_key   TEXT    NOT NULL,
    uploaded      INTEGER NOT NULL DEFAULT 0,
    downloaded    INTEGER NOT NULL DEFAULT 0,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (instance_id, torrent_hash)
);

CREATE INDEX IF NOT EXISTS idx_tracker_checkpoint_key
    ON tracker_torrent_checkpoints(instance_id, tracker_key);
