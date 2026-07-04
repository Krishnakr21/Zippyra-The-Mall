-- 000002_create_stores.up.sql
CREATE TABLE IF NOT EXISTS stores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    address TEXT NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100) NOT NULL,
    state_code VARCHAR(2) NOT NULL,
    pincode VARCHAR(6) NOT NULL,
    gstin VARCHAR(15) UNIQUE NOT NULL,
    gstin_verified_at TIMESTAMPTZ,
    latitude DECIMAL(10,8),
    longitude DECIMAL(11,8),
    capacity INT DEFAULT 50,
    current_occupancy INT DEFAULT 0,
    is_active BOOL DEFAULT true,
    qr_only_mode BOOL DEFAULT true,
    chain_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stores_chain_id ON stores(chain_id);
CREATE INDEX IF NOT EXISTS idx_stores_city ON stores(city);
CREATE INDEX IF NOT EXISTS idx_stores_state_code ON stores(state_code);
CREATE INDEX IF NOT EXISTS idx_stores_gstin ON stores(gstin);
