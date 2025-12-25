package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiterConfig holds rate limiter configuration
type RateLimiterConfig struct {
	RequestsPerMinute int
	BurstSize         int
	CleanupInterval   time.Duration
}

// DefaultRateLimiterConfig returns sensible defaults for production
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		RequestsPerMinute: 100,
		BurstSize:         20,
		CleanupInterval:   5 * time.Minute,
	}
}

// clientBucket tracks rate limit state for a single client
type clientBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// RateLimiter implements a token bucket rate limiter with per-client tracking
type RateLimiter struct {
	clients   map[string]*clientBucket
	mu        sync.RWMutex
	config    RateLimiterConfig
	rate      float64 // tokens per second
	maxTokens float64
}

// NewRateLimiter creates a new rate limiter with the given configuration
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		clients:   make(map[string]*clientBucket),
		config:    config,
		rate:      float64(config.RequestsPerMinute) / 60.0,
		maxTokens: float64(config.BurstSize),
	}

	// Start cleanup goroutine to remove stale entries
	go rl.cleanup()

	return rl
}

// cleanup removes stale client entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		cutoff := time.Now().Add(-rl.config.CleanupInterval)
		for key, bucket := range rl.clients {
			bucket.mu.Lock()
			if bucket.lastRefill.Before(cutoff) {
				delete(rl.clients, key)
			}
			bucket.mu.Unlock()
		}
		rl.mu.Unlock()
	}
}

// Allow checks if a request from the given client should be allowed
func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	bucket, exists := rl.clients[clientID]
	if !exists {
		bucket = &clientBucket{
			tokens:     rl.maxTokens,
			lastRefill: time.Now(),
		}
		rl.clients[clientID] = bucket
	}
	rl.mu.Unlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > rl.maxTokens {
		bucket.tokens = rl.maxTokens
	}
	bucket.lastRefill = now

	// Check if we have tokens available
	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// Middleware returns a Gin middleware handler for rate limiting
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Use X-Forwarded-For for proxied requests, fallback to ClientIP
		clientID := c.GetHeader("X-Forwarded-For")
		if clientID == "" {
			clientID = c.ClientIP()
		}

		if !rl.Allow(clientID) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": "60s",
			})
			return
		}

		c.Next()
	}
}

// RateLimitMiddleware creates a rate limiting middleware with default config
func RateLimitMiddleware() gin.HandlerFunc {
	limiter := NewRateLimiter(DefaultRateLimiterConfig())
	return limiter.Middleware()
}

// RateLimitMiddlewareWithConfig creates a rate limiting middleware with custom config
func RateLimitMiddlewareWithConfig(config RateLimiterConfig) gin.HandlerFunc {
	limiter := NewRateLimiter(config)
	return limiter.Middleware()
}
