package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zippyra/platform/services/catalog-service/internal/model"
)

type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type ProductRepo interface {
	GetByStoreAndBarcode(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error)
	Search(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error)
	GetByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error)
	Sync(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error)
	Upsert(ctx context.Context, p *model.Product) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
}

type productRepo struct {
	db DB
}

func NewProductRepo(db DB) ProductRepo {
	return &productRepo{db: db}
}

func (r *productRepo) GetByStoreAndBarcode(ctx context.Context, storeID uuid.UUID, barcode string) (*model.Product, error) {
	const query = `
		SELECT id, store_id, barcode, name, description, brand, category,
		       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
		       thumbnail_url, is_active, is_returnable, stock_quantity,
		       reorder_point, sync_seq, created_at, updated_at
		FROM products
		WHERE store_id = $1 AND barcode = $2 AND is_active = true`

	var p model.Product
	err := r.db.QueryRow(ctx, query, storeID, barcode).Scan(
		&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.Description, &p.Brand, &p.Category,
		&p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate, &p.Unit, &p.ImageURL,
		&p.ThumbnailURL, &p.IsActive, &p.IsReturnable, &p.StockQuantity,
		&p.ReorderPoint, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get product by store and barcode: %w", err)
	}

	return &p, nil
}

func (r *productRepo) Search(ctx context.Context, storeID uuid.UUID, query string, category *string, page, limit int) ([]model.Product, int, error) {
	offset := (page - 1) * limit

	baseQuery := `
		FROM products
		WHERE store_id = $1 AND is_active = true
		  AND (name ILIKE $2 OR brand ILIKE $2 OR category ILIKE $2)`

	countQuery := "SELECT COUNT(*)" + baseQuery
	var total int
	err := r.db.QueryRow(ctx, countQuery, storeID, "%"+query+"%").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	dataQuery := `
		SELECT id, store_id, barcode, name, description, brand, category,
		       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
		       thumbnail_url, is_active, is_returnable, stock_quantity,
		       reorder_point, sync_seq, created_at, updated_at
	` + baseQuery + `
		ORDER BY selling_price ASC
		LIMIT $3 OFFSET $4`

	args := []interface{}{storeID, "%" + query + "%", limit, offset}
	if category != nil {
		dataQuery = `
			SELECT id, store_id, barcode, name, description, brand, category,
			       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
			       thumbnail_url, is_active, is_returnable, stock_quantity,
			       reorder_point, sync_seq, created_at, updated_at
			FROM products
			WHERE store_id = $1 AND is_active = true
			  AND category = $2
			  AND (name ILIKE $3 OR brand ILIKE $3 OR category ILIKE $3)
			ORDER BY selling_price ASC
			LIMIT $4 OFFSET $5`
		args = []interface{}{storeID, *category, "%" + query + "%", limit, offset}
	}

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search products: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		err := rows.Scan(
			&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.Description, &p.Brand, &p.Category,
			&p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate, &p.Unit, &p.ImageURL,
			&p.ThumbnailURL, &p.IsActive, &p.IsReturnable, &p.StockQuantity,
			&p.ReorderPoint, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product row: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating product rows: %w", err)
	}

	return products, total, nil
}

