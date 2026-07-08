package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/zippyra/platform/services/payment-service/internal/model"
	"github.com/zippyra/platform/services/payment-service/internal/repository"
	"github.com/zippyra/platform/services/payment-service/internal/service"
	errors "github.com/zippyra/platform/shared/errors"
)

type WebhookHandler struct {
	svc           *service.PaymentService
	webhookRepo   *repository.WebhookRepository
	razorpay      *service.RazorpayClient
	webhookSecret string
	cfSecret      string
}

func NewWebhookHandler(svc *service.PaymentService, webhookRepo *repository.WebhookRepository, rzpSecret, cfSecret string) *WebhookHandler {
	return &WebhookHandler{
		svc:           svc,
		webhookRepo:   webhookRepo,
		razorpay:      service.NewRazorpayClient("", ""),
		webhookSecret: rzpSecret,
		cfSecret:      cfSecret,
	}
}

func (h *WebhookHandler) Razorpay(w http.ResponseWriter, r *http.Request) {
	// Step 1: Read body with 1MB limit
	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))
	if err != nil {
		errors.WriteError(w, 400, errors.ErrRequestTooLarge, "body too large", "")
		return
	}

	// Step 2: HMAC verification FIRST — before any DB operation
	sig := r.Header.Get("X-Razorpay-Signature")
	if !h.razorpay.VerifyWebhookSignature(body, sig, h.webhookSecret) {
		log.Warn().Str("ip", r.RemoteAddr).Msg("invalid razorpay webhook signature")
		errors.WriteError(w, 400, errors.ErrWebhookInvalidSignature, "invalid signature", "")
		return
	}

	// Step 3: Parse payload
	var event model.RazorpayWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		errors.WriteError(w, 400, errors.ErrValidationFailed, "invalid payload", "")
		return
	}

	// Step 4: Idempotency check
	inserted, err := h.webhookRepo.InsertIdempotent(r.Context(), &model.WebhookEvent{
		Gateway:      "RAZORPAY",
		EventID:      event.EventID,
		EventType:    event.Event,
		Payload:      body,
		HMACVerified: true,
	})
	if err != nil {
		errors.WriteInternalError(w, "")
		return
	}
	if !inserted {
		// already processed — return 200 immediately
		w.WriteHeader(http.StatusOK)
		return
	}

	// Step 5: Process event
	var processErr error
	switch event.Event {
	case "payment.captured":
		processErr = h.svc.HandlePaymentCaptured(r.Context(), event)
	case "payment.failed":
		processErr = h.svc.HandlePaymentFailed(r.Context(), event)
	case "payment.authorized":
		processErr = h.svc.HandlePaymentAuthorized(r.Context(), event)
	default:
		log.Info().Str("event", event.Event).Msg("unhandled razorpay event")
	}

	// Step 6: Mark processed
	errMsg := ""
	if processErr != nil {
		log.Error().Err(processErr).Msg("webhook processing failed")
		errMsg = processErr.Error()
	}
	h.webhookRepo.MarkProcessed(r.Context(), event.EventID, errMsg)

	// Must respond within 200ms
	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) Cashfree(w http.ResponseWriter, r *http.Request) {
	// Step 1: Read body with 1MB limit
	body, err := io.ReadAll(io.LimitReader(r.Body, 1*1024*1024))
	if err != nil {
		errors.WriteError(w, 400, errors.ErrRequestTooLarge, "body too large", "")
		return
	}

	// Step 2: HMAC verification FIRST — before any DB operation
	sig := r.Header.Get("x-webhook-signature")
	ts := r.Header.Get("x-webhook-timestamp")

	cf := service.NewCashfreeClient("", "")
	if !cf.VerifyWebhookSignature(sig, ts, string(body), h.cfSecret) {
		log.Warn().Str("ip", r.RemoteAddr).Msg("invalid cashfree webhook signature")
		errors.WriteError(w, 400, errors.ErrWebhookInvalidSignature, "invalid signature", "")
		return
	}

	// Step 3: Parse payload
	var event model.CashfreeWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		errors.WriteError(w, 400, errors.ErrValidationFailed, "invalid payload", "")
		return
	}

	// Step 4: Idempotency check
	eventID := event.Data.Order.OrderID + "_" + event.Data.Payment.PaymentStatus
	inserted, err := h.webhookRepo.InsertIdempotent(r.Context(), &model.WebhookEvent{
		Gateway:      "CASHFREE",
		EventID:      eventID,
		EventType:    event.EventType,
		Payload:      body,
		HMACVerified: true,
	})
	if err != nil {
		errors.WriteInternalError(w, "")
		return
	}
	if !inserted {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Step 5: Process event
	var processErr error
	if event.EventType == "PAYMENT_SUCCESS_WEBHOOK" || event.EventType == "PAYMENT_FAILED_WEBHOOK" {
		processErr = h.svc.HandleCashfreeWebhook(r.Context(), event)
	}

	// Step 6: Mark processed
	errMsg := ""
	if processErr != nil {
		log.Error().Err(processErr).Msg("webhook processing failed")
		errMsg = processErr.Error()
	}
	h.webhookRepo.MarkProcessed(r.Context(), eventID, errMsg)

	w.WriteHeader(http.StatusOK)
}
