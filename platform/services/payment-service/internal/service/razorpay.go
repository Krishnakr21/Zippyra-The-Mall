package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	sharedhttp "github.com/zippyra/platform/shared/http"
)

type RazorpayOrder struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Receipt  string `json:"receipt"`
	Status   string `json:"status"`
}

type RazorpayRefund struct {
	ID        string `json:"id"`
	Entity    string `json:"entity"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	PaymentID string `json:"payment_id"`
}

type RazorpayClient struct {
	keyID     string
	keySecret string
	client    *http.Client
	baseURL   string
}

func NewRazorpayClient(keyID, keySecret string) *RazorpayClient {
	return &RazorpayClient{
		keyID:     keyID,
		keySecret: keySecret,
		client:    sharedhttp.RazorpayClient, // 10s timeout
		baseURL:   "https://api.razorpay.com/v1",
	}
}

func (c *RazorpayClient) CreateOrder(ctx context.Context, amountPaise int64, currency, receipt string) (*RazorpayOrder, error) {
	body := map[string]interface{}{
		"amount":   amountPaise,
		"currency": currency,
		"receipt":  receipt,
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/orders", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("razorpay request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay error: status %d", resp.StatusCode)
	}

	var order RazorpayOrder
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &order, nil
}

func (c *RazorpayClient) VerifyWebhookSignature(body []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func (c *RazorpayClient) InitiateRefund(ctx context.Context, paymentID string, amountPaise int64) (*RazorpayRefund, error) {
	body := map[string]interface{}{"amount": amountPaise}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/payments/%s/refund", c.baseURL, paymentID),
		bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.SetBasicAuth(c.keyID, c.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("razorpay refund: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("razorpay refund error: status %d", resp.StatusCode)
	}

	var refund RazorpayRefund
	if err := json.NewDecoder(resp.Body).Decode(&refund); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &refund, nil
}

func (c *RazorpayClient) SetHTTPClient(client *http.Client) {
	c.client = client
}
