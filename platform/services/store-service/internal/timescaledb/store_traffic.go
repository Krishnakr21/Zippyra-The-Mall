package timescaledb

// StoreTrafficWriter writes to store_traffic hypertable.
// Chunk interval: 1 WEEK, Space partition: store_id
type StoreTrafficWriter struct{}
