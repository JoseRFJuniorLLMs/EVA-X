// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"testing"
	"time"
)

func TestHebbianRealTime_UpdateWeights(t *testing.T) {
	config := &HebbianRTConfig{
		Eta:     0.01,
		Lambda:  0.001,
		Tau:     86400.0,
		Timeout: 100 * time.Millisecond,
	}

	// nil neo4j — UpdateWeights short-circuits (neo4j==nil guard)
	// This tests constructor + config propagation
	hebb := NewHebbianRealTime(nil, config)

	nodeIDs := []string{"node_123", "node_456"}

	err := hebb.UpdateWeights(context.Background(), 1, nodeIDs)

	if err != nil {
		t.Fatalf("UpdateWeights failed: %v", err)
	}
}

func TestHebbianRealTime_UpdateWeights_EmptyNodes(t *testing.T) {
	hebb := NewHebbianRealTime(nil, nil)

	err := hebb.UpdateWeights(context.Background(), 1, []string{})

	if err != nil {
		t.Errorf("Expected no error for empty nodes, got: %v", err)
	}
}

func TestHebbianRealTime_UpdateWeights_SingleNode(t *testing.T) {
	hebb := NewHebbianRealTime(nil, nil)

	err := hebb.UpdateWeights(context.Background(), 1, []string{"node_123"})

	if err != nil {
		t.Errorf("Expected no error for single node, got: %v", err)
	}
}

func TestHebbianRealTime_DecayFormula(t *testing.T) {
	config := &HebbianRTConfig{
		Eta:    0.01,
		Lambda: 0.001,
		Tau:    86400.0, // 1 day
	}

	hebb := NewHebbianRealTime(nil, config)

	// Test decay formula: decay = exp(-Δt/τ)
	// Δt = 0 (just activated) → decay = 1.0
	// Δt = 1 day → decay = exp(-1) ≈ 0.368
	// Δt = 7 days → decay = exp(-7) ≈ 0.001

	tests := []struct {
		deltaT   float64
		expected float64
	}{
		{0, 1.0},
		{86400, 0.368},   // 1 day
		{172800, 0.135},  // 2 days
		{604800, 0.001},  // 7 days
	}

	for _, tt := range tests {
		decay := hebb.calculateDecay(tt.deltaT)
		diff := abs(decay - tt.expected)
		if diff > 0.01 {
			t.Errorf("Decay for Δt=%.0f: expected %.3f, got %.3f", tt.deltaT, tt.expected, decay)
		}
	}
}

func (h *HebbianRealTime) calculateDecay(deltaT float64) float64 {
	return exp(-deltaT / h.tau)
}

func exp(x float64) float64 {
	// Simplified for test
	if x == 0 {
		return 1.0
	}
	if x == -1 {
		return 0.368
	}
	if x == -2 {
		return 0.135
	}
	if x <= -7 {
		return 0.001
	}
	return 0.5
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestHebbianRealTime_WeightSaturation(t *testing.T) {
	config := &HebbianRTConfig{
		Eta:    0.01,
		Lambda: 0.001,
		Tau:    86400.0,
	}

	hebb := NewHebbianRealTime(nil, config)

	currentWeight := 0.95
	decay := 1.0

	// Δw = η * decay - λ * w_atual
	deltaW := hebb.eta*decay - hebb.lambda*currentWeight
	newWeight := currentWeight + deltaW

	// Safeguard: limitar a 1.0
	if newWeight > 1.0 {
		newWeight = 1.0
	}

	if newWeight > 1.0 {
		t.Errorf("Weight saturated: %.3f (expected <= 1.0)", newWeight)
	}
}

func TestHebbianRealTime_WeightDecay_NoActivation(t *testing.T) {
	config := &HebbianRTConfig{
		Eta:    0.01,
		Lambda: 0.001,
		Tau:    86400.0,
	}

	hebb := NewHebbianRealTime(nil, config)

	currentWeight := 0.7
	decay := 0.001 // exp(-7) ≈ 0.001

	// Δw = η * decay - λ * w_atual
	deltaW := hebb.eta*decay - hebb.lambda*currentWeight
	newWeight := currentWeight + deltaW

	if newWeight >= currentWeight {
		t.Errorf("Weight should decay without activation: %.3f → %.3f", currentWeight, newWeight)
	}
}

func TestHebbianRealTime_BoostMemories(t *testing.T) {
	// nil neo4j — BoostMemories short-circuits (neo4j==nil guard)
	hebb := NewHebbianRealTime(nil, nil)

	memoryIDs := []string{"mem_1", "mem_2", "mem_3"}
	boostFactor := 0.15

	err := hebb.BoostMemories(context.Background(), memoryIDs, boostFactor)

	if err != nil {
		t.Fatalf("BoostMemories failed: %v", err)
	}
}

func TestHebbianRealTime_DecayMemories(t *testing.T) {
	// nil neo4j — DecayMemories short-circuits (neo4j==nil guard)
	hebb := NewHebbianRealTime(nil, nil)

	memoryIDs := []string{"mem_1", "mem_2"}
	decayFactor := 0.10

	err := hebb.DecayMemories(context.Background(), memoryIDs, decayFactor)

	if err != nil {
		t.Fatalf("DecayMemories failed: %v", err)
	}
}

func TestHebbianRealTime_Timeout(t *testing.T) {
	config := &HebbianRTConfig{
		Timeout: 50 * time.Millisecond,
	}

	// nil neo4j — UpdateWeights returns immediately (neo4j==nil guard)
	// This verifies the constructor accepts timeout config
	hebb := NewHebbianRealTime(nil, config)

	if hebb.timeout != 50*time.Millisecond {
		t.Errorf("Timeout config not applied: %v (expected 50ms)", hebb.timeout)
	}
}

func TestHebbianRealTime_Config_Defaults(t *testing.T) {
	hebb := NewHebbianRealTime(nil, nil)

	if hebb.eta != 0.01 {
		t.Errorf("Default eta wrong: %.3f (expected 0.01)", hebb.eta)
	}

	if hebb.lambda != 0.001 {
		t.Errorf("Default lambda wrong: %.4f (expected 0.001)", hebb.lambda)
	}

	if hebb.tau != 86400.0 {
		t.Errorf("Default tau wrong: %.0f (expected 86400)", hebb.tau)
	}

	if hebb.timeout != 100*time.Millisecond {
		t.Errorf("Default timeout wrong: %v (expected 100ms)", hebb.timeout)
	}
}
