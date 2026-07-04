# payment-service

Multi-gateway payments (Razorpay, Cashfree, PayU), webhook processing, split payments, reconciliation. Tables: payments, tokenized_cards, reconciliation_log, payment_audit_log (immutable). Hypertable: payment_analytics_events (60-month retention — RBI compliance).

## Run Locally
```bash
go run ./cmd/server  # Port 8007
```
