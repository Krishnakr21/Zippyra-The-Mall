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

func TestNewQRTokenRepository(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)
	if repo == nil {
		t.Error("expected repo to not be nil")
	}
	if repo.db != mock {
		t.Error("expected db to be set")
	}
}

func TestQRTokenRepository_GetActiveToken(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)
	tokenID := uuid.New()
	storeID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(time.Hour)

	rows := pgxmock.NewRows([]string{
		"id", "store_id", "token", "token_type", "used_count", "is_active", "expires_at", "created_at",
	}).AddRow(
		tokenID, storeID, "test-token", "ENTRANCE", 0, true, expiresAt, now,
	)

	mock.ExpectQuery("SELECT (.+) FROM store_qr_tokens").
		WithArgs("test-token").
		WillReturnRows(rows)

	token, err := repo.GetActiveToken(context.Background(), "test-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token == nil {
		t.Fatal("expected token to not be nil")
	}
	if token.ID != tokenID {
		t.Errorf("expected token ID %s, got %s", tokenID, token.ID)
	}
	if token.Token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", token.Token)
	}
	if token.TokenType != "ENTRANCE" {
		t.Errorf("expected token type 'ENTRANCE', got '%s'", token.TokenType)
	}
}

func TestQRTokenRepository_GetActiveToken_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)

	mock.ExpectQuery("SELECT (.+) FROM store_qr_tokens").
		WithArgs("nonexistent").
		WillReturnError(pgx.ErrNoRows)

	token, err := repo.GetActiveToken(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != nil {
		t.Error("expected nil token for not found")
	}
}

func TestQRTokenRepository_GetActiveToken_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)

	mock.ExpectQuery("SELECT (.+) FROM store_qr_tokens").
		WithArgs("test").
		WillReturnError(errors.New("db error"))

	_, err = repo.GetActiveToken(context.Background(), "test")
	if err == nil {
		t.Error("expected error")
	}
}

func TestQRTokenRepository_IncrementUsedCount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)
	tokenID := uuid.New()

	rows := pgxmock.NewRows([]string{"id"}).AddRow(tokenID)

	mock.ExpectQuery("UPDATE store_qr_tokens").
		WithArgs(tokenID).
		WillReturnRows(rows)

	err = repo.IncrementUsedCount(context.Background(), tokenID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestQRTokenRepository_IncrementUsedCount_Error(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer mock.Close()

	repo := NewQRTokenRepository(mock)
	tokenID := uuid.New()

	mock.ExpectQuery("UPDATE store_qr_tokens").
		WithArgs(tokenID).
		WillReturnError(errors.New("db error"))

	err = repo.IncrementUsedCount(context.Background(), tokenID)
	if err == nil {
		t.Error("expected error")
	}
}
