# notification-service

Multi-channel notifications (FCM, SMS/Twilio, WhatsApp, Email/SES), delivery tracking, preference management. Tables: user_notification_prefs, notification_delivery_log. Subscribes to 8+ Kafka consumer groups.

## Run Locally
```bash
go run ./cmd/server  # Port 8011
```
