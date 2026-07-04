CREATE TABLE IF NOT EXISTS store_hours (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    day_of_week INT NOT NULL,
    opens_at VARCHAR(5) NOT NULL DEFAULT '09:00',
    closes_at VARCHAR(5) NOT NULL DEFAULT '22:00',
    is_closed BOOL NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(store_id, day_of_week)
);

CREATE INDEX IF NOT EXISTS idx_store_hours_store_id
    ON store_hours(store_id);
