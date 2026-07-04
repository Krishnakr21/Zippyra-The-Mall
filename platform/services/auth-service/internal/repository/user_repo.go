package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/zippyra/platform/services/auth-service/internal/model"
)

// UserRepository handles user database operations.
type UserRepository struct {
	db userDB
}

type userDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewUserRepository creates a new UserRepository.
func NewUserRepository(db userDB) *UserRepository {
	return NewUserRepositoryWithDB(db)
}

func NewUserRepositoryWithDB(db userDB) *UserRepository {
	return &UserRepository{db: db}
}

// UpsertByPhone creates a user if not exists, otherwise updates last_login_at.
// Returns the user and whether they are newly created.
func (r *UserRepository) UpsertByPhone(ctx context.Context, phone string) (*model.User, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO users (phone)
		VALUES ($1)
		ON CONFLICT (phone)
		DO UPDATE SET last_login_at = NOW(), updated_at = NOW()
		RETURNING id, phone, email, full_name, is_active, is_verified,
		          app_version, device_token, referral_code, last_login_at,
		          created_at, updated_at`

	var user model.User
	err := r.db.QueryRow(ctx, query, phone).Scan(
		&user.ID, &user.Phone, &user.Email, &user.FullName,
		&user.IsActive, &user.IsVerified, &user.AppVersion,
		&user.DeviceToken, &user.ReferralCode, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, false, err
	}

	// is_new_user: created_at == updated_at within 1 second
	isNew := user.UpdatedAt.Sub(user.CreatedAt) < time.Second

	return &user, isNew, nil
}

// GetByID fetches a user by their UUID.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, phone, email, full_name, is_active, is_verified,
		       app_version, device_token, referral_code, last_login_at,
		       created_at, updated_at
		FROM users WHERE id = $1`

	var user model.User
	err := r.db.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Phone, &user.Email, &user.FullName,
		&user.IsActive, &user.IsVerified, &user.AppVersion,
		&user.DeviceToken, &user.ReferralCode, &user.LastLoginAt,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}
