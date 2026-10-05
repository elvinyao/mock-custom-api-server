package admin

import (
	"mock-api-server/config"
	"os"
	"path/filepath"
	"testing"
)

func TestPostConfigReload_InvalidCandidatePreservesConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("port: 9000\nendpoints:\n  - path: /bad\n    method: GET\n    default:\n      status_code: 999\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cm := config.NewConfigManager(path)
	old := &config.Config{Port: 8080}
	cm.SetConfig(old)
	h := New(cm, nil, nil)
	called := false
	h.SetReloadCallback(func() { called = true })
	response := doRequest(setupRouter(h), "POST", "/admin/config/reload", nil)
	if response.Code < 400 {
		t.Fatalf("invalid reload accepted: %d %s", response.Code, response.Body.String())
	}
	if cm.GetConfig() != old || called {
		t.Fatal("invalid reload changed config or invoked callback")
	}
}
