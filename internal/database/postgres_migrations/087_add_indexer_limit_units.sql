ALTER TABLE torznab_indexers ADD COLUMN upload_limit_unit TEXT NOT NULL DEFAULT 'MB';
ALTER TABLE torznab_indexers ADD COLUMN download_limit_unit TEXT NOT NULL DEFAULT 'MB';