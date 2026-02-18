package state

import (
	"sync"
	"testing"
)

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_EmptyStore(t *testing.T) {
	s := New()
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if len(s.steps) != 0 {
		t.Errorf("expected empty steps map, got %d entries", len(s.steps))
	}
}

// ── GetStep ───────────────────────────────────────────────────────────────────

func TestGetStep_UnknownReturnsIdle(t *testing.T) {
	s := New()
	step := s.GetStep("myflow", "user1")
	if step != "idle" {
		t.Errorf("expected idle for unknown key, got %q", step)
	}
}

func TestGetStep_AfterSetStep(t *testing.T) {
	s := New()
	s.SetStep("myflow", "user1", "processing")
	step := s.GetStep("myflow", "user1")
	if step != "processing" {
		t.Errorf("expected processing, got %q", step)
	}
}

func TestGetStep_DifferentPartitions(t *testing.T) {
	s := New()
	s.SetStep("myflow", "user1", "step_a")
	s.SetStep("myflow", "user2", "step_b")

	if got := s.GetStep("myflow", "user1"); got != "step_a" {
		t.Errorf("user1 step = %q, want step_a", got)
	}
	if got := s.GetStep("myflow", "user2"); got != "step_b" {
		t.Errorf("user2 step = %q, want step_b", got)
	}
}

func TestGetStep_EmptyPartitionValue(t *testing.T) {
	s := New()
	s.SetStep("myflow", "", "initiated")
	step := s.GetStep("myflow", "")
	if step != "initiated" {
		t.Errorf("expected initiated for empty partition, got %q", step)
	}
}

func TestGetStep_DifferentScenarios(t *testing.T) {
	s := New()
	s.SetStep("flow_a", "p1", "step_x")
	s.SetStep("flow_b", "p1", "step_y")

	if got := s.GetStep("flow_a", "p1"); got != "step_x" {
		t.Errorf("flow_a step = %q, want step_x", got)
	}
	if got := s.GetStep("flow_b", "p1"); got != "step_y" {
		t.Errorf("flow_b step = %q, want step_y", got)
	}
}

// ── SetStep ───────────────────────────────────────────────────────────────────

func TestSetStep_Overwrites(t *testing.T) {
	s := New()
	s.SetStep("flow", "p", "first")
	s.SetStep("flow", "p", "second")
	if got := s.GetStep("flow", "p"); got != "second" {
		t.Errorf("expected second after overwrite, got %q", got)
	}
}

// ── ResetScenario ─────────────────────────────────────────────────────────────

func TestResetScenario_RemovesAllForScenario(t *testing.T) {
	s := New()
	s.SetStep("checkout", "u1", "cart_ready")
	s.SetStep("checkout", "u2", "payment_processing")
	s.SetStep("other", "u1", "idle")

	s.ResetScenario("checkout")

	if got := s.GetStep("checkout", "u1"); got != "idle" {
		t.Errorf("expected idle after reset, got %q", got)
	}
	if got := s.GetStep("checkout", "u2"); got != "idle" {
		t.Errorf("expected idle after reset, got %q", got)
	}
	// Other scenario untouched
	if got := s.GetStep("other", "u1"); got != "idle" {
		t.Errorf("other scenario should be idle, got %q", got)
	}
}

func TestResetScenario_DoesNotAffectOtherScenario(t *testing.T) {
	s := New()
	s.SetStep("A", "p1", "step1")
	s.SetStep("AB", "p1", "step2") // "AB" has prefix "A" but is a different scenario
	s.ResetScenario("A")

	// "A:p1" should be reset
	if got := s.GetStep("A", "p1"); got != "idle" {
		t.Errorf("A:p1 should be reset to idle, got %q", got)
	}
	// "AB:p1" should NOT be reset (prefix match uses "A:", not just "A")
	if got := s.GetStep("AB", "p1"); got != "step2" {
		t.Errorf("AB:p1 should remain step2, got %q", got)
	}
}

