package kafka

// =============================================================================
// ZIPPYRA — Kafka Topic Registry
// Complete topic constants for all 3 parts of the system
// 42 topics total: 28 customer app + 8 retailer/warehouse + 6 admin/platform
// Version 1.0 | March 2026 | Confidential
//
// RULES:
//   1. Topic names follow convention: {domain}.{event_name}
//   2. Every topic has a single producer — never multiple producers on one topic
//   3. Events are immutable once published — never update/delete a Kafka message
//   4. All financial topics (payment.*, order.*) retain for 30 days minimum
//   5. Partition key must be documented — wrong keys break ordering guarantees
// =============================================================================

// ─── DOMAIN: auth ────────────────────────────────────────────────────────────
const (
	TopicAuthOTPSent        = "auth.otp_sent"        // Producer: Auth | Part 1
	TopicAuthLoginSuccess   = "auth.login_success"   // Producer: Auth | Part 1
	TopicAuthLoginFailed    = "auth.login_failed"    // Producer: Auth | Part 1
	TopicAuthAccountLocked  = "auth.account_locked"  // Producer: Auth | Part 1
	TopicAuthSessionCreated = "auth.session_created" // Producer: Auth | Part 1
	TopicAuthSessionEnded   = "auth.session_ended"   // Producer: Auth | Part 1
	TopicAuthStaffLogin     = "auth.staff_login"     // Producer: Auth | Part 2 (retailer staff)
)

// ─── DOMAIN: account ─────────────────────────────────────────────────────────
const (
	TopicAccountDeletionRequested = "account.deletion_requested" // Producer: Auth | Part 1 — PII purge cascade, ALL services consume
)

// ─── DOMAIN: store ───────────────────────────────────────────────────────────
const (
	TopicStoreSessionStarted = "store.session_started" // Producer: Store | Part 1
	TopicStoreQueueJoined    = "store.queue_joined"    // Producer: Store | Part 1
	TopicStoreQRGenerated    = "store.qr_generated"    // Producer: Store | Part 3 (admin onboarding)
	TopicStoreServerRecovered = "store.server_recovered" // Producer: Store | Part 1 (disaster recovery)
)

// ─── DOMAIN: catalog ─────────────────────────────────────────────────────────
const (
	TopicCatalogProductUpdated     = "catalog.product_updated"      // Producer: Catalog | Part 1
	TopicCatalogImportJob          = "catalog.import_job"           // Producer: Admin | Part 3 (bulk catalog upload)
	TopicCatalogBulkImportComplete = "catalog.bulk_import_complete" // Producer: Catalog | Part 3
	TopicCatalogPriceUpdated       = "catalog.price_updated"        // Producer: Catalog | Part 2 (retailer price change)
)

// ─── DOMAIN: cart ────────────────────────────────────────────────────────────
const (
	TopicCartItemScanned      = "cart.item_scanned"       // Producer: Cart | Part 1 — HIGHEST VOLUME: 256 partitions
	TopicCartItemRemoved      = "cart.item_removed"       // Producer: Cart | Part 1
	TopicCartAbandoned        = "cart.abandoned"          // Producer: Cart (TTL expiry hook) | Part 1
	TopicCartCheckoutInitiated = "cart.checkout_initiated" // Producer: Cart | Part 1 (triggers offer evaluation)
	TopicCartItemAdded        = "cart.item_added"         // Producer: Cart | Part 2 (admin microservice map)
)

// ─── DOMAIN: payment ─────────────────────────────────────────────────────────
const (
	TopicPaymentInitiated    = "payment.initiated"     // Producer: Payment | Part 1
	TopicPaymentConfirmed    = "payment.confirmed"     // Producer: Payment | Part 1 — triggers order creation
	TopicPaymentFailed       = "payment.failed"        // Producer: Payment | Part 1
	TopicPaymentSplitInitiated = "payment.split_initiated" // Producer: Payment | Part 1 (loyalty + UPI split)
	TopicPaymentWebhookReceived = "payment.webhook_received" // Producer: Payment | Part 1 (audit trail)
	TopicOrderPaymentRequested = "order.payment_requested" // Producer: Order | consumed by Payment (admin map)
)

