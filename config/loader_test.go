package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_EndpointsFromConfigPaths(t *testing.T) {
	tempDir := t.TempDir()
	endpointDir := filepath.Join(tempDir, "endpoints")
	if err := os.MkdirAll(endpointDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	mainConfig := `server:
  logging:
    level: "debug"
health_check:
  enabled: true
endpoints:
  config_paths:
    - "./endpoints/single.yaml"
    - "./endpoints/multiple.yaml"
`
	singleEndpoint := `path: "/single"
method: "GET"
default:
  status_code: 200
  response_file: "./mocks/single.json"
`
	multipleEndpoints := `paths:
  - path: "/multi/a"
    method: "POST"
    default:
      status_code: 201
  - path: "/multi/b"
    method: "DELETE"
    default:
      status_code: 204
`

	mainConfigPath := filepath.Join(tempDir, "config.yaml")
	if err := os.WriteFile(mainConfigPath, []byte(mainConfig), 0o644); err != nil {
		t.Fatalf("write config failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(endpointDir, "single.yaml"), []byte(singleEndpoint), 0o644); err != nil {
		t.Fatalf("write single endpoint file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(endpointDir, "multiple.yaml"), []byte(multipleEndpoints), 0o644); err != nil {
		t.Fatalf("write multiple endpoint file failed: %v", err)
	}

	cfg, err := LoadConfig(mainConfigPath)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Fatalf("expected default port 8080, got %d", cfg.Server.Port)
	}
	if cfg.HealthCheck.Path != "/health" {
		t.Fatalf("expected default health path /health, got %q", cfg.HealthCheck.Path)
	}

	if len(cfg.Endpoints) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(cfg.Endpoints))
	}
	if len(cfg.EndpointConfigPaths) != 2 {
		t.Fatalf("expected 2 endpoint config paths, got %d", len(cfg.EndpointConfigPaths))
	}

	expectedPaths := []string{"/single", "/multi/a", "/multi/b"}
	for i, want := range expectedPaths {
		if cfg.Endpoints[i].Path != want {
			t.Fatalf("endpoint[%d] path mismatch: want %q, got %q", i, want, cfg.Endpoints[i].Path)
		}
	}

	expectedConfigPaths := []string{
		filepath.Clean(filepath.Join(tempDir, "endpoints", "single.yaml")),
		filepath.Clean(filepath.Join(tempDir, "endpoints", "multiple.yaml")),
	}
	for i, want := range expectedConfigPaths {
		if cfg.EndpointConfigPaths[i] != want {
			t.Fatalf("endpoint config path[%d] mismatch: want %q, got %q", i, want, cfg.EndpointConfigPaths[i])
		}
	}
}
