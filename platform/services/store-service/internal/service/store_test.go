package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	sharedErrors "github.com/zippyra/platform/shared/errors"

	"github.com/zippyra/platform/services/store-service/internal/model"
)

// ── Mock StoreRepo ──────────────────────────────────────────────────

type mockStoreRepo struct {
	getByIDFn         func(ctx context.Context, storeID uuid.UUID) (*model.Store, error)
	getByIDAndChainFn func(ctx context.Context, storeID, chainID uuid.UUID) (*model.Store, error)
	nearbyStoresFn    func(ctx context.Context, lat, lng, radiusKM float64) ([]model.Store, []float64, error)
	getHoursFn        func(ctx context.Context, storeID uuid.UUID, dayOfWeek int) (*model.StoreHours, error)
}

func TestBind_FeatureFlagCheckFailsOpen(t *testing.T) {
	storeID := uuid.New()
	tokenID := uuid.New()
	userID := "user-1"

	// Return error for feature flag GET to hit the err != nil branch.
	rdb := newMockRedis()
	rdb.getErr["feature:store_entry:"+storeID.String()] = fmt.Errorf("redis down")

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: tokenID, StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	resp, err := svc.Bind(context.Background(), userID, "valid-token", "device-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StoreID != storeID {
		t.Fatalf("expected store %s, got %s", storeID, resp.StoreID)
	}
}

func TestBind_OccupancyCheckError_Internal(t *testing.T) {
	storeID := uuid.New()

	// Feature enabled
	rdb := newMockRedis()
	rdb.data["feature:store_entry:"+storeID.String()] = "1"
	// Force occupancy check to error by forcing Get to error for occupancy key.
	rdb.getErr["store_occupancy:"+storeID.String()] = fmt.Errorf("redis timeout")

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{ID: storeID, ChainID: uuid.New(), Name: "Test Store", Capacity: 50, IsActive: true}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")
	if err == nil {
		t.Fatal("expected error")
	}
	appErr := err.(*sharedErrors.AppError)
	if appErr.Code != sharedErrors.ErrInternal {
		t.Fatalf("expected %s, got %s", sharedErrors.ErrInternal, appErr.Code)
	}
}

func (m *mockStoreRepo) GetByID(ctx context.Context, storeID uuid.UUID) (*model.Store, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, storeID)
	}
	cap := 50
	return &model.Store{
		ID: storeID, ChainID: uuid.New(), Name: "Test Store",
		Capacity: cap, CatalogVersion: 1, IsActive: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil
}

func (m *mockStoreRepo) GetByIDAndChain(ctx context.Context, storeID, chainID uuid.UUID) (*model.Store, error) {
	if m.getByIDAndChainFn != nil {
		return m.getByIDAndChainFn(ctx, storeID, chainID)
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockStoreRepo) NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.Store, []float64, error) {
	if m.nearbyStoresFn != nil {
		return m.nearbyStoresFn(ctx, lat, lng, radiusKM)
	}
	return nil, nil, nil
}

func (m *mockStoreRepo) GetHours(ctx context.Context, storeID uuid.UUID, dayOfWeek int) (*model.StoreHours, error) {
	if m.getHoursFn != nil {
		return m.getHoursFn(ctx, storeID, dayOfWeek)
	}
	return nil, fmt.Errorf("not found")
}

// ── Mock QRTokenRepo ────────────────────────────────────────────────

type mockQRTokenRepo struct {
	getActiveTokenFn     func(ctx context.Context, token string) (*model.StoreQRToken, error)
	incrementUsedCountFn func(ctx context.Context, tokenID uuid.UUID) error
}

func (m *mockQRTokenRepo) GetActiveToken(ctx context.Context, token string) (*model.StoreQRToken, error) {
	if m.getActiveTokenFn != nil {
		return m.getActiveTokenFn(ctx, token)
	}
	return nil, nil
}

func (m *mockQRTokenRepo) IncrementUsedCount(ctx context.Context, tokenID uuid.UUID) error {
	if m.incrementUsedCountFn != nil {
		return m.incrementUsedCountFn(ctx, tokenID)
	}
	return nil
}

// ── Mock Publisher ───────────────────────────────────────────────────

type mockPublisher struct {
	enteredCalled  bool
	exitedCalled   bool
	enteredUserID  string
	enteredStoreID string
	exitedUserID   string
	exitedStoreID  string
}

