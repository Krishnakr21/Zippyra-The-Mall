package service

import (
	"context"
	"net/http"
	"time"

	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type RazorpayClient struct {
	key    string
	secret string
	client *http.Client
}

func NewRazorpayClient(key, secret string) *RazorpayClient {
	return &RazorpayClient{
		key:    key,
		secret: secret,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *RazorpayClient) CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
	return &model.GatewayOrderResponse{GatewayOrderID: "rzp_" + p.ID.String()}, nil
}

func (c *RazorpayClient) Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
	return &model.GatewayRefundResponse{GatewayRefundID: "rf_" + p.ID.String()}, nil
}

// Ensure interface compatibility
var _ Gateway = (*RazorpayClient)(nil)
