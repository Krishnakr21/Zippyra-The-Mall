package timescaledb

// InventoryMovementsWriter writes to inventory_movements hypertable.
// Chunk interval: 1 WEEK, Space partition: store_id (4 partitions)
// Retention: 24 months hot → S3 Glacier archive
type InventoryMovementsWriter struct{}