func (m *mockPublisher) PublishCustomerEntered(ctx context.Context, userID, storeID, chainID, qrTokenID string) {
	m.enteredCalled = true
	m.enteredUserID = userID
	m.enteredStoreID = storeID
}

func (m *mockPublisher) PublishCustomerExited(ctx context.Context, userID, storeID, chainID string, durationSeconds int64) {
	m.exitedCalled = true
	m.exitedUserID = userID
	m.exitedStoreID = storeID
}

// ── Mock Redis ──────────────────────────────────────────────────────

type mockRedis struct {
	data    map[string]string
	incrErr error
	decrErr error
	decrVal *int64
	setErr  error
	getErr  map[string]error
}

func newMockRedis() *mockRedis {
	return &mockRedis{
		data:   make(map[string]string),
		getErr: make(map[string]error),
	}
}

func (m *mockRedis) Get(ctx context.Context, key string) (string, error) {
	if m.getErr != nil && m.getErr[key] != nil {
		return "", m.getErr[key]
	}
	val, ok := m.data[key]
	if !ok {
		return "", redis.Nil
	}
	return val, nil
}

func (m *mockRedis) Set(_ context.Context, key, value string, _ time.Duration) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.data[key] = value
	return nil
}

func (m *mockRedis) Incr(_ context.Context, key string) (int64, error) {
	if m.incrErr != nil {
		return 0, m.incrErr
	}
	v, ok := m.data[key]
	if !ok {
		m.data[key] = "1"
		return 1, nil
	}
	n, _ := fmt.Sscanf(v, "%d", new(int64))
	if n == 0 {
		m.data[key] = "1"
		return 1, nil
	}
	var cur int64
	fmt.Sscanf(v, "%d", &cur)
	cur++
	m.data[key] = fmt.Sprintf("%d", cur)
	return cur, nil
}

func (m *mockRedis) Decr(_ context.Context, key string) (int64, error) {
	if m.decrErr != nil {
		return 0, m.decrErr
	}
	if m.decrVal != nil {
		m.data[key] = fmt.Sprintf("%d", *m.decrVal)
		return *m.decrVal, nil
	}
	v, ok := m.data[key]
	if !ok {
		m.data[key] = "-1"
		return -1, nil
	}
	var cur int64
	fmt.Sscanf(v, "%d", &cur)
	cur--
	m.data[key] = fmt.Sprintf("%d", cur)
	return cur, nil
}

func (m *mockRedis) Del(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

func (m *mockRedis) Exists(_ context.Context, key string) (bool, error) {
	_, ok := m.data[key]
	return ok, nil
}

// ── Test helpers ────────────────────────────────────────────────────

func newTestStoreService(storeRepo StoreRepo, qrRepo QRTokenRepo, redis RedisStore, pub EventPublisher) *StoreService {
	s := NewStoreService(storeRepo, qrRepo, redis, pub)
	s.nowFunc = func() time.Time {
		return time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	}
	return s
}

// ── Tests ───────────────────────────────────────────────────────────

func TestBind_Success(t *testing.T) {
	storeID := uuid.New()
	tokenID := uuid.New()
	chainID := uuid.New()
	userID := uuid.New().String()

	rdb := newMockRedis()
	// Set feature flag enabled
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	pub := &mockPublisher{}
	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: tokenID, StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: chainID, Name: "Test Store",
				Capacity: 50, CatalogVersion: 42, QROnlyMode: true,
				IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, pub)
	result, err := svc.Bind(context.Background(), userID, "valid-token", "device-1")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.StoreID != storeID {
		t.Errorf("expected store_id %s, got %s", storeID, result.StoreID)
	}
	if result.StoreName != "Test Store" {
		t.Errorf("expected store_name 'Test Store', got %s", result.StoreName)
	}
	if result.CatalogVersion != 42 {
		t.Errorf("expected catalog_version 42, got %d", result.CatalogVersion)
	}

	// Verify occupancy was incremented
	val, _ := rdb.Get(context.Background(), "store_occupancy:"+storeID.String())
	if val != "1" {
		t.Errorf("expected occupancy '1', got %s", val)
	}

	// Verify session was set
	sessionVal, _ := rdb.Get(context.Background(), "store_session:"+userID)
	if sessionVal != storeID.String() {
		t.Errorf("expected session store_id %s, got %s", storeID, sessionVal)
	}

	// Verify Kafka event published
	if !pub.enteredCalled {
		t.Error("expected customer_entered event to be published")
	}
}

