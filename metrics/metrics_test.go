package metrics

import (
	"sync"
	"testing"
	"time"
)

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_InitializesEmpty(t *testing.T) {
	before := time.Now()
	s := New()
	after := time.Now()

	if s == nil {
		t.Fatal("New() returned nil")
	}
	if len(s.GetAll()) != 0 {
		t.Errorf("expected empty stats on creation")
	}
	st := s.StartTime()
	if st.Before(before) || st.After(after) {
		t.Errorf("StartTime %v not within expected range [%v, %v]", st, before, after)
	}
}

// ── Record ────────────────────────────────────────────────────────────────────

func TestRecord_FirstRequest_InitializesEntry(t *testing.T) {
	s := New()
	s.Record("GET", "/api/test", 200, 100)

	stats := s.GetAll()
	if len(stats) != 1 {
		t.Fatalf("expected 1 endpoint stat, got %d", len(stats))
	}
	st := stats[0]
	if st.Method != "GET" {
		t.Errorf("Method = %q, want GET", st.Method)
	}
	if st.Path != "/api/test" {
		t.Errorf("Path = %q, want /api/test", st.Path)
	}
	if st.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", st.RequestCount)
	}
	if st.MinMs != 100 {
		t.Errorf("MinMs = %d, want 100", st.MinMs)
	}
	if st.MaxMs != 100 {
		t.Errorf("MaxMs = %d, want 100", st.MaxMs)
	}
	if st.AvgMs != 100.0 {
		t.Errorf("AvgMs = %f, want 100", st.AvgMs)
	}
	if st.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0", st.ErrorCount)
	}
}

func TestRecord_MultipleRequests_UpdatesStats(t *testing.T) {
	s := New()
	s.Record("GET", "/api", 200, 50)
	s.Record("GET", "/api", 200, 150)
	s.Record("GET", "/api", 200, 100)

	stats := s.GetAll()
	if len(stats) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(stats))
	}
	st := stats[0]
	if st.RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3", st.RequestCount)
	}
	if st.MinMs != 50 {
		t.Errorf("MinMs = %d, want 50", st.MinMs)
	}
	if st.MaxMs != 150 {
		t.Errorf("MaxMs = %d, want 150", st.MaxMs)
	}
	if st.TotalMs != 300 {
		t.Errorf("TotalMs = %d, want 300", st.TotalMs)
	}
	wantAvg := float64(300) / float64(3)
	if st.AvgMs != wantAvg {
		t.Errorf("AvgMs = %f, want %f", st.AvgMs, wantAvg)
	}
}

func TestRecord_500_IncrementsErrorCount(t *testing.T) {
	s := New()
	s.Record("GET", "/api", 500, 10)
	s.Record("GET", "/api", 503, 10)
	s.Record("GET", "/api", 200, 10)

	stats := s.GetAll()
	if stats[0].ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2 (for 500 and 503)", stats[0].ErrorCount)
	}
}

func TestRecord_499_DoesNotIncrementErrorCount(t *testing.T) {
	s := New()
	s.Record("GET", "/api", 404, 10)
	s.Record("GET", "/api", 499, 10)

	stats := s.GetAll()
	if stats[0].ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0 for 4xx", stats[0].ErrorCount)
	}
}

func TestRecord_MinUpdatesWhenSmaller(t *testing.T) {
	s := New()
	s.Record("GET", "/", 200, 100)
	s.Record("GET", "/", 200, 10) // smaller
	stats := s.GetAll()
	if stats[0].MinMs != 10 {
		t.Errorf("MinMs = %d, want 10", stats[0].MinMs)
	}
}

func TestRecord_MaxUpdatesWhenLarger(t *testing.T) {
	s := New()
	s.Record("GET", "/", 200, 100)
	s.Record("GET", "/", 200, 500) // larger
	stats := s.GetAll()
	if stats[0].MaxMs != 500 {
		t.Errorf("MaxMs = %d, want 500", stats[0].MaxMs)
	}
}

