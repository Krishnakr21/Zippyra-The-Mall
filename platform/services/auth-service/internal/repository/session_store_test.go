package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pashagolub/pgxmock/v5"
)

func TestSessionStoreDB_CreateSession_Success(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	store := NewSessionStoreDBWithDB(mock)
	uid := uuid.New()

	mock.ExpectExec("INSERT INTO auth_sessions").
		WithArgs(uid, "d1", "m1", "1.2.3.4", "ua").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := store.CreateSession(context.Background(), uid, "d1", "m1", "1.2.3.4", "ua"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionStoreDB_CreateSession_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()

	store := NewSessionStoreDBWithDB(mock)
	uid := uuid.New()

	mock.ExpectExec("INSERT INTO auth_sessions").WillReturnError(errors.New("db"))
	if err := store.CreateSession(context.Background(), uid, "d1", "m1", "ip", "ua"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_UpdateSessionActivity(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New().String()
	uuidVal, _ := uuid.Parse(uid)
	mock.ExpectExec("UPDATE auth_sessions SET last_active_at").
		WithArgs(uuidVal, "d1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := store.UpdateSessionActivity(context.Background(), uid, "d1"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestSessionStoreDB_LogoutSession(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New().String()
	uuidVal, _ := uuid.Parse(uid)
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").
		WithArgs(uuidVal, "d1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := store.LogoutSession(context.Background(), uid, "d1"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestSessionStoreDB_HasActiveSession_InvalidUserID(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	_, err := store.HasActiveSession(context.Background(), "not-a-uuid", "d1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_HasActiveSession_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New().String()
	uuidVal, _ := uuid.Parse(uid)
	mock.ExpectQuery("SELECT EXISTS").WithArgs(uuidVal, "d1").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

	ok, err := store.HasActiveSession(context.Background(), uid, "d1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !ok {
		t.Fatal("expected true")
	}
}

func TestSessionStoreDB_ListSessions_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	sid := uuid.New()
	now := time.Now()

	mock.ExpectQuery("SELECT id, device_id").WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "device_model", "last_active_at", "ip_address"}).
			AddRow(sid, "d1", "m1", now, "ip"))

	rows, err := store.ListSessions(context.Background(), uid)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
}

func TestSessionStoreDB_ListSessions_QueryError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectQuery("SELECT id, device_id").WithArgs(uid).
		WillReturnError(errors.New("q"))
	_, err := store.ListSessions(context.Background(), uid)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_RevokeSession_NotFound(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	sid := uuid.New()

	mock.ExpectQuery("SELECT device_id FROM auth_sessions").WithArgs(sid, uid).
		WillReturnError(pgx.ErrNoRows)

	if err := store.RevokeSession(context.Background(), uid, sid); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_RevokeSession_Success(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	sid := uuid.New()

	mock.ExpectQuery("SELECT device_id FROM auth_sessions").WithArgs(sid, uid).
		WillReturnRows(pgxmock.NewRows([]string{"device_id"}).AddRow("d1"))
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").WithArgs(sid).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	if err := store.RevokeSession(context.Background(), uid, sid); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestSessionStoreDB_RevokeAllSessions(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").WithArgs(uid, "d1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 3))

	n, err := store.RevokeAllSessions(context.Background(), uid, "d1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3, got %d", n)
	}
}

func TestSessionStoreDB_RevokeAllSessions_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").
		WillReturnError(errors.New("e"))

	_, err := store.RevokeAllSessions(context.Background(), uid, "d1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_HasActiveSession_ScanError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectQuery("SELECT EXISTS").
		WithArgs(uid, "d1").
		WillReturnError(errors.New("scan"))

	_, err := store.HasActiveSession(context.Background(), uid.String(), "d1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_RevokeSession_SelectError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	sid := uuid.New()
	mock.ExpectQuery("SELECT device_id FROM auth_sessions").WithArgs(sid, uid).
		WillReturnError(errors.New("db"))

	if err := store.RevokeSession(context.Background(), uid, sid); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_RevokeSession_UpdateError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	sid := uuid.New()
	mock.ExpectQuery("SELECT device_id FROM auth_sessions").WithArgs(sid, uid).
		WillReturnRows(pgxmock.NewRows([]string{"device_id"}).AddRow("d1"))
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").WithArgs(sid).
		WillReturnError(errors.New("upd"))

	if err := store.RevokeSession(context.Background(), uid, sid); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_UpdateSessionActivity_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New().String()
	uuidVal, _ := uuid.Parse(uid)
	mock.ExpectExec("UPDATE auth_sessions SET last_active_at").
		WithArgs(uuidVal, "d1").
		WillReturnError(errors.New("e"))

	if err := store.UpdateSessionActivity(context.Background(), uid, "d1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_LogoutSession_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New().String()
	uuidVal, _ := uuid.Parse(uid)
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").
		WithArgs(uuidVal, "d1").
		WillReturnError(errors.New("e"))

	if err := store.LogoutSession(context.Background(), uid, "d1"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_ListSessions_ScanError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectQuery("SELECT id, device_id").WithArgs(uid).
		WillReturnRows(pgxmock.NewRows([]string{"id", "device_id", "device_model", "last_active_at", "ip_address"}).
			AddRow("bad-uuid", "d1", "m1", time.Now(), "ip"))

	_, err := store.ListSessions(context.Background(), uid)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSessionStoreDB_RevokeAllSessions_ContextTimeout(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	uid := uuid.New()
	mock.ExpectExec("UPDATE auth_sessions SET logged_out_at").WillReturnError(context.Canceled)
	_, _ = store.RevokeAllSessions(ctx, uid, "d1")
}

func TestSessionStoreDB_HasActiveSession_False(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	store := NewSessionStoreDBWithDB(mock)

	uid := uuid.New()
	mock.ExpectQuery("SELECT EXISTS").WithArgs(uid, "d1").
		WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))

	ok, err := store.HasActiveSession(context.Background(), uid.String(), "d1")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if ok {
		t.Fatal("expected false")
	}
}

func TestLoginAttemptRepository_Insert_ExecError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := &LoginAttemptRepository{db: mock}

	mock.ExpectExec("INSERT INTO login_attempts").WillReturnError(errors.New("e"))
	if err := repo.Insert(context.Background(), "p", "ip", "ua", "SENT"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAppVersionRepository_GetLatest_ScanError(t *testing.T) {
	mock, _ := pgxmock.NewPool()
	defer mock.Close()
	repo := NewAppVersionRepository(mock)

	mock.ExpectQuery("FROM app_versions").WithArgs("android").WillReturnError(errors.New("e"))
	_, err := repo.GetLatest(context.Background(), "android")
	if err == nil {
		t.Fatal("expected error")
	}
}

var _ = errors.New
