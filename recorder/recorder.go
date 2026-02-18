package recorder

import (
	"sync"
	"time"
)

// RecordedRequest stores information about a recorded HTTP interaction
type RecordedRequest struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	Method          string            `json:"method"`
	Path            string            `json:"path"`
	Query           string            `json:"query,omitempty"`
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`
	RequestBody     string            `json:"request_body,omitempty"`
	ResponseStatus  int               `json:"response_status"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	ResponseBody    string            `json:"response_body,omitempty"`
	DurationMs      int64             `json:"duration_ms"`
	MatchedRule     string            `json:"matched_rule,omitempty"`
}

// Recorder is a circular buffer that records HTTP interactions
type Recorder struct {
	mu         sync.RWMutex
	entries    []*RecordedRequest
	maxEntries int
	head       int // index of oldest entry
	count      int // number of entries stored
	nextID     uint64
}

// New creates a new Recorder with the given max capacity
func New(maxEntries int) *Recorder {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	return &Recorder{
		entries:    make([]*RecordedRequest, maxEntries),
		maxEntries: maxEntries,
	}
}

// Record adds a new entry to the recorder
func (r *Recorder) Record(entry *RecordedRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.nextID++
	entry.ID = formatID(r.nextID)

	if r.count < r.maxEntries {
		// Buffer not full yet
		r.entries[r.count] = entry
		r.count++
	} else {
		// Overwrite oldest entry
		r.entries[r.head] = entry
		r.head = (r.head + 1) % r.maxEntries
	}
}

// List returns entries in reverse-chronological order (newest first)
// limit=0 means return all
func (r *Recorder) List(limit, offset int) []*RecordedRequest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return nil
	}

	// Build ordered slice (newest first)
	ordered := make([]*RecordedRequest, r.count)
	for i := 0; i < r.count; i++ {
		// Newest entries are at the end; oldest is at head
		idx := (r.head + r.count - 1 - i) % r.maxEntries
		ordered[i] = r.entries[idx]
	}

	// Apply offset
	if offset >= len(ordered) {
		return nil
	}
	ordered = ordered[offset:]

	// Apply limit
	if limit > 0 && limit < len(ordered) {
		ordered = ordered[:limit]
	}

	return ordered
}

// Get returns a single recorded request by ID
func (r *Recorder) Get(id string) *RecordedRequest {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := 0; i < r.count; i++ {
		idx := (r.head + i) % r.maxEntries
		if r.entries[idx] != nil && r.entries[idx].ID == id {
			return r.entries[idx]
		}
	}
	return nil
}

// Clear removes all recorded entries
func (r *Recorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = make([]*RecordedRequest, r.maxEntries)
	r.head = 0
	r.count = 0
}

// Count returns the number of recorded entries
func (r *Recorder) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

func formatID(n uint64) string {
	return "req_" + uint64ToStr(n)
}

func uint64ToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
