-- 000004_create_products.up.sql
CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    barcode VARCHAR(50) NOT NULL,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    brand VARCHAR(255),
    category VARCHAR(255),
    hsn_code VARCHAR(8) NOT NULL,
    mrp DECIMAL(10,2) NOT NULL,
    selling_price DECIMAL(10,2) NOT NULL,
    gst_rate DECIMAL(5,2) DEFAULT 0,
    unit VARCHAR(20) DEFAULT 'piece',
    image_url TEXT,
    thumbnail_url TEXT,
    is_active BOOL DEFAULT true,
    is_returnable BOOL DEFAULT true,
    stock_quantity INT DEFAULT 0,
    reorder_point INT DEFAULT 10,
    sync_seq BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(store_id, barcode)
);

CREATE INDEX IF NOT EXISTS idx_products_store_id ON products(store_id);
CREATE INDEX IF NOT EXISTS idx_products_barcode ON products(barcode);
CREATE INDEX IF NOT EXISTS idx_products_sync_seq ON products(sync_seq);
CREATE INDEX IF NOT EXISTS idx_products_category ON products(category);
