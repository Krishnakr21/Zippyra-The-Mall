package kafka

// InventoryConsumer subscribes to:
//   - cart.item_scanned (inventory-hold-cg) — 256 partitions
//   - cart.item_removed (inventory-release-cg)
//   - cart.abandoned (inventory-release-cg)
//   - payment.confirmed (inventory-deduct-cg)
type InventoryConsumer struct{}
