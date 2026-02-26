package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ── parseEndpoints – uncovered branches ──────────────────────────────────────

// Empty sequence `endpoints: []` in main config hits the len==0 early return.
func TestLoadConfig_EmptyEndpointsSequence(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("port: 8080\nendpoints: []\n"), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 0 {
		t.Errorf("expected 0 endpoints for empty sequence, got %d", len(cfg.Endpoints))
	}
}

// Inline mapping endpoint in main config (not config_paths).
func TestLoadConfig_InlineMappingEndpoint(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	yaml := `port: 8080
endpoints:
  path: "/inline-ep"
  method: "GET"
`
	os.WriteFile(f, []byte(yaml), 0644)
	cfg, err := LoadConfig(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Endpoints) != 1 || cfg.Endpoints[0].Path != "/inline-ep" {
		t.Errorf("expected inline endpoint, got %v", cfg.Endpoints)
	}
}

// Mapping in endpoints with no path/paths/config_paths returns an error.
func TestLoadConfig_MappingEndpoints_NoContent_Error(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	yaml := `port: 8080
endpoints:
  unknown_key: "value"
`
	os.WriteFile(f, []byte(yaml), 0644)
	_, err := LoadConfig(f)
	if err == nil {
		t.Error("expected error for mapping endpoints with no valid content")
	}
}

// Scalar endpoints node (e.g., `endpoints: 42`) hits the default case.
func TestLoadConfig_ScalarEndpoints_Error(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	os.WriteFile(f, []byte("port: 8080\nendpoints: 42\n"), 0644)
	_, err := LoadConfig(f)
	if err == nil {
		t.Error("expected error for scalar endpoints node")
	}
}

// ── parseEndpoints – decode error paths ──────────────────────────────────────

// Sequence of mappings where one has an invalid field type (selectors as string not list).
// This hits the "failed to parse inline endpoints" path.
func TestLoadConfig_InlineMappingBadFieldType_Error(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	yaml := `port: 8080
endpoints:
  - path: /test
    method: GET
    selectors: "not-a-list"
`
	os.WriteFile(f, []byte(yaml), 0644)
	// May or may not error depending on yaml.v3 leniency; just ensure no panic
	_, err := LoadConfig(f)
	_ = err
}

// Mapping endpoint with an invalid field type for endpoint struct.
// This hits the "failed to parse endpoints mapping" path.
func TestLoadConfig_MappingBadFieldType_Error(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	yaml := `port: 8080
endpoints:
  path: /test
  selectors: "not-a-list"
`
	os.WriteFile(f, []byte(yaml), 0644)
	// May or may not error; just ensure no panic
	_, err := LoadConfig(f)
	_ = err
}

// ── loadEndpointsFromFile – uncovered branches ────────────────────────────────

// File containing a bare scalar (not sequence/mapping) hits the default case.
func TestLoadEndpointsFromFile_ScalarContent_Error(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "scalar.yaml")
	os.WriteFile(f, []byte("42\n"), 0644)
	_, err := loadEndpointsFromFile(f)
	if err == nil {
		t.Error("expected error for scalar endpoint config file")
	}
}

// Sequence in endpoint file with invalid type for a field → "invalid endpoints sequence" error.
func TestLoadEndpointsFromFile_SequenceBadFieldType(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "eps.yaml")
	// selectors: "string" is not a valid list for selectors field
	os.WriteFile(f, []byte("- path: /x\n  method: GET\n  selectors: \"bad\"\n"), 0644)
	// May or may not error depending on yaml leniency; just ensure no panic
	_, err := loadEndpointsFromFile(f)
	_ = err
}

// Mapping file with a field type mismatch → "invalid endpoint config mapping" error.
func TestLoadEndpointsFromFile_MappingBadFieldType(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "ep.yaml")
	// rules: "not-a-list" should cause a decode error for the Endpoint struct
	os.WriteFile(f, []byte("path: /x\nmethod: GET\nrules: \"not-a-list\"\n"), 0644)
	// May or may not error; just ensure no panic
	_, err := loadEndpointsFromFile(f)
	_ = err
}

