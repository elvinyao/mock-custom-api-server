package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"mock-api-server/admin"
	"mock-api-server/config"
	"mock-api-server/handler"
	"mock-api-server/metrics"
	"mock-api-server/middleware"
	"mock-api-server/recorder"
	"mock-api-server/state"
	"mock-api-server/ui"

	"github.com/gin-gonic/gin"
)

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Create logger for startup
	startupLogger := log.New(os.Stdout, "[STARTUP] ", log.LstdFlags)

	// Load configuration
	startupLogger.Printf("Loading configuration from: %s", *configPath)
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		startupLogger.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	warnings := config.ValidateConfig(cfg)
	for _, warn := range warnings {
		startupLogger.Printf("[WARN] %s", warn)
	}

	// Create config manager
	cfgManager := config.NewConfigManager(*configPath)
	cfgManager.SetConfig(cfg)

	// Create zap logger
	zapLogger, err := middleware.NewLogger(
		cfg.Server.Logging.Level,
		cfg.Server.Logging.LogFormat,
		cfg.Server.Logging.LogFile,
	)
	if err != nil {
		startupLogger.Printf("[WARN] Failed to create zap logger, using default: %v", err)
	}

	// Set Gin mode based on log level
	if cfg.Server.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// ── New subsystems ──────────────────────────────────────────────────────

	// Shared state store (scenarios)
	stateStore := state.New()

	// Metrics store
	metricsStore := metrics.New()

	// Request recorder (if enabled)
	var rec *recorder.Recorder
	recordingCfg := cfg.Server.Recording
	if recordingCfg.Enabled {
		maxEntries := recordingCfg.MaxEntries
		if maxEntries <= 0 {
			maxEntries = 1000
		}
		rec = recorder.New(maxEntries)
		startupLogger.Printf("Request recording enabled (max %d entries)", maxEntries)
	}

	// ── Router setup ────────────────────────────────────────────────────────

	router := gin.New()

	// 1. CORS (must be first)
	if cfg.Server.CORS.Enabled {
		router.Use(middleware.CORS(cfg.Server.CORS))
		startupLogger.Printf("CORS enabled")
	}

	// 2. Logging / Recovery
	if zapLogger != nil {
		router.Use(middleware.Logger(zapLogger, cfg.Server.Logging.AccessLog))
		router.Use(middleware.Recovery(zapLogger, cfg.Server.ErrorHandling.ShowDetails))
	} else {
		router.Use(gin.Logger())
		router.Use(gin.Recovery())
	}

	// 3. Metrics collection
	excludePaths := buildExcludePaths(cfg)
	router.Use(middleware.Metrics(metricsStore, excludePaths))

	// 4. Request recording
	if rec != nil {
		maxBodyBytes := recordingCfg.MaxBodyBytes
		if maxBodyBytes <= 0 {
			maxBodyBytes = 65536
		}
		router.Use(middleware.RequestRecorder(rec, recordingCfg.RecordBody, maxBodyBytes, excludePaths))
	}

	// ── Health check ────────────────────────────────────────────────────────
	if cfg.HealthCheck.Enabled {
		healthPath := cfg.HealthCheck.Path
		if healthPath == "" {
			healthPath = "/health"
		}
		router.GET(healthPath, handler.HealthHandler(cfgManager))
		startupLogger.Printf("Health check endpoint registered at: %s", healthPath)
	}

	// ── Admin API ───────────────────────────────────────────────────────────
	adminCfg := cfg.Server.AdminAPI
	adminPrefix := "/mock-admin"
	if adminCfg.Prefix != "" {
		adminPrefix = adminCfg.Prefix
	}

	if adminCfg.Enabled {
		adminHandler := admin.New(cfgManager, rec, metricsStore, stateStore)
		adminHandler.RegisterRoutes(router, adminPrefix, adminCfg.Auth)
		startupLogger.Printf("Admin API enabled at: %s", adminPrefix)

		// Web UI under admin prefix
		ui.RegisterRoutes(router, adminPrefix+"/ui")
		startupLogger.Printf("Web UI available at: %s/ui/", adminPrefix)
	}

	// ── Mock handler ────────────────────────────────────────────────────────
	mockHandler := handler.NewMockHandlerWithState(cfgManager, stateStore)
	mockHandler.RegisterRoutes(router)

	// ── Config watcher (hot reload) ─────────────────────────────────────────
	if cfg.Server.HotReload {
		stdLogger := log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)
		watcher := config.NewWatcher(*configPath, cfgManager, stdLogger)
		watcher.OnReload = handler.ClearFileCache
		watcher.Start(cfg.Server.ReloadIntervalSec)
		defer watcher.Stop()
		startupLogger.Printf("Hot reload enabled, watching: %s", *configPath)
	}

	// ── Start server ────────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	startupLogger.Printf("Starting Mock API Server on %s", addr)
	startupLogger.Printf("Loaded %d endpoint(s)", len(cfg.Endpoints))

	tlsCfg := cfg.Server.TLS
	if tlsCfg.Enabled {
		if tlsCfg.CertFile == "" || tlsCfg.KeyFile == "" {
			startupLogger.Fatalf("TLS enabled but cert_file or key_file is missing")
		}
		startupLogger.Printf("TLS enabled (cert: %s)", tlsCfg.CertFile)
		if err := router.RunTLS(addr, tlsCfg.CertFile, tlsCfg.KeyFile); err != nil {
			startupLogger.Fatalf("Failed to start TLS server: %v", err)
		}
	} else {
		if err := router.Run(addr); err != nil {
			startupLogger.Fatalf("Failed to start server: %v", err)
		}
	}
}

// buildExcludePaths returns paths that should be excluded from metrics/recording.
// Falls back to sensible defaults derived from the actual config values so that
// a custom admin prefix or health path is correctly excluded.
func buildExcludePaths(cfg *config.Config) []string {
	exclude := cfg.Server.Recording.ExcludePaths
	if len(exclude) == 0 {
		adminPrefix := cfg.Server.AdminAPI.Prefix
		if adminPrefix == "" {
			adminPrefix = "/mock-admin"
		}
		healthPath := cfg.HealthCheck.Path
		if healthPath == "" {
			healthPath = "/health"
		}
		exclude = []string{healthPath, adminPrefix + "/"}
	}
	return exclude
}
