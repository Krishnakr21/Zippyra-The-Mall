package service

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	jwt5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"github.com/zippyra/platform/services/exit-validation-service/internal/kafka"
	"github.com/zippyra/platform/services/exit-validation-service/internal/model"
	sharederrors "github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/jwt"
)

// -- Mocks --

type mockRepository struct {
	updateTokenUsedFn         func(ctx context.Context, tokenHash string) (uuid.UUID, error)
	storeHasRFIDFn            func(ctx context.Context, storeIDStr string) (bool, error)
	getTokenStatusByOrderIDFn func(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error)
}

func (m *mockRepository) UpdateTokenUsed(ctx context.Context, tokenHash string) (uuid.UUID, error) {
	if m.updateTokenUsedFn != nil {
		return m.updateTokenUsedFn(ctx, tokenHash)
	}
	return uuid.New(), nil
}

func (m *mockRepository) StoreHasRFID(ctx context.Context, storeIDStr string) (bool, error) {
	if m.storeHasRFIDFn != nil {
		return m.storeHasRFIDFn(ctx, storeIDStr)
	}
	return false, nil
}

func (m *mockRepository) GetTokenStatusByOrderID(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error) {
	if m.getTokenStatusByOrderIDFn != nil {
		return m.getTokenStatusByOrderIDFn(ctx, orderIDStr)
	}
	return uuid.New(), false, time.Now(), nil
}

type mockRedis struct {
	redis.Cmdable
	setNXFn func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd
}

func (m *mockRedis) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
	if m.setNXFn != nil {
		return m.setNXFn(ctx, key, value, expiration)
	}
	cmd := redis.NewBoolCmd(ctx)
	cmd.SetVal(true)
	return cmd
}

type mockToken struct {
	mqtt.Token
	err error
}

func (m *mockToken) Wait() bool { return true }
func (m *mockToken) Error() error { return m.err }

type mockMQTTClient struct {
	mqtt.Client
	publishFn func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
}

func (m *mockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	if m.publishFn != nil {
		return m.publishFn(topic, qos, retained, payload)
	}
	return &mockToken{}
}

type mockRoundTripper struct {
	roundTripFn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFn(req)
}

// -- Test Helpers --

func generateTestExitToken(orderID, userID, storeID string, expiresAt time.Time, priv ed25519.PrivateKey) (string, error) {
	claims := jwt.ExitTokenClaims{
		OrderID: orderID,
		UserID:  userID,
		StoreID: storeID,
		RegisteredClaims: jwt5.RegisteredClaims{
			ExpiresAt: jwt5.NewNumericDate(expiresAt),
			IssuedAt:  jwt5.NewNumericDate(time.Now()),
			ID:        "test-jti-" + uuid.New().String(),
		},
	}
	token := jwt5.NewWithClaims(jwt5.SigningMethodEdDSA, claims)
	return token.SignedString(priv)
}

