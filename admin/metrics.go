package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// getMetrics returns aggregate request statistics per endpoint
func (h *Handler) getMetrics(c *gin.Context) {
	if h.metrics == nil {
		c.JSON(http.StatusOK, gin.H{
			"uptime_sec": 0,
			"endpoints":  []interface{}{},
		})
		return
	}

	stats := h.metrics.GetAll()
	c.JSON(http.StatusOK, gin.H{
		"uptime_sec": h.metrics.UptimeSeconds(),
		"endpoints":  stats,
	})
}