func TestRecord_DifferentEndpointsTrackedIndependently(t *testing.T) {
	s := New()
	s.Record("GET", "/a", 200, 10)
	s.Record("POST", "/b", 200, 20)
	s.Record("GET", "/a", 200, 30) // same endpoint as first

	stats := s.GetAll()
	if len(stats) != 2 {
		t.Fatalf("expected 2 distinct endpoints, got %d", len(stats))
	}

	// Build map for lookup
	byKey := make(map[string]*EndpointStats)
	for _, st := range stats {
		byKey[st.Method+" "+st.Path] = st
	}

	a := byKey["GET /a"]
	if a == nil || a.RequestCount != 2 {
		t.Errorf("GET /a should have 2 requests")
	}
	b := byKey["POST /b"]
	if b == nil || b.RequestCount != 1 {
		t.Errorf("POST /b should have 1 request")
	}
}

func TestRecord_SameMethodDifferentPath(t *testing.T) {
	s := New()
	s.Record("GET", "/users", 200, 50)
	s.Record("GET", "/orders", 200, 80)
	stats := s.GetAll()
	if len(stats) != 2 {
		t.Errorf("expected 2 entries for different paths, got %d", len(stats))
	}
}

func TestRecord_AllStatusCodesBelow500_NoErrors(t *testing.T) {
	s := New()
	for _, code := range []int{100, 200, 201, 301, 400, 404, 499} {
		s.Record("GET", "/test", code, 5)
	}
	stats := s.GetAll()
	if stats[0].ErrorCount != 0 {
		t.Errorf("expected 0 errors for status codes < 500, got %d", stats[0].ErrorCount)
	}
}

// ── GetAll ────────────────────────────────────────────────────────────────────

func TestGetAll_EmptyStore(t *testing.T) {
	s := New()
	stats := s.GetAll()
	if stats == nil {
		t.Errorf("GetAll on empty store should return empty slice, not nil")
	}
	if len(stats) != 0 {
		t.Errorf("expected 0 stats, got %d", len(stats))
	}
}

func TestGetAll_AvgMs_ZeroRequestCount(t *testing.T) {
	// AvgMs should not divide by zero (not directly testable without injection,
	// but we ensure that single requests produce correct avg)
	s := New()
	s.Record("GET", "/ping", 200, 42)
	stats := s.GetAll()
	if stats[0].AvgMs != 42.0 {
		t.Errorf("AvgMs = %f, want 42.0", stats[0].AvgMs)
	}
}

// ── UptimeSeconds / StartTime ──────────────────────────────────────────────────

func TestUptimeSeconds_Positive(t *testing.T) {
	s := New()
	time.Sleep(5 * time.Millisecond)
	uptime := s.UptimeSeconds()
	if uptime <= 0 {
		t.Errorf("UptimeSeconds() = %f, want > 0", uptime)
	}
}

func TestStartTime_BeforeNow(t *testing.T) {
	before := time.Now()
	s := New()
	after := time.Now()
	st := s.StartTime()
	if st.Before(before) || st.After(after) {
		t.Errorf("StartTime %v not in expected range [%v, %v]", st, before, after)
	}
}

// ── Concurrent access ─────────────────────────────────────────────────────────

func TestRecord_Concurrent(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			code := 200
			if n%10 == 0 {
				code = 500
			}
			s.Record("GET", "/concurrent", code, int64(n))
		}(i)
	}
	wg.Wait()

	stats := s.GetAll()
	if len(stats) != 1 {
		t.Fatalf("expected 1 endpoint stat, got %d", len(stats))
	}
	if stats[0].RequestCount != 100 {
		t.Errorf("RequestCount = %d, want 100", stats[0].RequestCount)
	}
}

func TestGetAll_Concurrent(t *testing.T) {
	s := New()
	for i := 0; i < 10; i++ {
		s.Record("GET", "/path", 200, 10)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.GetAll()
		}()
	}
	wg.Wait()
}
