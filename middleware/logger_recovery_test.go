package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ── Logger ────────────────────────────────────────────────────────────────────

func TestLogger_AccessLogDisabled_PassesThrough(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Logger(logger, false))
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestLogger_AccessLogEnabled_200(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Logger(logger, true))
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	req, _ := http.NewRequest("GET", "/ok?q=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestLogger_AccessLogEnabled_400(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Logger(logger, true))
	r.GET("/bad", func(c *gin.Context) {
		c.JSON(http.StatusBadRequest, gin.H{})
	})

	req, _ := http.NewRequest("GET", "/bad", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestLogger_AccessLogEnabled_500(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Logger(logger, true))
	r.GET("/err", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{})
	})

	req, _ := http.NewRequest("GET", "/err", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestLogger_WithMatchedRuleAndResponseFile(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Logger(logger, true))
	r.GET("/match", func(c *gin.Context) {
		c.Set("matched_rule", "rule_1")
		c.Set("response_file", "resp.json")
		c.JSON(http.StatusOK, gin.H{})
	})

	req, _ := http.NewRequest("GET", "/match", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── TextLogger ────────────────────────────────────────────────────────────────

func TestTextLogger_AccessLogDisabled(t *testing.T) {
	r := gin.New()
	r.Use(TextLogger(false))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestTextLogger_AccessLogEnabled(t *testing.T) {
	r := gin.New()
	r.Use(TextLogger(true))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req, _ := http.NewRequest("GET", "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── NewLogger ─────────────────────────────────────────────────────────────────

func TestNewLogger_JSONFormat(t *testing.T) {
	logger, err := NewLogger("info", "json", "")
	if err != nil {
		t.Fatalf("NewLogger(json) error: %v", err)
	}
	if logger == nil {
		t.Fatal("logger is nil")
	}
	logger.Sync()
}

func TestNewLogger_TextFormat(t *testing.T) {
	logger, err := NewLogger("debug", "text", "")
	if err != nil {
		t.Fatalf("NewLogger(text) error: %v", err)
	}
	if logger == nil {
		t.Fatal("logger is nil")
	}
	logger.Sync()
}

func TestNewLogger_AllLevels(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error", "unknown"} {
		logger, err := NewLogger(level, "json", "")
		if err != nil {
			t.Errorf("NewLogger(%q) error: %v", level, err)
			continue
		}
		if logger == nil {
			t.Errorf("NewLogger(%q) returned nil", level)
		}
		logger.Sync()
	}
}

// ── Recovery ─────────────────────────────────────────────────────────────────

func TestRecovery_NoPanic_PassesThrough(t *testing.T) {
	logger := zaptest.NewLogger(t)
	r := gin.New()
	r.Use(Recovery(logger, false))
	r.GET("/safe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req, _ := http.NewRequest("GET", "/safe", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRecovery_PanicWithoutDetails_500(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	r := gin.New()
	r.Use(Recovery(logger, false))
	r.GET("/panic", func(c *gin.Context) {
		panic("something went wrong")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestRecovery_PanicWithDetails_500(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	r := gin.New()
	r.Use(Recovery(logger, true))
	r.GET("/panic", func(c *gin.Context) {
		panic("detailed error")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// ── SimpleRecovery ────────────────────────────────────────────────────────────

func TestSimpleRecovery_NoPanic(t *testing.T) {
	r := gin.New()
	r.Use(SimpleRecovery(false))
	r.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{})
	})

	req, _ := http.NewRequest("GET", "/ok", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestSimpleRecovery_Panic_500(t *testing.T) {
	r := gin.New()
	r.Use(SimpleRecovery(false))
	r.GET("/panic", func(c *gin.Context) {
		panic("oops")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestSimpleRecovery_PanicWithDetails(t *testing.T) {
	r := gin.New()
	r.Use(SimpleRecovery(true))
	r.GET("/panic", func(c *gin.Context) {
		panic("show me")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}
