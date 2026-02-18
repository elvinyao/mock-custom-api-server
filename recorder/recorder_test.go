package recorder

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func makeEntry(method, path string) *RecordedRequest {
	return &RecordedRequest{
		Method:         method,
		Path:           path,
		Timestamp:      time.Now(),
		ResponseStatus: 200,
	}
}

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_DefaultCapacity(t *testing.T) {
	r := New(0)
	if r.maxEntries != 1000 {
		t.Fatalf("expected maxEntries=1000, got %d", r.maxEntries)
	}
}

func TestNew_NegativeCapacity(t *testing.T) {
	r := New(-5)
	if r.maxEntries != 1000 {
		t.Fatalf("expected maxEntries=1000 for negative input, got %d", r.maxEntries)
	}
}

func TestNew_CustomCapacity(t *testing.T) {
	r := New(42)
	if r.maxEntries != 42 {
		t.Fatalf("expected maxEntries=42, got %d", r.maxEntries)
	}
	if len(r.entries) != 42 {
		t.Fatalf("expected entries slice len=42, got %d", len(r.entries))
	}
}

// ── Record / Count ────────────────────────────────────────────────────────────

func TestRecord_IDAssignment(t *testing.T) {
	r := New(10)
	e1 := makeEntry("GET", "/a")
	e2 := makeEntry("POST", "/b")
	r.Record(e1)
	r.Record(e2)
	if e1.ID != "req_1" {
		t.Errorf("first entry ID = %q, want req_1", e1.ID)
	}
	if e2.ID != "req_2" {
		t.Errorf("second entry ID = %q, want req_2", e2.ID)
	}
}

func TestRecord_Count(t *testing.T) {
	r := New(10)
	if r.Count() != 0 {
		t.Fatalf("expected 0 count initially")
	}
	r.Record(makeEntry("GET", "/"))
	r.Record(makeEntry("GET", "/"))
	if r.Count() != 2 {
		t.Fatalf("expected count=2, got %d", r.Count())
	}
}

func TestRecord_CircularBuffer_Overflow(t *testing.T) {
	r := New(3)
	entries := make([]*RecordedRequest, 5)
	for i := 0; i < 5; i++ {
		entries[i] = makeEntry("GET", fmt.Sprintf("/path/%d", i))
		r.Record(entries[i])
	}
	// Buffer holds only 3; count stays at 3
	if r.Count() != 3 {
		t.Fatalf("expected count=3, got %d", r.Count())
	}
	// The oldest 2 entries should be gone; newest 3 (indices 2,3,4) remain.
	// List returns newest first.
	list := r.List(0, 0)
	if len(list) != 3 {
		t.Fatalf("expected 3 entries in list, got %d", len(list))
	}
	// Newest first: /path/4, /path/3, /path/2
	if list[0].Path != "/path/4" {
		t.Errorf("list[0].Path = %q, want /path/4", list[0].Path)
	}
	if list[2].Path != "/path/2" {
		t.Errorf("list[2].Path = %q, want /path/2", list[2].Path)
	}
}

func TestRecord_CircularBuffer_HeadAdvances(t *testing.T) {
	r := New(2)
	e1 := makeEntry("GET", "/1")
	e2 := makeEntry("GET", "/2")
	e3 := makeEntry("GET", "/3")
	r.Record(e1)
	r.Record(e2)
	r.Record(e3) // overwrites e1
	if r.head != 1 {
		t.Errorf("head should be 1, got %d", r.head)
	}
}

// ── List ─────────────────────────────────────────────────────────────────────

func TestList_EmptyReturnsNil(t *testing.T) {
	r := New(10)
	if r.List(0, 0) != nil {
		t.Errorf("expected nil from empty recorder")
	}
}

func TestList_NewestFirst(t *testing.T) {
	r := New(10)
	for i := 1; i <= 5; i++ {
		r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", i)))
	}
	list := r.List(0, 0)
	if list[0].Path != "/p/5" {
		t.Errorf("expected newest first, got %q", list[0].Path)
	}
	if list[4].Path != "/p/1" {
		t.Errorf("expected oldest last, got %q", list[4].Path)
	}
}

func TestList_LimitZeroReturnsAll(t *testing.T) {
	r := New(10)
	for i := 0; i < 7; i++ {
		r.Record(makeEntry("GET", "/"))
	}
	if len(r.List(0, 0)) != 7 {
		t.Errorf("limit=0 should return all 7 entries")
	}
}

func TestList_LimitTruncates(t *testing.T) {
	r := New(10)
	for i := 0; i < 7; i++ {
		r.Record(makeEntry("GET", "/"))
	}
	if len(r.List(3, 0)) != 3 {
		t.Errorf("limit=3 should return 3 entries")
	}
}