// ─── DOMAIN: order ───────────────────────────────────────────────────────────
const (
	TopicOrderCreated   = "order.created"   // Producer: Order | Part 1
	TopicOrderCompleted = "order.completed" // Producer: Order | Part 1 — triggers loyalty, analytics, notification
)

// ─── DOMAIN: exit ────────────────────────────────────────────────────────────
const (
	TopicExitValidated    = "exit.validated"     // Producer: Exit Validation | Part 1
	TopicExitDenied       = "exit.denied"        // Producer: Exit Validation | Part 1
	TopicExitOverrideUsed = "exit.override_used" // Producer: Exit Validation | Part 1 — compliance, 30d retention
	TopicExitAlarmTriggered = "exit.alarm_triggered" // Producer: Exit Validation | Part 2 (RFID alarm)
)

// ─── DOMAIN: refund ──────────────────────────────────────────────────────────
const (
	TopicRefundRequested = "refund.requested" // Producer: Order | Part 1
	TopicRefundCompleted = "refund.completed" // Producer: Payment | Part 1
)

// ─── DOMAIN: inventory ───────────────────────────────────────────────────────
const (
	TopicInventoryLowStock          = "inventory.low_stock"           // Producer: Inventory | Part 1 & 2
	TopicInventoryShrinkageDetected = "inventory.shrinkage_detected"  // Producer: Inventory | Part 1 & 2
	TopicInventoryHoldRequest       = "inventory.hold_request"        // Producer: Cart (consumed by Inventory) | Part 2 admin map
	TopicInventoryShrinkageAlert    = "inventory.shrinkage_alert"     // Producer: Inventory | Part 2 (RFID delta engine)
	TopicInventoryReorderTriggered  = "inventory.reorder_triggered"   // Producer: Inventory | NEW — smart replenishment
)

// ─── DOMAIN: warehouse (Part 2) ──────────────────────────────────────────────
const (
	TopicWarehouseGRNDiscrepancy    = "warehouse.grn_discrepancy"    // Producer: Warehouse | Part 2
	TopicWarehouseTransferDispatched = "warehouse.transfer_dispatched" // Producer: Warehouse | Part 2
	TopicWarehouseGRNConfirmed      = "warehouse.grn_confirmed"      // Producer: Warehouse | Part 2 (consumed by QC)
)

// ─── DOMAIN: transfer (Part 2) ───────────────────────────────────────────────
const (
	TopicTransferDispatched = "transfer.dispatched" // Producer: Transfer Service | Part 2
	TopicTransferConfirmed  = "transfer.confirmed"  // Producer: Transfer Service | Part 2
)

// ─── DOMAIN: qc (Part 2) ─────────────────────────────────────────────────────
const (
	TopicQCInspectionComplete = "qc.inspection_complete" // Producer: QC Service | Part 2
)

// ─── DOMAIN: loyalty ─────────────────────────────────────────────────────────
const (
	TopicLoyaltyPointsEarned = "loyalty.points_earned" // Producer: Loyalty | Part 1 (admin map)
	TopicLoyaltyTierChanged  = "loyalty.tier_changed"  // Producer: Loyalty | NEW — tier upgrade/downgrade events
)

// ─── DOMAIN: support ─────────────────────────────────────────────────────────
const (
	TopicSupportTicketCreated   = "support.ticket_created"   // Producer: Support | Part 1
	TopicSupportTicketEscalated = "support.ticket_escalated" // Producer: Support | Part 1
)

// ─── DOMAIN: user (new) ──────────────────────────────────────────────────────
const (
	TopicUserSegmentUpdated = "user.segment_updated" // Producer: Analytics | NEW — RFM segment change
	TopicUserAccountMerged  = "user.account_merged"  // Producer: Auth | NEW — phone number SIM swap/port
)

// ─── DOMAIN: device (Part 3) ─────────────────────────────────────────────────
const (
	TopicDeviceOffline         = "device.offline"          // Producer: Device Mgmt | Part 3
	TopicDeviceAlarm           = "device.alarm"            // Producer: Device Mgmt | Part 3
	TopicDeviceHeartbeatMissed = "device.heartbeat_missed" // Producer: Device Mgmt | Part 3 (NEW)
)

