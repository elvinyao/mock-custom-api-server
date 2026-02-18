package metrics

import (
	"sync"
	"time"
)

// EndpointStats holds aggregated statistics for a single endpoint
type EndpointStats struct {
	Method       string  `json:"method"`
	Path         string  `json:"path"`
	RequestCount int64   `json:"request_count"`
	ErrorCount   int64   `json:"error_count"`
	TotalMs      int64   `json:"total_ms"`
	MinMs        int64   `json:"min_ms"`
	MaxMs        int64   `json:"max_ms"`
	AvgMs        float64 `json:"avg_ms"`
}

// Store holds in-memory metrics per endpoint
type Store struct {
	mu        sync.RWMutex
	stats     map[string]*endpointRaw
	startTime time.Time
}

type endpointRaw struct {
	method       string
	path         string
	requestCount int64
	errorCount   int64
	totalMs      int64
	minMs        int64
	maxMs        int64
}

// New creates a new metrics Store
func New() *Store {
	return &Store{
		stats:     make(map[string]*endpointRaw),
		startTime: time.Now(),
	}
}

// Record records a single request observation
func (s *Store) Record(method, path string, statusCode int, durationMs int64) {
	key := method + " " + path

	s.mu.Lock()
	defer s.mu.Unlock()

	raw, ok := s.stats[key]
	if !ok {
		raw = &endpointRaw{
			method: method,
			path:   path,
			minMs:  durationMs,
			maxMs:  durationMs,
		}
		s.stats[key] = raw
	}

	raw.requestCount++
	raw.totalMs += durationMs
	if durationMs < raw.minMs {
		raw.minMs = durationMs
	}
	if durationMs > raw.maxMs {
		raw.maxMs = durationMs
	}
	if statusCode >= 500 {
		raw.errorCount++
	}
}

// GetAll returns all endpoint stats
func (s *Store) GetAll() []*EndpointStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*EndpointStats, 0, len(s.stats))
	for _, raw := range s.stats {
		avg := float64(0)
		if raw.requestCount > 0 {
			avg = float64(raw.totalMs) / float64(raw.requestCount)
		}
		result = append(result, &EndpointStats{
			Method:       raw.method,
			Path:         raw.path,
			RequestCount: raw.requestCount,
			ErrorCount:   raw.errorCount,
			TotalMs:      raw.totalMs,
			MinMs:        raw.minMs,
			MaxMs:        raw.maxMs,
			AvgMs:        avg,
		})
	}
	return result
}

// UptimeSeconds returns seconds since the store was created
func (s *Store) UptimeSeconds() float64 {
	return time.Since(s.startTime).Seconds()
}

// StartTime returns when the metrics store was initialized
func (s *Store) StartTime() time.Time {
	return s.startTime
}
