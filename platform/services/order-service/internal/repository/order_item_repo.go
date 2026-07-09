package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/services/order-service/internal/model"
)

type OrderItemRepository struct {
	pool *pgxpool.Pool
}

func NewOrderItemRepository(pool *pgxpool.Pool) *OrderItemRepository {
	return &OrderItemRepository{pool: pool}
}

func (r *OrderItemRepository) CreateItems(ctx context.Context, tx pgx.Tx, items []model.OrderItem) error {
	query := `
		INSERT INTO order_items (
			id, order_id, product_id, barcode, product_name,
			quantity, unit_price, gst_rate, cgst_amount, sgst_amount,
			igst_amount, gst_amount, total_price, hsn_code,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW(), NOW())`

	for _, item := range items {
		_, err := tx.Exec(ctx, query,
			item.ID, item.OrderID, item.ProductID, item.Barcode, item.ProductName,
			item.Quantity, item.UnitPrice, item.GSTRate, item.CGSTAmount, item.SGSTAmount,
			item.IGSTAmount, item.GSTAmount, item.TotalPrice, item.HSNCode)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *OrderItemRepository) GetByOrderID(ctx context.Context, orderID string) ([]model.OrderItem, error) {
	query := `
		SELECT id, order_id, product_id, barcode, product_name,
		       quantity, unit_price, gst_rate, cgst_amount, sgst_amount,
		       igst_amount, gst_amount, total_price, hsn_code, created_at, updated_at
		FROM order_items WHERE order_id = $1`

	rows, err := r.pool.Query(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.OrderItem
	for rows.Next() {
		var item model.OrderItem
		err = rows.Scan(
			&item.ID, &item.OrderID, &item.ProductID, &item.Barcode, &item.ProductName,
			&item.Quantity, &item.UnitPrice, &item.GSTRate, &item.CGSTAmount, &item.SGSTAmount,
			&item.IGSTAmount, &item.GSTAmount, &item.TotalPrice, &item.HSNCode, &item.CreatedAt, &item.UpdatedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *OrderItemRepository) GetProductByID(ctx context.Context, productID string) (*model.Product, error) {
	query := `
		SELECT id, store_id, barcode, name, hsn_code, mrp, selling_price, gst_rate,
		       is_active, is_returnable, stock_quantity, created_at, updated_at
		FROM products WHERE id = $1`

	var p model.Product
	var isActive bool
	err := r.pool.QueryRow(ctx, query, productID).Scan(
		&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate,
		&isActive, &p.IsReturnable, &p.StockQuantity, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *OrderItemRepository) UpdateProductStock(ctx context.Context, productID string, quantityChange int) error {
	query := `
		UPDATE products
		SET stock_quantity = stock_quantity + $1, updated_at = NOW()
		WHERE id = $2`
	_, err := r.pool.Exec(ctx, query, quantityChange, productID)
	return err
}
