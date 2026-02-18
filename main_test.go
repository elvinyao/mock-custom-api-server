package main

import (
	"testing"

	"mock-api-server/config"
)

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
		Server: config.ServerConfig{
			Recording: config.RecordingConfig{
				ExcludePaths: []string{"/custom1", "/custom2"},
			},
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
