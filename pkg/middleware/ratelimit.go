package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"job-tracker/pkg/response"
)

type clientBucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter implements an in-memory token bucket rate limiter by client IP.
type RateLimiter struct {
	mu          sync.Mutex
	clients     map[string]*clientBucket
	rate        float64 // tokens per second
	burst       float64 // maximum bucket capacity
	cleanupTick time.Duration
}

// NewRateLimiter creates a new RateLimiter with rate (requests per second) and burst limit.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	if rps <= 0 {
		rps = 10 // default 10 requests per second (~600/min)
	}
	if burst <= 0 {
		burst = 30
	}

	rl := &RateLimiter{
		clients:     make(map[string]*clientBucket),
		rate:        rps,
		burst:       float64(burst),
		cleanupTick: 3 * time.Minute,
	}

	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupTick)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, client := range rl.clients {
			if now.Sub(client.lastSeen) > 5*time.Minute {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Limit returns a middleware that limits requests according to the token bucket policy.
func (rl *RateLimiter) Limit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client IP (checking X-Forwarded-For first for reverse proxies like Render/Vercel/Cloudflare)
			ip := getClientIP(r)

			rl.mu.Lock()
			now := time.Now()
			bucket, exists := rl.clients[ip]
			if !exists {
				bucket = &clientBucket{
					tokens:   rl.burst,
					lastSeen: now,
				}
				rl.clients[ip] = bucket
			}

			// Calculate token replenishment since lastSeen
			elapsed := now.Sub(bucket.lastSeen).Seconds()
			bucket.tokens += elapsed * rl.rate
			if bucket.tokens > rl.burst {
				bucket.tokens = rl.burst
			}
			bucket.lastSeen = now

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", rl.burst))

			if bucket.tokens < 1.0 {
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Retry-After", "1")
				rl.mu.Unlock()

				response.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests, please slow down.", nil)
				return
			}

			bucket.tokens -= 1.0
			remaining := int(bucket.tokens)
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// getClientIP resolves the actual client IP, respecting common cloud proxy headers.
func getClientIP(r *http.Request) string {
	// Check standard cloud headers
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		return strings.TrimSpace(cf)
	}
	if xReal := r.Header.Get("X-Real-IP"); xReal != "" {
		return strings.TrimSpace(xReal)
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
