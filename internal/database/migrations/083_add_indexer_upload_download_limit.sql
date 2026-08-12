ALTER TABLE torznab_indexers ADD COLUMN upload_limit_bytes INTEGER NOT NULL DEFAULT 0;
ALTER TABLE torznab_indexers ADD COLUMN download_limit_bytes INTEGER NOT NULL DEFAULT 0;