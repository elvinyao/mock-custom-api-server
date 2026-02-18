package admin

import (
	"net/http"
	"testing"
	"time"

	"mock-api-server/recorder"
)

// ── addEndpoint – nil body still produces 400 (empty path+method) ────────────

func TestAddEndpoint_EmptyBody_400(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	r := setupRouter(h)
	// nil body → empty JSON → path and method empty → 400
	w := doRequest(r, "POST", "/admin/endpoints", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for empty endpoint", w.Code)
	}
}

// ── listRequests – invalid query params ──────────────────────────────────────

func TestListRequests_InvalidLimit_UsesDefault(t *testing.T) {
	h, _, rec, _, _ := newTestHandler()
	for i := 0; i < 5; i++ {
		rec.Record(&recorder.RecordedRequest{
			Method:         "GET",
			Path:           "/x",
			Timestamp:      time.Now(),
			ResponseStatus: 200,
		})
	}
	r := setupRouter(h)
	// Invalid limit string
	w := doRequest(r, "GET", "/admin/requests?limit=notanumber", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestListRequests_NegativeLimit_UsesDefault(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests?limit=-5", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestListRequests_InvalidOffset_UsesDefault(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests?offset=bad", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestListRequests_NegativeOffset_UsesDefault(t *testing.T) {
	h, _, _, _, _ := newTestHandler()
	r := setupRouter(h)
	w := doRequest(r, "GET", "/admin/requests?offset=-1", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
