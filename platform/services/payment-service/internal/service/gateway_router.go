package service

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type Gateway interface {
	CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error)
	Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error)
}

type GatewayRouter struct {
	razorpay        Gateway
	cashfree        Gateway
	razorpayFails   int64  // sync/atomic
	circuitOpen     int32  // sync/atomic: 0=closed, 1=open
	circuitOpenedAt int64  // sync/atomic: unix timestamp
	threshold       int64  // failures before open (default 5)
	resetAfter      int64  // seconds before half-open (default 30)
}

func NewGatewayRouter(razorpay Gateway, cashfree Gateway) *GatewayRouter {
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

func (r *GatewayRouter) RecordSuccess(gateway string) {
	if gateway != "RAZORPAY" {
		return
	}
	atomic.StoreInt64(&r.razorpayFails, 0)
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

func (r *GatewayRouter) CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
	if r.SelectGateway() == "RAZORPAY" {
		return r.razorpay.CreateOrder(ctx, p)
	}
	return r.cashfree.CreateOrder(ctx, p)
}

func (r *GatewayRouter) Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
	if p.Gateway == "RAZORPAY" {
		return r.razorpay.Refund(ctx, p, amount)
	}
	return r.cashfree.Refund(ctx, p, amount)
}
