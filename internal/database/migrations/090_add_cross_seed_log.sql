-- Cross-seed log: records all successful cross-seeds for dedup and publish-date tracking
CREATE TABLE IF NOT EXISTS cross_seed_log (
    infohash TEXT NOT NULL PRIMARY KEY,
    publish_date TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);