func TestExitService_Validate(t *testing.T) {
	// Generate keys for tests
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate keys: %v", err)
	}

	storeID := uuid.New().String()
	userID := uuid.New().String()
	orderID := uuid.New().String()
	gateID := "gate-001"

	t.Run("Valid exit token -> gate OPEN command sent, event published", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{
			storeHasRFIDFn: func(ctx context.Context, storeIDStr string) (bool, error) {
				return false, nil
			},
		}
		rdb := &mockRedis{}

		var mu sync.Mutex
		var kafkaEvents []string
		producer := kafka.NewProducer([]string{"localhost:9092"})
		producer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
			mu.Lock()
			kafkaEvents = append(kafkaEvents, topic)
			mu.Unlock()
			return nil
		}

		var mqttPubs []string
		mqttClient := &mockMQTTClient{
			publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
				mu.Lock()
				mqttPubs = append(mqttPubs, topic)
				mu.Unlock()
				return &mockToken{}
			},
		}
		gateCommander := NewGateCommander(mqttClient, 1)

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		res, err := svc.Validate(context.Background(), tokenStr, storeID, gateID, userID)
		if err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}

		if res.Status != "APPROVED" || res.GateCommand != "OPEN" {
			t.Errorf("expected APPROVED and OPEN, got status: %s, cmd: %s", res.Status, res.GateCommand)
		}

		// Wait slightly to ensure fire-and-forget MQTT routines executed
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(mqttPubs) != 1 || mqttPubs[0] != fmt.Sprintf("zippyra/store/%s/gate/%s/command", storeID, gateID) {
			t.Errorf("expected 1 MQTT publication to gate topic, got: %v", mqttPubs)
		}

		if len(kafkaEvents) != 1 || kafkaEvents[0] != "store.customer_exited" {
			t.Errorf("expected customer exited event published, got: %v", kafkaEvents)
		}
	})

	t.Run("Expired token -> ErrExitTokenExpired", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(-10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{}
		rdb := &mockRedis{}
		producer := kafka.NewProducer(nil)
		gateCommander := NewMockGateCommander()

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		_, err = svc.Validate(context.Background(), tokenStr, storeID, gateID, userID)
		if err == nil {
			t.Fatal("expected expired token error, got nil")
		}

		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrExitTokenExpired {
				t.Errorf("expected error code ErrExitTokenExpired, got: %s", appErr.Code)
			}
		} else {
			t.Errorf("expected AppError, got: %v", err)
		}
	})

	t.Run("Wrong store -> ErrExitWrongStore, alarm published", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{}
		rdb := &mockRedis{}

		var mu sync.Mutex
		var kafkaEvents []string
		producer := kafka.NewProducer(nil)
		producer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
			mu.Lock()
			kafkaEvents = append(kafkaEvents, topic)
			mu.Unlock()
			return nil
		}
		gateCommander := NewMockGateCommander()

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		wrongStoreID := uuid.New().String()
		_, err = svc.Validate(context.Background(), tokenStr, wrongStoreID, gateID, userID)
		if err == nil {
			t.Fatal("expected wrong store error, got nil")
		}

		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrExitWrongStore {
				t.Errorf("expected code ErrExitWrongStore, got: %s", appErr.Code)
			}
		} else {
			t.Errorf("expected AppError, got: %v", err)
		}

		mu.Lock()
		defer mu.Unlock()
		if len(kafkaEvents) != 1 || kafkaEvents[0] != "exit.alarm" {
			t.Errorf("expected wrong store alarm to be published, got: %v", kafkaEvents)
		}
	})

	t.Run("Already used token (Redis SETNX fail) -> ErrExitTokenUsed, gate DENY sent, alarm published", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{}
		rdb := &mockRedis{
			setNXFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
				cmd := redis.NewBoolCmd(ctx)
				cmd.SetVal(false) // already exists
				return cmd
			},
		}

		var mu sync.Mutex
		var kafkaEvents []string
		producer := kafka.NewProducer(nil)
		producer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
			mu.Lock()
			kafkaEvents = append(kafkaEvents, topic)
			mu.Unlock()
			return nil
		}

		var mqttPubs []string
		var lastPayload string
		mqttClient := &mockMQTTClient{
			publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
				mu.Lock()
				mqttPubs = append(mqttPubs, topic)
				if b, ok := payload.([]byte); ok {
					lastPayload = string(b)
				}
				mu.Unlock()
				return &mockToken{}
			},
		}
		gateCommander := NewGateCommander(mqttClient, 1)

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		_, err = svc.Validate(context.Background(), tokenStr, storeID, gateID, userID)
		if err == nil {
			t.Fatal("expected token used error, got nil")
		}

		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrExitTokenUsed {
				t.Errorf("expected code ErrExitTokenUsed, got: %s", appErr.Code)
			}
		}

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(mqttPubs) != 1 {
			t.Fatalf("expected 1 MQTT publication, got: %v", mqttPubs)
		}
		if !bytesContains(lastPayload, "DENY") {
			t.Errorf("expected DENY gate command, got: %s", lastPayload)
		}

		if len(kafkaEvents) != 1 || kafkaEvents[0] != "exit.alarm" {
			t.Errorf("expected alarm to be published, got: %v", kafkaEvents)
		}
	})

	t.Run("Already used token (DB update fails) -> ErrExitTokenUsed, gate DENY sent, alarm published", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{
			updateTokenUsedFn: func(ctx context.Context, tokenHash string) (uuid.UUID, error) {
				return uuid.Nil, pgx.ErrNoRows // no rows updated
			},
		}
		rdb := &mockRedis{
			setNXFn: func(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.BoolCmd {
				cmd := redis.NewBoolCmd(ctx)
				cmd.SetVal(true) // Redis setnx succeeds
				return cmd
			},
		}

		var mu sync.Mutex
		var kafkaEvents []string
		producer := kafka.NewProducer(nil)
		producer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
			mu.Lock()
			kafkaEvents = append(kafkaEvents, topic)
			mu.Unlock()
			return nil
		}

		var mqttPubs []string
		var lastPayload string
		mqttClient := &mockMQTTClient{
			publishFn: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
				mu.Lock()
				mqttPubs = append(mqttPubs, topic)
				if b, ok := payload.([]byte); ok {
					lastPayload = string(b)
				}
				mu.Unlock()
				return &mockToken{}
			},
		}
		gateCommander := NewGateCommander(mqttClient, 1)

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		_, err = svc.Validate(context.Background(), tokenStr, storeID, gateID, userID)
		if err == nil {
			t.Fatal("expected token used error, got nil")
		}

		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrExitTokenUsed {
				t.Errorf("expected code ErrExitTokenUsed, got: %s", appErr.Code)
			}
		}

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(mqttPubs) != 1 {
			t.Fatalf("expected 1 MQTT publication, got: %v", mqttPubs)
		}
		if !bytesContains(lastPayload, "DENY") {
			t.Errorf("expected DENY gate command, got: %s", lastPayload)
		}

		if len(kafkaEvents) != 1 || kafkaEvents[0] != "exit.alarm" {
			t.Errorf("expected alarm to be published, got: %v", kafkaEvents)
		}
	})

	t.Run("Wrong user -> ErrForbidden", func(t *testing.T) {
		tokenStr, err := generateTestExitToken(orderID, userID, storeID, time.Now().Add(10*time.Minute), priv)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		repo := &mockRepository{}
		rdb := &mockRedis{}
		producer := kafka.NewProducer(nil)
		gateCommander := NewMockGateCommander()

		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")
		wrongUserID := uuid.New().String()
		_, err = svc.Validate(context.Background(), tokenStr, storeID, gateID, wrongUserID)
		if err == nil {
			t.Fatal("expected forbidden error, got nil")
		}

		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrForbidden {
				t.Errorf("expected code ErrForbidden, got: %s", appErr.Code)
			}
		}
	})

	t.Run("StaffOverride success", func(t *testing.T) {
		repo := &mockRepository{}
		rdb := &mockRedis{}
		var kafkaEvents []string
		producer := kafka.NewProducer(nil)
		producer.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
			kafkaEvents = append(kafkaEvents, topic)
			return nil
		}
		gateCommander := NewMockGateCommander()
		svc := NewExitService(repo, rdb, producer, gateCommander, pub, "")

		req := &model.StaffOverrideRequest{
			StoreID: uuid.New().String(),
			GateID:  "gate-1",
			UserID:  uuid.New().String(),
			Reason:  "SYSTEM_ERROR",
		}

		res, err := svc.StaffOverride(context.Background(), uuid.New().String(), req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != "APPROVED" || res.GateCommand != "OPEN" {
			t.Errorf("expected APPROVED and OPEN, got: %+v", res)
		}
		if len(kafkaEvents) != 1 || kafkaEvents[0] != "exit.staff_override" {
			t.Errorf("expected staff override event, got: %v", kafkaEvents)
		}
	})

	t.Run("GetTokenStatus cases", func(t *testing.T) {
		orderID := uuid.New()
		expiresAt := time.Now().Add(5 * time.Minute)
		repo := &mockRepository{
			getTokenStatusByOrderIDFn: func(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error) {
				return orderID, true, expiresAt, nil
			},
		}
		svc := NewExitService(repo, nil, nil, nil, pub, "")

		// Case 1: Status found
		res, err := svc.GetTokenStatus(context.Background(), orderID.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.OrderID != orderID.String() || !res.IsUsed || res.IsExpired {
			t.Errorf("unexpected status response: %+v", res)
		}

		// Case 2: Not found
		repo.getTokenStatusByOrderIDFn = func(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error) {
			return uuid.Nil, false, time.Time{}, pgx.ErrNoRows
		}
		_, err = svc.GetTokenStatus(context.Background(), orderID.String())
		var appErr *sharederrors.AppError
		if errors.As(err, &appErr) {
			if appErr.Code != sharederrors.ErrOrderNotFound {
				t.Errorf("expected ErrOrderNotFound code, got: %s", appErr.Code)
			}
		} else {
			t.Errorf("expected AppError, got: %v", err)
		}

		// Case 3: Other DB error
		repo.getTokenStatusByOrderIDFn = func(ctx context.Context, orderIDStr string) (uuid.UUID, bool, time.Time, error) {
			return uuid.Nil, false, time.Time{}, errors.New("db error")
		}
		_, err = svc.GetTokenStatus(context.Background(), orderID.String())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("decrementStoreOccupancy cases", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewExitService(repo, nil, nil, nil, pub, "http://localhost:8081")

		// Case 1: Success
		svc.httpClient = &http.Client{
			Transport: &mockRoundTripper{
				roundTripFn: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
					}, nil
				},
			},
		}
		svc.decrementStoreOccupancy(context.Background(), uuid.New().String())

		// Case 2: Non-OK status
		svc.httpClient = &http.Client{
			Transport: &mockRoundTripper{
				roundTripFn: func(req *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
					}, nil
				},
			},
		}
		svc.decrementStoreOccupancy(context.Background(), uuid.New().String())

		// Case 3: Client Error
		svc.httpClient = &http.Client{
			Transport: &mockRoundTripper{
				roundTripFn: func(req *http.Request) (*http.Response, error) {
					return nil, errors.New("network error")
				},
			},
		}
		svc.decrementStoreOccupancy(context.Background(), uuid.New().String())
	})

	t.Run("parseAndVerifyExitToken failures", func(t *testing.T) {
		repo := &mockRepository{}
		svc := NewExitService(repo, nil, nil, nil, pub, "")

		// Case 1: Invalid signing method alg (HMAC instead of Ed25519)
		token := jwt5.NewWithClaims(jwt5.SigningMethodHS256, jwt5.MapClaims{"order_id": "1"})
		tokenStr, _ := token.SignedString([]byte("secret"))
		_, err := svc.Validate(context.Background(), tokenStr, storeID, gateID, userID)
		if err == nil {
			t.Fatal("expected error for invalid signing method, got nil")
		}

		// Case 2: Completely malformed token
		_, err = svc.Validate(context.Background(), "not-a-token", storeID, gateID, userID)
		if err == nil {
			t.Fatal("expected error for malformed token, got nil")
		}
	})
}

func bytesContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || stringsContains(s, sub))
}

func stringsContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
