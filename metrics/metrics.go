package metrics

import (
	"sort"
	"sync"
	"time"
)

const maxSamples = 1000 // per-endpoint circular buffer size for percentile computation

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
	P95Ms        int64   `json:"p95_ms"`
	P99Ms        int64   `json:"p99_ms"`
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
	// circular sample buffer for percentile computation
	samples      [maxSamples]int64
	sampleHead   int
	sampleCount  int
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

	// Store sample in circular buffer
	raw.samples[raw.sampleHead] = durationMs
	raw.sampleHead = (raw.sampleHead + 1) % maxSamples
	if raw.sampleCount < maxSamples {
		raw.sampleCount++
	}
}

// percentile returns the p-th percentile (0–100) from a sorted slice.
func percentile(sorted []int64, p float64) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
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

		// Build sorted copy of the sample buffer for percentile calculation
		n := raw.sampleCount
		buf := make([]int64, n)
		for i := 0; i < n; i++ {
			buf[i] = raw.samples[(raw.sampleHead-n+i+maxSamples)%maxSamples]
		}
		sort.Slice(buf, func(i, j int) bool { return buf[i] < buf[j] })

		result = append(result, &EndpointStats{
			Method:       raw.method,
			Path:         raw.path,
			RequestCount: raw.requestCount,
			ErrorCount:   raw.errorCount,
			TotalMs:      raw.totalMs,
			MinMs:        raw.minMs,
			MaxMs:        raw.maxMs,
			AvgMs:        avg,
			P95Ms:        percentile(buf, 95),
			P99Ms:        percentile(buf, 99),
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
