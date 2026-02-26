package config

import (
	"sync"
	"testing"
	"time"
)

// ── NewConfigManager ──────────────────────────────────────────────────────────

func TestNewConfigManager_NotNil(t *testing.T) {
	cm := NewConfigManager("/some/path/config.yaml")
	if cm == nil {
		t.Fatal("NewConfigManager returned nil")
	}
}

func TestNewConfigManager_ConfigPathStored(t *testing.T) {
	cm := NewConfigManager("/etc/myconfig.yaml")
	if cm.GetConfigPath() != "/etc/myconfig.yaml" {
		t.Errorf("GetConfigPath = %q, want /etc/myconfig.yaml", cm.GetConfigPath())
	}
}

func TestNewConfigManager_InitialConfigNil(t *testing.T) {
	cm := NewConfigManager("x")
	if cm.GetConfig() != nil {
		t.Error("initial GetConfig should return nil")
	}
}

// ── GetConfig / SetConfig ─────────────────────────────────────────────────────

func TestSetConfig_GetConfig_RoundTrip(t *testing.T) {
	cm := NewConfigManager("")
	cfg := &Config{
		Port: 9090,
	}
	cm.SetConfig(cfg)

	got := cm.GetConfig()
	if got == nil {
		t.Fatal("GetConfig returned nil after SetConfig")
	}
	if got.Port != 9090 {
		t.Errorf("Port = %d, want 9090", got.Port)
	}
}

// ── GetLoadedAt ───────────────────────────────────────────────────────────────

func TestGetLoadedAt_ZeroBeforeSet(t *testing.T) {
	cm := NewConfigManager("")
	if !cm.GetLoadedAt().IsZero() {
		t.Error("expected zero time before SetConfig")
	}
}

func TestGetLoadedAt_SetAfterSetConfig(t *testing.T) {
	before := time.Now()
	cm := NewConfigManager("")
	cm.SetConfig(&Config{})
	after := time.Now()

	loaded := cm.GetLoadedAt()
	if loaded.Before(before) || loaded.After(after) {
		t.Errorf("GetLoadedAt() = %v, not in range [%v, %v]", loaded, before, after)
	}
}

// ── Runtime endpoints ─────────────────────────────────────────────────────────

func TestAddRuntimeEndpoint_AppendedToList(t *testing.T) {
	cm := NewConfigManager("")
	ep := Endpoint{Path: "/rt/v1", Method: "GET"}
	cm.AddRuntimeEndpoint(ep)

	eps := cm.GetRuntimeEndpoints()
	if len(eps) != 1 {
		t.Fatalf("expected 1 runtime endpoint, got %d", len(eps))
	}
	if eps[0].Path != "/rt/v1" {
		t.Errorf("Path = %q, want /rt/v1", eps[0].Path)
	}
}

func TestGetRuntimeEndpoints_ReturnsCopy(t *testing.T) {
	cm := NewConfigManager("")
	cm.AddRuntimeEndpoint(Endpoint{Path: "/a"})
	eps1 := cm.GetRuntimeEndpoints()
	eps1[0].Path = "/modified"

	eps2 := cm.GetRuntimeEndpoints()
	if eps2[0].Path != "/a" {
		t.Error("GetRuntimeEndpoints should return a copy; mutation leaked")
	}
}

func TestRemoveRuntimeEndpoint_ValidIndex(t *testing.T) {
	cm := NewConfigManager("")
	cm.AddRuntimeEndpoint(Endpoint{Path: "/a"})
	cm.AddRuntimeEndpoint(Endpoint{Path: "/b"})
	cm.AddRuntimeEndpoint(Endpoint{Path: "/c"})

	ok := cm.RemoveRuntimeEndpoint(1)
	if !ok {
		t.Fatal("expected true for valid index")
	}
	eps := cm.GetRuntimeEndpoints()
	if len(eps) != 2 {
		t.Fatalf("expected 2 after removal, got %d", len(eps))
	}
	if eps[0].Path != "/a" || eps[1].Path != "/c" {
		t.Errorf("unexpected paths after removal: %v %v", eps[0].Path, eps[1].Path)
	}
}

