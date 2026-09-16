package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/abuamar142/auth-service/internal/response"
)

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*tokenBucket
	rate    float64 // max tokens (burst)
	window  time.Duration
}

// newRateLimiter creates a rate limiter: max `rate` requests per `window` per key.
// Tokens refill linearly over the window.
func newRateLimiter(rate int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		buckets: make(map[string]*tokenBucket),
		rate:    float64(rate),
		window:  window,
	}

	// Cleanup stale entries every minute
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()

	return rl
}

func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists {
		b = &tokenBucket{
			tokens:     float64(rl.rate) - 1, // consume one immediately
			maxTokens:  float64(rl.rate),
			refillRate: float64(rl.rate) / rl.window.Seconds(),
			lastRefill: now,
		}
		rl.buckets[key] = b
		return true
	}

	// Refill tokens
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window * 2)
	for k, b := range rl.buckets {
		if b.lastRefill.Before(cutoff) {
			delete(rl.buckets, k)
		}
	}
}

// RateLimit returns a middleware that limits requests per IP.
// rate = max requests per window.
func RateLimit(rate int, window time.Duration) func(http.Handler) http.Handler {
	limiter := newRateLimiter(rate, window)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
				ip = fwd
			}

			if !limiter.allow(ip) {
				response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests", "try again later")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitAuth returns a stricter rate limiter for auth endpoints (login, register).
func RateLimitAuth() func(http.Handler) http.Handler {
	return RateLimit(10, time.Minute) // 10 requests per minute per IP
}
