-- Stable identity for manual copies and a single source for automatic followers.
ALTER TABLE automations ADD COLUMN sync_key TEXT NOT NULL DEFAULT '';
ALTER TABLE automations ADD COLUMN sync_source_id BIGINT REFERENCES automations(id) ON DELETE SET NULL;
ALTER TABLE automations ADD COLUMN sync_preserve_enabled INTEGER NOT NULL DEFAULT 1;
UPDATE automations SET sync_key = 'legacy-' || CAST(id AS TEXT);
CREATE UNIQUE INDEX idx_automations_instance_sync_key ON automations(sync_key, instance_id);
CREATE INDEX idx_automations_sync_source ON automations(sync_source_id);
