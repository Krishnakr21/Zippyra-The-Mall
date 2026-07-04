package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	mw "github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/services/auth-service/internal/model"
)

// ── Mock UserRepo ────────────────────────────────────────────────────

type mockUserRepo struct {
	upsertFn  func(ctx context.Context, phone string) (*model.User, bool, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*model.User, error)
}

func (m *mockUserRepo) UpsertByPhone(ctx context.Context, phone string) (*model.User, bool, error) {
	if m.upsertFn != nil {
		return m.upsertFn(ctx, phone)
	}
	now := time.Now()
	return &model.User{
		ID:        uuid.New(),
		Phone:     phone,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}, true, nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, fmt.Errorf("not implemented")
}

// ── Mock LoginAttemptRepo ────────────────────────────────────────────

type mockLoginAttemptRepo struct {
	insertCalled bool
	updateCalled bool
	insertErr    error
	updateErr    error
	insertDone   chan struct{}
	updateDone   chan struct{}
}

func (m *mockLoginAttemptRepo) Insert(ctx context.Context, phone, ip, userAgent, status string) error {
	m.insertCalled = true
	if m.insertDone != nil {
		close(m.insertDone)
	}
	return m.insertErr
}

func (m *mockLoginAttemptRepo) UpdateStatus(ctx context.Context, phone, status string) error {
	m.updateCalled = true
	if m.updateDone != nil {
		close(m.updateDone)
	}
	return m.updateErr
}

// ── Mock EventPublisher ──────────────────────────────────────────────

type mockPublisher struct {
	published bool
	userID    string
	isNewUser bool
}

func (m *mockPublisher) PublishLoginEvent(ctx context.Context, userID string, isNewUser bool) {
	m.published = true
	m.userID = userID
	m.isNewUser = isNewUser
}

// ── Mock SessionStore ────────────────────────────────────────────────

type mockSessionStore struct {
	createErr    error
	updateErr    error
	logoutErr    error
	hasActive    bool
	hasActiveErr error
	listResult   []SessionRow
	listErr      error
	revokeErr    error
	revokeAllCnt int
	revokeAllErr error
}

func (m *mockSessionStore) CreateSession(ctx context.Context, userID uuid.UUID, deviceID, deviceModel, ip, userAgent string) error {
	return m.createErr
}

func (m *mockSessionStore) UpdateSessionActivity(ctx context.Context, userID, deviceID string) error {
	return m.updateErr
}

func (m *mockSessionStore) LogoutSession(ctx context.Context, userID, deviceID string) error {
	return m.logoutErr
}

func (m *mockSessionStore) HasActiveSession(ctx context.Context, userID, deviceID string) (bool, error) {
	return m.hasActive, m.hasActiveErr
}

func (m *mockSessionStore) ListSessions(ctx context.Context, userID uuid.UUID) ([]SessionRow, error) {
	return m.listResult, m.listErr
}

func (m *mockSessionStore) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	return m.revokeErr
}

func (m *mockSessionStore) RevokeAllSessions(ctx context.Context, userID uuid.UUID, currentDeviceID string) (int, error) {
	return m.revokeAllCnt, m.revokeAllErr
}

// ── Mock TokenGenerator ─────────────────────────────────────────────

type mockTokenGenerator struct {
	genAccessErr  error
	genRefreshErr error
	validateErr   error
	blacklistErr  error
	isBlackErr    error
}

func (m *mockTokenGenerator) GenerateAccessToken(userID, userType, phone, deviceID string) (string, error) {
	if m.genAccessErr != nil {
		return "", m.genAccessErr
	}
	return "mock-access-token", nil
}

func (m *mockTokenGenerator) GenerateRefreshToken(userID, userType, phone, deviceID string) (string, error) {
	if m.genRefreshErr != nil {
		return "", m.genRefreshErr
	}
	return "mock-refresh-token", nil
}

func (m *mockTokenGenerator) ValidateToken(tokenString string) (*mw.Claims, error) {
	if m.validateErr != nil {
		return nil, m.validateErr
	}
	return &mw.Claims{UserID: "user-1", UserType: "CUSTOMER", DeviceID: "d1"}, nil
}

func (m *mockTokenGenerator) BlacklistToken(ctx context.Context, jti string, expiresAt time.Time) error {
	return m.blacklistErr
}

func (m *mockTokenGenerator) IsBlacklisted(ctx context.Context, jti string) (bool, error) {
	return false, m.isBlackErr
}
