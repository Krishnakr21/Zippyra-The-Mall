package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/zippyra/platform/shared/errors"
)

// 5-layer rate limiting Lua script — ALL layers checked in one atomic round-trip.
// Keys: phone_10m, phone_1m, ip_10m, ip_1h, global_day
// Args: limit1, exp1, limit2, exp2, limit3, exp3, limit4, exp4, limit5, exp5
const rateLimitLua = `
local keys = {KEYS[1], KEYS[2], KEYS[3], KEYS[4], KEYS[5]}
local limits = {tonumber(ARGV[1]), tonumber(ARGV[3]), tonumber(ARGV[5]), tonumber(ARGV[7]), tonumber(ARGV[9])}
local ttls = {tonumber(ARGV[2]), tonumber(ARGV[4]), tonumber(ARGV[6]), tonumber(ARGV[8]), tonumber(ARGV[10])}

for i = 1, 5 do
  local current = redis.call("GET", keys[i])
  if current and tonumber(current) >= limits[i] then
    return i
  end
end

for i = 1, 5 do
  local current = redis.call("INCR", keys[i])
  if current == 1 then
    redis.call("EXPIRE", keys[i], ttls[i])
  end
end

return 0
`

// CheckRateLimit runs the atomic 5-layer Lua script.
// Returns the layer number that was exceeded (1-5) or 0 if all passed.
func CheckRateLimit(rdb *redis.Client, phone, ip, date string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	script := redis.NewScript(rateLimitLua)

	keys := []string{
		"otp_phone_10m:" + phone, // Layer 1: 5 per phone per 10min
		"otp_phone_1m:" + phone,  // Layer 2: 3 per phone per 1min
		"otp_ip_10m:" + ip,       // Layer 3: 10 per IP per 10min
		"otp_ip_1h:" + ip,        // Layer 4: 20 per IP per 1hr
		"otp_global:" + date,     // Layer 5: 100 per day globally
	}

	args := []interface{}{
		5, 600,     // Layer 1: limit=5, ttl=600s (10min)
		3, 60,      // Layer 2: limit=3, ttl=60s (1min)
		10, 600,    // Layer 3: limit=10, ttl=600s
		20, 3600,   // Layer 4: limit=20, ttl=3600s (1hr)
		100, 86400, // Layer 5: limit=100, ttl=86400s (24hr)
	}

	result, err := script.Run(ctx, rdb, keys, args...).Int()
	if err != nil {
		log.Error().Err(err).Msg("rate limit lua script failed")
		return 0, err
	}

	if result > 0 {
		log.Warn().Int("layer", result).Str("ip", ip).Msg("rate limit exceeded")
	}

	return result, nil
}

// WriteRateLimitError writes a 429 response with the appropriate error code.
func WriteRateLimitError(w http.ResponseWriter, requestID string) {
	errors.WriteError(w, http.StatusTooManyRequests, errors.ErrRateLimitExceeded,
		"Too many requests. Please try again later.", requestID)
}