// ─── DOMAIN: admin (Part 3) ──────────────────────────────────────────────────
const (
	TopicAdminStoreOnboarded  = "admin.store_onboarded"  // Producer: Admin Service | Part 3
	TopicAdminConfigChanged   = "admin.config_changed"   // Producer: Admin Service | Part 3
	TopicAdminAuditEvent      = "admin.audit_event"      // Producer: ALL services | Part 3 — sole consumer: Audit Service
)

// ─── DOMAIN: erp (Part 3) ────────────────────────────────────────────────────
const (
	TopicERPSyncComplete = "erp.sync_complete" // Producer: Integration Service | Part 3
	TopicERPSyncFailed   = "erp.sync_failed"   // Producer: Integration Service | Part 3
	TopicERPSyncJobs     = "erp.sync_jobs"     // Producer: Integration Service | Part 3 (job queue)
)

// ─── DOMAIN: offer ───────────────────────────────────────────────────────────
const (
	TopicOfferApplied = "offer.applied" // Producer: Offer Service | Part 1 (admin map)
)

// ─── DOMAIN: store capacity (new) ────────────────────────────────────────────
const (
	TopicStoreCapacityAlert = "store.capacity_alert" // Producer: Store Service | NEW — triggers manager notification
	TopicStoreGoingOffline  = "store.going_offline"  // Producer: Store/Ops | NEW — graceful store shutdown
)

// =============================================================================
// TOPIC METADATA — partition counts, keys, retention
// =============================================================================

// TopicConfig holds the MSK configuration for a topic
type TopicConfig struct {
	Name           string
	Partitions     int
	ReplicationFactor int
	RetentionMs    int64  // -1 = use broker default
	PartitionKey   string // field used as Kafka partition key
	OrderingScope  string // what ordering is guaranteed within a partition
	Phase          int    // which build phase this topic is introduced
	VolumePerDay   string // estimated events per day at 100M users
}

