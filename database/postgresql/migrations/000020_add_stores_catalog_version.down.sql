-- 000020_add_stores_catalog_version.down.sql
ALTER TABLE stores DROP COLUMN IF EXISTS catalog_version;