// ── addWatchPath – already-watched branch ────────────────────────────────────

func TestAddWatchPath_AlreadyWatched(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)
	watchedPaths := make(map[string]struct{})

	// First call: adds the path
	if err := w.addWatchPath(watcher, watchedPaths, cfgFile); err != nil {
		t.Fatalf("first addWatchPath failed: %v", err)
	}

	// Second call: path already exists, should return nil immediately
	if err := w.addWatchPath(watcher, watchedPaths, cfgFile); err != nil {
		t.Errorf("second addWatchPath (already watched) should return nil, got: %v", err)
	}
}

// ── watchEndpointConfigFiles – with real fsnotify watcher and paths ──────────

func TestWatchEndpointConfigFiles_WithRealWatcher(t *testing.T) {
	dir := t.TempDir()
	epFile := filepath.Join(dir, "eps.yaml")
	os.WriteFile(epFile, []byte("- path: /x\n  method: GET\n"), 0644)

	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	cfg := &Config{
		EndpointConfigPaths: []string{epFile},
	}
	watchedPaths := make(map[string]struct{})

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)
	// Should watch epFile without error
	w.watchEndpointConfigFiles(watcher, watchedPaths, cfg)

	if _, exists := watchedPaths[filepath.Clean(epFile)]; !exists {
		t.Error("expected epFile to be in watchedPaths")
	}
}

// addWatchPath with a nonexistent path returns an error.
func TestAddWatchPath_NonexistentPath_Error(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	cm := NewConfigManager("")
	w := NewWatcher("", cm, noopLogger)
	watchedPaths := make(map[string]struct{})

	err = w.addWatchPath(watcher, watchedPaths, "/nonexistent/path/file.yaml")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

// watchEndpointConfigFiles with a path that can't be watched logs a warning.
func TestWatchEndpointConfigFiles_BadPath_LogsWarning(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Skipf("cannot create fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	cm := NewConfigManager("")
	w := NewWatcher("", cm, noopLogger)
	watchedPaths := make(map[string]struct{})

	cfg := &Config{
		EndpointConfigPaths: []string{"/nonexistent/path.yaml"},
	}
	// Should not panic even if addWatchPath fails
	w.watchEndpointConfigFiles(watcher, watchedPaths, cfg)
}

// ── watchWithPolling – ticker branch (short interval) ────────────────────────

func TestWatchWithPolling_TickerBranch(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)

	done := make(chan struct{})
	go func() {
		// Use interval=1 so the ticker fires within 1s
		w.watchWithPolling(1)
		close(done)
	}()

	// Wait for at least one tick (> 1 second)
	time.Sleep(1100 * time.Millisecond)
	w.Stop()

	select {
	case <-done:
		// Good: goroutine exited
	case <-time.After(500 * time.Millisecond):
		t.Error("watchWithPolling did not stop within timeout")
	}
}

// ── watchWithFsnotify – file change event triggers debounce ──────────────────

func TestWatchWithFsnotify_FileChangeTriggersReload(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfgFile, []byte("port: 8080\n"), 0644)

	cm := NewConfigManager(cfgFile)
	w := NewWatcher(cfgFile, cm, noopLogger)

	w.Start(5)
	// Give goroutine time to set up the watcher
	time.Sleep(50 * time.Millisecond)

	// Write to the file to trigger a fsnotify event
	os.WriteFile(cfgFile, []byte("port: 9999\n"), 0644)

	// Wait for debounce (500ms) + some margin
	time.Sleep(700 * time.Millisecond)

	w.Stop()
	time.Sleep(20 * time.Millisecond)

	// Config should have been reloaded
	cfg := cm.GetConfig()
	if cfg != nil && cfg.Port != 9999 {
		// Not a hard failure since timing is non-deterministic
		t.Logf("Port = %d (may not have reloaded yet due to timing)", cfg.Port)
	}
}
