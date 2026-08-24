-- Copyright (c) 2025-2026, s0up and the autobrr contributors.
-- SPDX-License-Identifier: GPL-2.0-or-later

-- Optional ISO 3166-1 alpha-2 country code for the instance (lowercase, e.g. "cn").
-- Used to display a flag icon next to the instance name in the sidebar and lists.
ALTER TABLE instances ADD COLUMN country_code TEXT NOT NULL DEFAULT '';

-- Rebuild instances_view to expose the new column (matches migration 087).
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
