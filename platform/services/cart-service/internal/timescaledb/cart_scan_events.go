package timescaledb

// CartScanEventsWriter writes to cart_scan_events hypertable.
// Chunk interval: 1 WEEK, Space partition: store_id
type CartScanEventsWriter struct{}
