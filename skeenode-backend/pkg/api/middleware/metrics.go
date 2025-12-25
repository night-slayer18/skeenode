package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequestsTotal counts total HTTP requests
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "skeenode",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPRequestDuration tracks request latency
	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "skeenode",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path"},
	)

	// HTTPRequestSize tracks request body size
	HTTPRequestSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "skeenode",
			Subsystem: "http",
			Name:      "request_size_bytes",
			Help:      "HTTP request body size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 10MB
		},
		[]string{"method", "path"},
	)

	// HTTPResponseSize tracks response body size
	HTTPResponseSize = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "skeenode",
			Subsystem: "http",
			Name:      "response_size_bytes",
			Help:      "HTTP response body size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 6), // 100B to 10MB
		},
		[]string{"method", "path"},
	)

	// HTTPActiveRequests tracks in-flight requests
	HTTPActiveRequests = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "skeenode",
			Subsystem: "http",
			Name:      "requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)
)

// MetricsMiddleware records HTTP request metrics
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip metrics endpoint to avoid self-scraping noise
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		path := normalizePath(c.FullPath())
		method := c.Request.Method

		// Track in-flight requests
		HTTPActiveRequests.Inc()
		defer HTTPActiveRequests.Dec()

		// Track request size
		if c.Request.ContentLength > 0 {
			HTTPRequestSize.WithLabelValues(method, path).Observe(float64(c.Request.ContentLength))
		}

		// Process request
		c.Next()

		// Record metrics after response
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		HTTPRequestsTotal.WithLabelValues(method, path, status).Inc()
		HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		HTTPResponseSize.WithLabelValues(method, path).Observe(float64(c.Writer.Size()))
	}
}

// normalizePath normalizes the path for consistent metric labels
// Replaces dynamic segments with placeholders
func normalizePath(path string) string {
	if path == "" {
		return "unknown"
	}
	return path
}