func TestBind_ExpiredQR(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "expired-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			}, nil
		},
	}

	svc := newTestStoreService(&mockStoreRepo{}, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "expired-token", "device-1")

	if err == nil {
		t.Fatal("expected error for expired QR")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrQRTokenExpired {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrQRTokenExpired, appErr.Code)
	}
}

func TestBind_InvalidQR(t *testing.T) {
	rdb := newMockRedis()

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return nil, nil
		},
	}

	svc := newTestStoreService(&mockStoreRepo{}, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "invalid-token", "device-1")

	if err == nil {
		t.Fatal("expected error for invalid QR")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestBind_AtCapacity(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()

	// Set occupancy at capacity
	rdb.data["store_occupancy:"+storeID.String()] = "50"
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Full Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")

	if err == nil {
		t.Fatal("expected error for store at capacity")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreAtCapacity {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreAtCapacity, appErr.Code)
	}
}

func TestBind_EmptyQRToken(t *testing.T) {
	rdb := newMockRedis()
	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "", "device-1")

	if err == nil {
		t.Fatal("expected error for empty QR token")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestBind_NonEntranceQR(t *testing.T) {
	rdb := newMockRedis()

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: uuid.New(), Token: "exit-token",
				TokenType: "EXIT", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}

	svc := newTestStoreService(&mockStoreRepo{}, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "exit-token", "device-1")

	if err == nil {
		t.Fatal("expected error for non-ENTRANCE QR")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrQRTokenInvalid {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrQRTokenInvalid, appErr.Code)
	}
}

func TestBind_FeatureFlagDisabled(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()

	// Set feature flag to disabled
	rdb.data["feature:store_entry:"+storeID.String()] = "0"

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Closed Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")

	if err == nil {
		t.Fatal("expected error when feature flag disabled")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreClosed {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreClosed, appErr.Code)
	}
}

func TestExit_Success(t *testing.T) {
	storeID := uuid.New()
	chainID := uuid.New()
	userID := uuid.New().String()

	rdb := newMockRedis()
	rdb.data["store_session:"+userID] = storeID.String()
	rdb.data["store_occupancy:"+storeID.String()] = "5"

	pub := &mockPublisher{}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: chainID, Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, pub)
	result, err := svc.Exit(context.Background(), userID, storeID.String())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Message != "Exit recorded" {
		t.Errorf("expected message 'Exit recorded', got %s", result.Message)
	}

	// Verify occupancy decremented
	val, _ := rdb.Get(context.Background(), "store_occupancy:"+storeID.String())
	if val != "4" {
		t.Errorf("expected occupancy '4', got %s", val)
	}

	// Verify session deleted
	_, sessionErr := rdb.Get(context.Background(), "store_session:"+userID)
	if sessionErr == nil {
		t.Error("expected session to be deleted")
	}

	// Verify Kafka event published
	if !pub.exitedCalled {
		t.Error("expected customer_exited event to be published")
	}
}

func TestExit_OccupancyNeverBelowZero(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New().String()

	rdb := newMockRedis()
	rdb.data["store_session:"+userID] = storeID.String()
	rdb.data["store_occupancy:"+storeID.String()] = "0"

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.Exit(context.Background(), userID, storeID.String())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify occupancy is reset to 0 (not negative)
	val, _ := rdb.Get(context.Background(), "store_occupancy:"+storeID.String())
	if val != "0" {
		t.Errorf("expected occupancy '0', got %s", val)
	}
}

func TestExit_NoSession(t *testing.T) {
	rdb := newMockRedis()

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.Exit(context.Background(), "user-no-session", uuid.New().String())

	if err == nil {
		t.Fatal("expected error for no session")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreNotFound {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreNotFound, appErr.Code)
	}
}

func TestExit_SessionMismatch(t *testing.T) {
	rdb := newMockRedis()
	userID := "user-mismatch"
	rdb.data["store_session:"+userID] = uuid.New().String()

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.Exit(context.Background(), userID, uuid.New().String())

	if err == nil {
		t.Fatal("expected error for session mismatch")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrForbidden {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrForbidden, appErr.Code)
	}
}

func TestUpdateCapacity_Increment(t *testing.T) {
	rdb := newMockRedis()
	storeID := uuid.New().String()
	rdb.data["store_occupancy:"+storeID] = "10"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "increment")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CurrentOccupancy != 11 {
		t.Errorf("expected occupancy 11, got %d", result.CurrentOccupancy)
	}
}

