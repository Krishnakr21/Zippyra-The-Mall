package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/zippyra/platform/services/auth-service/internal/service"
)

// SessionStoreDB implements service.SessionStore using pgxpool.
type SessionStoreDB struct {
	db sessionDB
}

type sessionDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewSessionStoreDB creates a new SessionStoreDB.
func NewSessionStoreDB(db sessionDB) *SessionStoreDB {
	return NewSessionStoreDBWithDB(db)
}

func NewSessionStoreDBWithDB(db sessionDB) *SessionStoreDB {
	return &SessionStoreDB{db: db}
}

func (s *SessionStoreDB) CreateSession(ctx context.Context, userID uuid.UUID, deviceID, deviceModel, ip, userAgent string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO auth_sessions (user_id, device_id, device_model, ip_address, user_agent, last_active_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (user_id, device_id)
		DO UPDATE SET last_active_at = NOW(), ip_address = $4, user_agent = $5`

	_, err := s.db.Exec(ctx, query, userID, deviceID, deviceModel, ip, userAgent)
	return err
}

func (s *SessionStoreDB) UpdateSessionActivity(ctx context.Context, userID, deviceID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	uid, _ := uuid.Parse(userID)
	query := `UPDATE auth_sessions SET last_active_at = NOW() WHERE user_id = $1 AND device_id = $2`
	_, err := s.db.Exec(ctx, query, uid, deviceID)
	return err
}

func (s *SessionStoreDB) HasActiveSession(ctx context.Context, userID, deviceID string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	uid, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user_id: %w", err)
	}

	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE user_id = $1 AND device_id = $2 AND logged_out_at IS NULL)`
	if err := s.db.QueryRow(ctx, query, uid, deviceID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SessionStoreDB) LogoutSession(ctx context.Context, userID, deviceID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	uid, _ := uuid.Parse(userID)
	query := `UPDATE auth_sessions SET logged_out_at = NOW() WHERE user_id = $1 AND device_id = $2 AND logged_out_at IS NULL`
	_, err := s.db.Exec(ctx, query, uid, deviceID)
	return err
}

func (s *SessionStoreDB) ListSessions(ctx context.Context, userID uuid.UUID) ([]service.SessionRow, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, device_id, device_model, last_active_at, ip_address
		FROM auth_sessions
		WHERE user_id = $1 AND logged_out_at IS NULL
		ORDER BY last_active_at DESC`

	rows, err := s.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []service.SessionRow
	for rows.Next() {
		var row service.SessionRow
		var lastActive time.Time
		if err := rows.Scan(&row.ID, &row.DeviceID, &row.DeviceModel, &lastActive, &row.IPAddress); err != nil {
			return nil, err
		}
		row.LastActiveAt = lastActive
		result = append(result, row)
	}

	return result, nil
}

func (s *SessionStoreDB) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var deviceID string
	query := `SELECT device_id FROM auth_sessions WHERE id = $1 AND user_id = $2 AND logged_out_at IS NULL`
	err := s.db.QueryRow(ctx, query, sessionID, userID).Scan(&deviceID)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("session not found")
	}
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `UPDATE auth_sessions SET logged_out_at = NOW() WHERE id = $1`, sessionID)
	return err
}

func (s *SessionStoreDB) RevokeAllSessions(ctx context.Context, userID uuid.UUID, currentDeviceID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := s.db.Exec(ctx,
		`UPDATE auth_sessions SET logged_out_at = NOW()
		 WHERE user_id = $1 AND device_id != $2 AND logged_out_at IS NULL`,
		userID, currentDeviceID)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}
