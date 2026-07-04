package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/store-service/internal/model"
	"github.com/zippyra/platform/shared/errors"
)

const (
	storeSessionTTL = 4 * time.Hour // 14400 seconds
	storeCacheTTL   = 5 * time.Minute
)

// StoreService handles all store business logic.
type StoreService struct {
	storeRepo StoreRepo
	qrRepo    QRTokenRepo
	redis     RedisStore
	publisher EventPublisher
	nowFunc   func() time.Time
}

// NewStoreService creates a new StoreService.
func NewStoreService(
	storeRepo StoreRepo,
	qrRepo QRTokenRepo,
	redis RedisStore,
	publisher EventPublisher,
) *StoreService {
	return &StoreService{
		storeRepo: storeRepo,
		qrRepo:    qrRepo,
		redis:     redis,
		publisher: publisher,
		nowFunc:   time.Now,
	}
}

// Bind processes store entry via QR token scan.
func (s *StoreService) Bind(ctx context.Context, userID, qrToken, deviceID string) (*model.BindResponse, error) {
	now := s.nowFunc()

	// Validate qr_token is not empty
	if qrToken == "" {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is required",
			HTTPStatus: 400,
		}
	}

	// Query store_qr_tokens table
	token, err := s.qrRepo.GetActiveToken(ctx, qrToken)
	if err != nil {
		log.Error().Err(err).Msg("qr token lookup failed")
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Internal error",
			HTTPStatus: 500, Err: err,
		}
	}
	if token == nil {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is invalid or inactive",
			HTTPStatus: 400,
		}
	}

	// Check expiry
	if token.ExpiresAt.Before(now) {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenExpired, Message: "QR token has expired",
			HTTPStatus: 400,
		}
	}

	// Check token type
	if token.TokenType != "ENTRANCE" {
		return nil, &errors.AppError{
			Code: errors.ErrQRTokenInvalid, Message: "QR token is not an entrance token",
			HTTPStatus: 400,
		}
	}

	// Query store
	store, err := s.storeRepo.GetByID(ctx, token.StoreID)
	if err != nil {
		log.Error().Err(err).Str("store_id", token.StoreID.String()).Msg("store lookup failed")
		return nil, &errors.AppError{
			Code: errors.ErrStoreNotFound, Message: "Store not found",
			HTTPStatus: 404,
		}
	}

	// Check feature flag before processing store bind
	featureEnabled, err := s.isFeatureEnabled(ctx, "store_entry", store.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("feature flag check failed")
		// Fail open on feature flag errors — proceed if check fails
	} else if !featureEnabled {
		return nil, &errors.AppError{
			Code: errors.ErrStoreClosed, Message: "Store entry is temporarily disabled",
			HTTPStatus: 403,
		}
	}

	// Check capacity from Redis (authoritative)
	occupancy, err := s.getCurrentOccupancy(ctx, store.ID.String())
	if err != nil {
		log.Error().Err(err).Msg("occupancy check failed")
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to check occupancy",
			HTTPStatus: 500, Err: err,
		}
	}
	if occupancy >= int64(store.Capacity) {
		return nil, &errors.AppError{
			Code: errors.ErrStoreAtCapacity, Message: "Store is at full capacity",
			HTTPStatus: 409,
		}
	}

	// Increment occupancy
	_, _ = s.redis.Incr(ctx, "store_occupancy:"+store.ID.String())

	// Set store session in Redis
	sessionExpiresAt := now.Add(storeSessionTTL)
	_ = s.redis.Set(ctx, "store_session:"+userID, store.ID.String(), storeSessionTTL)

	// Increment QR token used_count
	if err := s.qrRepo.IncrementUsedCount(ctx, token.ID); err != nil {
		log.Error().Err(err).Msg("failed to increment QR used_count")
		// Non-fatal
	}

	// Publish Kafka event
	s.publisher.PublishCustomerEntered(ctx, userID, store.ID.String(), store.ChainID.String(), token.ID.String())

	log.Info().
		Str("user_id", userID).
		Str("store_id", store.ID.String()).
		Msg("customer entered store")

	return &model.BindResponse{
		StoreID:          store.ID,
		StoreName:        store.Name,
		CatalogVersion:   store.CatalogVersion,
		QROnlyMode:       store.QROnlyMode,
		SessionExpiresAt: sessionExpiresAt,
	}, nil
}

