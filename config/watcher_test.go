package config

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

var noopLogger = log.New(io.Discard, "", 0)

// ── NewWatcher ────────────────────────────────────────────────────────────────

func TestNewWatcher_NotNil(t *testing.T) {
	cm := NewConfigManager("/config.yaml")
	w := NewWatcher("/config.yaml", cm, noopLogger)
	if w == nil {
		t.Fatal("NewWatcher returned nil")
	}
	if w.configPath != "/config.yaml" {
		t.Errorf("configPath = %q, want /config.yaml", w.configPath)
	}
}

// ── Start / Stop ──────────────────────────────────────────────────────────────

func TestWatcher_StartStop(t *testing.T) {
	// Create a real temp config file so fsnotify can watch it
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)

	w.Start(5)
	// Give goroutine a moment to start
	time.Sleep(20 * time.Millisecond)
	// Stop should not hang or panic
	w.Stop()
	// Give goroutine time to stop
	time.Sleep(20 * time.Millisecond)
}

// ── reloadConfig ──────────────────────────────────────────────────────────────

func TestReloadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 9000\n"), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)
	w.reloadConfig(nil, nil)

	cfg := cm.GetConfig()
	if cfg == nil {
		t.Fatal("config should be set after reload")
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %d, want 9000", cfg.Port)
	}
}

func TestReloadConfig_InvalidFile_KeepsOldConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 7777\n"), 0644)

	cm := NewConfigManager(cfgFile)
	// Load initial good config
	cfg, _ := LoadConfig(cfgFile)
	cm.SetConfig(cfg)

	// Now corrupt the file
	os.WriteFile(cfgFile, []byte("{ invalid yaml ["), 0644)

	w := NewWatcher(cfgFile, cm, noopLogger)
	w.reloadConfig(nil, nil) // should log error and keep old config

	current := cm.GetConfig()
	if current == nil {
		t.Fatal("config should not be nil after failed reload")
	}
	if current.Port != 7777 {
		t.Errorf("Port = %d, want 7777 (old config kept)", current.Port)
	}
}

func TestReloadConfig_WithValidationWarnings(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	// Config with empty endpoint path triggers validation warning
	yaml := `port: 8080
endpoints:
  - method: "GET"
`
	os.WriteFile(cfgFile, []byte(yaml), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)
	// Should not panic even with validation warnings
	w.reloadConfig(nil, nil)
	if cm.GetConfig() == nil {
		t.Fatal("config should be set even if there are warnings")
	}
}

// ── watchEndpointConfigFiles ──────────────────────────────────────────────────

func TestWatchEndpointConfigFiles_NilWatcher(t *testing.T) {
	cm := NewConfigManager("")
	w := NewWatcher("", cm, noopLogger)
	// Should not panic
	w.watchEndpointConfigFiles(nil, nil, &Config{})
}

func TestWatchEndpointConfigFiles_NilPaths(t *testing.T) {
	cm := NewConfigManager("")
	w := NewWatcher("", cm, noopLogger)
	// Nil watchedPaths: should not panic
	w.watchEndpointConfigFiles(nil, nil, nil)
}

func TestWatchEndpointConfigFiles_NilConfig(t *testing.T) {
	cm := NewConfigManager("")
	w := NewWatcher("", cm, noopLogger)
	w.watchEndpointConfigFiles(nil, make(map[string]struct{}), nil)
}

// ── watchWithPolling ──────────────────────────────────────────────────────────

func TestWatchWithPolling_StopsOnClose(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)

	done := make(chan struct{})
	go func() {
		w.watchWithPolling(100) // 100s interval but we stop it
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	w.Stop()

	select {
	case <-done:
		// Good: goroutine exited
	case <-time.After(500 * time.Millisecond):
		t.Error("watchWithPolling did not stop within timeout")
	}
}
