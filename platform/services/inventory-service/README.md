# inventory-service

Real-time stock management, hold/release on scan, shrinkage detection, smart reorder. Tables: stock. Hypertable: inventory_movements. Critical consumers: cart.item_scanned, cart.item_removed, payment.confirmed.

## Run Locally
```bash
go run ./cmd/server  # Port 8005
```
