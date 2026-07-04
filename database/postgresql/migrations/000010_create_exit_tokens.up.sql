-- 000010_create_exit_tokens.up.sql
CREATE TABLE IF NOT EXISTS exit_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    user_id UUID NOT NULL REFERENCES users(id),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    is_used BOOL DEFAULT false,
    used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_exit_tokens_token_hash ON exit_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_exit_tokens_order_id ON exit_tokens(order_id);
CREATE INDEX IF NOT EXISTS idx_exit_tokens_used_expires ON exit_tokens(is_used, expires_at);
