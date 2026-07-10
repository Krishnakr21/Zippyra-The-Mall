package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	jwt5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/exit-validation-service/internal/kafka"
	"github.com/zippyra/platform/services/exit-validation-service/internal/model"
	sharederrors "github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/jwt"
)

var (
	ErrExitTokenExpired = &sharederrors.AppError{Code: sharederrors.ErrExitTokenExpired, Message: "exit token expired", HTTPStatus: http.StatusUnauthorized}
	ErrExitWrongStore   = &sharederrors.AppError{Code: sharederrors.ErrExitWrongStore, Message: "exit token wrong store", HTTPStatus: http.StatusBadRequest}
	ErrExitTokenUsed    = &sharederrors.AppError{Code: sharederrors.ErrExitTokenUsed, Message: "exit token already used", HTTPStatus: http.StatusConflict}
	ErrForbidden        = &sharederrors.AppError{Code: sharederrors.ErrForbidden, Message: "forbidden", HTTPStatus: http.StatusForbidden}
)

type ExitTokenRepository interface {
	UpdateTokenUsed(ctx context.Context, tokenHash string) (uuid.UUID, error)
	StoreHasRFID(ctx context.Context, storeIDStr string) (bool, error)
	GetTokenStatusByOrderID(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error)
}

type ExitService struct {
	repo            ExitTokenRepository
	redis           redis.Cmdable
	kafkaProducer   *kafka.Producer
	gate            *GateCommander
	pubKey          ed25519.PublicKey
	storeServiceURL string
	httpClient      *http.Client
}

func NewExitService(
	repo ExitTokenRepository,
	rdb redis.Cmdable,
	kafkaProducer *kafka.Producer,
	gate *GateCommander,
	pubKey ed25519.PublicKey,
	storeServiceURL string,
) *ExitService {
	return &ExitService{
		repo:            repo,
		redis:           rdb,
		kafkaProducer:   kafkaProducer,
		gate:            gate,
		pubKey:          pubKey,
		storeServiceURL: storeServiceURL,
		httpClient:      &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *ExitService) Validate(ctx context.Context, tokenString string, requestStoreID string, requestGateID string, authenticatedUserID string) (*model.ExitValidationResponse, error) {
	// 1. Parse and verify exit token signature & expiry
	claims, err := s.parseAndVerifyExitToken(tokenString)
	if err != nil {
		return nil, err
	}

	tokenHash := sha256Hex(tokenString)
	userID := claims.UserID
	orderID := claims.OrderID
	storeID := claims.StoreID

	// 2. Verify store_id in token matches store_id in request
	if storeID != requestStoreID {
		// Wrong store = alarm — security event published
		_ = s.kafkaProducer.PublishAlarm(ctx, "WRONG_STORE_ATTEMPT", userID, requestStoreID, requestGateID)
		return nil, ErrExitWrongStore
	}

	// 3. Verify user_id in token matches authenticated user
	if userID != authenticatedUserID {
		return nil, ErrForbidden
	}

	// 4. One-time use check (atomic): Redis SET NX — 100ms timeout
	key := fmt.Sprintf("exit_used:%s", tokenHash)
	redisCtx, cancelRedis := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelRedis()

	set, redisErr := s.redis.SetNX(redisCtx, key, 1, 1*time.Hour).Result()
	if redisErr != nil {
		log.Warn().Err(redisErr).Msg("Redis SETNX check failed, relying on DB fallback")
	} else if !set {
		// Already used — trigger alarm
		_ = s.gate.SendCommand(ctx, requestStoreID, requestGateID, "DENY", userID, orderID)
		_ = s.kafkaProducer.PublishAlarm(ctx, "QR_REPLAY_ATTEMPT", userID, requestStoreID, requestGateID)
		return nil, ErrExitTokenUsed
	}

	// 5. Check exit_token in DB
	dbCtx, cancelDB := context.WithTimeout(ctx, 5*time.Second)
	defer cancelDB()

	_, err = s.repo.UpdateTokenUsed(dbCtx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already used in DB or does not exist
			_ = s.gate.SendCommand(ctx, requestStoreID, requestGateID, "DENY", userID, orderID)
			_ = s.kafkaProducer.PublishAlarm(ctx, "QR_REPLAY_ATTEMPT", userID, requestStoreID, requestGateID)
			return nil, ErrExitTokenUsed
		}
		return nil, &sharederrors.AppError{
			Code:       sharederrors.ErrInternal,
			Message:    fmt.Sprintf("database update failed: %v", err),
			HTTPStatus: http.StatusInternalServerError,
			Err:        err,
		}
	}

	// 6. Check RFID presence
	hasRFID, rfidErr := s.repo.StoreHasRFID(dbCtx, requestStoreID)
	if rfidErr != nil {
		log.Warn().Err(rfidErr).Msg("failed to check RFID registration")
	}
	if hasRFID {
		// Verify RFID tags match cart items
		// For pilot: skip RFID check, log warning
		log.Warn().Str("store_id", requestStoreID).Msg("RFID_PAD registered, but RFID check is skipped for pilot")
	}

	// 7. Send MQTT gate command (OPEN, fire-and-forget, non-blocking)
	_ = s.gate.SendCommand(ctx, requestStoreID, requestGateID, "OPEN", userID, orderID)

	// 8. Decrement store occupancy via store-service (async)
	go s.decrementStoreOccupancy(context.Background(), requestStoreID)

	// 9. Publish Kafka event: store.customer_exited
	correlationID := middleware.GetReqID(ctx)
	_ = s.kafkaProducer.PublishCustomerExited(ctx, userID, requestStoreID, orderID, requestGateID, correlationID)

	return &model.ExitValidationResponse{
		Status:      "APPROVED",
		GateCommand: "OPEN",
		OrderID:     orderID,
		Message:     "Exit approved. Gate opening.",
	}, nil
}

