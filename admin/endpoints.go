package admin

import (
	"net/http"
	"strconv"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
)

type endpointEntry struct {
	Index    int             `json:"index"`
	Source   string          `json:"source"` // "file" | "runtime"
	Endpoint config.Endpoint `json:"endpoint"`
}

// listEndpoints returns all registered endpoints (file-based + runtime)
func (h *Handler) listEndpoints(c *gin.Context) {
	cfg := h.configManager.GetConfig()
	var result []endpointEntry

	if cfg != nil {
		for i, ep := range cfg.Endpoints {
			result = append(result, endpointEntry{
				Index:    i,
				Source:   "file",
				Endpoint: ep,
			})
		}
	}

	runtimeEps := h.configManager.GetRuntimeEndpoints()
	for i, ep := range runtimeEps {
		result = append(result, endpointEntry{
			Index:    i,
			Source:   "runtime",
			Endpoint: ep,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"total":     len(result),
		"endpoints": result,
	})
}

// addEndpoint adds an endpoint at runtime (in-memory only)
func (h *Handler) addEndpoint(c *gin.Context) {
	var ep config.Endpoint
	if err := c.ShouldBindJSON(&ep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if ep.Path == "" || ep.Method == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path and method are required"})
		return
	}
	h.configManager.AddRuntimeEndpoint(ep)
	runtimeEps := h.configManager.GetRuntimeEndpoints()
	c.JSON(http.StatusCreated, gin.H{
		"message": "Endpoint added",
		"index":   len(runtimeEps) - 1,
	})
}

// updateEndpoint modifies a runtime endpoint by index
func (h *Handler) updateEndpoint(c *gin.Context) {
	idStr := c.Param("id")
	index, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var ep config.Endpoint
	if err := c.ShouldBindJSON(&ep); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.configManager.UpdateRuntimeEndpoint(index, ep) {
		c.JSON(http.StatusNotFound, gin.H{"error": "runtime endpoint not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint updated"})
}

// deleteEndpoint removes a runtime endpoint by index
func (h *Handler) deleteEndpoint(c *gin.Context) {
	idStr := c.Param("id")
	index, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if !h.configManager.RemoveRuntimeEndpoint(index) {
		c.JSON(http.StatusNotFound, gin.H{"error": "runtime endpoint not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint deleted"})
}
