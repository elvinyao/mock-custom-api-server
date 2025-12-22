package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Logger returns a gin middleware for logging requests
func Logger(logger *zap.Logger, accessLog bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !accessLog {
			c.Next()
			return
		}

		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get matched rule and response file from context
		matchedRule, _ := c.Get("matched_rule")
		responseFile, _ := c.Get("response_file")

		// Build log fields
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.Int("status", c.Writer.Status()),
			zap.Duration("latency", latency),
			zap.String("client_ip", c.ClientIP()),
			zap.Int("body_size", c.Writer.Size()),
		}

		if query != "" {
			fields = append(fields, zap.String("query", query))
		}

		if matchedRule != nil {
			fields = append(fields, zap.Any("matched_rule", matchedRule))
		}

		if responseFile != nil {
			fields = append(fields, zap.Any("response_file", responseFile))
		}

		// Log based on status code
		status := c.Writer.Status()
		switch {
		case status >= 500:
			logger.Error("Request completed", fields...)
		case status >= 400:
			logger.Warn("Request completed", fields...)
		default:
			logger.Info("Request completed", fields...)
		}
	}
}

// TextLogger returns a simple text-based logger for development
func TextLogger(accessLog bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !accessLog {
			c.Next()
			return
		}

		// Use default Gin logger
		gin.Logger()(c)
	}
}

// NewLogger creates a new zap logger based on configuration
func NewLogger(level, format, logFile string) (*zap.Logger, error) {
	var config zap.Config

	if format == "json" {
		config = zap.NewProductionConfig()
	} else {
		config = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch level {
	case "debug":
		config.Level.SetLevel(zap.DebugLevel)
	case "info":
		config.Level.SetLevel(zap.InfoLevel)
	case "warn":
		config.Level.SetLevel(zap.WarnLevel)
	case "error":
		config.Level.SetLevel(zap.ErrorLevel)
	default:
		config.Level.SetLevel(zap.InfoLevel)
	}

	// Set output paths
	if logFile != "" {
		config.OutputPaths = []string{"stdout", logFile}
		config.ErrorOutputPaths = []string{"stderr", logFile}
	}

	return config.Build()
}
