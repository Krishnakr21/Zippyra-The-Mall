-- 000011_create_hsn_gst_rates.up.sql
-- Needs 12,000+ rows seeded via: zippyra-admin seed-hsn
CREATE TABLE IF NOT EXISTS hsn_gst_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hsn_code VARCHAR(8) UNIQUE NOT NULL,
    description TEXT NOT NULL,
    gst_rate DECIMAL(5,2) NOT NULL,
    cgst_rate DECIMAL(5,2) NOT NULL,
    sgst_rate DECIMAL(5,2) NOT NULL,
    igst_rate DECIMAL(5,2) NOT NULL,
    cess_rate DECIMAL(5,2) DEFAULT 0,
    effective_from DATE NOT NULL,
    is_active BOOL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hsn_gst_rates_hsn_code ON hsn_gst_rates(hsn_code);
CREATE INDEX IF NOT EXISTS idx_hsn_gst_rates_gst_rate ON hsn_gst_rates(gst_rate);
