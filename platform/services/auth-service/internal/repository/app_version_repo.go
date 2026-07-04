package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/zippyra/platform/services/auth-service/internal/model"
)

// AppVersionRepository handles app_versions table operations.
type AppVersionRepository struct {
	db appVersionDB
}

type appVersionDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// NewAppVersionRepository creates a new AppVersionRepository.
func NewAppVersionRepository(db appVersionDB) *AppVersionRepository {
	return &AppVersionRepository{db: db}
}

// GetLatest returns the latest app version for the given platform.
func (r *AppVersionRepository) GetLatest(ctx context.Context, platform string) (*model.AppVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `
		SELECT id, platform, version, min_supported_version,
		       is_force_update, release_notes, created_at, updated_at
		FROM app_versions
		WHERE platform = $1
		ORDER BY created_at DESC
		LIMIT 1`

	var av model.AppVersion
	err := r.db.QueryRow(ctx, query, platform).Scan(
		&av.ID, &av.Platform, &av.Version, &av.MinSupportedVersion,
		&av.IsForceUpdate, &av.ReleaseNotes, &av.CreatedAt, &av.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &av, nil
}
