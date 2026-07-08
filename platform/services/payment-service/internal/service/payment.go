package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
)

type GatewayRouter struct {
	razorpay        *RazorpayClient
	cashfree        *CashfreeClient
	razorpayFails   int64 // sync/atomic
	circuitOpen     int32 // sync/atomic: 0=closed, 1=open
	circuitOpenedAt int64 // sync/atomic: unix timestamp
	threshold       int64 // failures before open (default 5)
	resetAfter      int64 // seconds before half-open (default 30)
}

func NewGatewayRouter(razorpay *RazorpayClient, cashfree *CashfreeClient) *GatewayRouter {
	return &GatewayRouter{
		razorpay:   razorpay,
		cashfree:   cashfree,
		threshold:  5,
		resetAfter: 30,
	}
}

func (r *GatewayRouter) SelectGateway() string {
	if atomic.LoadInt32(&r.circuitOpen) == 0 {
		return "RAZORPAY"
	}
	// check if reset window passed
	openedAt := atomic.LoadInt64(&r.circuitOpenedAt)
	if time.Now().Unix()-openedAt > r.resetAfter {
		// half-open: try Razorpay again
		atomic.StoreInt32(&r.circuitOpen, 0)
		atomic.StoreInt64(&r.razorpayFails, 0)
		return "RAZORPAY"
	}
	return "CASHFREE"
}

func (r *GatewayRouter) RecordFailure(gateway string) {
	if gateway != "RAZORPAY" {
		return
	}
	fails := atomic.AddInt64(&r.razorpayFails, 1)
	if fails >= r.threshold {
		atomic.StoreInt32(&r.circuitOpen, 1)
		atomic.StoreInt64(&r.circuitOpenedAt, time.Now().Unix())
		log.Warn().Int64("failures", fails).Msg("Razorpay circuit OPEN, routing to Cashfree")
	}
}

func (r *GatewayRouter) RecordSuccess(gateway string) {
	if gateway != "RAZORPAY" {
		return
	}
	atomic.StoreInt64(&r.razorpayFails, 0)
}

func (r *GatewayRouter) CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
	gw := r.SelectGateway()
	if gw == "RAZORPAY" {
		resp, err := r.razorpay.CreateOrder(ctx, p.AmountPaise, p.Currency, p.ID.String())
		if err != nil {
			return nil, err
		}
		return &model.GatewayOrderResponse{GatewayOrderID: resp.ID}, nil
	} else {
		return r.cashfree.CreateOrder(ctx, p)
	}
}

func (r *GatewayRouter) Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
	if p.Gateway == "RAZORPAY" {
		var gatewayPaymentID string
		if p.GatewayPaymentID != nil {
			gatewayPaymentID = *p.GatewayPaymentID
		}
		resp, err := r.razorpay.InitiateRefund(ctx, gatewayPaymentID, amount)
		if err != nil {
			return nil, err
		}
		return &model.GatewayRefundResponse{GatewayRefundID: resp.ID}, nil
	} else {
		return r.cashfree.Refund(ctx, p, amount)
	}
}

type PaymentService struct {
	pool          repository.DB
	repo          *repository.PaymentRepository
	outboxRepo    *repository.OutboxRepository
	gatewayRouter *GatewayRouter
}

func NewPaymentService(pool repository.DB, repo *repository.PaymentRepository, outboxRepo *repository.OutboxRepository, router *GatewayRouter) *PaymentService {
	return &PaymentService{
		pool:          pool,
		repo:          repo,
		outboxRepo:    outboxRepo,
		gatewayRouter: router,
	}
}