func (s *ExitService) StaffOverride(ctx context.Context, staffID string, req *model.StaffOverrideRequest) (*model.ExitValidationResponse, error) {
	// Open gate
	_ = s.gate.SendCommand(ctx, req.StoreID, req.GateID, "OPEN", req.UserID, "")

	// Publish audit event
	_ = s.kafkaProducer.PublishStaffOverride(ctx, staffID, req.UserID, req.StoreID, req.GateID, req.Reason)

	return &model.ExitValidationResponse{
		Status:      "APPROVED",
		GateCommand: "OPEN",
		OrderID:     "",
		Message:     "Staff override successful. Gate opening.",
	}, nil
}

func (s *ExitService) GetTokenStatus(ctx context.Context, orderID string) (*model.ExitTokenStatusResponse, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	retOrderID, isUsed, expiresAt, err := s.repo.GetTokenStatusByOrderID(dbCtx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, &sharederrors.AppError{
				Code:       sharederrors.ErrOrderNotFound,
				Message:    "exit token not found for this order",
				HTTPStatus: http.StatusNotFound,
			}
		}
		return nil, &sharederrors.AppError{
			Code:       sharederrors.ErrInternal,
			Message:    err.Error(),
			HTTPStatus: http.StatusInternalServerError,
			Err:        err,
		}
	}

	isExpired := expiresAt.Before(time.Now())

	return &model.ExitTokenStatusResponse{
		OrderID:   retOrderID.String(),
		IsUsed:    isUsed,
		ExpiresAt: expiresAt,
		IsExpired: isExpired,
	}, nil
}

func (s *ExitService) parseAndVerifyExitToken(tokenString string) (*jwt.ExitTokenClaims, error) {
	token, err := jwt5.ParseWithClaims(tokenString, &jwt.ExitTokenClaims{}, func(t *jwt5.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt5.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.pubKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt5.ErrTokenExpired) {
			return nil, ErrExitTokenExpired
		}
		return nil, &sharederrors.AppError{
			Code:       sharederrors.ErrExitTokenInvalid,
			Message:    fmt.Sprintf("failed to parse token: %v", err),
			HTTPStatus: http.StatusUnauthorized,
			Err:        err,
		}
	}

	claims, ok := token.Claims.(*jwt.ExitTokenClaims)
	if !ok || !token.Valid {
		return nil, &sharederrors.AppError{
			Code:       sharederrors.ErrExitTokenInvalid,
			Message:    "invalid token claims",
			HTTPStatus: http.StatusUnauthorized,
		}
	}

	// Double check expiration manually
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, ErrExitTokenExpired
	}

	return claims, nil
}

func (s *ExitService) decrementStoreOccupancy(ctx context.Context, storeID string) {
	if s.storeServiceURL == "" {
		log.Warn().Msg("Store service URL not configured, skipping occupancy decrement")
		return
	}

	url := fmt.Sprintf("%s/v1/store/%s/capacity", s.storeServiceURL, storeID)
	bodyBytes, _ := json.Marshal(map[string]string{
		"action": "decrement",
	})

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(bodyBytes))
	if err != nil {
		log.Error().Err(err).Msg("Failed to create store occupancy decrement request")
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Str("url", url).Msg("Store occupancy decrement request failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status_code", resp.StatusCode).Str("url", url).Msg("Store occupancy decrement request returned non-OK status")
	} else {
		log.Info().Str("store_id", storeID).Msg("Successfully decremented store occupancy")
	}
}

func sha256Hex(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
