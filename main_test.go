package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mock-api-server/config"
)

func TestCheckConfigCommand_ExitsWithoutStartingServer(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		valid         bool
	}{
		{"valid", "port: 8080\nendpoints:\n  - path: /ok\n    method: GET\n", true},
		{"invalid", "endpoints:\n  - path: /bad\n    method: GET\n    default:\n      status_code: 999\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCheckConfigCommand_Helper$")
			cmd.Env = append(os.Environ(), "MOCK_CHECK_CONFIG_HELPER="+path)
			output, err := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("check-config started a server instead of exiting: %s", output)
			}
			if tc.valid && err != nil {
				t.Fatalf("valid config rejected: %v: %s", err, output)
			}
			if !tc.valid && (err == nil || !strings.Contains(string(output), "default.status_code")) {
				t.Fatalf("missing validation failure: %v: %s", err, output)
			}
		})
	}
}

func TestCheckConfigCommand_Helper(t *testing.T) {
	path := os.Getenv("MOCK_CHECK_CONFIG_HELPER")
	if path == "" {
		return
	}
	flag.CommandLine = flag.NewFlagSet("mock-api-server", flag.ExitOnError)
	os.Args = []string{"mock-api-server", "--config", path, "--check-config"}
	main()
}

func TestBuildExcludePaths_DefaultsWhenEmpty(t *testing.T) {
	cfg := &config.Config{}
	paths := buildExcludePaths(cfg)
	if len(paths) == 0 {
		t.Error("expected default exclude paths when config is empty")
	}
	found := false
	for _, p := range paths {
		if p == "/health" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected /health in default exclude paths, got %v", paths)
	}
}

func TestBuildExcludePaths_UsesConfiguredPaths(t *testing.T) {
	cfg := &config.Config{
		Recording: config.RecordingConfig{
			ExcludePaths: []string{"/custom1", "/custom2"},
		},
	}
	paths := buildExcludePaths(cfg)
	if len(paths) != 2 {
		t.Errorf("expected 2 configured paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/custom1" || paths[1] != "/custom2" {
		t.Errorf("expected [/custom1 /custom2], got %v", paths)
	}
}
