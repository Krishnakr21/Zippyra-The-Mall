# Service Registry (v2.0)

All services utilize **Ed25519 JWT Auth** and are scoped by `user_type`.

| Service | Port | JWT Type | Kafka Produce/Consume | 
|---------|------|----------|-----------------------|
| auth-service | 8080 | PUBLIC | Produces `auth.otp_sent` |
| store-service | 8082 | CUSTOMER | Produces `store.session_started` |
| catalog-service | 8083 | CUSTOMER/STAFF | Multi-layer Redis cache |
| cart-service | 8084 | CUSTOMER | Produces `cart.item_scanned` (256p) |
| payment-service | 8085 | CUSTOMER | Handles Razorpay/Cashfree |
| order-service | 8086 | SYSTEM | Consumes `payment.confirmed` |
| exit-validation | 8087 | CUSTOMER | RFID / Gate integration |
| notification | 8088 | SYSTEM | Twilio / FCM / WhatsApp |
| loyalty | 8089 | CUSTOMER | Points & Tiers engine |
| inventory | 8090 | STAFF | Hold/Release logic |
| warehouse | 8091 | STAFF | GRN / Transfer logic |
| analytics | 8092 | CHAIN_HQ | ClickHouse / ML features |
| support | 8093 | ALL | Ticket / SLA mgmt |
| retailer-auth | 8094 | STAFF | Login via Device Bind |
| admin-auth | 8095 | ADMIN | 2FA mandated |
| chain-hq | 8096 | CHAIN_HQ | Multi-store reports |
| admin-store | 8097 | ADMIN | 10-step Onboarding |
| compliance | 8098 | SYSTEM | DPDPA PII Purge |
