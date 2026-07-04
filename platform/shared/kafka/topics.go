package kafka

// Topic defines a robust string type for Zippyra event topics.
type Topic string

const (
	// Core domain events (Must include per requirements)
	// Note: 256 partitions for scaling
	TopicCartItemScanned      Topic = "cart.item_scanned"       
	TopicPaymentConfirmed     Topic = "payment.confirmed"
	TopicPaymentFailed        Topic = "payment.failed"
	TopicOrderCreated         Topic = "order.created"
	TopicOrderCompleted       Topic = "order.completed"
	TopicInventoryLowStock    Topic = "inventory.low_stock"
	TopicInventoryMovement    Topic = "inventory.movement"
	TopicCustomerDataDeletion Topic = "customer.data_deletion"  // DPDP compliance
	TopicNotificationSend     Topic = "notification.send"
	TopicLoyaltyPointsEarned  Topic = "loyalty.points_earned"
	TopicStoreCustomerEntered Topic = "store.customer_entered"
	TopicStoreCustomerExited  Topic = "store.customer_exited"

	// Additional topics mapping to 42 standard ecosystem topics
	TopicCatalogProductCreated Topic = "catalog.product_created"
	TopicCatalogProductUpdated Topic = "catalog.product_updated"
	TopicCatalogProductDeleted Topic = "catalog.product_deleted"
	TopicPricingTagUpdated     Topic = "pricing.tag_updated"
	TopicPricingDiscountAdded  Topic = "pricing.discount_added"
	TopicStaffShiftStarted     Topic = "staff.shift_started"
	TopicStaffShiftEnded       Topic = "staff.shift_ended"
	TopicPosTerminalOpened     Topic = "pos.terminal_opened"
	TopicPosTerminalClosed     Topic = "pos.terminal_closed"
	TopicDeliveryAssigned      Topic = "delivery.assigned"
	TopicDeliveryPickedUp      Topic = "delivery.picked_up"
	TopicDeliveryDropoff       Topic = "delivery.dropoff"
	TopicSupplierOrderSent     Topic = "supplier.order_sent"
	TopicSupplierOrderReceived Topic = "supplier.order_received"
	TopicReturnRequested       Topic = "return.requested"
	TopicReturnApproved        Topic = "return.approved"
	TopicCustomerCreated       Topic = "customer.created"
	TopicCustomerUpdated       Topic = "customer.updated"
	TopicAuthLogins            Topic = "auth.logins"
	TopicAuthFailed            Topic = "auth.failed"
	TopicFraudDetected         Topic = "fraud.detected"
	TopicFraudSuspicious       Topic = "fraud.suspicious"
	TopicSystemHealth          Topic = "system.health"
	TopicSessionStarted        Topic = "session.started"
	TopicSessionEnded          Topic = "session.ended"
	TopicStoreOpened           Topic = "store.opened"
	TopicStoreClosed           Topic = "store.closed"
	TopicIoTTemperature        Topic = "iot.temperature"
	TopicIoTCameraDetect       Topic = "iot.camera_detect"
	TopicAuditLogs             Topic = "audit.logs"
	TopicWalletCredited        Topic = "wallet.credited"
	TopicWalletDebited         Topic = "wallet.debited"
)