// AllTopics is the complete MSK topic configuration registry
var AllTopics = []TopicConfig{

	// ── HIGH VOLUME: requires careful partitioning ──────────────────────────

	{TopicCartItemScanned, 256, 3, 7 * 86400 * 1000, "store_id",
		"All scans from same store are ordered", 0, "1.2B/day"},

	{TopicInventoryHoldRequest, 256, 3, 7 * 86400 * 1000, "store_id + sku_id hash",
		"Hold requests for same SKU in same store are ordered", 0, "800M/day"},

	{TopicAdminAuditEvent, 128, 3, 90 * 86400 * 1000, "service_name",
		"Audit events per service are ordered", 3, "500M/day"},

	{TopicCartItemAdded, 128, 3, 7 * 86400 * 1000, "store_id",
		"Cart adds per store are ordered", 0, "400M/day"},

	// ── FINANCIAL: strict ordering, longer retention ─────────────────────────

	{TopicPaymentConfirmed, 128, 3, 30 * 86400 * 1000, "user_id",
		"All payment events for same user are ordered — prevents duplicate orders", 0, "40M/day"},

	{TopicPaymentInitiated, 128, 3, 7 * 86400 * 1000, "user_id",
		"Payment initiation per user ordered", 0, "50M/day"},

	{TopicPaymentFailed, 64, 3, 7 * 86400 * 1000, "user_id",
		"Payment failures per user ordered", 0, "8M/day"},

	{TopicPaymentSplitInitiated, 32, 3, 7 * 86400 * 1000, "user_id",
		"Split payment events per user ordered", 1, "5M/day"},

	{TopicPaymentWebhookReceived, 32, 3, 90 * 86400 * 1000, "payment_id",
		"All webhook events for same payment ordered", 0, "55M/day"},

	{TopicOrderCreated, 128, 3, 30 * 86400 * 1000, "store_id",
		"Orders per store are ordered", 0, "40M/day"},

	{TopicOrderCompleted, 128, 3, 30 * 86400 * 1000, "store_id",
		"Order completions per store ordered — loyalty + analytics guaranteed order", 0, "40M/day"},

	{TopicOrderPaymentRequested, 64, 3, 7 * 86400 * 1000, "order_id",
		"Payment requests per order ordered", 0, "40M/day"},

	// ── EXIT VALIDATION ──────────────────────────────────────────────────────

	{TopicExitValidated, 64, 3, 30 * 86400 * 1000, "store_id",
		"Exit events per store ordered — gate state correctness depends on this", 0, "40M/day"},

	{TopicExitDenied, 32, 3, 30 * 86400 * 1000, "store_id",
		"Exit denials per store ordered", 0, "2M/day"},

	{TopicExitOverrideUsed, 16, 3, 30 * 86400 * 1000, "store_id",
		"Override events per store ordered for compliance", 0, "200K/day"},

	{TopicExitAlarmTriggered, 16, 3, 30 * 86400 * 1000, "store_id",
		"RFID alarms per store ordered", 2, "1M/day"},

	// ── AUTH ─────────────────────────────────────────────────────────────────

	{TopicAuthOTPSent, 32, 3, 7 * 86400 * 1000, "phone_prefix",
		"OTP events partitioned by country code prefix", 0, "20M/day"},

	{TopicAuthLoginSuccess, 64, 3, 7 * 86400 * 1000, "user_id",
		"Login events per user ordered", 0, "12M/day"},

	{TopicAuthLoginFailed, 32, 3, 7 * 86400 * 1000, "phone",
		"Failed logins per phone ordered", 0, "3M/day"},

	{TopicAuthAccountLocked, 8, 3, 7 * 86400 * 1000, "phone",
		"Lockout events per phone ordered", 0, "100K/day"},

	{TopicAuthSessionCreated, 64, 3, 7 * 86400 * 1000, "user_id",
		"Session events per user ordered", 0, "15M/day"},

	{TopicAuthSessionEnded, 32, 3, 7 * 86400 * 1000, "user_id",
		"Session ends per user ordered", 0, "12M/day"},

	{TopicAuthStaffLogin, 16, 3, 7 * 86400 * 1000, "store_id",
		"Staff logins per store ordered", 2, "500K/day"},

	// ── ACCOUNT ──────────────────────────────────────────────────────────────

	{TopicAccountDeletionRequested, 16, 3, 30 * 86400 * 1000, "user_id",
		"Deletion cascade must be ordered per user", 0, "50K/day"},

	// ── STORE ────────────────────────────────────────────────────────────────

	{TopicStoreSessionStarted, 64, 3, 7 * 86400 * 1000, "store_id",
		"Store sessions per store ordered", 0, "12M/day"},

	{TopicStoreQueueJoined, 16, 3, 7 * 86400 * 1000, "store_id",
		"Queue events per store ordered", 1, "500K/day"},

	{TopicStoreQRGenerated, 8, 3, 7 * 86400 * 1000, "store_id",
		"QR generation events per store", 3, "15K/day"},

	{TopicStoreCapacityAlert, 16, 3, 7 * 86400 * 1000, "store_id",
		"Capacity alerts per store ordered", 1, "200K/day"},

	{TopicStoreGoingOffline, 8, 3, 7 * 86400 * 1000, "store_id",
		"Store offline events ordered per store", 2, "50K/day"},

	{TopicStoreServerRecovered, 8, 3, 7 * 86400 * 1000, "store_id",
		"Recovery events per store", 0, "10K/day"},

	// ── CATALOG ──────────────────────────────────────────────────────────────

	{TopicCatalogProductUpdated, 32, 3, 7 * 86400 * 1000, "store_id",
		"Product updates per store ordered — Redis cache invalidation must be ordered", 0, "5M/day"},

	{TopicCatalogImportJob, 16, 3, 7 * 86400 * 1000, "store_id",
		"Import jobs per store ordered", 3, "50K/day"},

	{TopicCatalogBulkImportComplete, 8, 3, 7 * 86400 * 1000, "store_id",
		"Completion events per store", 3, "15K/day"},

	{TopicCatalogPriceUpdated, 32, 3, 7 * 86400 * 1000, "store_id",
		"Price updates per store ordered", 2, "2M/day"},

	// ── CART ─────────────────────────────────────────────────────────────────

	{TopicCartItemRemoved, 64, 3, 7 * 86400 * 1000, "store_id",
		"Removals per store ordered — inventory hold release must be in order", 0, "200M/day"},

	{TopicCartAbandoned, 32, 3, 7 * 86400 * 1000, "store_id",
		"Abandonment events per store", 0, "5M/day"},

	{TopicCartCheckoutInitiated, 64, 3, 7 * 86400 * 1000, "user_id",
		"Checkout initiation per user ordered", 0, "45M/day"},

	// ── REFUND ───────────────────────────────────────────────────────────────

	{TopicRefundRequested, 16, 3, 30 * 86400 * 1000, "order_id",
		"Refund events per order ordered", 0, "500K/day"},

	{TopicRefundCompleted, 16, 3, 30 * 86400 * 1000, "order_id",
		"Refund completions per order ordered", 0, "400K/day"},

	// ── INVENTORY ────────────────────────────────────────────────────────────

	{TopicInventoryLowStock, 32, 3, 7 * 86400 * 1000, "store_id",
		"Low stock alerts per store", 0, "2M/day"},

	{TopicInventoryShrinkageDetected, 16, 3, 30 * 86400 * 1000, "store_id",
		"Shrinkage events per store ordered", 0, "500K/day"},

	{TopicInventoryShrinkageAlert, 32, 3, 30 * 86400 * 1000, "store_id",
		"RFID-based shrinkage alerts per store", 2, "1M/day"},

	{TopicInventoryReorderTriggered, 16, 3, 7 * 86400 * 1000, "store_id",
		"Reorder triggers per store ordered", 3, "200K/day"},

	// ── WAREHOUSE (Part 2) ───────────────────────────────────────────────────

	{TopicWarehouseGRNDiscrepancy, 8, 3, 7 * 86400 * 1000, "warehouse_id",
		"GRN discrepancies per warehouse ordered", 2, "50K/day"},

	{TopicWarehouseGRNConfirmed, 8, 3, 7 * 86400 * 1000, "warehouse_id",
		"GRN confirmations per warehouse", 2, "100K/day"},

	{TopicWarehouseTransferDispatched, 8, 3, 7 * 86400 * 1000, "warehouse_id",
		"Transfer dispatches per warehouse ordered", 2, "80K/day"},

	// ── TRANSFER (Part 2) ────────────────────────────────────────────────────

	{TopicTransferDispatched, 8, 3, 7 * 86400 * 1000, "from_store_id",
		"Transfers per source store ordered", 2, "80K/day"},

	{TopicTransferConfirmed, 8, 3, 7 * 86400 * 1000, "to_store_id",
		"Transfer confirmations per destination store", 2, "80K/day"},

	// ── QC (Part 2) ──────────────────────────────────────────────────────────

	{TopicQCInspectionComplete, 8, 3, 7 * 86400 * 1000, "grn_id",
		"QC completions per GRN ordered", 2, "100K/day"},

	// ── LOYALTY ──────────────────────────────────────────────────────────────

	{TopicLoyaltyPointsEarned, 32, 3, 30 * 86400 * 1000, "user_id",
		"Loyalty events per user ordered", 0, "40M/day"},

	{TopicLoyaltyTierChanged, 16, 3, 30 * 86400 * 1000, "user_id",
		"Tier changes per user strictly ordered", 1, "500K/day"},

	// ── SUPPORT ──────────────────────────────────────────────────────────────

	{TopicSupportTicketCreated, 16, 3, 7 * 86400 * 1000, "store_id",
		"Support tickets per store", 0, "500K/day"},

	{TopicSupportTicketEscalated, 8, 3, 7 * 86400 * 1000, "store_id",
		"Escalations per store", 0, "50K/day"},

	// ── USER (New) ───────────────────────────────────────────────────────────

	{TopicUserSegmentUpdated, 32, 3, 7 * 86400 * 1000, "user_id",
		"Segment changes per user ordered", 2, "2M/day"},

	{TopicUserAccountMerged, 8, 3, 30 * 86400 * 1000, "user_id",
		"Account merges per user — strictly ordered for PII safety", 2, "10K/day"},

	// ── DEVICE (Part 3) ──────────────────────────────────────────────────────

	{TopicDeviceOffline, 16, 3, 7 * 86400 * 1000, "store_id",
		"Device events per store ordered", 3, "1M/day"},

	{TopicDeviceAlarm, 16, 3, 7 * 86400 * 1000, "store_id",
		"Device alarms per store ordered", 3, "200K/day"},

	{TopicDeviceHeartbeatMissed, 16, 3, 7 * 86400 * 1000, "store_id",
		"Missed heartbeats per store ordered", 3, "500K/day"},

	// ── ADMIN (Part 3) ───────────────────────────────────────────────────────

	{TopicAdminStoreOnboarded, 4, 3, 30 * 86400 * 1000, "chain_id",
		"Onboarding events per chain ordered", 3, "500/day"},

	{TopicAdminConfigChanged, 8, 3, 7 * 86400 * 1000, "store_id",
		"Config changes per store ordered", 3, "50K/day"},

	// ── ERP (Part 3) ─────────────────────────────────────────────────────────

	{TopicERPSyncComplete, 8, 3, 7 * 86400 * 1000, "store_id",
		"ERP sync completions per store", 3, "15K/day"},

	{TopicERPSyncFailed, 8, 3, 7 * 86400 * 1000, "store_id",
		"ERP sync failures per store", 3, "2K/day"},

	{TopicERPSyncJobs, 16, 3, 7 * 86400 * 1000, "store_id",
		"ERP job queue — ordered per store to prevent parallel syncs", 3, "15K/day"},

	// ── OFFER ────────────────────────────────────────────────────────────────

	{TopicOfferApplied, 32, 3, 7 * 86400 * 1000, "store_id",
		"Offer application events per store", 0, "40M/day"},
}

