package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// LoginAttemptRepository handles login_attempts table operations.
type LoginAttemptRepository struct {
	db loginAttemptDB
}

type loginAttemptDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// NewLoginAttemptRepository creates a new LoginAttemptRepository.
func NewLoginAttemptRepository(db loginAttemptDB) *LoginAttemptRepository {
	return NewLoginAttemptRepositoryWithDB(db)
}

func NewLoginAttemptRepositoryWithDB(db loginAttemptDB) *LoginAttemptRepository {
	return &LoginAttemptRepository{db: db}
}

// Insert records a new login attempt.
func (r *LoginAttemptRepository) Insert(ctx context.Context, phone, ipAddress, userAgent, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `INSERT INTO login_attempts (phone, ip_address, user_agent, status) VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, query, phone, ipAddress, userAgent, status)
	return err
}

// UpdateStatus updates the status of the latest login attempt for a phone number.
func (r *LoginAttemptRepository) UpdateStatus(ctx context.Context, phone, status string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		UPDATE login_attempts SET status = $2
		WHERE id = (
			SELECT id FROM login_attempts
			WHERE phone = $1 ORDER BY created_at DESC LIMIT 1
		)`
	_, err := r.db.Exec(ctx, query, phone, status)
	return err
}
