-- 000022_create_return_requests.up.sql
CREATE TABLE IF NOT EXISTS return_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id),
    user_id UUID NOT NULL REFERENCES users(id),
    store_id UUID NOT NULL REFERENCES stores(id),
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING_STAFF_APPROVAL',
    reason VARCHAR(100) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]',
    refund_initiated BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_return_requests_order_id ON return_requests(order_id);
CREATE INDEX IF NOT EXISTS idx_return_requests_user_id ON return_requests(user_id);

-- Make payment_id unique on orders to support ON CONFLICT (payment_id) DO NOTHING
ALTER TABLE orders ADD CONSTRAINT unique_orders_payment_id UNIQUE (payment_id);
