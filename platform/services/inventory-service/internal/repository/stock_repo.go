package repository

// StockRepository handles the CRITICAL stock table.
// UNIQUE(sku_id, store_id). qty_reserved incremented on cart.item_scanned,
// decremented on cart.item_removed or order.completed.
type StockRepository struct{}