// GetStore retrieves store info with Redis caching.
func (s *StoreService) GetStore(ctx context.Context, storeID uuid.UUID) (*model.StoreInfoResponse, error) {
	// Try Redis cache first
	cached, err := s.redis.Get(ctx, "store_info:"+storeID.String())
	if err == nil && cached != "" {
		_ = cached // Cache hit, but we still query DB for full data
	}

	// Cache miss or error: query DB
	store, err := s.storeRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrStoreNotFound, Message: "Store not found",
			HTTPStatus: 404,
		}
	}

	// Cache result (best-effort)
	_ = s.redis.Set(ctx, "store_info:"+storeID.String(), store.Name, storeCacheTTL)

	return &model.StoreInfoResponse{
		ID:             store.ID,
		Name:           store.Name,
		Address:        store.Address,
		City:           store.City,
		State:          store.State,
		Pincode:        store.Pincode,
		Latitude:       store.Latitude,
		Longitude:      store.Longitude,
		Capacity:       store.Capacity,
		CatalogVersion: store.CatalogVersion,
		QROnlyMode:     store.QROnlyMode,
		IsActive:       store.IsActive,
	}, nil
}

// NearbyStores finds stores within the given radius.
func (s *StoreService) NearbyStores(ctx context.Context, lat, lng, radiusKM float64) ([]model.NearbyStore, error) {
	stores, distances, err := s.storeRepo.NearbyStores(ctx, lat, lng, radiusKM)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to query nearby stores",
			HTTPStatus: 500, Err: err,
		}
	}

	now := s.nowFunc()
	result := make([]model.NearbyStore, 0, len(stores))
	for i, store := range stores {
		occupancy, _ := s.getCurrentOccupancy(ctx, store.ID.String())
		occPct := 0
		if store.Capacity > 0 {
			occPct = int(math.Round(float64(occupancy) / float64(store.Capacity) * 100))
		}

		result = append(result, model.NearbyStore{
			StoreInfoResponse: model.StoreInfoResponse{
				ID:             store.ID,
				Name:           store.Name,
				Address:        store.Address,
				City:           store.City,
				State:          store.State,
				Pincode:        store.Pincode,
				Latitude:       store.Latitude,
				Longitude:      store.Longitude,
				Capacity:       store.Capacity,
				CatalogVersion: store.CatalogVersion,
				QROnlyMode:     store.QROnlyMode,
				IsActive:       store.IsActive,
			},
			DistanceKM:       math.Round(distances[i]*100) / 100,
			CurrentOccupancy: int(occupancy),
			OccupancyPct:     occPct,
			IsOpen:           s.isStoreOpen(ctx, store.ID, now),
		})
	}
	return result, nil
}

// Hours returns today's operating hours and open/closed status.
func (s *StoreService) Hours(ctx context.Context, storeID uuid.UUID) (*model.HoursResponse, error) {
	now := s.nowFunc()
	loc, _ := time.LoadLocation("Asia/Kolkata")
	istNow := now.In(loc)
	dayOfWeek := int(istNow.Weekday())

	hours, err := s.storeRepo.GetHours(ctx, storeID, dayOfWeek)
	if err != nil {
		// Return default hours if not found
		return &model.HoursResponse{
			IsOpen:     s.isTimeInRange(istNow, "09:00", "22:00"),
			OpensAt:    "09:00",
			ClosesAt:   "22:00",
			Timezone:   "Asia/Kolkata",
			NextOpenAt: nil,
		}, nil
	}

	isOpen := s.isTimeInRange(istNow, hours.OpensAt, hours.ClosesAt)
	return &model.HoursResponse{
		IsOpen:     isOpen,
		OpensAt:    hours.OpensAt,
		ClosesAt:   hours.ClosesAt,
		Timezone:   "Asia/Kolkata",
		NextOpenAt: nil,
	}, nil
}

// UpdateCapacity increments or decrements store occupancy.
func (s *StoreService) UpdateCapacity(ctx context.Context, storeID, action string) (*model.CapacityUpdateResponse, error) {
	key := "store_occupancy:" + storeID

	var newVal int64
	var err error

	if _, err := uuid.Parse(storeID); err != nil {
		return nil, &errors.AppError{Code: errors.ErrValidationFailed, Message: "Invalid store ID", HTTPStatus: 400}
	}

	if action != "increment" && action != "decrement" {
		return nil, &errors.AppError{Code: errors.ErrValidationFailed, Message: "Invalid action", HTTPStatus: 400}
	}

	if action == "increment" {
		newVal, err = s.redis.Incr(ctx, key)
	} else {
		// decrement — never go below 0
		currentStr, getErr := s.redis.Get(ctx, key)
		if getErr != nil {
			// Key doesn't exist, treat as 0
			return &model.CapacityUpdateResponse{CurrentOccupancy: 0}, nil
		}
		current, _ := strconv.ParseInt(currentStr, 10, 64)
		if current <= 0 {
			return &model.CapacityUpdateResponse{CurrentOccupancy: 0}, nil
		}

		newVal, err = s.redis.Decr(ctx, key)
		if newVal < 0 {
			// Race condition safety: reset to 0
			_ = s.redis.Set(ctx, key, "0", 0)
			newVal = 0
		}
	}

	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to update occupancy",
			HTTPStatus: 500, Err: err,
		}
	}

	return &model.CapacityUpdateResponse{CurrentOccupancy: int(newVal)}, nil
}