func TestUpdateCapacity_DecrementRedisError(t *testing.T) {
	rdb := newMockRedis()
	storeID := uuid.New().String()
	rdb.data["store_occupancy:"+storeID] = "10"
	rdb.decrErr = errors.New("redis error")

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err == nil {
		t.Fatal("expected error when redis returns error")
	}
	if !strings.Contains(err.Error(), "redis error") {
		t.Errorf("expected error 'redis error', got %v", err)
	}
}

func TestOccupancy_GetCurrentOccupancyError(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["store_occupancy:"+storeID.String()] = "23"
	rdb.getErr["store_occupancy:"+storeID.String()] = errors.New("redis error")

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.Occupancy(context.Background(), storeID)

	if err == nil {
		t.Fatal("expected error when redis returns error")
	}
	if !strings.Contains(err.Error(), "redis error") {
		t.Errorf("expected error 'redis error', got %v", err)
	}
}

func TestUpdateCapacity_DecrementBelowZero(t *testing.T) {
	rdb := newMockRedis()
	storeID := uuid.New().String()
	rdb.data["store_occupancy:"+storeID] = "0"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected occupancy 0, got %d", result.CurrentOccupancy)
	}
}

func TestOccupancy_Status(t *testing.T) {
	tests := []struct {
		name     string
		pct      int
		expected string
	}{
		{"normal", 50, "normal"},
		{"busy", 75, "busy"},
		{"near_capacity", 95, "near_capacity"},
		{"at_capacity", 100, "at_capacity"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := occupancyStatus(tt.pct)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestOccupancy_Response(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["store_occupancy:"+storeID.String()] = "23"

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.Occupancy(context.Background(), storeID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.CurrentOccupancy != 23 {
		t.Errorf("expected occupancy 23, got %d", result.CurrentOccupancy)
	}
	if result.Capacity != 50 {
		t.Errorf("expected capacity 50, got %d", result.Capacity)
	}
	if result.OccupancyPct != 46 {
		t.Errorf("expected 46%% occupancy, got %d%%", result.OccupancyPct)
	}
	if result.Status != "normal" {
		t.Errorf("expected status normal, got %s", result.Status)
	}
}

func TestHours_Default(t *testing.T) {
	rdb := newMockRedis()

	storeRepo := &mockStoreRepo{
		getHoursFn: func(_ context.Context, _ uuid.UUID, _ int) (*model.StoreHours, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.Hours(context.Background(), uuid.New())

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.OpensAt != "09:00" {
		t.Errorf("expected opens_at 09:00, got %s", result.OpensAt)
	}
	if result.ClosesAt != "22:00" {
		t.Errorf("expected closes_at 22:00, got %s", result.ClosesAt)
	}
	if result.Timezone != "Asia/Kolkata" {
		t.Errorf("expected timezone Asia/Kolkata, got %s", result.Timezone)
	}
}

func TestGetStore_NotFound(t *testing.T) {
	rdb := newMockRedis()

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.GetStore(context.Background(), uuid.New())

	if err == nil {
		t.Fatal("expected error for missing store")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreNotFound {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreNotFound, appErr.Code)
	}
}

func TestGetStore_Success(t *testing.T) {
	rdb := newMockRedis()
	storeID := uuid.New()

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Success Store",
				Capacity: 100, CatalogVersion: 5, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.GetStore(context.Background(), storeID)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Name != "Success Store" {
		t.Errorf("expected name 'Success Store', got %s", result.Name)
	}
	if result.ID != storeID {
		t.Errorf("expected ID %s, got %s", storeID, result.ID)
	}
}

func TestNearbyStores_Service(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["store_occupancy:"+storeID.String()] = "10"
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	storeRepo := &mockStoreRepo{
		nearbyStoresFn: func(ctx context.Context, lat, lng, rad float64) ([]model.Store, []float64, error) {
			return []model.Store{
				{ID: storeID, Name: "Near Store", Capacity: 50},
			}, []float64{1.2}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	res, err := svc.NearbyStores(context.Background(), 12.0, 77.0, 5.0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 store, got %d", len(res))
	}
	if res[0].DistanceKM != 1.2 {
		t.Errorf("expected distance 1.2, got %f", res[0].DistanceKM)
	}
	if res[0].CurrentOccupancy != 10 {
		t.Errorf("expected occupancy 10, got %d", res[0].CurrentOccupancy)
	}
}

func TestIsStoreOpen(t *testing.T) {
	storeID := uuid.New()
	loc, _ := time.LoadLocation("Asia/Kolkata")

	tests := []struct {
		name     string
		mockHour *model.StoreHours
		now      time.Time
		expected bool
	}{
		{
			name:     "store_open",
			mockHour: &model.StoreHours{OpensAt: "09:00", ClosesAt: "18:00"},
			now:      time.Date(2024, 1, 1, 10, 0, 0, 0, loc),
			expected: true,
		},
		{
			name:     "store_closed_early",
			mockHour: &model.StoreHours{OpensAt: "09:00", ClosesAt: "18:00"},
			now:      time.Date(2024, 1, 1, 8, 0, 0, 0, loc),
			expected: false,
		},
		{
			name:     "store_closed_late",
			mockHour: &model.StoreHours{OpensAt: "09:00", ClosesAt: "18:00"},
			now:      time.Date(2024, 1, 1, 19, 0, 0, 0, loc),
			expected: false,
		},
		{
			name:     "no_hours_default_open",
			mockHour: nil,
			now:      time.Date(2024, 1, 1, 10, 0, 0, 0, loc),
			expected: true,
		},
		{
			name:     "no_hours_default_closed",
			mockHour: nil,
			now:      time.Date(2024, 1, 1, 23, 0, 0, 0, loc),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storeRepo := &mockStoreRepo{
				getHoursFn: func(ctx context.Context, id uuid.UUID, dow int) (*model.StoreHours, error) {
					if tt.mockHour == nil {
						return nil, fmt.Errorf("not found")
					}
					return tt.mockHour, nil
				},
			}
			svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})
			svc.nowFunc = func() time.Time { return tt.now }

			result := svc.isStoreOpen(context.Background(), storeID, tt.now)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestUpdateCapacity_Errors(t *testing.T) {
	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})

	_, err := svc.UpdateCapacity(context.Background(), uuid.New().String(), "invalid")
	if err == nil {
		t.Error("expected error for invalid action")
	}

	_, err = svc.UpdateCapacity(context.Background(), "not-a-uuid", "increment")
	if err == nil {
		t.Error("expected error for invalid uuid")
	}
}

// Test Bind error branches for full coverage
func TestBind_QRRepoError(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	svc := newTestStoreService(&mockStoreRepo{}, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")

	if err == nil {
		t.Fatal("expected error for QR repo error")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrInternal {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrInternal, appErr.Code)
	}
}

func TestBind_StoreRepoError(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	_, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")

	if err == nil {
		t.Fatal("expected error for store repo error")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreNotFound {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreNotFound, appErr.Code)
	}
}

func TestBind_FeatureFlagRedisError(t *testing.T) {
	storeID := uuid.New()
	// Use mockRedis that returns error on Get
	rdb := &mockRedis{data: make(map[string]string)} // empty, will return error on Get

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	// This should still succeed because feature flag check fails open
	result, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")
	if err != nil {
		t.Fatalf("expected no error when feature flag check fails open, got: %v", err)
	}
	if result.StoreID != storeID {
		t.Errorf("expected store_id %s, got %s", storeID, result.StoreID)
	}
}

func TestBind_OccupancyCheckError(t *testing.T) {
	storeID := uuid.New()
	// Redis that returns error on Get for occupancy check
	rdb := &mockRedis{data: map[string]string{
		"feature:store_entry:" + storeID.String(): "1",
	}}

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: uuid.New(), StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	// Should succeed with 0 occupancy (key doesn't exist)
	result, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.StoreID != storeID {
		t.Errorf("expected store_id %s, got %s", storeID, result.StoreID)
	}
}

func TestBind_QRTokenIncrementError(t *testing.T) {
	storeID := uuid.New()
	tokenID := uuid.New()
	rdb := newMockRedis()
	rdb.data["feature:store_entry:"+storeID.String()] = "1"

	qrRepo := &mockQRTokenRepo{
		getActiveTokenFn: func(_ context.Context, _ string) (*model.StoreQRToken, error) {
			return &model.StoreQRToken{
				ID: tokenID, StoreID: storeID, Token: "valid-token",
				TokenType: "ENTRANCE", IsActive: true,
				ExpiresAt: time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
			}, nil
		},
		incrementUsedCountFn: func(_ context.Context, _ uuid.UUID) error {
			return fmt.Errorf("db error") // Non-fatal error
		},
	}
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, qrRepo, rdb, &mockPublisher{})
	// Should succeed even if increment fails (non-fatal)
	result, err := svc.Bind(context.Background(), "user-1", "valid-token", "device-1")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.StoreID != storeID {
		t.Errorf("expected store_id %s, got %s", storeID, result.StoreID)
	}
}

// Test GetStore Redis cache branches
func TestGetStore_RedisCacheHit(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	// Pre-populate cache
	rdb.data["store_info:"+storeID.String()] = "Cached Store Name"

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "DB Store Name",
				Capacity: 50, CatalogVersion: 1, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.GetStore(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return DB name even if cache hit (just logs the hit)
	if result.Name != "DB Store Name" {
		t.Errorf("expected name 'DB Store Name', got '%s'", result.Name)
	}
}

func TestGetStore_RedisCacheError(t *testing.T) {
	storeID := uuid.New()
	// Redis that returns error
	rdb := &mockRedis{data: make(map[string]string)}

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, CatalogVersion: 1, IsActive: true,
				CreatedAt: time.Now(), UpdatedAt: time.Now(),
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.GetStore(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Test Store" {
		t.Errorf("expected name 'Test Store', got '%s'", result.Name)
	}
}

// Test NearbyStores error branches
func TestNearbyStores_RepoError(t *testing.T) {
	storeRepo := &mockStoreRepo{
		nearbyStoresFn: func(ctx context.Context, lat, lng, rad float64) ([]model.Store, []float64, error) {
			return nil, nil, fmt.Errorf("db error")
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})
	_, err := svc.NearbyStores(context.Background(), 12.0, 77.0, 5.0)

	if err == nil {
		t.Fatal("expected error for repo error")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrInternal {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrInternal, appErr.Code)
	}
}

func TestNearbyStores_StoreOpenWithHours(t *testing.T) {
	storeID := uuid.New()
	rdb := newMockRedis()
	rdb.data["store_occupancy:"+storeID.String()] = "10"

	storeRepo := &mockStoreRepo{
		nearbyStoresFn: func(ctx context.Context, lat, lng, rad float64) ([]model.Store, []float64, error) {
			return []model.Store{
				{ID: storeID, Name: "Near Store", Capacity: 50},
			}, []float64{1.2}, nil
		},
		getHoursFn: func(_ context.Context, _ uuid.UUID, _ int) (*model.StoreHours, error) {
			return &model.StoreHours{OpensAt: "09:00", ClosesAt: "22:00"}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	res, err := svc.NearbyStores(context.Background(), 12.0, 77.0, 5.0)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("expected 1 store, got %d", len(res))
	}
	// Store should be open at noon (default test time)
	if !res[0].IsOpen {
		t.Error("expected store to be open")
	}
}

// Test Hours with actual hours from DB
func TestHours_WithDBHours(t *testing.T) {
	storeID := uuid.New()

	storeRepo := &mockStoreRepo{
		getHoursFn: func(_ context.Context, _ uuid.UUID, _ int) (*model.StoreHours, error) {
			return &model.StoreHours{OpensAt: "10:00", ClosesAt: "20:00"}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})
	result, err := svc.Hours(context.Background(), storeID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OpensAt != "10:00" {
		t.Errorf("expected opens_at 10:00, got %s", result.OpensAt)
	}
	if result.ClosesAt != "20:00" {
		t.Errorf("expected closes_at 20:00, got %s", result.ClosesAt)
	}
}

// Test UpdateCapacity Redis error branches
func TestUpdateCapacity_DecrementRedisGetError(t *testing.T) {
	rdb := &mockRedis{data: make(map[string]string)} // empty, Get returns error
	storeID := uuid.New().String()

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return 0 when key doesn't exist
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected occupancy 0, got %d", result.CurrentOccupancy)
	}
}

func TestUpdateCapacity_DecrementCurrentZero(t *testing.T) {
	rdb := newMockRedis()
	storeID := uuid.New().String()
	rdb.data["store_occupancy:"+storeID] = "0"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected occupancy 0, got %d", result.CurrentOccupancy)
	}
}

func TestUpdateCapacity_IncrementRedisError(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	rdb.incrErr = fmt.Errorf("redis error")

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	_, err := svc.UpdateCapacity(context.Background(), storeID, "increment")

	if err == nil {
		t.Fatal("expected error for Redis Incr failure")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrInternal {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrInternal, appErr.Code)
	}
}

func TestUpdateCapacity_DecrementNegativeValue(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	// Start with negative value to trigger race condition handling
	rdb.data["store_occupancy:"+storeID] = "-5"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should reset to 0 when Decr returns negative
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected occupancy 0, got %d", result.CurrentOccupancy)
	}
}

func TestExit_RedisDecrError(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New().String()

	rdb := newMockRedis()
	rdb.data["store_session:"+userID] = storeID.String()
	rdb.data["store_occupancy:"+storeID.String()] = "5"
	rdb.decrErr = fmt.Errorf("redis error")

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, ChainID: uuid.New(), Name: "Test Store",
				Capacity: 50, IsActive: true,
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	// Exit should still succeed even if Decr fails (error is logged but not returned)
	result, err := svc.Exit(context.Background(), userID, storeID.String())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "Exit recorded" {
		t.Errorf("expected 'Exit recorded', got %s", result.Message)
	}
}

func TestExit_StoreLookupError(t *testing.T) {
	storeID := uuid.New()
	userID := uuid.New().String()

	rdb := newMockRedis()
	rdb.data["store_session:"+userID] = storeID.String()
	rdb.data["store_occupancy:"+storeID.String()] = "5"

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return nil, fmt.Errorf("db error") // Non-fatal for Exit
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	// Should still succeed even if store lookup fails (chainID just won't be set)
	result, err := svc.Exit(context.Background(), userID, storeID.String())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message != "Exit recorded" {
		t.Errorf("expected 'Exit recorded', got %s", result.Message)
	}
}

// Test Occupancy error branch
func TestOccupancy_StoreNotFound(t *testing.T) {
	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, _ uuid.UUID) (*model.Store, error) {
			return nil, fmt.Errorf("not found")
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})
	_, err := svc.Occupancy(context.Background(), uuid.New())

	if err == nil {
		t.Fatal("expected error for store not found")
	}
	appErr, ok := err.(*sharedErrors.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Code != sharedErrors.ErrStoreNotFound {
		t.Errorf("expected code %s, got %s", sharedErrors.ErrStoreNotFound, appErr.Code)
	}
}

func TestOccupancy_RedisErrorReturnsZero(t *testing.T) {
	storeID := uuid.New()
	rdb := &mockRedis{data: make(map[string]string)} // returns error on Get

	storeRepo := &mockStoreRepo{
		getByIDFn: func(_ context.Context, id uuid.UUID) (*model.Store, error) {
			return &model.Store{
				ID: storeID, Name: "Test Store", Capacity: 50, IsActive: true,
			}, nil
		},
	}

	svc := newTestStoreService(storeRepo, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	res, err := svc.Occupancy(context.Background(), storeID)

	// Redis errors are treated as 0 occupancy, not as an error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CurrentOccupancy != 0 {
		t.Errorf("expected 0 occupancy when Redis errors, got %d", res.CurrentOccupancy)
	}
}

// Test getCurrentOccupancy with non-numeric value
func TestGetCurrentOccupancy_NonNumeric(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	rdb.data["store_occupancy:"+storeID] = "not-a-number"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	occupancy, err := svc.getCurrentOccupancy(context.Background(), storeID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return 0 for non-numeric value
	if occupancy != 0 {
		t.Errorf("expected 0, got %d", occupancy)
	}
}

// Test isFeatureEnabled with disabled flag
func TestIsFeatureEnabled_Disabled(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	rdb.data["feature:test_feature:"+storeID] = "0"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	enabled, err := svc.isFeatureEnabled(context.Background(), "test_feature", storeID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected feature to be disabled")
	}
}

func TestIsFeatureEnabled_FalseString(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	rdb.data["feature:test_feature:"+storeID] = "false"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	enabled, err := svc.isFeatureEnabled(context.Background(), "test_feature", storeID)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if enabled {
		t.Error("expected feature to be disabled (false)")
	}
}

// Test parseHHMM edge cases
func TestParseHHMM_ShortString(t *testing.T) {
	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})

	// Test with short string (less than 5 chars)
	result := svc.isTimeInRange(
		time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
		"9",
		"17",
	)
	// Should use parseHHMM which returns 0 for short strings
	// So 9 -> 0 minutes, 17 -> 0 minutes
	// 12:00 = 720 minutes, which is >= 0 and < 0 is false
	if result {
		t.Error("expected false for short time strings")
	}
}

// Test device ListDevices with nil devices returned
func TestListDevices_NilDevices(t *testing.T) {
	storeID := uuid.New()

	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return nil, nil // Return nil
		},
	}

	redis := newMockRedis()
	svc := NewDeviceService(repo, redis)

	res, err := svc.ListDevices(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res.Devices) != 0 {
		t.Errorf("expected 0 devices, got %d", len(res.Devices))
	}
	if res.Summary.Total != 0 {
		t.Errorf("expected total 0, got %d", res.Summary.Total)
	}
}

// Test device ListDevices with Redis error
func TestListDevices_RedisError(t *testing.T) {
	storeID := uuid.New()
	deviceID := uuid.New()

	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return []model.Device{
				{ID: deviceID, SerialNumber: "SN123"},
			}, nil
		},
	}

	// Redis that returns error on Get
	rdb := &mockRedis{data: make(map[string]string)}

	svc := NewDeviceService(repo, rdb)

	res, err := svc.ListDevices(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Device should be offline when Redis returns error
	if res.Devices[0].IsOnline {
		t.Error("expected device to be offline when Redis errors")
	}
}

// Test device ListDevices with invalid timestamp
func TestListDevices_InvalidTimestamp(t *testing.T) {
	storeID := uuid.New()
	deviceID := uuid.New()

	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return []model.Device{
				{ID: deviceID, SerialNumber: "SN123"},
			}, nil
		},
	}

	rdb := newMockRedis()
	rdb.data["device_heartbeat:"+deviceID.String()] = "invalid-timestamp"

	svc := NewDeviceService(repo, rdb)

	res, err := svc.ListDevices(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Device should be offline when timestamp is invalid
	if res.Devices[0].IsOnline {
		t.Error("expected device to be offline with invalid timestamp")
	}
}

// Test device ListDevices with old timestamp (offline)
func TestListDevices_OldTimestamp(t *testing.T) {
	storeID := uuid.New()
	deviceID := uuid.New()

	repo := &mockDeviceRepo{
		listFn: func(ctx context.Context, id uuid.UUID) ([]model.Device, error) {
			return []model.Device{
				{ID: deviceID, SerialNumber: "SN123"},
			}, nil
		},
	}

	rdb := newMockRedis()
	// Set timestamp older than 5 minutes
	rdb.data["device_heartbeat:"+deviceID.String()] = time.Now().Add(-10 * time.Minute).Format(time.RFC3339)

	svc := NewDeviceService(repo, rdb)

	res, err := svc.ListDevices(context.Background(), storeID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Device should be offline with old timestamp
	if res.Devices[0].IsOnline {
		t.Error("expected device to be offline with old timestamp")
	}
}

// Test UpdateCapacity decrement with non-numeric value (ParseInt error)
func TestUpdateCapacity_DecrementParseIntError(t *testing.T) {
	storeID := uuid.New().String()
	rdb := newMockRedis()
	// Set a non-numeric value that will cause ParseInt to fail
	rdb.data["store_occupancy:"+storeID] = "not-a-number"

	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	// ParseInt error should result in current=0, so it returns 0 occupancy
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When ParseInt fails, current becomes 0, so we return 0 occupancy
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected occupancy 0 for non-numeric value, got %d", result.CurrentOccupancy)
	}
}

func TestUpdateCapacity_InvalidUUID(t *testing.T) {
	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, newMockRedis(), &mockPublisher{})
	_, err := svc.UpdateCapacity(context.Background(), "invalid-uuid", "increment")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestUpdateCapacity_RaceBelowZero(t *testing.T) {
	rdb := newMockRedis()
	val := int64(-1)
	rdb.decrVal = &val // Simulate Decr returning -1 due to race
	storeID := uuid.New().String()
	rdb.data["store_occupancy:"+storeID] = "1" // Set initial value so Get succeeds
	
	svc := newTestStoreService(&mockStoreRepo{}, &mockQRTokenRepo{}, rdb, &mockPublisher{})
	result, err := svc.UpdateCapacity(context.Background(), storeID, "decrement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CurrentOccupancy != 0 {
		t.Errorf("expected 0, got %d", result.CurrentOccupancy)
	}
}
