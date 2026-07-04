# order-service

Order lifecycle, refund requests, approval workflow, invoice generation. Tables: orders (partitioned by range), order_items, refunds, refund_requests, approval_workflow.

## Run Locally
```bash
go run ./cmd/server  # Port 8006
```
