package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"skeenode/pkg/auth"
)

const (
	// AuthHeaderKey is the standard Authorization header
	AuthHeaderKey = "Authorization"
	// APIKeyHeaderKey is the custom API key header
	APIKeyHeaderKey = "X-API-Key"
	// ContextUserKey is the key used to store user claims in context
	ContextUserKey = "user"
	// ContextRequestIDKey is the key used to store request ID
	ContextRequestIDKey = "request_id"
)

// AuthConfig holds authentication middleware configuration
type AuthConfig struct {
	JWTService  *auth.JWTService
	APIKeyStore auth.APIKeyStore
	SkipPaths   []string // Paths that don't require authentication
}

// AuthMiddleware creates a middleware that validates JWT or API key authentication
func AuthMiddleware(config AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if path should skip authentication
		for _, path := range config.SkipPaths {
			if matchPath(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Try JWT first
		if claims := tryJWTAuth(c, config.JWTService); claims != nil {
			setUserContext(c, claims)
			c.Next()
			return
		}

		// Try API key
		if claims := tryAPIKeyAuth(c, config.APIKeyStore); claims != nil {
			setUserContext(c, claims)
			c.Next()
			return
		}

		// No valid authentication found
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "authentication required",
			"hint":  "provide Bearer token or X-API-Key header",
		})
	}
}

// tryJWTAuth attempts to authenticate via JWT Bearer token
func tryJWTAuth(c *gin.Context, jwtService *auth.JWTService) *auth.Claims {
	if jwtService == nil {
		return nil
	}

	authHeader := c.GetHeader(AuthHeaderKey)
	if authHeader == "" {
		return nil
	}

	// Expect "Bearer <token>"
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return nil
	}

	claims, err := jwtService.ValidateToken(parts[1])
	if err != nil {
		return nil
	}

	return claims
}

// tryAPIKeyAuth attempts to authenticate via API key
func tryAPIKeyAuth(c *gin.Context, store auth.APIKeyStore) *auth.Claims {
	if store == nil {
		return nil
	}

	apiKey := c.GetHeader(APIKeyHeaderKey)
	if apiKey == "" {
		return nil
	}

	info, err := store.ValidateKey(c.Request.Context(), apiKey)
	if err != nil {
		return nil
	}

	// Convert API key info to claims
	return &auth.Claims{
		UserID:   info.OwnerID,
		Username: info.Name,
		Role:     info.Role,
		OrgID:    info.OrgID,
	}
}

// setUserContext stores user claims in the request context
func setUserContext(c *gin.Context, claims *auth.Claims) {
	c.Set(ContextUserKey, claims)
}

// GetUserFromContext retrieves user claims from the request context
func GetUserFromContext(c *gin.Context) (*auth.Claims, bool) {
	value, exists := c.Get(ContextUserKey)
	if !exists {
		return nil, false
	}
	claims, ok := value.(*auth.Claims)
	return claims, ok
}

// RequireRole creates a middleware that requires a minimum role level
func RequireRole(required auth.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := GetUserFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		if !claims.Role.HasPermission(required) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":    "insufficient permissions",
				"required": required,
				"current":  claims.Role,
			})
			return
		}

		c.Next()
	}
}

// RequireOwnership checks that the user owns the resource or is an admin
func RequireOwnership(getOwnerID func(c *gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := GetUserFromContext(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "authentication required",
			})
			return
		}

		// Admins can access any resource
		if claims.Role.HasPermission(auth.RoleAdmin) {
			c.Next()
			return
		}

		// Check ownership
		ownerID := getOwnerID(c)
		if ownerID != claims.UserID {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "you do not own this resource",
			})
			return
		}

		c.Next()
	}
}

// matchPath checks if a request path matches a pattern
// Supports wildcards: /api/* matches /api/anything
func matchPath(path, pattern string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}
	return path == pattern
}

// OptionalAuth is middleware that extracts user info if present but doesn't require it
func OptionalAuth(config AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try JWT
		if claims := tryJWTAuth(c, config.JWTService); claims != nil {
			setUserContext(c, claims)
		} else if claims := tryAPIKeyAuth(c, config.APIKeyStore); claims != nil {
			// Try API key
			setUserContext(c, claims)
		}
		c.Next()
	}
}
