package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/services/auth-service/internal/middleware"
	"github.com/zippyra/platform/shared/errors"
	"github.com/zippyra/platform/shared/logger"
)

var phoneRegex = regexp.MustCompile(`^\+91[6-9]\d{9}$`)

// OTPService handles OTP generation, storage, and verification.
type OTPService struct {
	rdb              *redis.Client
	loginAttemptRepo LoginAttemptRepo
	salt             string
	appEnv           string
}

// NewOTPService creates a new OTPService.
func NewOTPService(
	rdb *redis.Client,
	loginAttemptRepo LoginAttemptRepo,
	salt, appEnv string,
) *OTPService {
	return &OTPService{
		rdb:              rdb,
		loginAttemptRepo: loginAttemptRepo,
		salt:             salt,
		appEnv:           appEnv,
	}
}

// ValidatePhone checks if a phone number matches the Indian mobile format.
func ValidatePhone(phone string) bool {
	return phoneRegex.MatchString(phone)
}

// GenerateOTP creates a cryptographically secure 6-digit OTP using crypto/rand.
// NEVER uses math/rand.
func GenerateOTP() (string, error) {
	max := big.NewInt(999999)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("failed to generate OTP: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashOTP creates a SHA-256 hash of the OTP with phone and salt.
func (s *OTPService) HashOTP(otp, phone string) string {
	h := sha256.New()
	h.Write([]byte(otp + phone + s.salt))
	return hex.EncodeToString(h.Sum(nil))
}

// SendOTP generates an OTP, stores the hash in Redis, and sends it.
// Returns an error code string if rate limited or validation fails.
func (s *OTPService) SendOTP(ctx context.Context, phone, ip, userAgent string) (string, int, error) {
	// 1. Validate phone format
	if !ValidatePhone(phone) {
		return errors.ErrInvalidPhone, 400, fmt.Errorf("invalid phone format")
	}

	// 2. Check 5-layer rate limit
	date := time.Now().Format("2006-01-02")
	layer, err := middleware.CheckRateLimit(s.rdb, phone, ip, date)
	if err != nil {
		log.Error().Err(err).Msg("rate limit check failed")
		return errors.ErrInternal, 500, err
	}
	if layer > 0 {
		log.Warn().
			Str("phone", logger.MaskPhone(phone)).
			Int("layer", layer).
			Msg("OTP rate limit exceeded")
		return errors.ErrRateLimitExceeded, 429, fmt.Errorf("rate limit exceeded at layer %d", layer)
	}

	// 3. Generate 6-digit OTP with crypto/rand
	otp, err := GenerateOTP()
	if err != nil {
		return errors.ErrInternal, 500, err
	}

	// 4. Hash OTP: sha256(otp + phone + salt)
	hash := s.HashOTP(otp, phone)

	// 5. Store hash in Redis with 10-minute TTL
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	pipe := s.rdb.Pipeline()
	pipe.Set(rCtx, "otp:"+phone, hash, 600*time.Second)
	pipe.Set(rCtx, "otp_attempts:"+phone, "0", 600*time.Second)
	_, err = pipe.Exec(rCtx)
	if err != nil {
		log.Error().Err(err).Msg("failed to store OTP in Redis")
		return errors.ErrInternal, 500, err
	}

	// 6. Log OTP in non-production environments ONLY
	if s.appEnv == "local" || s.appEnv == "pilot" {
		log.Info().
			Str("phone", logger.MaskPhone(phone)).
			Str("otp", otp).
			Msg("OTP generated")
	}

	// 7. In production: send via Twilio (placeholder — uses TwilioClient from shared/http)
	// if s.appEnv == "production" { sendViaTwilio(phone, otp) }

	// 8. Record login attempt
	go func() {
		if err := s.loginAttemptRepo.Insert(context.Background(), logger.MaskPhone(phone), ip, userAgent, "SENT"); err != nil {
			log.Error().Err(err).Msg("failed to record login attempt")
		}
	}()

	return "", 0, nil
}

// VerifyOTP validates the OTP and returns error code if invalid.
func (s *OTPService) VerifyOTP(ctx context.Context, phone, otp string) (string, int, error) {
	// 1. Validate phone format
	if !ValidatePhone(phone) {
		return errors.ErrInvalidPhone, 400, fmt.Errorf("invalid phone format")
	}

	// 2. Get stored hash
	rCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	storedHash, err := s.rdb.Get(rCtx, "otp:"+phone).Result()
	if err == redis.Nil {
		return errors.ErrOTPExpired, 400, fmt.Errorf("OTP expired or not found")
	}
	if err != nil {
		return errors.ErrInternal, 500, err
	}

	// 3. Hash incoming OTP
	incomingHash := s.HashOTP(otp, phone)

	// 4. Constant-time comparison — prevents timing attacks
	storedBytes := []byte(storedHash)
	incomingBytes := []byte(incomingHash)
	if subtle.ConstantTimeCompare(storedBytes, incomingBytes) != 1 {
		// Increment attempt counter
		rCtx2, cancel2 := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel2()

		attempts, _ := s.rdb.Incr(rCtx2, "otp_attempts:"+phone).Result()
		if attempts >= 3 {
			// Force re-send by deleting OTP
			s.rdb.Del(rCtx2, "otp:"+phone)
			s.rdb.Del(rCtx2, "otp_attempts:"+phone)
			return errors.ErrOTPMaxAttempts, 429, fmt.Errorf("max OTP attempts exceeded")
		}
		return errors.ErrOTPInvalid, 400, fmt.Errorf("invalid OTP")
	}

	// 5. OTP is valid — delete immediately (one-time use)
	rCtx3, cancel3 := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel3()

	s.rdb.Del(rCtx3, "otp:"+phone)
	s.rdb.Del(rCtx3, "otp_attempts:"+phone)

	return "", 0, nil
}
