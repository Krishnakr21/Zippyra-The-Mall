package model

import (
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	StoreID       uuid.UUID  `json:"store_id" db:"store_id"`
	Barcode       string     `json:"barcode" db:"barcode"`
	Name          string     `json:"name" db:"name"`
	Description   *string    `json:"description,omitempty" db:"description"`
	Brand         string     `json:"brand" db:"brand"`
	Category      string     `json:"category" db:"category"`
	HSNCode       string     `json:"hsn_code" db:"hsn_code"`
	MRP           float64    `json:"mrp" db:"mrp"`
	SellingPrice  float64    `json:"selling_price" db:"selling_price"`
	GSTRate       float64    `json:"gst_rate" db:"gst_rate"`
	Unit          string     `json:"unit" db:"unit"`
	ImageURL      *string    `json:"image_url,omitempty" db:"image_url"`
	ThumbnailURL  *string    `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
	IsActive      bool       `json:"is_active" db:"is_active"`
	IsReturnable  bool       `json:"is_returnable" db:"is_returnable"`
	StockQuantity int        `json:"stock_quantity" db:"stock_quantity"`
	ReorderPoint  int        `json:"reorder_point" db:"reorder_point"`
	ReorderQuantity int      `json:"reorder_quantity" db:"reorder_quantity"`
	SyncSeq       int64      `json:"sync_seq" db:"sync_seq"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

type ProductResponse struct {
	Product    Product `json:"product"`
	CacheHit   bool    `json:"cache_hit"`
}

type ProductSearchRequest struct {
	StoreID  uuid.UUID `json:"store_id" validate:"required"`
	Query    string    `json:"query" validate:"required,max=100"`
	Category *string   `json:"category,omitempty"`
	Page     int       `json:"page" validate:"min=1"`
	Limit    int       `json:"limit" validate:"min=1,max=100"`
}

type ProductListResponse struct {
	Products []Product `json:"products"`
	Total    int       `json:"total"`
	Page     int       `json:"page"`
	Limit    int       `json:"limit"`
}

type UpsertProductRequest struct {
	StoreID       uuid.UUID  `json:"store_id" validate:"required"`
	Barcode       string     `json:"barcode" validate:"required"`
	Name          string     `json:"name" validate:"required"`
	Brand         string     `json:"brand" validate:"required"`
	Category      string     `json:"category" validate:"required"`
	HSNCode       string     `json:"hsn_code" validate:"required"`
	MRP           float64    `json:"mrp" validate:"required,gt=0"`
	SellingPrice  float64    `json:"selling_price" validate:"required,gt=0"`
	GSTRate       float64    `json:"gst_rate" validate:"required,gte=0,lte=100"`
	Unit          string     `json:"unit" validate:"required"`
	StockQuantity int        `json:"stock_quantity" validate:"required,gte=0"`
	Description   *string    `json:"description,omitempty"`
	ImageURL      *string    `json:"image_url,omitempty"`
	ThumbnailURL  *string    `json:"thumbnail_url,omitempty"`
	IsReturnable  *bool      `json:"is_returnable,omitempty"`
	ReorderPoint  *int       `json:"reorder_point,omitempty"`
	ReorderQuantity *int     `json:"reorder_quantity,omitempty"`
}

type SyncRequest struct {
	StoreID     uuid.UUID `json:"store_id" validate:"required"`
	LastSyncSeq int64     `json:"last_sync_seq" validate:"min=0"`
	Limit       int       `json:"limit" validate:"min=1,max=500"`
}

type SyncResponse struct {
	Products     []Product `json:"products"`
	NextSyncSeq  int64     `json:"next_sync_seq"`
	HasMore      bool      `json:"has_more"`
	TotalChanges int       `json:"total_changes"`
}