// =============================================================================
// CONSUMER GROUP REGISTRY
// Each consumer group has exactly one responsibility.
// Never share a consumer group between two different processing concerns.
// =============================================================================

// ConsumerGroup defines a Kafka consumer group and what it processes
type ConsumerGroup struct {
	GroupID      string
	Service      string
	Topics       []string
	ProcessingType string // REAL_TIME | BATCH | ANALYTICS
	MaxLag       int    // alert if consumer lag exceeds this
}

var AllConsumerGroups = []ConsumerGroup{

	// ── Order creation pipeline ────────────────────────────────────────────
	{"order-creation-cg", "Order Service",
		[]string{TopicPaymentConfirmed},
		"REAL_TIME", 1000},

	// ── Inventory management ───────────────────────────────────────────────
	{"inventory-hold-cg", "Inventory Service",
		[]string{TopicCartItemScanned, TopicCartItemAdded},
		"REAL_TIME", 5000},

	{"inventory-release-cg", "Inventory Service",
		[]string{TopicCartItemRemoved, TopicCartAbandoned},
		"REAL_TIME", 5000},

	{"inventory-deduct-cg", "Inventory Service",
		[]string{TopicPaymentConfirmed},
		"REAL_TIME", 1000},

	// ── Loyalty engine ────────────────────────────────────────────────────
	{"loyalty-credit-cg", "Loyalty Service",
		[]string{TopicOrderCompleted},
		"REAL_TIME", 2000},

	{"loyalty-deduct-cg", "Loyalty Service",
		[]string{TopicPaymentSplitInitiated},
		"REAL_TIME", 500},

	{"loyalty-refund-cg", "Loyalty Service",
		[]string{TopicRefundCompleted},
		"REAL_TIME", 500},

	{"loyalty-tier-check-cg", "Loyalty Service",
		[]string{TopicLoyaltyPointsEarned},
		"REAL_TIME", 1000},

	// ── Exit validation ───────────────────────────────────────────────────
	{"exit-preauth-cg", "Exit Validation Service",
		[]string{TopicOrderCreated},
		"REAL_TIME", 1000},

	{"exit-complete-cg", "Order Service",
		[]string{TopicExitValidated},
		"REAL_TIME", 1000},

	// ── Analytics pipeline ────────────────────────────────────────────────
	{"analytics-orders-cg", "Analytics Service",
		[]string{TopicOrderCompleted, TopicOrderCreated},
		"ANALYTICS", 50000},

	{"analytics-payments-cg", "Analytics Service",
		[]string{TopicPaymentConfirmed, TopicPaymentFailed, TopicPaymentInitiated},
		"ANALYTICS", 50000},

	{"analytics-cart-cg", "Analytics Service",
		[]string{TopicCartItemScanned, TopicCartAbandoned},
		"ANALYTICS", 100000},

	{"analytics-exit-cg", "Analytics Service",
		[]string{TopicExitValidated, TopicExitDenied, TopicExitOverrideUsed},
		"ANALYTICS", 20000},

	{"analytics-inventory-cg", "Analytics Service",
		[]string{TopicInventoryShrinkageDetected, TopicInventoryShrinkageAlert},
		"ANALYTICS", 10000},

	{"analytics-auth-cg", "Analytics Service",
		[]string{TopicAuthLoginSuccess, TopicAuthLoginFailed, TopicAuthSessionCreated},
		"ANALYTICS", 30000},

	// ── Notification pipeline ─────────────────────────────────────────────
	{"notif-order-cg", "Notification Service",
		[]string{TopicOrderCreated, TopicOrderCompleted},
		"REAL_TIME", 2000},

	{"notif-payment-cg", "Notification Service",
		[]string{TopicPaymentConfirmed, TopicPaymentFailed},
		"REAL_TIME", 2000},

	{"notif-refund-cg", "Notification Service",
		[]string{TopicRefundCompleted, TopicRefundRequested},
		"REAL_TIME", 1000},

	{"notif-loyalty-cg", "Notification Service",
		[]string{TopicLoyaltyTierChanged},
		"REAL_TIME", 500},

	{"notif-inventory-cg", "Notification Service",
		[]string{TopicInventoryLowStock, TopicInventoryShrinkageAlert},
		"REAL_TIME", 1000},

	{"notif-auth-cg", "Notification Service",
		[]string{TopicAuthAccountLocked},
		"REAL_TIME", 500},

	{"notif-device-cg", "Notification Service",
		[]string{TopicDeviceOffline, TopicDeviceHeartbeatMissed},
		"REAL_TIME", 500},

	{"notif-support-cg", "Notification Service",
		[]string{TopicSupportTicketEscalated},
		"REAL_TIME", 500},

	// ── Redis cache invalidation ──────────────────────────────────────────
	{"cache-invalidate-cg", "Catalog Service",
		[]string{TopicCatalogProductUpdated, TopicCatalogPriceUpdated},
		"REAL_TIME", 5000},

	// ── PII purge cascade ─────────────────────────────────────────────────
	{"pii-purge-auth-cg", "Auth Service",
		[]string{TopicAccountDeletionRequested},
		"REAL_TIME", 100},

	{"pii-purge-order-cg", "Order Service",
		[]string{TopicAccountDeletionRequested},
		"REAL_TIME", 100},

	{"pii-purge-loyalty-cg", "Loyalty Service",
		[]string{TopicAccountDeletionRequested},
		"REAL_TIME", 100},

	// ── Audit Service (Part 3) ────────────────────────────────────────────
	{"audit-ingest-cg", "Audit Service",
		[]string{TopicAdminAuditEvent},
		"BATCH", 500000}, // batch insert 500 events at a time

	// ── ERP integration (Part 3) ──────────────────────────────────────────
	{"erp-sync-trigger-cg", "Integration Service",
		[]string{TopicOrderCompleted, TopicERPSyncJobs},
		"BATCH", 10000},

	// ── Warehouse pipeline (Part 2) ───────────────────────────────────────
	{"warehouse-qc-cg", "QC Service",
		[]string{TopicWarehouseGRNConfirmed},
		"REAL_TIME", 500},

	{"warehouse-transfer-cg", "Transfer Service",
		[]string{TopicWarehouseTransferDispatched},
		"REAL_TIME", 500},

	// ── Catalog ElasticSearch indexing ────────────────────────────────────
	{"es-indexing-cg", "Catalog Service (ES worker)",
		[]string{TopicCatalogProductUpdated, TopicCatalogPriceUpdated, TopicCatalogBulkImportComplete},
		"REAL_TIME", 10000},

	// ── ClickHouse ingestion ──────────────────────────────────────────────
	{"clickhouse-orders-cg", "ClickHouse Kafka Engine",
		[]string{TopicOrderCompleted},
		"ANALYTICS", 100000},

	{"clickhouse-cart-cg", "ClickHouse Kafka Engine",
		[]string{TopicCartItemScanned},
		"ANALYTICS", 200000},

	{"clickhouse-exit-cg", "ClickHouse Kafka Engine",
		[]string{TopicExitDenied, TopicExitOverrideUsed},
		"ANALYTICS", 50000},
}
