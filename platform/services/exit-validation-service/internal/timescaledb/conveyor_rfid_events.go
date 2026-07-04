package timescaledb

// ConveyorRFIDWriter writes to conveyor_rfid_events hypertable (Phase 3).
// Extremely high frequency: 500+ tags/sec at conveyor speed.
type ConveyorRFIDWriter struct{}
