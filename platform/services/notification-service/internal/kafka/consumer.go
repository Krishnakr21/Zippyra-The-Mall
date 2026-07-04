package kafka

// NotificationConsumer subscribes to 8+ topics across 8 consumer groups:
//   notif-order-cg, notif-payment-cg, notif-refund-cg, notif-loyalty-cg,
//   notif-inventory-cg, notif-auth-cg, notif-device-cg, notif-support-cg
type NotificationConsumer struct{}
