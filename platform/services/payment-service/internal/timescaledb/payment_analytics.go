package timescaledb

// PaymentAnalyticsWriter writes to payment_analytics_events hypertable.
// Chunk interval: 1 MONTH, Space partition: gateway
// Retention: 60 months (RBI/SEBI compliance)
type PaymentAnalyticsWriter struct{}