func (r *productRepo) GetByCategory(ctx context.Context, storeID uuid.UUID, category string, page, limit int) ([]model.Product, int, error) {
	offset := (page - 1) * limit

	countQuery := `SELECT COUNT(*) FROM products WHERE store_id = $1 AND category = $2 AND is_active = true`
	var total int
	err := r.db.QueryRow(ctx, countQuery, storeID, category).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count category products: %w", err)
	}

	dataQuery := `
		SELECT id, store_id, barcode, name, description, brand, category,
		       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
		       thumbnail_url, is_active, is_returnable, stock_quantity,
		       reorder_point, sync_seq, created_at, updated_at
		FROM products
		WHERE store_id = $1 AND category = $2 AND is_active = true
		ORDER BY selling_price ASC
		LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, dataQuery, storeID, category, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get products by category: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		err := rows.Scan(
			&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.Description, &p.Brand, &p.Category,
			&p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate, &p.Unit, &p.ImageURL,
			&p.ThumbnailURL, &p.IsActive, &p.IsReturnable, &p.StockQuantity,
			&p.ReorderPoint, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product row: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating product rows: %w", err)
	}

	return products, total, nil
}

func (r *productRepo) Sync(ctx context.Context, storeID uuid.UUID, lastSyncSeq int64, limit int) ([]model.Product, error) {
	query := `
		SELECT id, store_id, barcode, name, description, brand, category,
		       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
		       thumbnail_url, is_active, is_returnable, stock_quantity,
		       reorder_point, sync_seq, created_at, updated_at
		FROM products
		WHERE store_id = $1 AND sync_seq > $2
		ORDER BY sync_seq ASC
		LIMIT $3`

	rows, err := r.db.Query(ctx, query, storeID, lastSyncSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to sync products: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		err := rows.Scan(
			&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.Description, &p.Brand, &p.Category,
			&p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate, &p.Unit, &p.ImageURL,
			&p.ThumbnailURL, &p.IsActive, &p.IsReturnable, &p.StockQuantity,
			&p.ReorderPoint, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan product row: %w", err)
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating product rows: %w", err)
	}

	return products, nil
}

func (r *productRepo) Upsert(ctx context.Context, p *model.Product) error {
	query := `
		INSERT INTO products (
			store_id, barcode, name, description, brand, category,
			hsn_code, mrp, selling_price, gst_rate, unit, image_url,
			thumbnail_url, is_active, is_returnable, stock_quantity,
			reorder_point, sync_seq, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		ON CONFLICT (store_id, barcode) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			brand = EXCLUDED.brand,
			category = EXCLUDED.category,
			hsn_code = EXCLUDED.hsn_code,
			mrp = EXCLUDED.mrp,
			selling_price = EXCLUDED.selling_price,
			gst_rate = EXCLUDED.gst_rate,
			unit = EXCLUDED.unit,
			image_url = EXCLUDED.image_url,
			thumbnail_url = EXCLUDED.thumbnail_url,
			is_active = EXCLUDED.is_active,
			is_returnable = EXCLUDED.is_returnable,
			stock_quantity = EXCLUDED.stock_quantity,
			reorder_point = EXCLUDED.reorder_point,
			updated_at = NOW(),
			sync_seq = sync_seq + 1
		RETURNING id, sync_seq, created_at, updated_at`

	err := r.db.QueryRow(ctx, query,
		p.StoreID, p.Barcode, p.Name, p.Description, p.Brand, p.Category,
		p.HSNCode, p.MRP, p.SellingPrice, p.GSTRate, p.Unit, p.ImageURL,
		p.ThumbnailURL, p.IsActive, p.IsReturnable, p.StockQuantity,
		p.ReorderPoint, p.CreatedAt, p.UpdatedAt,
	).Scan(&p.ID, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to upsert product: %w", err)
	}

	return nil
}

func (r *productRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	query := `
		SELECT id, store_id, barcode, name, description, brand, category,
		       hsn_code, mrp, selling_price, gst_rate, unit, image_url,
		       thumbnail_url, is_active, is_returnable, stock_quantity,
		       reorder_point, sync_seq, created_at, updated_at
		FROM products
		WHERE id = $1`

	var p model.Product
	err := r.db.QueryRow(ctx, query, id).Scan(
		&p.ID, &p.StoreID, &p.Barcode, &p.Name, &p.Description, &p.Brand, &p.Category,
		&p.HSNCode, &p.MRP, &p.SellingPrice, &p.GSTRate, &p.Unit, &p.ImageURL,
		&p.ThumbnailURL, &p.IsActive, &p.IsReturnable, &p.StockQuantity,
		&p.ReorderPoint, &p.SyncSeq, &p.CreatedAt, &p.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get product by id: %w", err)
	}

	return &p, nil
}
