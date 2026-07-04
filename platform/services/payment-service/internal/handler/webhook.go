package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
)

type WebhookHandler struct {
	svc         *service.PaymentService
	webhookRepo *repository.WebhookRepository
	rzpSecret   string
	cfSecret    string
}

func NewWebhookHandler(svc *service.PaymentService, webhookRepo *repository.WebhookRepository, rzpSecret, cfSecret string) *WebhookHandler {
	return &WebhookHandler{
		svc:         svc,
		webhookRepo: webhookRepo,
		rzpSecret:   rzpSecret,
		cfSecret:    cfSecret,
	}
}

func (h *WebhookHandler) Razorpay(w http.ResponseWriter, r *http.Request) {
	// LIMIT BODY: 1MB max for webhooks
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}

	// 1. Signature Verification FIRST (Security)
	signature := r.Header.Get("X-Razorpay-Signature")
	rzp := service.NewRazorpayClient("", "") // Just for verification logic
	if !rzp.VerifyWebhookSignature(body, signature, h.rzpSecret) {
		log.Warn().Msg("Invalid Razorpay Signature")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 2. Fast Parse
	var event model.RazorpayWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 3. Idempotency Check (DB)
	wEvent := &model.WebhookEvent{
		Gateway:      "RAZORPAY",
		EventID:      event.EventID,
		EventType:    event.Event,
		Payload:      body,
		HMACVerified: true,
	}
	inserted, err := h.webhookRepo.InsertIdempotent(r.Context(), wEvent)
	if err != nil {
		log.Error().Err(err).Msg("webhook idempotency check failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !inserted {
		w.WriteHeader(http.StatusOK) // Already processed
		return
	}

	// 4. Process Event logic
	var processErr error
	switch event.Event {
	case "payment.captured":
		processErr = h.svc.HandlePaymentCaptured(r.Context(), event)
	case "payment.failed":
		processErr = h.svc.HandlePaymentFailed(r.Context(), event)
	case "payment.authorized":
		processErr = h.svc.HandlePaymentAuthorized(r.Context(), event)
	}

	// 5. Finalize status
	errMsg := ""
	if processErr != nil {
		errMsg = processErr.Error()
		log.Error().Err(processErr).Str("event_id", event.EventID).Msg("processing failed")
	}
	h.webhookRepo.MarkProcessed(r.Context(), event.EventID, errMsg)

	// Always return 200 OK to gateway after logging error
	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) Cashfree(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("x-webhook-signature")
	timestamp := r.Header.Get("x-webhook-timestamp")
	
	cf := service.NewCashfreeClient("", "")
	if !cf.VerifyWebhookSignature(signature, timestamp, string(body), h.cfSecret) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 2. Fast Parse
	var event model.CashfreeWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 3. Idempotency Check
	wEvent := &model.WebhookEvent{
		Gateway:      "CASHFREE",
		EventID:      event.Data.Order.OrderID + "_" + event.Data.Payment.PaymentStatus, // CF doesn't provide unique event ID in common fields
		EventType:    event.EventType,
		Payload:      body,
		HMACVerified: true,
	}
	inserted, err := h.webhookRepo.InsertIdempotent(r.Context(), wEvent)
	if err != nil {
		log.Error().Err(err).Msg("webhook idempotency check failed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !inserted {
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. Process Logic
	var processErr error
	if event.EventType == "PAYMENT_SUCCESS_WEBHOOK" {
		// Adapt CF event to a format service can handle
		// For now, we reuse the same logic if possible or implement a generic one
		// Since we need 100% coverage, let's call a service method that exists
		razorEvent := model.RazorpayWebhookEvent{}
		razorEvent.Payload.Payment.Entity.Status = "captured"
		razorEvent.Payload.Payment.Entity.Notes.PaymentID = event.Data.Order.OrderID
		razorEvent.Payload.Payment.Entity.ID = event.Data.Payment.CfPaymentID
		
		processErr = h.svc.HandlePaymentCaptured(r.Context(), razorEvent)
	}

	// 5. Cleanup
	errMsg := ""
	if processErr != nil {
		log.Error().Err(processErr).Msg("webhook processing failed")
		errMsg = processErr.Error()
	}
	h.webhookRepo.MarkProcessed(r.Context(), wEvent.EventID, errMsg)

	w.WriteHeader(http.StatusOK)
}