func TestList_OffsetSkips(t *testing.T) {
	r := New(10)
	for i := 1; i <= 5; i++ {
		r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", i)))
	}
	list := r.List(0, 2)
	if len(list) != 3 {
		t.Fatalf("expected 3 after offset=2, got %d", len(list))
	}
	if list[0].Path != "/p/3" {
		t.Errorf("first after offset 2 should be /p/3, got %q", list[0].Path)
	}
}

func TestList_OffsetBeyondCount(t *testing.T) {
	r := New(10)
	r.Record(makeEntry("GET", "/"))
	if r.List(0, 99) != nil {
		t.Errorf("expected nil when offset >= count")
	}
}

func TestList_LimitAndOffset(t *testing.T) {
	r := New(20)
	for i := 1; i <= 10; i++ {
		r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", i)))
	}
	// Newest first: /p/10, /p/9 ... /p/1
	// offset=2, limit=3 → /p/8, /p/7, /p/6
	list := r.List(3, 2)
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
	if list[0].Path != "/p/8" {
		t.Errorf("expected /p/8, got %q", list[0].Path)
	}
}

func TestList_AfterCircularOverflow_OrderCorrect(t *testing.T) {
	r := New(3)
	for i := 1; i <= 5; i++ {
		r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", i)))
	}
	list := r.List(0, 0)
	// Should be /p/5, /p/4, /p/3 (newest first)
	want := []string{"/p/5", "/p/4", "/p/3"}
	for i, w := range want {
		if list[i].Path != w {
			t.Errorf("list[%d].Path = %q, want %q", i, list[i].Path, w)
		}
	}
}

// ── Get ──────────────────────────────────────────────────────────────────────

func TestGet_FoundByID(t *testing.T) {
	r := New(10)
	e := makeEntry("GET", "/target")
	r.Record(e)
	found := r.Get(e.ID)
	if found == nil {
		t.Fatal("expected to find entry by ID")
	}
	if found.Path != "/target" {
		t.Errorf("expected path /target, got %q", found.Path)
	}
}

func TestGet_NotFound(t *testing.T) {
	r := New(10)
	r.Record(makeEntry("GET", "/"))
	if r.Get("req_99999") != nil {
		t.Errorf("expected nil for non-existent ID")
	}
}

func TestGet_EmptyRecorder(t *testing.T) {
	r := New(10)
	if r.Get("req_1") != nil {
		t.Errorf("expected nil from empty recorder")
	}
}

// ── Clear ─────────────────────────────────────────────────────────────────────

func TestClear_ResetsState(t *testing.T) {
	r := New(10)
	for i := 0; i < 5; i++ {
		r.Record(makeEntry("GET", "/"))
	}
	r.Clear()
	if r.Count() != 0 {
		t.Errorf("expected count=0 after clear, got %d", r.Count())
	}
	if r.head != 0 {
		t.Errorf("expected head=0 after clear, got %d", r.head)
	}
	if r.List(0, 0) != nil {
		t.Errorf("expected nil list after clear")
	}
}

func TestClear_ThenRecord(t *testing.T) {
	r := New(10)
	r.Record(makeEntry("GET", "/old"))
	r.Clear()
	e := makeEntry("GET", "/new")
	r.Record(e)
	if r.Count() != 1 {
		t.Errorf("expected count=1 after clear+record")
	}
	if e.ID != "req_2" {
		// nextID is not reset by Clear, so ID continues from where it was
		t.Logf("ID after clear+record: %s (nextID not reset, that's ok)", e.ID)
	}
}

// ── Concurrent access ─────────────────────────────────────────────────────────

func TestRecord_Concurrent(t *testing.T) {
	r := New(1000)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", n)))
		}(i)
	}
	wg.Wait()
	if r.Count() != 100 {
		t.Errorf("expected 100 after concurrent records, got %d", r.Count())
	}
}

func TestListAndRecord_Concurrent(t *testing.T) {
	r := New(100)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			r.Record(makeEntry("GET", fmt.Sprintf("/p/%d", n)))
		}(i)
		go func() {
			defer wg.Done()
			r.List(10, 0)
		}()
	}
	wg.Wait()
}

// ── uint64ToStr / formatID ────────────────────────────────────────────────────

func TestUint64ToStr(t *testing.T) {
	cases := []struct {
		input uint64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{123, "123"},
		{1000000, "1000000"},
	}
	for _, c := range cases {
		got := uint64ToStr(c.input)
		if got != c.want {
			t.Errorf("uint64ToStr(%d) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestFormatID(t *testing.T) {
	cases := []struct {
		input uint64
		want  string
	}{
		{0, "req_0"},
		{1, "req_1"},
		{42, "req_42"},
	}
	for _, c := range cases {
		got := formatID(c.input)
		if got != c.want {
			t.Errorf("formatID(%d) = %q, want %q", c.input, got, c.want)
		}
	}
}
