package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"mock-api-server/config"
	"mock-api-server/handler"
	"mock-api-server/middleware"

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

	// Create Gin router
	router := gin.New()

	// Add middleware
	if zapLogger != nil {
		router.Use(middleware.Logger(zapLogger, cfg.Server.Logging.AccessLog))
		router.Use(middleware.Recovery(zapLogger, cfg.Server.ErrorHandling.ShowDetails))
	} else {
		router.Use(gin.Logger())
		router.Use(gin.Recovery())
	}

	// Register health check endpoint if enabled
	if cfg.HealthCheck.Enabled {
		healthPath := cfg.HealthCheck.Path
		if healthPath == "" {
			healthPath = "/health"
		}
		router.GET(healthPath, handler.HealthHandler(cfgManager))
		startupLogger.Printf("Health check endpoint registered at: %s", healthPath)
	}

	// Create and register mock handler
	mockHandler := handler.NewMockHandler(cfgManager)
	mockHandler.RegisterRoutes(router)

	// Start config watcher if hot reload is enabled
	if cfg.Server.HotReload {
		stdLogger := log.New(os.Stdout, "[CONFIG] ", log.LstdFlags)
		watcher := config.NewWatcher(*configPath, cfgManager, stdLogger)
		watcher.Start(cfg.Server.ReloadIntervalSec)
		defer watcher.Stop()
		startupLogger.Printf("Hot reload enabled, watching: %s", *configPath)
	}

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	startupLogger.Printf("Starting Mock API Server on %s", addr)
	startupLogger.Printf("Loaded %d endpoint(s)", len(cfg.Endpoints))

	if err := router.Run(addr); err != nil {
		startupLogger.Fatalf("Failed to start server: %v", err)
	}
}
