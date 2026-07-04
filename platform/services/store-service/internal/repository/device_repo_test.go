package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
)

func TestNewDeviceRepository(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewDeviceRepository(mock)
	if repo == nil {
		t.Error("expected repo to not be nil")
	}
	if repo.db != mock {
		t.Error("expected db to be set")
	}
}

func TestDeviceRepository_ListByStoreID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewDeviceRepository(mock)
	storeID := uuid.New()
	deviceID := uuid.New()
	now := time.Now()

	rows := pgxmock.NewRows([]string{
		"id", "store_id", "device_type", "serial_number", "is_online",
		"last_heartbeat_at", "firmware_version", "created_at", "updated_at",
	}).AddRow(
		deviceID, storeID, "POS", "SN12345", true,
		now, "v1.0.0", now, now,
	)

	mock.ExpectQuery("SELECT (.+) FROM devices").
		WithArgs(storeID).
		WillReturnRows(rows)

	devices, err := repo.ListByStoreID(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 1 {
		t.Errorf("expected 1 device, got %d", len(devices))
	}
	if devices[0].ID != deviceID {
		t.Errorf("expected device ID %s, got %s", deviceID, devices[0].ID)
	}
	if devices[0].SerialNumber != "SN12345" {
		t.Errorf("expected serial number 'SN12345', got '%s'", devices[0].SerialNumber)
	}
}

func TestDeviceRepository_ListByStoreID_Empty(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewDeviceRepository(mock)
	storeID := uuid.New()

	rows := pgxmock.NewRows([]string{
		"id", "store_id", "device_type", "serial_number", "is_online",
		"last_heartbeat_at", "firmware_version", "created_at", "updated_at",
	})

	mock.ExpectQuery("SELECT (.+) FROM devices").
		WithArgs(storeID).
		WillReturnRows(rows)

	devices, err := repo.ListByStoreID(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(devices))
	}
}

func TestDeviceRepository_ListByStoreID_QueryError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewDeviceRepository(mock)
	storeID := uuid.New()

	mock.ExpectQuery("SELECT (.+) FROM devices").
		WithArgs(storeID).
		WillReturnError(errors.New("db error"))

	_, err = repo.ListByStoreID(context.Background(), storeID)
	if err == nil {
		t.Error("expected error")
	}
}

func TestDeviceRepository_ListByStoreID_ScanError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewDeviceRepository(mock)
	storeID := uuid.New()
	now := time.Now()

	// Return invalid data that will fail scan
	rows := pgxmock.NewRows([]string{
		"id", "store_id", "device_type", "serial_number", "is_online",
		"last_heartbeat_at", "firmware_version", "created_at", "updated_at",
	}).AddRow(
		"invalid-uuid", storeID, "POS", "SN12345", true,
		now, "v1.0.0", now, now,
	)

	mock.ExpectQuery("SELECT (.+) FROM devices").
		WithArgs(storeID).
		WillReturnRows(rows)

	_, err = repo.ListByStoreID(context.Background(), storeID)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}
}
