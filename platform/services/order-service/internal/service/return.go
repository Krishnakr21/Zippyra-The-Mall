package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
)

var (
	ErrOrderNotFound       = errors.New("ORDER_NOT_FOUND")
	ErrReturnWindowClosed  = errors.New("RETURN_WINDOW_CLOSED")
	ErrItemNotReturnable   = errors.New("ITEM_NOT_RETURNABLE")
	ErrReturnAlreadyExists = errors.New("RETURN_ALREADY_REQUESTED")
	ErrUnauthorized        = errors.New("UNAUTHORIZED")
	ErrForbidden           = errors.New("FORBIDDEN")
	ErrReturnNotFound      = errors.New("RETURN_NOT_FOUND")
	ErrItemNotFound        = errors.New("ITEM_NOT_FOUND")
)

type ReturnService struct {
	orderRepo         *repository.OrderRepository
	orderItemRepo     *repository.OrderItemRepository
	returnRepo        *repository.ReturnRepository
	redis             redis.Cmdable
	producer          KafkaPublisher
	paymentServiceURL string
	httpClient        *http.Client
}

func NewReturnService(
	orderRepo *repository.OrderRepository,
	orderItemRepo *repository.OrderItemRepository,
	returnRepo *repository.ReturnRepository,
	redis redis.Cmdable,
	producer KafkaPublisher,
	paymentServiceURL string,
) *ReturnService {
	return &ReturnService{
		orderRepo:         orderRepo,
		orderItemRepo:     orderItemRepo,
		returnRepo:        returnRepo,
		redis:             redis,
		producer:          producer,
		paymentServiceURL: paymentServiceURL,
		httpClient:        &http.Client{Timeout: 5 * time.Second},
	}
}

func (s *ReturnService) CreateReturn(ctx context.Context, userID string, orderID string, itemIDs []string, reason string) (*model.ReturnRequest, error) {
	// 1. Fetch order
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("fetch order: %w", err)
	}
	if order == nil {
		return nil, ErrOrderNotFound
	}

	// 2. Validate order belongs to user
	if order.UserID.String() != userID {
		return nil, ErrForbidden
	}

	// 3. Check return window ends at
	if order.ReturnWindowEndsAt != nil && order.ReturnWindowEndsAt.Before(time.Now()) {
		return nil, ErrReturnWindowClosed
	}

	// 4. Check no existing pending return request
	hasPending, err := s.returnRepo.HasPendingReturn(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("check pending return: %w", err)
	}
	if hasPending {
		return nil, ErrReturnAlreadyExists
	}

	// Fetch order items to validate item details
	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("fetch order items: %w", err)
	}

	// Map order items by ID for fast lookup
	itemsMap := make(map[string]model.OrderItem)
	for _, item := range orderItems {
		itemsMap[item.ID.String()] = item
	}

	// 5. Validate returnability of each item
	for _, itemID := range itemIDs {
		orderItem, ok := itemsMap[itemID]
		if !ok {
			return nil, ErrItemNotFound
		}

		product, err := s.orderItemRepo.GetProductByID(ctx, orderItem.ProductID.String())
		if err != nil {
			return nil, fmt.Errorf("fetch product metadata: %w", err)
		}
		if product == nil {
			return nil, fmt.Errorf("product details not found in registry: %s", orderItem.ProductID.String())
		}

		if !product.IsReturnable {
			return nil, ErrItemNotReturnable
		}
	}

	// 6. Create return request
	retReq := &model.ReturnRequest{
		ID:              uuid.New(),
		OrderID:         order.ID,
		UserID:          order.UserID,
		StoreID:         order.StoreID,
		Status:          model.ReturnStatusPendingApproval,
		Reason:          reason,
		Items:           itemIDs,
		RefundInitiated: false,
	}

	err = s.returnRepo.CreateReturnRequest(ctx, retReq)
	if err != nil {
		return nil, fmt.Errorf("create return request in DB: %w", err)
	}

	return retReq, nil
}

