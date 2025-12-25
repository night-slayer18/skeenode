package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "skeenode/pkg/api/middleware"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerMinute: 10,
		BurstSize:         5,
		CleanupInterval:   time.Minute,
	}
	limiter := NewRateLimiter(config)
	
	// Should allow first 5 requests (burst)
	for i := 0; i < 5; i++ {
		if !limiter.Allow("client1") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksExcessRequests(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerMinute: 60, // 1 per second
		BurstSize:         2,
		CleanupInterval:   time.Minute,
	}
	limiter := NewRateLimiter(config)
	
	// Use burst
	limiter.Allow("client1")
	limiter.Allow("client1")
	
	// Third request should be blocked
	if limiter.Allow("client1") {
		t.Error("third request should be blocked after burst exhausted")
	}
}

func TestRateLimiter_SeparatesClients(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}
	limiter := NewRateLimiter(config)
	
	// Use client1's burst
	limiter.Allow("client1")
	
	// Client2 should still have its own quota
	if !limiter.Allow("client2") {
		t.Error("different client should have separate quota")
	}
}

func TestRateLimiter_RefillsOverTime(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerMinute: 6000, // 100 per second for quick test
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}
	limiter := NewRateLimiter(config)
	
	// Use the token
	limiter.Allow("client1")
	
	// Wait for refill
	time.Sleep(20 * time.Millisecond)
	
	// Should have refilled
	if !limiter.Allow("client1") {
		t.Error("token should have refilled after waiting")
	}
}

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerMinute: 60,
		BurstSize:         1,
		CleanupInterval:   time.Minute,
	}
	
	router := gin.New()
	router.Use(RateLimitMiddlewareWithConfig(config))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	
	// First request succeeds
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("first request expected 200, got %d", w.Code)
	}
	
	// Second request should be rate limited
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req)
	
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("second request expected 429, got %d", w2.Code)
	}
}
