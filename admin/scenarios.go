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

// resetScenario resets state for a given scenario name.
// If the query param ?partition_key=<value> is provided, only that specific
// partition is reset; otherwise all partitions for the scenario are cleared.
func (h *Handler) resetScenario(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scenario name required"})
		return
	}
	partitionKey := c.Query("partition_key")
	if h.stateStore != nil {
		if partitionKey != "" {
			h.stateStore.ResetPartition(name, partitionKey)
		} else {
			h.stateStore.ResetScenario(name)
		}
	}
	resp := gin.H{
		"message":  "Scenario reset",
		"scenario": name,
	}
	if partitionKey != "" {
		resp["partition_key"] = partitionKey
	}
	c.JSON(http.StatusOK, resp)
}
