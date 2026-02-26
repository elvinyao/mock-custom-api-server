package admin

import (
	"net/http"
	"os"

	"mock-api-server/config"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

// getConfig returns the current configuration as JSON
func (h *Handler) getConfig(c *gin.Context) {
	cfg := h.configManager.GetConfig()
	if cfg == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "configuration not loaded"})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// postConfigReload triggers a hot-reload of the configuration
func (h *Handler) postConfigReload(c *gin.Context) {
	path := h.configManager.GetConfigPath()
	cfg, err := config.LoadConfig(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.configManager.SetConfig(cfg)
	if h.onReload != nil {
		h.onReload()
	}
	c.JSON(http.StatusOK, gin.H{
		"message":         "Configuration reloaded",
		"endpoints_count": len(cfg.Endpoints),
	})
}

// writeEndpointToFile writes a single endpoint as YAML to the given file
func writeEndpointToFile(path string, ep config.Endpoint) error {
	data, err := yaml.Marshal([]config.Endpoint{ep})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
