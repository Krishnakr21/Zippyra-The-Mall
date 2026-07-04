package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v5"
)

func TestUserRepository_UpsertByPhone_Success_NewUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewUserRepositoryWithDB(mock)
	id := uuid.New()
	now := time.Now()
	lastLogin := now

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("+919876543210").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "phone", "email", "full_name", "is_active", "is_verified", "app_version", "device_token", "referral_code", "last_login_at", "created_at", "updated_at",
		}).AddRow(id, "+919876543210", nil, nil, true, false, nil, nil, nil, &lastLogin, now, now))

	user, isNew, err := repo.UpsertByPhone(context.Background(), "+919876543210")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if user == nil || user.ID != id {
		t.Fatal("unexpected user")
	}
	if !isNew {
		t.Fatal("expected isNew=true")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUserRepository_UpsertByPhone_ScanError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewUserRepositoryWithDB(mock)
	mock.ExpectQuery("INSERT INTO users").
		WithArgs("+919876543210").
		WillReturnError(errors.New("db"))

	_, _, err = repo.UpsertByPhone(context.Background(), "+919876543210")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUserRepository_GetByID_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewUserRepositoryWithDB(mock)
	id := uuid.New()
	now := time.Now()
	lastLogin := now

	mock.ExpectQuery("SELECT id, phone").
		WithArgs(id).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "phone", "email", "full_name", "is_active", "is_verified", "app_version", "device_token", "referral_code", "last_login_at", "created_at", "updated_at",
		}).AddRow(id, "+919876543210", nil, nil, true, false, nil, nil, nil, &lastLogin, now, now))

	user, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if user.ID != id {
		t.Fatal("unexpected user")
	}
}

func TestUserRepository_GetByID_NotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewUserRepositoryWithDB(mock)
	id := uuid.New()

	mock.ExpectQuery("SELECT id, phone").
		WithArgs(id).
		WillReturnError(pgx.ErrNoRows)

	_, err = repo.GetByID(context.Background(), id)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoginAttemptRepository_InsertAndUpdate(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := &LoginAttemptRepository{db: mock}
	mock.ExpectExec("INSERT INTO login_attempts").
		WithArgs("p", "ip", "ua", "SENT").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Insert(context.Background(), "p", "ip", "ua", "SENT"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	mock.ExpectExec("UPDATE login_attempts SET status").
		WithArgs("p", "SUCCESS").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := repo.UpdateStatus(context.Background(), "p", "SUCCESS"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestAppVersionRepository_GetLatest(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := NewAppVersionRepository(mock)
	id := uuid.New()
	now := time.Now()

	mock.ExpectQuery("FROM app_versions").
		WithArgs("android").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "platform", "version", "min_supported_version", "is_force_update", "release_notes", "created_at", "updated_at",
		}).AddRow(id, "android", "1.2.3", "1.0.0", false, nil, now, now))

	av, err := repo.GetLatest(context.Background(), "android")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if av.Platform != "android" {
		t.Fatal("unexpected platform")
	}
}

func TestConstructors_Coverage(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	if NewUserRepository(mock) == nil {
		t.Fatal("expected user repo")
	}
	if NewLoginAttemptRepository(mock) == nil {
		t.Fatal("expected login attempt repo")
	}
	if NewLoginAttemptRepositoryWithDB(mock) == nil {
		t.Fatal("expected login attempt repo")
	}
	if NewSessionStoreDB(mock) == nil {
		t.Fatal("expected session store")
	}
}
