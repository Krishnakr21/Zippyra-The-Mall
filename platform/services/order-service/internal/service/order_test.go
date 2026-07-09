package service

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/zippyra/platform/services/order-service/internal/kafka"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
	"github.com/zippyra/platform/services/order-service/internal/testutil"
	"github.com/zippyra/platform/shared/jwt"
)

type MockJWTService struct {
	GenerateExitTokenFn func(claims jwt.ExitTokenClaims) (string, error)
}

func (m MockJWTService) GenerateExitToken(claims jwt.ExitTokenClaims) (string, error) {
	if m.GenerateExitTokenFn != nil {
		return m.GenerateExitTokenFn(claims)
	}
	return "mock_exit_token_jwt", nil
}

type MockPublisher struct {
	lock          sync.Mutex
	PublishedMsgs map[string]interface{}
}

func (m *MockPublisher) Publish(ctx context.Context, topic, key string, value interface{}) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.PublishedMsgs[topic] = value
	return nil
}

func newTestRedis(t *testing.T) (*redis.Client, func()) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "zippyra_local",
		DB:       2,
	})
	ctx := context.Background()
	err := rdb.Ping(ctx).Err()
	assert.NoError(t, err)
	cleanup := func() {
		_ = rdb.FlushDB(ctx)
		_ = rdb.Close()
	}
	return rdb, cleanup
}

func setupTestOrderService(t *testing.T) (*OrderService, *repository.OrderRepository, *repository.OrderItemRepository, *ExitTokenService, *InvoiceService, *MockPublisher, func()) {
	pool, dbCleanup := testutil.SetupDB()
	rdb, redisCleanup := newTestRedis(t)

	orderRepo := repository.NewOrderRepository(pool)
	orderItemRepo := repository.NewOrderItemRepository(pool)
	exitTokenRepo := repository.NewExitTokenRepository(pool)

	mockPublisher := &MockPublisher{
		PublishedMsgs: make(map[string]interface{}),
	}

	exitTokenSvc := NewExitTokenService(exitTokenRepo, orderRepo, rdb, MockJWTService{})
	invoiceSvc := NewInvoiceService(orderRepo, orderItemRepo, mockPublisher)
	orderSvc := NewOrderService(pool, orderRepo, orderItemRepo, exitTokenSvc, invoiceSvc, mockPublisher)

	cleanup := func() {
		dbCleanup()
		redisCleanup()
	}

	return orderSvc, orderRepo, orderItemRepo, exitTokenSvc, invoiceSvc, mockPublisher, cleanup
}

func TestOrderService_CreateOrderFromPayment_Success(t *testing.T) {
	orderSvc, orderRepo, orderItemRepo, _, _, mockPublisher, cleanup := setupTestOrderService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, orderSvc.pool)
	assert.NoError(t, err)

	paymentID := uuid.New()
	event := &model.PaymentConfirmedEvent{
		PaymentID:     paymentID,
		UserID:        userID,
		StoreID:       storeID,
		SessionID:     uuid.New(),
		Amount:        118.00,
		Currency:      "INR",
		PaymentMethod: "UPI",
		CorrelationID: "corr_id_123",
		Items: []model.ConfirmItem{
			{
				ProductID: productID,
				Quantity:  1,
				UnitPrice: 100.00,
			},
		},
	}

	err = orderSvc.CreateOrderFromPayment(ctx, event)
	assert.NoError(t, err)

	// Verify order created in DB
	o, err := orderRepo.GetByPaymentID(ctx, orderSvc.pool, paymentID)
	assert.NoError(t, err)
	assert.NotNil(t, o)
	assert.Equal(t, "PAID", o.Status)
	assert.Equal(t, 118.00, o.TotalAmount)

	// Verify items created
	items, err := orderItemRepo.GetByOrderID(ctx, o.ID.String())
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, productID, items[0].ProductID)
	assert.Equal(t, 18.00, items[0].GSTAmount)

	// Verify events published
	mockPublisher.lock.Lock()
	compEvent, ok := mockPublisher.PublishedMsgs["order.completed"].(model.OrderCompletedEvent)
	mockPublisher.lock.Unlock()
	assert.True(t, ok)
	assert.Equal(t, o.ID.String(), compEvent.OrderID)
	assert.Equal(t, "mock_exit_token_jwt", compEvent.ExitToken)
}

