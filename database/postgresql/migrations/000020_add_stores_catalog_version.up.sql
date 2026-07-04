-- 000020_add_stores_catalog_version.up.sql
ALTER TABLE stores ADD COLUMN IF NOT EXISTS catalog_version INTEGER NOT NULL DEFAULT 1;
