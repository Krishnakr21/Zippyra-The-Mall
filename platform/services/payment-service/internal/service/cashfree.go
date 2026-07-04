package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/zippyra/platform/services/payment-service/internal/model"
)

type CashfreeClient struct {
	appID     string
	secretKey string
	client    *http.Client
}

func NewCashfreeClient(appID, secretKey string) *CashfreeClient {
	return &CashfreeClient{
		appID:     appID,
		secretKey: secretKey,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *CashfreeClient) CreateOrder(ctx context.Context, p *model.Payment) (*model.GatewayOrderResponse, error) {
	return &model.GatewayOrderResponse{GatewayOrderID: "cf_" + p.ID.String()}, nil
}

func (c *CashfreeClient) Refund(ctx context.Context, p *model.Payment, amount int64) (*model.GatewayRefundResponse, error) {
	return &model.GatewayRefundResponse{GatewayRefundID: "cf_rf_" + p.ID.String()}, nil
}

func (c *CashfreeClient) VerifyWebhookSignature(signature, timestamp, body, secret string) bool {
	data := timestamp + body
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	expected := base64.StdEncoding.EncodeToString(h.Sum(nil))
	return signature == expected
}

// Ensure interface compatibility
var _ Gateway = (*CashfreeClient)(nil)
