package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Recovery returns a gin middleware for recovering from panics
func Recovery(logger *zap.Logger, showDetails bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic
				logger.Error("Panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				// Build response
				response := gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
				}

				if showDetails {
					response["error"].(gin.H)["details"] = err
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			}
		}()

		c.Next()
	}
}

// SimpleRecovery returns a simple recovery middleware without zap
func SimpleRecovery(showDetails bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				response := gin.H{
					"error": gin.H{
						"code":    "INTERNAL_ERROR",
						"message": "An unexpected error occurred",
					},
				}

				if showDetails {
					response["error"].(gin.H)["details"] = err
				}

				c.AbortWithStatusJSON(http.StatusInternalServerError, response)
			}
		}()

		c.Next()
	}
}