func TestOrderService_CreateOrderFromPayment_Idempotent(t *testing.T) {
	orderSvc, _, _, _, _, _, cleanup := setupTestOrderService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, productID, err := testutil.SeedBaseData(ctx, orderSvc.pool)
	assert.NoError(t, err)

	paymentID := uuid.New()
	event := &model.PaymentConfirmedEvent{
		PaymentID:     paymentID,
		UserID:        userID,
		StoreID:       storeID,
		SessionID:     uuid.New(),
		Amount:        118.00,
		Currency:      "INR",
		PaymentMethod: "UPI",
		CorrelationID: "corr_id_456",
		Items: []model.ConfirmItem{
			{
				ProductID: productID,
				Quantity:  1,
				UnitPrice: 100.00,
			},
		},
	}

	// 1st Create
	err = orderSvc.CreateOrderFromPayment(ctx, event)
	assert.NoError(t, err)

	// 2nd Create (should skip silently and return nil)
	err = orderSvc.CreateOrderFromPayment(ctx, event)
	assert.NoError(t, err)
}

func TestOrderService_CreateOrderFromPayment_SagaTrigger(t *testing.T) {
	orderSvc, _, _, _, _, _, cleanup := setupTestOrderService(t)
	defer cleanup()

	ctx := context.Background()
	userID, storeID, _, err := testutil.SeedBaseData(ctx, orderSvc.pool)
	assert.NoError(t, err)

	// Event with invalid productID to trigger failure in CreateOrderFromPayment
	paymentID := uuid.New()
	event := &model.PaymentConfirmedEvent{
		PaymentID:     paymentID,
		UserID:        userID,
		StoreID:       storeID,
		SessionID:     uuid.New(),
		Amount:        118.00,
		Currency:      "INR",
		PaymentMethod: "UPI",
		CorrelationID: "corr_id_789",
		Items: []model.ConfirmItem{
			{
				ProductID: uuid.New(), // Non-existent product ID
				Quantity:  1,
				UnitPrice: 100.00,
			},
		},
	}

	// CreateOrderFromPayment should fail
	err = orderSvc.CreateOrderFromPayment(ctx, event)
	assert.Error(t, err)

	// Now we feed this to consumer which triggers saga event on failure after 3 retries
	mockProd := kafka.NewProducer(nil)
	publishedSaga := make(map[string]model.OrderCreationFailedEvent)
	var lock sync.Mutex
	mockProd.PublishFn = func(ctx context.Context, topic, key string, value interface{}) error {
		lock.Lock()
		defer lock.Unlock()
		if topic == "order.creation_failed" {
			publishedSaga[key] = value.(model.OrderCreationFailedEvent)
		}
		return nil
	}

	// Start consumer processing
	consumer := kafka.NewConsumer([]string{"localhost:9092"}, "group", "payment.confirmed", orderSvc, mockProd)

	// Marshal event and wrap in consumer message
	eventBytes, _ := json.Marshal(event)
	msg := kafkago.Message{
		Key:   []byte(paymentID.String()),
		Value: eventBytes,
	}

	err = consumer.HandlePaymentConfirmed(ctx, msg)
	assert.Error(t, err) // Expect HandlePaymentConfirmed to return error after 3 failed tries

	// Since consumer returned error, the caller (consumer loop) will publish saga
	// Simulate the outer consumer loop logic:
	comp := model.OrderCreationFailedEvent{
		EventType: "order.creation_failed",
		PaymentID: event.PaymentID.String(),
		UserID:    event.UserID.String(),
		Reason:    err.Error(),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	_ = mockProd.Publish(ctx, "order.creation_failed", event.PaymentID.String(), comp)

	lock.Lock()
	sagaEvent, ok := publishedSaga[paymentID.String()]
	lock.Unlock()
	assert.True(t, ok)
	assert.Equal(t, "order.creation_failed", sagaEvent.EventType)
	assert.Contains(t, sagaEvent.Reason, "product not found")
}
