-- 000009_create_payment_webhook_events.up.sql
CREATE TABLE IF NOT EXISTS payment_webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    gateway VARCHAR(50) NOT NULL,
    event_id VARCHAR(255) UNIQUE NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    hmac_verified BOOL DEFAULT false,
    processed BOOL DEFAULT false,
    processed_at TIMESTAMPTZ,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pw_events_event_id ON payment_webhook_events(event_id);
CREATE INDEX IF NOT EXISTS idx_pw_events_gateway_processed ON payment_webhook_events(gateway, processed);
CREATE INDEX IF NOT EXISTS idx_pw_events_processed_at ON payment_webhook_events(processed_at);
