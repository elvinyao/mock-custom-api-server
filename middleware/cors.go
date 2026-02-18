package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware that sets CORS headers based on configuration
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	if !cfg.Enabled {
		return func(c *gin.Context) { c.Next() }
	}

	// Set defaults
	allowedOrigins := cfg.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	allowedMethods := cfg.AllowedMethods
	if len(allowedMethods) == 0 {
		allowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"}
	}
	allowedHeaders := cfg.AllowedHeaders
	if len(allowedHeaders) == 0 {
		allowedHeaders = []string{"Content-Type", "Authorization", "X-Request-ID"}
	}

	allowedMethodsStr := strings.Join(allowedMethods, ", ")
	allowedHeadersStr := strings.Join(allowedHeaders, ", ")
	exposedHeadersStr := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := ""
	if cfg.MaxAgeSeconds > 0 {
		maxAge = strconv.Itoa(cfg.MaxAgeSeconds)
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// Determine the allowed origin header value
		allowOrigin := ""
		for _, o := range allowedOrigins {
			if o == "*" {
				if cfg.AllowCredentials {
					// Cannot use * with credentials; echo the specific origin
					allowOrigin = origin
				} else {
					allowOrigin = "*"
				}
				break
			} else if o == origin {
				allowOrigin = origin
				break
			}
		}

		if allowOrigin == "" {
			// Origin not allowed; skip CORS headers but continue
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", allowOrigin)
		c.Header("Access-Control-Allow-Methods", allowedMethodsStr)
		c.Header("Access-Control-Allow-Headers", allowedHeadersStr)
		if exposedHeadersStr != "" {
			c.Header("Access-Control-Expose-Headers", exposedHeadersStr)
		}
		if cfg.AllowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		if maxAge != "" {
			c.Header("Access-Control-Max-Age", maxAge)
		}

		// Handle preflight
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
