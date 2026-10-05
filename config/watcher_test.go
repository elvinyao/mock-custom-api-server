package config

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func TestReloadConfig_WithValidationErrors_KeepsOldConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	// Config with empty endpoint path triggers validation warning
	yaml := `port: 8080
endpoints:
  - method: "GET"
`
	os.WriteFile(cfgFile, []byte(yaml), 0644)

	cm := NewConfigManager(cfgFile)
	old := &Config{Port: 7777}
	cm.SetConfig(old)
	w := NewWatcher(cfgFile, cm, noopLogger)
	// A syntactically valid file with an invalid endpoint must not activate.
	w.reloadConfig(nil, nil)
	if cm.GetConfig() != old {
		t.Fatal("invalid config replaced the last known good configuration")
	}
}

func TestWatcher_AtomicReplacementTwiceAndUnrelatedFilesIgnored(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "config.yaml")
	epPath := filepath.Join(dir, "endpoint.yaml")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write(mainPath, "port: 8080\nendpoints: [endpoint.yaml]\n")
	write(epPath, "path: /first\nmethod: GET\n")
	cfg, err := LoadConfig(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	cm := NewConfigManager(mainPath)
	cm.SetConfig(cfg)
	w := NewWatcher(mainPath, cm, noopLogger)
	reloads := make(chan struct{}, 10)
	w.OnReload = func() { reloads <- struct{}{} }
	w.Start(1)
	defer w.Stop()
	time.Sleep(100 * time.Millisecond)
	for _, path := range []string{"/second", "/third"} {
		write(epPath+".tmp", "path: "+path+"\nmethod: GET\n")
		if err := os.Rename(epPath+".tmp", epPath); err != nil {
			t.Fatal(err)
		}
		select {
		case <-reloads:
		case <-time.After(3 * time.Second):
			t.Fatal("atomic replacement did not reload")
		}
		if got := cm.GetConfig().Endpoints[0].Path; got != path {
			t.Fatalf("path=%s, want %s", got, path)
		}
	}
	write(filepath.Join(dir, "unrelated.yaml"), "not a config")
	select {
	case <-reloads:
		t.Fatal("unrelated file triggered reload")
	case <-time.After(700 * time.Millisecond):
	}
}

func TestWatcher_StopCancelsPendingReloadAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("port: 8080\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cm := NewConfigManager(path)
	old := &Config{Port: 8080}
	cm.SetConfig(old)
	w := NewWatcher(path, cm, noopLogger)
	w.Start(1)
	time.Sleep(100 * time.Millisecond)
	if err := os.WriteFile(path, []byte("port: 9000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	w.Stop()
	time.Sleep(600 * time.Millisecond)
	if cm.GetConfig() != old {
		t.Error("config reloaded after Stop")
	}
	w.Stop()
}

func TestWatcher_FallbackHonorsConfiguredInterval(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "initially-missing")
	path := filepath.Join(dir, "config.yaml")
	cm := NewConfigManager(path)
	w := NewWatcher(path, cm, noopLogger)
	reloaded := make(chan struct{}, 2)
	w.OnReload = func() { reloaded <- struct{}{} }
	w.Start(1)
	defer w.Stop()
	// An absent parent forces the fsnotify setup to use polling.
	time.Sleep(100 * time.Millisecond)
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("port: 9000\n"), 0644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-reloaded:
		if cm.GetConfig().Port != 9000 {
			t.Fatal("unexpected polling result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fallback ignored the one-second reload interval")
	}
}

type watcherLogEvents chan string

func (events watcherLogEvents) Write(p []byte) (int, error) {
	select {
	case events <- string(p):
	default:
	}
	return len(p), nil
}

func TestWatcher_RecoversWhenNewEndpointFileIsFixed(t *testing.T) {
	for _, initiallyMissing := range []bool{true, false} {
		name := "invalid"
		if initiallyMissing {
			name = "missing"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			mainPath := filepath.Join(dir, "config.yaml")
			newPath := filepath.Join(dir, "new", "endpoint.yaml")
			write := func(path, content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write(mainPath, "endpoints:\n  - path: /old\n    method: GET\n")
			old, err := LoadValidatedConfig(mainPath)
			if err != nil {
				t.Fatal(err)
			}
			if !initiallyMissing {
				if err := os.Mkdir(filepath.Dir(newPath), 0755); err != nil {
					t.Fatal(err)
				}
				write(newPath, "path: /new\nmethod: GET\ndefault:\n  status_code: 999\n")
			}
			cm := NewConfigManager(mainPath)
			cm.SetConfig(old)
			logs := make(watcherLogEvents, 20)
			w := NewWatcher(mainPath, cm, log.New(logs, "", 0))
			reloaded := make(chan struct{}, 10)
			w.OnReload = func() { reloaded <- struct{}{} }
			w.Start(1)
			defer w.Stop()
			waitLog := func(fragment string) {
				t.Helper()
				deadline := time.After(2 * time.Second)
				for {
					select {
					case message := <-logs:
						if strings.Contains(message, fragment) {
							return
						}
					case <-deadline:
						t.Fatalf("did not observe %q", fragment)
					}
				}
			}
			waitLog("Started watching")
			write(mainPath, "endpoints: [new/endpoint.yaml]\n")
			waitLog("Failed to reload")
			if cm.GetConfig() != old {
				t.Fatal("failed candidate replaced the active config")
			}
			if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
				t.Fatal(err)
			}
			write(newPath, "path: /new\nmethod: GET\n")
			select {
			case <-reloaded:
			case <-time.After(2500 * time.Millisecond):
				t.Fatal("fixing only the new endpoint file did not recover")
			}
			if got := cm.GetConfig().Endpoints[0].Path; got != "/new" {
				t.Fatalf("active endpoint = %q", got)
			}
			select {
			case <-reloaded:
				t.Fatal("retry continued after successful activation")
			case <-time.After(1200 * time.Millisecond):
			}
		})
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
