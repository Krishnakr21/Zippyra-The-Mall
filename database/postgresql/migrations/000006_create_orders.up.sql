-- 000006_create_orders.up.sql
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number VARCHAR(50) UNIQUE NOT NULL,
    user_id UUID NOT NULL REFERENCES users(id),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    session_id UUID NOT NULL,
    status VARCHAR(50) DEFAULT 'PENDING',
    supply_type VARCHAR(20) DEFAULT 'intrastate',
    subtotal DECIMAL(10,2) NOT NULL,
    gst_total DECIMAL(10,2) DEFAULT 0,
    cgst_total DECIMAL(10,2) DEFAULT 0,
    sgst_total DECIMAL(10,2) DEFAULT 0,
    igst_total DECIMAL(10,2) DEFAULT 0,
    discount_total DECIMAL(10,2) DEFAULT 0,
    total_amount DECIMAL(10,2) NOT NULL,
    payment_method VARCHAR(20) DEFAULT 'UPI',
    payment_id UUID,
    irn VARCHAR(64),
    irn_ack_no VARCHAR(20),
    irn_ack_date TIMESTAMPTZ,
    exit_token TEXT,
    exit_token_expires_at TIMESTAMPTZ,
    invoice_url TEXT,
    return_window_ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_user_id_created_at ON orders(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_store_id_created_at ON orders(store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);
CREATE INDEX IF NOT EXISTS idx_orders_order_number ON orders(order_number);
CREATE INDEX IF NOT EXISTS idx_orders_payment_id ON orders(payment_id);
