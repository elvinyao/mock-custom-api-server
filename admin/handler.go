package admin

import (
	"net"
	"net/http"
	"strings"
	"time"

	"mock-api-server/config"
	"mock-api-server/metrics"
	"mock-api-server/recorder"
	"mock-api-server/state"

	"github.com/gin-gonic/gin"
)

// Handler holds dependencies for the admin API
type Handler struct {
	configManager *config.ConfigManager
	recorder      *recorder.Recorder
	metrics       *metrics.Store
	stateStore    *state.ScenarioStore
	startTime     time.Time
}

// New creates a new admin Handler
func New(
	cfgManager *config.ConfigManager,
	rec *recorder.Recorder,
	metricsStore *metrics.Store,
	stateStore *state.ScenarioStore,
) *Handler {
	return &Handler{
		configManager: cfgManager,
		recorder:      rec,
		metrics:       metricsStore,
		stateStore:    stateStore,
		startTime:     time.Now(),
	}
}

// ipAllowlist returns a middleware that restricts access to the listed CIDR/IP
// ranges. If allowedIPs is empty the middleware is a no-op.
func ipAllowlist(allowedIPs []string) gin.HandlerFunc {
	if len(allowedIPs) == 0 {
		return func(c *gin.Context) { c.Next() }
	}
	var nets []*net.IPNet
	var addrs []net.IP
	for _, raw := range allowedIPs {
		if strings.Contains(raw, "/") {
			_, ipNet, err := net.ParseCIDR(raw)
			if err == nil {
				nets = append(nets, ipNet)
			}
		} else {
			if ip := net.ParseIP(raw); ip != nil {
				addrs = append(addrs, ip)
			}
		}
	}
	return func(c *gin.Context) {
		remoteIP := net.ParseIP(c.ClientIP())
		if remoteIP == nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		for _, ip := range addrs {
			if ip.Equal(remoteIP) {
				c.Next()
				return
			}
		}
		for _, ipNet := range nets {
			if ipNet.Contains(remoteIP) {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	}
}

// RegisterRoutes mounts the admin API under the given prefix
func (h *Handler) RegisterRoutes(r *gin.Engine, prefix string, auth config.AdminAuth) {
	group := r.Group(prefix)

	// IP allowlist (checked before auth)
	if len(auth.AllowedIPs) > 0 {
		group.Use(ipAllowlist(auth.AllowedIPs))
	}

	// Optionally apply basic auth
	if auth.Enabled && auth.Username != "" {
		group.Use(gin.BasicAuth(gin.Accounts{
			auth.Username: auth.Password,
		}))
	}

	// Health / info
	group.GET("/health", h.getHealth)
	group.GET("/config", h.getConfig)
	group.POST("/config/reload", h.postConfigReload)

	// Endpoints management
	group.GET("/endpoints", h.listEndpoints)
	group.POST("/endpoints", h.addEndpoint)
	group.PUT("/endpoints/:id", h.updateEndpoint)
	group.DELETE("/endpoints/:id", h.deleteEndpoint)

	// Request history
	group.GET("/requests", h.listRequests)
	group.DELETE("/requests", h.clearRequests)
	group.GET("/requests/:id", h.getRequest)

	// Scenarios
	group.GET("/scenarios", h.listScenarios)
	group.POST("/scenarios/:name/reset", h.resetScenario)

	// Metrics
	group.GET("/metrics", h.getMetrics)
}

// getHealth returns server health and uptime
func (h *Handler) getHealth(c *gin.Context) {
	cfg := h.configManager.GetConfig()
	endpointsCount := 0
	if cfg != nil {
		endpointsCount = len(cfg.Endpoints)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"uptime_sec": time.Since(h.startTime).Seconds(),
		"config": gin.H{
			"loaded_at":       h.configManager.GetLoadedAt().Format(time.RFC3339),
			"endpoints_count": endpointsCount,
		},
	})
}
