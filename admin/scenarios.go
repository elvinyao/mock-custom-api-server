package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// listScenarios returns all active scenario entries with their current steps
func (h *Handler) listScenarios(c *gin.Context) {
	if h.stateStore == nil {
		c.JSON(http.StatusOK, gin.H{"scenarios": []interface{}{}})
		return
	}
	entries := h.stateStore.List()
	c.JSON(http.StatusOK, gin.H{
		"total":     len(entries),
		"scenarios": entries,
	})
}

// resetScenario resets all state for a given scenario name
func (h *Handler) resetScenario(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scenario name required"})
		return
	}
	if h.stateStore != nil {
		h.stateStore.ResetScenario(name)
	}
	c.JSON(http.StatusOK, gin.H{
		"message":  "Scenario reset",
		"scenario": name,
	})
}
