-- Daily traffic statistics per instance.
-- Tracked in the UI timezone; date is stored as '2006-01-02'.
CREATE TABLE IF NOT EXISTS instance_daily_traffic (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id   INTEGER NOT NULL REFERENCES instances(id) ON DELETE CASCADE,
    date          TEXT    NOT NULL,
    uploaded      INTEGER NOT NULL DEFAULT 0,
    downloaded    INTEGER NOT NULL DEFAULT 0,
    peak_ul_speed INTEGER NOT NULL DEFAULT 0,
    peak_dl_speed INTEGER NOT NULL DEFAULT 0,
    data_source   TEXT    NOT NULL DEFAULT 'session',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_instance_daily_date ON instance_daily_traffic(instance_id, date);

-- Per-instance opt-in toggle for daily traffic collection.
ALTER TABLE instances ADD COLUMN daily_traffic_enabled BOOLEAN NOT NULL DEFAULT 1;

DROP VIEW IF EXISTS instances_view;
CREATE VIEW instances_view AS
SELECT
    i.id,
    n.value AS name,
    h.value AS host,
    u.value AS username,
    i.password_encrypted,
    i.api_key_encrypted,
    bu.value AS basic_username,
    i.basic_password_encrypted,
    i.tls_skip_verify,
    i.sort_order,
    i.is_active,
    i.has_local_filesystem_access,
    i.use_hardlinks,
    i.hardlink_base_dir,
    i.hardlink_dir_preset,
    i.use_reflinks,
    i.fallback_to_regular_mode,
    i.daily_traffic_enabled,
    i.country_code
FROM instances i
LEFT JOIN string_pool n ON i.name_id = n.id
LEFT JOIN string_pool h ON i.host_id = h.id
LEFT JOIN string_pool u ON i.username_id = u.id
LEFT JOIN string_pool bu ON i.basic_username_id = bu.id;
