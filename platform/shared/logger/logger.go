package logger

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

// STRICT PII LOGGING RULES:
// NEVER log: full phone number, full email, card numbers, JWT tokens, customer names
// ALWAYS log: user_id (UUID), request_id, store_id, masked phone (last 4 digits only)


// Init initializes the global logger.
// Log levels: DEBUG (staging/dev), INFO (production)
// Always includes: service_name, environment, trace_id, timestamp.
func Init(serviceName, environment string) {
	zerolog.TimeFieldFormat = time.RFC3339

	level := zerolog.DebugLevel
	if environment == "production" || environment == "prod" {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	log.Logger = zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service_name", serviceName).
		Str("environment", environment).
		Logger()
}

// Ctx returns a logger instance with trace_id attached from the context.
func Ctx(ctx context.Context) *zerolog.Logger {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		l := log.Logger.With().Str("trace_id", span.SpanContext().TraceID().String()).Logger()
		return &l
	}
	l := log.Logger.With().Str("trace_id", "").Logger()
	return &l
}

// Field Helpers for consistent and safe logging

func WithMaskedPhone(phone string) *zerolog.Event {
	return log.Info().Str("phone_masked", MaskPhone(phone))
}

func WithMaskedEmail(email string) *zerolog.Event {
	return log.Info().Str("email_masked", MaskEmail(email))
}

func WithRequestID(requestID string) *zerolog.Event {
	return log.Info().Str("request_id", requestID)
}

func WithUserID(userID string) *zerolog.Event {
	return log.Info().Str("user_id", userID)
}

func WithStoreID(storeID string) *zerolog.Event {
	return log.Info().Str("store_id", storeID)
}

func WithService(name string) *zerolog.Event {
	return log.Info().Str("service_name", name)
}
