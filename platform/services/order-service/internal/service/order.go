package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zippyra/platform/services/order-service/internal/model"
	"github.com/zippyra/platform/services/order-service/internal/repository"
)

type OrderService struct {
	pool          *pgxpool.Pool
	orderRepo     *repository.OrderRepository
	orderItemRepo *repository.OrderItemRepository
	exitTokenSvc  *ExitTokenService
	invoiceSvc    *InvoiceService
	producer      KafkaPublisher
}

func NewOrderService(
	pool *pgxpool.Pool,
	orderRepo *repository.OrderRepository,
	orderItemRepo *repository.OrderItemRepository,
	exitTokenSvc *ExitTokenService,
	invoiceSvc *InvoiceService,
	producer KafkaPublisher,
) *OrderService {
	return &OrderService{
		pool:          pool,
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		exitTokenSvc:  exitTokenSvc,
		invoiceSvc:    invoiceSvc,
		producer:      producer,
	}
}

func (s *OrderService) CreateOrderFromPayment(ctx context.Context, event *model.PaymentConfirmedEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Build order items and aggregate totals
	var orderItems []model.OrderItem
	var subtotal float64
	var gstTotal float64
	var cgstTotal float64
	var sgstTotal float64

	orderID := uuid.New()

	for _, item := range event.Items {
		prod, err := s.orderItemRepo.GetProductByID(ctx, item.ProductID.String())
		if err != nil {
			return fmt.Errorf("fetch product %s: %w", item.ProductID, err)
		}
		if prod == nil {
			return fmt.Errorf("product not found: %s", item.ProductID)
		}

		unitPrice := item.UnitPrice
		if unitPrice <= 0 {
			unitPrice = prod.SellingPrice
		}

		// Calculate GST details
		gstRate := prod.GSTRate
		itemSubtotal := unitPrice * float64(item.Quantity)
		itemGst := itemSubtotal * (gstRate / 100.0)
		itemCgst := itemGst / 2.0
		itemSgst := itemGst / 2.0
		itemTotal := itemSubtotal + itemGst

		orderItem := model.OrderItem{
			ID:          uuid.New(),
			OrderID:     orderID,
			ProductID:   prod.ID,
			Barcode:     prod.Barcode,
			ProductName: prod.Name,
			Quantity:    item.Quantity,
			UnitPrice:   unitPrice,
			GSTRate:     gstRate,
			CGSTAmount:  itemCgst,
			SGSTAmount:  itemSgst,
			IGSTAmount:  0.0,
			GSTAmount:   itemGst,
			TotalPrice:  itemTotal,
			HSNCode:     prod.HSNCode,
		}

		orderItems = append(orderItems, orderItem)
		subtotal += itemSubtotal
		gstTotal += itemGst
		cgstTotal += itemCgst
		sgstTotal += itemSgst
	}

	orderNum := generateOrderNumber()
	returnEnds := time.Now().Add(24 * time.Hour) // 24h return window

	order := &model.Order{
		ID:                 orderID,
		OrderNumber:        orderNum,
		UserID:             event.UserID,
		StoreID:            event.StoreID,
		SessionID:          event.SessionID,
		Status:             "PAID",
		SupplyType:         "intrastate",
		Subtotal:           subtotal,
		GSTTotal:           gstTotal,
		CGSTTotal:          cgstTotal,
		SGSTTotal:          sgstTotal,
		IGSTTotal:          0.0,
		DiscountTotal:      0.0,
		TotalAmount:        subtotal + gstTotal,
		PaymentMethod:      event.PaymentMethod,
		PaymentID:          &event.PaymentID,
		ReturnWindowEndsAt: &returnEnds,
	}

	// Insert order idempotently
	savedOrder, created, err := s.orderRepo.UpsertOrder(ctx, tx, order)
	if err != nil {
		return fmt.Errorf("upsert order in DB: %w", err)
	}

	if !created {
		// Already created, commit and exit silently (idempotency check)
		_ = tx.Commit(ctx)
		return nil
	}

	// Link items and save them
	for i := range orderItems {
		orderItems[i].OrderID = savedOrder.ID
	}
	err = s.orderItemRepo.CreateItems(ctx, tx, orderItems)
	if err != nil {
		return fmt.Errorf("save order items: %w", err)
	}

	// Commit order creation
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit order transaction: %w", err)
	}

	// Issue exit token
	exitToken, err := s.exitTokenSvc.Issue(ctx, savedOrder)
	if err != nil {
		return fmt.Errorf("issue exit token: %w", err)
	}

	// Update order with exit token
	expiresAt := time.Now().Add(10 * time.Minute)
	err = s.orderRepo.UpdateExitToken(ctx, savedOrder.ID, exitToken, expiresAt)
	if err != nil {
		return fmt.Errorf("save exit token to order: %w", err)
	}
	savedOrder.ExitToken = &exitToken
	savedOrder.ExitTokenExpiresAt = &expiresAt

	// Publish order.completed event
	compEvent := model.OrderCompletedEvent{
		EventType:     "order.completed",
		OrderID:       savedOrder.ID.String(),
		OrderNumber:   savedOrder.OrderNumber,
		UserID:        savedOrder.UserID.String(),
		StoreID:       savedOrder.StoreID.String(),
		TotalAmount:   savedOrder.TotalAmount,
		ExitToken:     exitToken,
		Timestamp:     time.Now().Format(time.RFC3339),
		CorrelationID: event.CorrelationID,
	}

	err = s.producer.Publish(ctx, "order.completed", savedOrder.ID.String(), compEvent)
	if err != nil {
		// Log but do not fail the flow since order is already saved and committed
		fmt.Printf("failed to publish order.completed event: %v\n", err)
	}

	// Generate invoice async (non-blocking)
	go s.invoiceSvc.GenerateAsync(context.Background(), savedOrder)

	return nil
}

func (s *OrderService) GetByID(ctx context.Context, id string) (*model.Order, error) {
	o, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o == nil {
		return nil, nil
	}

	items, err := s.orderItemRepo.GetByOrderID(ctx, id)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (s *OrderService) GetHistory(ctx context.Context, userID string, storeIDStr string, page, limit int) ([]model.Order, error) {
	offset := (page - 1) * limit
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var storeID *uuid.UUID
	if storeIDStr != "" {
		parsed, err := uuid.Parse(storeIDStr)
		if err == nil {
			storeID = &parsed
		}
	}

	orders, err := s.orderRepo.GetHistory(ctx, userID, storeID, limit, offset)
	if err != nil {
		return nil, err
	}

	for i := range orders {
		items, err := s.orderItemRepo.GetByOrderID(ctx, orders[i].ID.String())
		if err == nil {
			orders[i].Items = items
		}
	}
	return orders, nil
}

func generateOrderNumber() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("ZP-%d-%X", time.Now().Year(), b)
}