func TestRemoveRuntimeEndpoint_InvalidIndex(t *testing.T) {
	cm := NewConfigManager("")
	if cm.RemoveRuntimeEndpoint(0) {
		t.Error("expected false for empty list")
	}
	cm.AddRuntimeEndpoint(Endpoint{Path: "/a"})
	if cm.RemoveRuntimeEndpoint(-1) {
		t.Error("expected false for negative index")
	}
	if cm.RemoveRuntimeEndpoint(99) {
		t.Error("expected false for out-of-range index")
	}
}

func TestUpdateRuntimeEndpoint_ValidIndex(t *testing.T) {
	cm := NewConfigManager("")
	cm.AddRuntimeEndpoint(Endpoint{Path: "/original"})

	ok := cm.UpdateRuntimeEndpoint(0, Endpoint{Path: "/updated"})
	if !ok {
		t.Fatal("expected true for valid index")
	}
	eps := cm.GetRuntimeEndpoints()
	if eps[0].Path != "/updated" {
		t.Errorf("Path = %q, want /updated", eps[0].Path)
	}
}

func TestUpdateRuntimeEndpoint_InvalidIndex(t *testing.T) {
	cm := NewConfigManager("")
	if cm.UpdateRuntimeEndpoint(0, Endpoint{}) {
		t.Error("expected false for empty list")
	}
}

// ── GetAllEndpoints ───────────────────────────────────────────────────────────

func TestGetAllEndpoints_CombinesFileAndRuntime(t *testing.T) {
	cm := NewConfigManager("")
	cfg := &Config{
		Endpoints: []Endpoint{
			{Path: "/file/ep1"},
			{Path: "/file/ep2"},
		},
	}
	cm.SetConfig(cfg)
	cm.AddRuntimeEndpoint(Endpoint{Path: "/runtime/ep1"})

	all := cm.GetAllEndpoints()
	if len(all) != 3 {
		t.Fatalf("expected 3 endpoints, got %d", len(all))
	}
	paths := make(map[string]bool)
	for _, ep := range all {
		paths[ep.Path] = true
	}
	if !paths["/file/ep1"] || !paths["/file/ep2"] || !paths["/runtime/ep1"] {
		t.Errorf("missing expected paths in GetAllEndpoints: %v", all)
	}
}

func TestGetAllEndpoints_NilConfig(t *testing.T) {
	cm := NewConfigManager("")
	cm.AddRuntimeEndpoint(Endpoint{Path: "/runtime/only"})
	all := cm.GetAllEndpoints()
	if len(all) != 1 || all[0].Path != "/runtime/only" {
		t.Errorf("expected 1 runtime endpoint, got %v", all)
	}
}

func TestGetAllEndpoints_BothEmpty(t *testing.T) {
	cm := NewConfigManager("")
	cm.SetConfig(&Config{})
	all := cm.GetAllEndpoints()
	if len(all) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(all))
	}
}

// ── Concurrent access ─────────────────────────────────────────────────────────

func TestConfigManager_Concurrent(t *testing.T) {
	cm := NewConfigManager("")
	var wg sync.WaitGroup

	// Concurrent SetConfig + GetConfig
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			cm.SetConfig(&Config{Port: 8080})
		}()
		go func() {
			defer wg.Done()
			cm.GetConfig()
		}()
	}

	// Concurrent AddRuntimeEndpoint + GetRuntimeEndpoints
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			cm.AddRuntimeEndpoint(Endpoint{Path: "/concurrent"})
		}()
		go func() {
			defer wg.Done()
			cm.GetRuntimeEndpoints()
		}()
	}

	wg.Wait()
}
