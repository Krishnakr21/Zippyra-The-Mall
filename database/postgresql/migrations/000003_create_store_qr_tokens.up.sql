-- 000003_create_store_qr_tokens.up.sql
CREATE TABLE IF NOT EXISTS store_qr_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    token VARCHAR(255) UNIQUE NOT NULL,
    token_type VARCHAR(50) DEFAULT 'ENTRANCE',
    is_active BOOL DEFAULT true,
    expires_at TIMESTAMPTZ NOT NULL,
    used_count INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_store_qr_tokens_store_id ON store_qr_tokens(store_id);
CREATE INDEX IF NOT EXISTS idx_store_qr_tokens_token ON store_qr_tokens(token);
CREATE INDEX IF NOT EXISTS idx_store_qr_tokens_expires_at ON store_qr_tokens(expires_at);
