-- 000014_create_devices.up.sql
-- DEP 4: if no RFID_PAD for store_id → qr_only_mode = true
CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    store_id UUID NOT NULL REFERENCES stores(id), -- store_id is the universal partition key
    device_type VARCHAR(50) NOT NULL,
    serial_number VARCHAR(255) UNIQUE NOT NULL,
    firmware_version VARCHAR(50),
    is_online BOOL DEFAULT false,
    last_heartbeat_at TIMESTAMPTZ,
    ip_address INET,
    mac_address MACADDR,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_devices_store_type ON devices(store_id, device_type);
CREATE INDEX IF NOT EXISTS idx_devices_is_online ON devices(is_online);
CREATE INDEX IF NOT EXISTS idx_devices_serial_number ON devices(serial_number);
