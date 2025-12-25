package middleware

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// ValidatorConfig holds validation configuration
type ValidatorConfig struct {
	MaxBodySize       int64    // Maximum request body size in bytes
	AllowedJobTypes   []string // Allowed job types
	CommandBlacklist  []string // Dangerous command patterns
	MaxNameLength     int      // Maximum job name length
	MaxCommandLength  int      // Maximum command length
}

// DefaultValidatorConfig returns safe defaults
func DefaultValidatorConfig() ValidatorConfig {
	return ValidatorConfig{
		MaxBodySize:       1 << 20, // 1MB
		AllowedJobTypes:   []string{"SHELL", "DOCKER", "HTTP"},
		CommandBlacklist:  []string{"rm -rf /", ":(){ :|:& };:", "mkfs", "dd if="},
		MaxNameLength:     256,
		MaxCommandLength:  4096,
	}
}

// Validator performs request validation
type Validator struct {
	config           ValidatorConfig
	dangerousPattern *regexp.Regexp
}

// NewValidator creates a new validator with the given config
func NewValidator(config ValidatorConfig) *Validator {
	// Build dangerous command pattern
	patterns := make([]string, len(config.CommandBlacklist))
	for i, p := range config.CommandBlacklist {
		patterns[i] = regexp.QuoteMeta(p)
	}
	pattern := regexp.MustCompile(strings.Join(patterns, "|"))

	return &Validator{
		config:           config,
		dangerousPattern: pattern,
	}
}

// ValidateCommand checks if a command is safe to execute
func (v *Validator) ValidateCommand(command string) error {
	if len(command) > v.config.MaxCommandLength {
		return &ValidationError{
			Field:   "command",
			Message: "command exceeds maximum length",
		}
	}

	if v.dangerousPattern.MatchString(command) {
		return &ValidationError{
			Field:   "command",
			Message: "command contains potentially dangerous patterns",
		}
	}

	return nil
}

// ValidateJobType checks if job type is allowed
func (v *Validator) ValidateJobType(jobType string) error {
	for _, allowed := range v.config.AllowedJobTypes {
		if jobType == allowed {
			return nil
		}
	}
	return &ValidationError{
		Field:   "type",
		Message: "invalid job type",
	}
}

// ValidateName checks job name
func (v *Validator) ValidateName(name string) error {
	if len(name) == 0 {
		return &ValidationError{
			Field:   "name",
			Message: "name is required",
		}
	}
	if len(name) > v.config.MaxNameLength {
		return &ValidationError{
			Field:   "name",
			Message: "name exceeds maximum length",
		}
	}
	return nil
}

// ValidationError represents a validation failure
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// BodySizeLimitMiddleware limits request body size
func BodySizeLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxBytes {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "request body too large",
			})
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent MIME type sniffing
		c.Header("X-Content-Type-Options", "nosniff")
		// Prevent clickjacking
		c.Header("X-Frame-Options", "DENY")
		// Enable XSS filter
		c.Header("X-XSS-Protection", "1; mode=block")
		// Strict Transport Security (enable in production with HTTPS)
		// c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		
		c.Next()
	}
}

// RequestIDMiddleware adds request ID for tracing
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// generateRequestID creates a simple request ID
func generateRequestID() string {
	// Simple implementation - in production use UUID or similar
	return "req-" + randomString(16)
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
