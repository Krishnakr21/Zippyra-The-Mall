# cart-service

Cart management, barcode scanning, coupon validation, GST calculation, checkout. Tables: coupons, coupon_usage, hsn_gst_rates. Hypertable: cart_scan_events. Kafka: cart.item_scanned (256 partitions — highest volume topic in system).

## Run Locally
```bash
go run ./cmd/server  # Port 8004
```
