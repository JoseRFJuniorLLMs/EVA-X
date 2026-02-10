package telemetry

import (
	"sync"
	"time"
)

// PsychMetrics tracks the psychological state of the EVA-Mind core
type PsychMetrics struct {
	mu sync.RWMutex

	CurrentEnneatype int
	StressLevel      float64 // 0.0 to 1.0
	FocusMetric      float64 // Entropy or Focus level
	PrimingLatency   int64   // ms

	// Counters
	TotalSwitches        int64
	TotalDesintegrations int64
	TotalIntegrations    int64
}

var GlobalMetrics = &PsychMetrics{}

func (m *PsychMetrics) UpdateType(enneatype int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.CurrentEnneatype != enneatype && m.CurrentEnneatype != 0 {
		m.TotalSwitches++
	}
	m.CurrentEnneatype = enneatype
}

func (m *PsychMetrics) UpdateStress(level float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StressLevel = level
}

func (m *PsychMetrics) RecordLatency(ms int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Simple moving average or just last value for now
	m.PrimingLatency = ms
}

func (m *PsychMetrics) RecordIntegration() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalIntegrations++
}

func (m *PsychMetrics) RecordDesintegration() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.TotalDesintegrations++
}

func GetSnapshot() map[string]interface{} {
	GlobalMetrics.mu.RLock()
	defer GlobalMetrics.mu.RUnlock()

	return map[string]interface{}{
		"enneatype":       GlobalMetrics.CurrentEnneatype,
		"stress_level":    GlobalMetrics.StressLevel,
		"priming_latency": GlobalMetrics.PrimingLatency,
		"switches":        GlobalMetrics.TotalSwitches,
		"integrations":    GlobalMetrics.TotalIntegrations,
		"desintegrations": GlobalMetrics.TotalDesintegrations,
		"timestamp":       time.Now(),
	}
}
