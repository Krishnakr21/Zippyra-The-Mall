-- 000013_create_offer_rules.up.sql
-- DEP 7: every store must have at least one row (even inactive) to prevent Cart nil panic
CREATE TABLE IF NOT EXISTS offer_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    name VARCHAR(255) NOT NULL,
    rule_type VARCHAR(50) NOT NULL,
    conditions JSONB DEFAULT '{}',
    discount_value DECIMAL(10,2) NOT NULL,
    max_discount DECIMAL(10,2),
    is_active BOOL DEFAULT true,
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ,
    priority INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_offer_rules_store_active ON offer_rules(store_id, is_active);
CREATE INDEX IF NOT EXISTS idx_offer_rules_valid_from ON offer_rules(valid_from);
CREATE INDEX IF NOT EXISTS idx_offer_rules_valid_until ON offer_rules(valid_until);