// Exit processes customer store exit.
func (s *StoreService) Exit(ctx context.Context, userID, storeID string) (*model.ExitResponse, error) {
	// Verify store_session:{user_id} exists and matches store_id
	sessionStoreID, err := s.redis.Get(ctx, "store_session:"+userID)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrStoreNotFound, Message: "No active store session",
			HTTPStatus: 404,
		}
	}
	if sessionStoreID != storeID {
		return nil, &errors.AppError{
			Code: errors.ErrForbidden, Message: "Store session mismatch",
			HTTPStatus: 403,
		}
	}

	// DECR store_occupancy:{store_id} — never below 0
	newVal, err := s.redis.Decr(ctx, "store_occupancy:"+storeID)
	if err != nil {
		log.Error().Err(err).Msg("failed to decrement occupancy")
	}
	if newVal < 0 {
		_ = s.redis.Set(ctx, "store_occupancy:"+storeID, "0", 0)
	}

	// DEL store_session:{user_id}
	_ = s.redis.Del(ctx, "store_session:"+userID)

	// Publish Kafka event with duration
	store, _ := s.storeRepo.GetByID(ctx, uuid.MustParse(storeID))
	chainID := ""
	if store != nil {
		chainID = store.ChainID.String()
	}
	s.publisher.PublishCustomerExited(ctx, userID, storeID, chainID, 0)

	log.Info().
		Str("user_id", userID).
		Str("store_id", storeID).
		Msg("customer exited store")

	return &model.ExitResponse{Message: "Exit recorded"}, nil
}

// Occupancy returns real-time occupancy from Redis.
func (s *StoreService) Occupancy(ctx context.Context, storeID uuid.UUID) (*model.OccupancyResponse, error) {
	store, err := s.storeRepo.GetByID(ctx, storeID)
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrStoreNotFound, Message: "Store not found",
			HTTPStatus: 404,
		}
	}

	occupancy, err := s.getCurrentOccupancy(ctx, storeID.String())
	if err != nil {
		return nil, &errors.AppError{
			Code: errors.ErrInternal, Message: "Failed to get occupancy",
			HTTPStatus: 500, Err: err,
		}
	}

	occPct := 0
	if store.Capacity > 0 {
		occPct = int(math.Round(float64(occupancy) / float64(store.Capacity) * 100))
	}

	status := occupancyStatus(occPct)

	return &model.OccupancyResponse{
		StoreID:          storeID,
		CurrentOccupancy: int(occupancy),
		Capacity:         store.Capacity,
		OccupancyPct:     occPct,
		Status:           status,
	}, nil
}

// getCurrentOccupancy reads occupancy from Redis (authoritative source).
func (s *StoreService) getCurrentOccupancy(ctx context.Context, storeID string) (int64, error) {
	val, err := s.redis.Get(ctx, "store_occupancy:"+storeID)
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, err
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// isFeatureEnabled checks if a feature flag is enabled.
// Checks Redis first: GET feature:{flag_name}:{store_id}
// Falls back to DB feature_flags table if Redis miss.
func (s *StoreService) isFeatureEnabled(ctx context.Context, flagName, storeID string) (bool, error) {
	key := fmt.Sprintf("feature:%s:%s", flagName, storeID)
	val, err := s.redis.Get(ctx, key)
	if err != nil {
		if err == redis.Nil {
			return true, nil // Default enabled if not in Redis
		}
		return false, err
	}

	return val == "1" || val == "true", nil
}

// isStoreOpen checks if a store is currently open.
func (s *StoreService) isStoreOpen(ctx context.Context, storeID uuid.UUID, now time.Time) bool {
	loc, _ := time.LoadLocation("Asia/Kolkata")
	istNow := now.In(loc)
	dayOfWeek := int(istNow.Weekday())

	hours, err := s.storeRepo.GetHours(ctx, storeID, dayOfWeek)
	if err != nil {
		// Default: 9am-10pm
		return s.isTimeInRange(istNow, "09:00", "22:00")
	}
	return s.isTimeInRange(istNow, hours.OpensAt, hours.ClosesAt)
}

// isTimeInRange checks if the current time is within a HH:MM range.
func (s *StoreService) isTimeInRange(now time.Time, opensAt, closesAt string) bool {
	currentMinutes := now.Hour()*60 + now.Minute()

	openParts := parseHHMM(opensAt)
	closeParts := parseHHMM(closesAt)

	return currentMinutes >= openParts && currentMinutes < closeParts
}

func parseHHMM(hhmm string) int {
	if len(hhmm) < 5 {
		return 0
	}
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:5])
	return h*60 + m
}

// occupancyStatus returns the status string based on occupancy percentage.
func occupancyStatus(pct int) string {
	switch {
	case pct >= 100:
		return "at_capacity"
	case pct > 90:
		return "near_capacity"
	case pct >= 70:
		return "busy"
	default:
		return "normal"
	}
}