func (s *ReturnService) AcceptReturn(ctx context.Context, returnID string, itemsVerified []string) error {
	ret, err := s.returnRepo.GetReturnRequestByID(ctx, returnID)
	if err != nil {
		return fmt.Errorf("fetch return request: %w", err)
	}
	if ret == nil {
		return ErrReturnNotFound
	}

	if ret.Status != model.ReturnStatusPendingApproval {
		return errors.New("return request is already processed")
	}

	// Load order items and build lookup map
	orderItems, err := s.orderItemRepo.GetByOrderID(ctx, ret.OrderID.String())
	if err != nil {
		return fmt.Errorf("fetch order items: %w", err)
	}

	itemsMap := make(map[string]model.OrderItem)
	for _, item := range orderItems {
		itemsMap[item.ID.String()] = item
	}

	// Build map of verified items
	verifiedMap := make(map[string]bool)
	for _, id := range itemsVerified {
		verifiedMap[id] = true
	}

	// Calculate refund amount & process inventory updates
	var refundAmountFloat float64
	for _, itemID := range ret.Items {
		if !verifiedMap[itemID] {
			continue // Only refund and restore inventory for verified items
		}

		orderItem, ok := itemsMap[itemID]
		if !ok {
			return fmt.Errorf("returned item not found on order: %s", itemID)
		}

		// Restore inventory in Redis: INCRBY stock:{store_id}:{product_id} {quantity}
		redisKey := fmt.Sprintf("stock:%s:%s", ret.StoreID.String(), orderItem.ProductID.String())
		err = s.redis.IncrBy(ctx, redisKey, int64(orderItem.Quantity)).Err()
		if err != nil {
			// Log but do not fail
			fmt.Printf("failed to restore redis stock for key %s: %v\n", redisKey, err)
		}

		// Update product stock in DB
		err = s.orderItemRepo.UpdateProductStock(ctx, orderItem.ProductID.String(), orderItem.Quantity)
		if err != nil {
			return fmt.Errorf("update product stock in DB: %w", err)
		}

		// Publish inventory.movement Kafka event
		movement := model.InventoryMovementEvent{
			EventType: "inventory.movement",
			StoreID:   ret.StoreID.String(),
			ProductID: orderItem.ProductID.String(),
			Quantity:  orderItem.Quantity,
			Type:      "RETURN",
			Timestamp: time.Now().Format(time.RFC3339),
		}
		_ = s.producer.Publish(ctx, "inventory.movement", orderItem.ProductID.String(), movement)

		refundAmountFloat += orderItem.TotalPrice
	}

	// Update status to ACCEPTED
	err = s.returnRepo.UpdateReturnStatus(ctx, returnID, model.ReturnStatusAccepted, true)
	if err != nil {
		return fmt.Errorf("update return status: %w", err)
	}

	// Load the order to get the payment ID
	order, err := s.orderRepo.GetByID(ctx, ret.OrderID.String())
	if err != nil {
		return fmt.Errorf("fetch associated order: %w", err)
	}
	if order == nil {
		return ErrOrderNotFound
	}

	// Initiate refund via payment-service HTTP call
	if order.PaymentID != nil {
		refundAmountPaise := int64(refundAmountFloat * 100.0)
		refundReqBody, _ := json.Marshal(map[string]interface{}{
			"amount_paise": refundAmountPaise,
		})

		url := fmt.Sprintf("%s/v1/payments/%s/refund", s.paymentServiceURL, order.PaymentID.String())
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(refundReqBody))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, httpErr := s.httpClient.Do(req)
			if httpErr != nil {
				fmt.Printf("refund HTTP request failed: %v\n", httpErr)
			} else {
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					fmt.Printf("refund endpoint returned status code: %d\n", resp.StatusCode)
				}
			}
		}
	}

	// Reverse loyalty points: publish loyalty.points_reversed Kafka event
	loyaltyEvent := model.LoyaltyPointsReversedEvent{
		EventType: "loyalty.points_reversed",
		OrderID:   ret.OrderID.String(),
		UserID:    ret.UserID.String(),
		Amount:    refundAmountFloat,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	_ = s.producer.Publish(ctx, "loyalty.points_reversed", ret.OrderID.String(), loyaltyEvent)

	return nil
}

func (s *ReturnService) SetPaymentServiceURL(url string) {
	s.paymentServiceURL = url
}
