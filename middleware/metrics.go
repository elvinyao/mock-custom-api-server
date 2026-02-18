package middleware

import (
	"strings"
	"time"

	"mock-api-server/metrics"

	"github.com/gin-gonic/gin"
)

// Metrics returns a middleware that records per-endpoint request statistics
func Metrics(store *metrics.Store, excludePaths []string) gin.HandlerFunc {
	if store == nil {
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip excluded paths
		for _, excluded := range excludePaths {
			if strings.HasPrefix(path, excluded) {
				c.Next()
				return
			}
		}

		start := time.Now()
		c.Next()
		durationMs := time.Since(start).Milliseconds()

		store.Record(c.Request.Method, path, c.Writer.Status(), durationMs)
	}
}
