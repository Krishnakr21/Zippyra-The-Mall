package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v4"
)

func TestNewStoreRepository(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	if repo == nil {
		t.Error("expected repo to not be nil")
	}
	if repo.db != mock {
		t.Error("expected db to be set")
	}
}

func TestStoreRepository_GetByID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()
	chainID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "chain_id", "name", "address", "city", "state", "pincode",
		"latitude", "longitude", "capacity", "catalog_version",
		"qr_only_mode", "is_active", "created_at", "updated_at",
	}).AddRow(
		storeID, chainID, "Test Store", "123 Main St", "City", "State", "123456",
		nil, nil, int32(100), int32(1),
		false, true, now, now,
	)

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(storeID).
		WillReturnRows(rows)

	store, err := repo.GetByID(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.ID != storeID {
		t.Errorf("expected store ID %s, got %s", storeID, store.ID)
	}
	if store.Name != "Test Store" {
		t.Errorf("expected name 'Test Store', got '%s'", store.Name)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestStoreRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(storeID).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), storeID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestStoreRepository_GetByIDAndChain(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()
	chainID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "chain_id", "name", "address", "city", "state", "pincode",
		"latitude", "longitude", "capacity", "catalog_version",
		"qr_only_mode", "is_active", "created_at", "updated_at",
	}).AddRow(
		storeID, chainID, "Test Store", "123 Main St", "City", "State", "123456",
		nil, nil, int32(100), int32(1),
		false, true, now, now,
	)

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(storeID, chainID).
		WillReturnRows(rows)

	store, err := repo.GetByIDAndChain(context.Background(), storeID, chainID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.ID != storeID {
		t.Errorf("expected store ID %s, got %s", storeID, store.ID)
	}
	if store.ChainID != chainID {
		t.Errorf("expected chain ID %s, got %s", chainID, store.ChainID)
	}
}

func TestStoreRepository_GetByIDAndChain_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()
	chainID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(storeID, chainID).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByIDAndChain(context.Background(), storeID, chainID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}
}

func TestStoreRepository_NearbyStores(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()
	chainID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "chain_id", "name", "address", "city", "state", "pincode",
		"latitude", "longitude", "capacity", "catalog_version",
		"qr_only_mode", "is_active", "created_at", "updated_at",
		"distance_km",
	}).AddRow(
		storeID, chainID, "Nearby Store", "456 Oak St", "City", "State", "123456",
		nil, nil, int32(50), int32(1),
		false, true, now, now,
		1.5,
	)

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(12.34, 56.78, 5.0).
		WillReturnRows(rows)

	stores, distances, err := repo.NearbyStores(context.Background(), 12.34, 56.78, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stores) != 1 {
		t.Errorf("expected 1 store, got %d", len(stores))
	}
	if len(distances) != 1 {
		t.Errorf("expected 1 distance, got %d", len(distances))
	}
	if distances[0] != 1.5 {
		t.Errorf("expected distance 1.5, got %f", distances[0])
	}
}

func TestStoreRepository_NearbyStores_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(12.34, 56.78, 5.0).
		WillReturnError(errors.New("db error"))

	_, _, err = repo.NearbyStores(context.Background(), 12.34, 56.78, 5.0)
	if err == nil {
		t.Error("expected error")
	}
}

func TestStoreRepository_NearbyStores_ScanError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "chain_id", "name", "address", "city", "state", "pincode",
		"latitude", "longitude", "capacity", "catalog_version",
		"qr_only_mode", "is_active", "created_at", "updated_at",
		"distance_km",
	}).AddRow(
		"invalid-uuid", "invalid-uuid", "Store", "Addr", "City", "State", "123456",
		nil, nil, int32(50), int32(1),
		false, true, now, now,
		1.5,
	)

	mock.ExpectQuery("SELECT (.+) FROM stores").
		WithArgs(12.34, 56.78, 5.0).
		WillReturnRows(rows)

	_, _, err = repo.NearbyStores(context.Background(), 12.34, 56.78, 5.0)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}

func TestStoreRepository_GetHours(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()

	rows := pgxmock.NewRows([]string{"store_id", "day_of_week", "opens_at", "closes_at"}).
		AddRow(storeID, 1, "09:00", "22:00")

	mock.ExpectQuery("SELECT (.+) FROM store_hours").
		WithArgs(storeID, 1).
		WillReturnRows(rows)

	hours, err := repo.GetHours(context.Background(), storeID, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if hours.StoreID != storeID {
		t.Errorf("expected store ID %s, got %s", storeID, hours.StoreID)
	}
	if hours.OpensAt != "09:00" {
		t.Errorf("expected opens_at 09:00, got %s", hours.OpensAt)
	}
	if hours.ClosesAt != "22:00" {
		t.Errorf("expected closes_at 22:00, got %s", hours.ClosesAt)
	}
}

func TestStoreRepository_GetHours_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewStoreRepository(mock)
	storeID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM store_hours").
		WithArgs(storeID, 1).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetHours(context.Background(), storeID, 1)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected pgx.ErrNoRows, got %v", err)
	}
}
