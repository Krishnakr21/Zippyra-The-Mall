-- 000012_create_app_versions.up.sql
-- Seed 2 rows (android + ios) via: zippyra-admin seed-app-versions
CREATE TABLE IF NOT EXISTS app_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(10) NOT NULL,
    version VARCHAR(20) NOT NULL,
    min_supported_version VARCHAR(20) NOT NULL,
    is_force_update BOOL DEFAULT false,
    release_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(platform, version)
);
