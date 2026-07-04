# Key Data Flows

## Scan-to-Payment
1. User barcode scan → Cart Service → Kafka: cart.item_scanned (256 partitions)
2. Inventory Service consumes → HOLD stock
3. Checkout → Payment Service → Gateway (Razorpay/Cashfree)
4. payment.confirmed → Order created → Stock deducted → Loyalty credited
5. Exit token generated → Notification sent

## Exit Gate (RFID)
1. RFID antenna scans all items → MQTT → Exit Validation Service
2. Match tags against order_items → gate opens / alarm triggers
3. Order marked COMPLETED