func TestResetScenario_NonExistent(t *testing.T) {
	s := New()
	// Should not panic or error
	s.ResetScenario("nonexistent")
	if len(s.steps) != 0 {
		t.Errorf("expected no entries after resetting nonexistent scenario")
	}
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestList_EmptyReturnsEmpty(t *testing.T) {
	s := New()
	entries := s.List()
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d", len(entries))
	}
}

func TestList_ReturnsAllEntries(t *testing.T) {
	s := New()
	s.SetStep("flow", "u1", "step_a")
	s.SetStep("flow", "u2", "step_b")
	s.SetStep("other", "u3", "step_c")

	entries := s.List()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Build a map for assertion since order is non-deterministic
	found := make(map[string]string) // "scenario:partition" -> step
	for _, e := range entries {
		found[e.Scenario+":"+e.PartitionKey] = e.CurrentStep
	}
	if found["flow:u1"] != "step_a" {
		t.Errorf("flow:u1 = %q, want step_a", found["flow:u1"])
	}
	if found["flow:u2"] != "step_b" {
		t.Errorf("flow:u2 = %q, want step_b", found["flow:u2"])
	}
	if found["other:u3"] != "step_c" {
		t.Errorf("other:u3 = %q, want step_c", found["other:u3"])
	}
}

func TestList_EntryFields(t *testing.T) {
	s := New()
	s.SetStep("myflow", "partitionX", "confirmed")
	entries := s.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry")
	}
	e := entries[0]
	if e.Scenario != "myflow" {
		t.Errorf("Scenario = %q, want myflow", e.Scenario)
	}
	if e.PartitionKey != "partitionX" {
		t.Errorf("PartitionKey = %q, want partitionX", e.PartitionKey)
	}
	if e.CurrentStep != "confirmed" {
		t.Errorf("CurrentStep = %q, want confirmed", e.CurrentStep)
	}
}

// ── buildKey / splitKey ───────────────────────────────────────────────────────

func TestBuildKey(t *testing.T) {
	got := buildKey("scenario", "partition")
	if got != "scenario:partition" {
		t.Errorf("buildKey = %q, want scenario:partition", got)
	}
}

func TestBuildKey_EmptyPartition(t *testing.T) {
	got := buildKey("myscenario", "")
	if got != "myscenario:" {
		t.Errorf("buildKey empty partition = %q, want myscenario:", got)
	}
}

func TestSplitKey_WithColon(t *testing.T) {
	sc, pv := splitKey("myflow:user123")
	if sc != "myflow" {
		t.Errorf("scenario = %q, want myflow", sc)
	}
	if pv != "user123" {
		t.Errorf("partitionValue = %q, want user123", pv)
	}
}

func TestSplitKey_NoColon(t *testing.T) {
	sc, pv := splitKey("justscenario")
	if sc != "justscenario" {
		t.Errorf("scenario = %q, want justscenario", sc)
	}
	if pv != "" {
		t.Errorf("partitionValue = %q, want empty", pv)
	}
}

func TestSplitKey_ColonInPartition(t *testing.T) {
	// First colon is the separator; value may contain colons
	sc, pv := splitKey("flow:part:extra")
	if sc != "flow" {
		t.Errorf("scenario = %q, want flow", sc)
	}
	if pv != "part:extra" {
		t.Errorf("partitionValue = %q, want part:extra", pv)
	}
}

// ── Concurrent access ─────────────────────────────────────────────────────────

func TestStore_ConcurrentSetGet(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		partition := string(rune('a' + i%26))
		go func(p string) {
			defer wg.Done()
			s.SetStep("flow", p, "running")
		}(partition)
		go func(p string) {
			defer wg.Done()
			s.GetStep("flow", p)
		}(partition)
	}
	wg.Wait()
}

func TestStore_ConcurrentResetAndList(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		s.SetStep("flow", string(rune('a'+i)), "active")
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s.ResetScenario("flow")
	}()
	go func() {
		defer wg.Done()
		s.List()
	}()
	wg.Wait()
}
