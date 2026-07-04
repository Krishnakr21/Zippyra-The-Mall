package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/zippyra/platform/services/store-service/internal/model"
)

// dbPool is the interface for database operations.
type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// StoreRepository handles store database operations.
type StoreRepository struct {
	db dbPool
}

// NewStoreRepository creates a new StoreRepository.
func NewStoreRepository(db dbPool) *StoreRepository {
	return &StoreRepository{db: db}
}

// GetByID retrieves an active store by its ID.
func (r *StoreRepository) GetByID(ctx context.Context, storeID uuid.UUID) (*model.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s model.Store
	err := r.db.QueryRow(ctx,
		`SELECT id, chain_id, name, address, city, state, pincode,
		        latitude, longitude, capacity, catalog_version,
		        qr_only_mode, is_active, created_at, updated_at
		 FROM stores WHERE id = $1 AND is_active = true`, storeID,
	).Scan(
		&s.ID, &s.ChainID, &s.Name, &s.Address, &s.City, &s.State, &s.Pincode,
		&s.Latitude, &s.Longitude, &s.Capacity, &s.CatalogVersion,
		&s.QROnlyMode, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// GetByIDAndChain retrieves a store filtered by both ID and chain.
func (r *StoreRepository) GetByIDAndChain(ctx context.Context, storeID, chainID uuid.UUID) (*model.Store, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var s model.Store
	err := r.db.QueryRow(ctx,
		`SELECT id, chain_id, name, address, city, state, pincode,
		        latitude, longitude, capacity, catalog_version,
		        qr_only_mode, is_active, created_at, updated_at
		 FROM stores WHERE id = $1 AND chain_id = $2 AND is_active = true`, storeID, chainID,
	).Scan(
		&s.ID, &s.ChainID, &s.Name, &s.Address, &s.City, &s.State, &s.Pincode,
		&s.Latitude, &s.Longitude, &s.Capacity, &s.CatalogVersion,
		&s.QROnlyMode, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// NearbyStores finds active stores within the given radius using Haversine formula.
func (r *StoreRepository) NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.Store, []float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.db.Query(ctx, `
		SELECT id, chain_id, name, address, city, state, pincode,
		       latitude, longitude, capacity, catalog_version,
		       qr_only_mode, is_active, created_at, updated_at,
		       (6371 * acos(
		           cos(radians($1)) * cos(radians(latitude)) *
		           cos(radians(longitude) - radians($2)) +
		           sin(radians($1)) * sin(radians(latitude))
		       )) AS distance_km
		FROM stores
		WHERE is_active = true
		  AND latitude IS NOT NULL
		  AND longitude IS NOT NULL
		  AND (6371 * acos(
		           cos(radians($1)) * cos(radians(latitude)) *
		           cos(radians(longitude) - radians($2)) +
		           sin(radians($1)) * sin(radians(latitude))
		       )) <= $3
		ORDER BY distance_km ASC
		LIMIT 20`, lat, lng, radiusKM)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var stores []model.Store
	var distances []float64
	for rows.Next() {
		var s model.Store
		var dist float64
		if err := rows.Scan(
			&s.ID, &s.ChainID, &s.Name, &s.Address, &s.City, &s.State, &s.Pincode,
			&s.Latitude, &s.Longitude, &s.Capacity, &s.CatalogVersion,
			&s.QROnlyMode, &s.IsActive, &s.CreatedAt, &s.UpdatedAt, &dist,
		); err != nil {
			return nil, nil, err
		}
		stores = append(stores, s)
		distances = append(distances, dist)
	}
	return stores, distances, rows.Err()
}

// GetHours retrieves the operating hours for a store on a given day.
func (r *StoreRepository) GetHours(ctx context.Context, storeID uuid.UUID, dayOfWeek int) (*model.StoreHours, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var h model.StoreHours
	err := r.db.QueryRow(ctx,
		`SELECT store_id, day_of_week, opens_at, closes_at
		 FROM store_hours WHERE store_id = $1 AND day_of_week = $2`, storeID, dayOfWeek,
	).Scan(&h.StoreID, &h.DayOfWeek, &h.OpensAt, &h.ClosesAt)
	if err != nil {
		return nil, err
	}
	return &h, nil
}