func (s *PaymentService) InitiatePayment(ctx context.Context, p *model.Payment) (*model.Payment, error) {
	// 1. Check idempotency
	existing, err := s.repo.GetByIdempotencyKey(ctx, p.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	// 2. Select Gateway
	p.Gateway = s.gatewayRouter.SelectGateway()
	p.ID = uuid.New()
	p.Status = model.PaymentStatusPending

	// 3. Create Gateway Order
	resp, err := s.gatewayRouter.CreateOrder(ctx, p)
	if err != nil {
		s.gatewayRouter.RecordFailure(p.Gateway)
		return nil, fmt.Errorf("gateway order: %w", err)
	}
	s.gatewayRouter.RecordSuccess(p.Gateway)
	p.GatewayOrderID = &resp.GatewayOrderID

	// 4. Persistence with Outbox
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.Create(ctx, tx, p); err != nil {
		return nil, fmt.Errorf("create payment: %w", err)
	}

	payload, _ := json.Marshal(p)
	if err := s.outboxRepo.Create(ctx, tx, "payments.initiated", payload); err != nil {
		return nil, fmt.Errorf("create outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return p, nil
}

func (s *PaymentService) GetStatus(ctx context.Context, id string) (*model.Payment, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PaymentService) ListHistory(ctx context.Context, userID string, limit, offset int) ([]model.Payment, error) {
	return s.repo.GetByUserID(ctx, userID, limit, offset)
}

func (s *PaymentService) InitiateRefund(ctx context.Context, id string, amountPaise int64) error {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found")
	}

	if p.Status == model.PaymentStatusRefunded {
		return fmt.Errorf("payment already refunded")
	}

	resp, err := s.gatewayRouter.Refund(ctx, p, amountPaise)
	if err != nil {
		return fmt.Errorf("gateway refund: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	p.Status = model.PaymentStatusRefunded
	p.RefundID = &resp.GatewayRefundID
	p.RefundAmountPaise = amountPaise

	if err := s.repo.Update(ctx, tx, p); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{"payment_id": id, "status": "REFUNDED", "amount": amountPaise})
	if err := s.outboxRepo.Create(ctx, tx, "payments.refunded", payload); err != nil {
		return fmt.Errorf("create outbox: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PaymentService) HandlePaymentCaptured(ctx context.Context, event model.RazorpayWebhookEvent) error {
	pID, err := uuid.Parse(event.Payload.Payment.Entity.Notes.PaymentID)
	if err != nil {
		return fmt.Errorf("invalid payment id: %w", err)
	}

	p, err := s.repo.GetByID(ctx, pID.String())
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found")
	}

	p.Status = model.PaymentStatusSuccess
	p.GatewayPaymentID = &event.Payload.Payment.Entity.ID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.Update(ctx, tx, p); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	payload, _ := json.Marshal(p)
	if err := s.outboxRepo.Create(ctx, tx, "payments.completed", payload); err != nil {
		return fmt.Errorf("create outbox: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PaymentService) HandlePaymentFailed(ctx context.Context, event model.RazorpayWebhookEvent) error {
	pID, err := uuid.Parse(event.Payload.Payment.Entity.Notes.PaymentID)
	if err != nil {
		return fmt.Errorf("invalid payment id: %w", err)
	}

	p, err := s.repo.GetByID(ctx, pID.String())
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found")
	}

	p.Status = model.PaymentStatusFailed
	p.FailureReason = &event.Payload.Payment.Entity.ErrorDescription

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.Update(ctx, tx, p); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	payload, _ := json.Marshal(p)
	if err := s.outboxRepo.Create(ctx, tx, "payments.failed", payload); err != nil {
		return fmt.Errorf("create outbox: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *PaymentService) HandlePaymentAuthorized(ctx context.Context, event model.RazorpayWebhookEvent) error {
	pID, err := uuid.Parse(event.Payload.Payment.Entity.Notes.PaymentID)
	if err != nil {
		return fmt.Errorf("invalid payment id: %w", err)
	}

	p, err := s.repo.GetByID(ctx, pID.String())
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found")
	}

	p.Status = model.PaymentStatusAuthorized
	p.GatewayPaymentID = &event.Payload.Payment.Entity.ID

	return s.repo.Update(ctx, nil, p)
}

func (s *PaymentService) HandleCashfreeWebhook(ctx context.Context, event model.CashfreeWebhookEvent) error {
	pID := event.Data.Order.OrderID
	p, err := s.repo.GetByID(ctx, pID)
	if err != nil {
		return fmt.Errorf("get payment: %w", err)
	}
	if p == nil {
		return fmt.Errorf("payment not found")
	}

	if event.Data.Payment.PaymentStatus == "SUCCESS" {
		p.Status = model.PaymentStatusSuccess
	} else {
		p.Status = model.PaymentStatusFailed
	}
	p.GatewayPaymentID = &event.Data.Payment.CfPaymentID

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.repo.Update(ctx, tx, p); err != nil {
		return fmt.Errorf("update payment: %w", err)
	}

	payload, _ := json.Marshal(p)
	topic := "payments.completed"
	if p.Status == model.PaymentStatusFailed {
		topic = "payments.failed"
	}
	if err := s.outboxRepo.Create(ctx, tx, topic, payload); err != nil {
		return fmt.Errorf("create outbox: %w", err)
	}

	return tx.Commit(ctx)
}
