package timescaledb

// RFIDScanEventsWriter writes to rfid_scan_events hypertable.
// Chunk interval: 1 DAY, Space partition: store_id
// Peak volume: 50–200 tags/sec per store
type RFIDScanEventsWriter struct{}
