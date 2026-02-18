package state

import (
	"sync"
)

const defaultStep = "idle"

// ScenarioEntry holds the current step for a scenario+partition key
type ScenarioEntry struct {
	Scenario     string `json:"scenario"`
	PartitionKey string `json:"partition_key"`
	CurrentStep  string `json:"current_step"`
}

// ScenarioStore stores current steps for stateful scenarios
type ScenarioStore struct {
	mu    sync.RWMutex
	steps map[string]string // key: "scenarioName:partitionKeyValue" -> current step
}

// New creates a new ScenarioStore
func New() *ScenarioStore {
	return &ScenarioStore{
		steps: make(map[string]string),
	}
}

// GetStep returns the current step for a scenario + partition key value
// Returns defaultStep ("idle") if not set
func (s *ScenarioStore) GetStep(scenario, partitionValue string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := buildKey(scenario, partitionValue)
	if step, ok := s.steps[key]; ok {
		return step
	}
	return defaultStep
}

// SetStep updates the step for a scenario + partition key value
func (s *ScenarioStore) SetStep(scenario, partitionValue, step string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := buildKey(scenario, partitionValue)
	s.steps[key] = step
}

// ResetScenario resets all entries for a given scenario name
func (s *ScenarioStore) ResetScenario(scenario string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prefix := scenario + ":"
	for k := range s.steps {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(s.steps, k)
		}
	}
}

// List returns all active scenario entries
func (s *ScenarioStore) List() []ScenarioEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]ScenarioEntry, 0, len(s.steps))
	for k, step := range s.steps {
		scenario, partitionValue := splitKey(k)
		result = append(result, ScenarioEntry{
			Scenario:     scenario,
			PartitionKey: partitionValue,
			CurrentStep:  step,
		})
	}
	return result
}

func buildKey(scenario, partitionValue string) string {
	return scenario + ":" + partitionValue
}

func splitKey(key string) (scenario, partitionValue string) {
	for i, c := range key {
		if c == ':' {
			return key[:i], key[i+1:]
		}
	}
	return key, ""
}
