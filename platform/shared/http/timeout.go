package http

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

var (
	// RazorpayClient — 10s total timeout, 3s dial
	// Use for: create order, fetch payment status
	RazorpayClient *http.Client

	// CashfreeClient — 10s total timeout, 3s dial
	// Use for: fallback payment gateway
	CashfreeClient *http.Client

	// TwilioClient — 5s total timeout, 2s dial
	// Use for: OTP SMS sending
	TwilioClient *http.Client

	// WhatsAppClient — 5s total timeout, 2s dial
	// Use for: receipt delivery, exit QR notification
	WhatsAppClient *http.Client

	// InternalClient — 3s total timeout, 1s dial
	// Use for: service-to-service HTTP calls
	InternalClient *http.Client

	// IRPClient — 15s total timeout, 5s dial
	// Use for: GST IRP e-invoice generation (NIC API can be slow)
	IRPClient *http.Client

	// GSTNClient — 8s total timeout, 3s dial
	// Use for: real-time GSTIN validation
	GSTNClient *http.Client
)

func init() {
	RazorpayClient = createClient(10*time.Second, 3*time.Second)
	CashfreeClient = createClient(10*time.Second, 3*time.Second)
	TwilioClient = createClient(5*time.Second, 2*time.Second)
	WhatsAppClient = createClient(5*time.Second, 2*time.Second)
	InternalClient = createClient(3*time.Second, 1*time.Second)
	IRPClient = createClient(15*time.Second, 5*time.Second)
	GSTNClient = createClient(8*time.Second, 3*time.Second)
}

func createClient(totalTimeout, dialTimeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: totalTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   dialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   dialTimeout, // Usually same as dial for consistency
			ResponseHeaderTimeout: totalTimeout / 2,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
		},
	}
}

// NewRequestWithTimeout creates an http.Request with context timeout.
// Use this for every outbound HTTP call.
func NewRequestWithTimeout(ctx context.Context, timeout time.Duration, method, url string, body io.Reader) (*http.Request, context.CancelFunc, error) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(reqCtx, method, url, body)
	if err != nil {
		cancel()
		return nil, nil, err
	}
	return req, cancel, nil
}
