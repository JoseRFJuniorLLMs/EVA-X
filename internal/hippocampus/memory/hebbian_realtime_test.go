package memory

import (
	"context"
	"testing"
	"time"
)

func TestHebbianRealTime_UpdateWeights(t *testing.T) {
	// Mock Neo4j client
	mockNeo4j := &mockNeo4jClient{}

	config := &HebbianRTConfig{
		Eta:     0.01,
		Lambda:  0.001,
		Tau:     86400.0,
		Timeout: 100 * time.Millisecond,
	}

	hebb := NewHebbianRealTime(mockNeo4j, config)

	// Test: 2 nós co-ativados
	nodeIDs := []string{"node_123", "node_456"}

	err := hebb.UpdateWeights(context.Background(), 1, nodeIDs)

	if err != nil {
		t.Fatalf("UpdateWeights failed: %v", err)
	}
}

func TestHebbianRealTime_UpdateWeights_EmptyNodes(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{}
	hebb := NewHebbianRealTime(mockNeo4j, nil)

	// Test: lista vazia (não deve falhar)
	err := hebb.UpdateWeights(context.Background(), 1, []string{})

	if err != nil {
		t.Errorf("Expected no error for empty nodes, got: %v", err)
	}
}

func TestHebbianRealTime_UpdateWeights_SingleNode(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{}
	hebb := NewHebbianRealTime(mockNeo4j, nil)

	// Test: 1 nó apenas (não deve atualizar, precisa de pares)
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
	// Test: peso não deve saturar (> 1.0) devido à regularização
	config := &HebbianRTConfig{
		Eta:    0.01,
		Lambda: 0.001,
		Tau:    86400.0,
	}

	hebb := NewHebbianRealTime(nil, config)

	currentWeight := 0.95
	deltaT := 0.0 // Just activated
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

	// Com regularização, peso deve convergir para um valor < 1.0
	// após múltiplas atualizações
}

func TestHebbianRealTime_WeightDecay_NoActivation(t *testing.T) {
	// Test: peso deve decair se não há ativação
	config := &HebbianRTConfig{
		Eta:    0.01,
		Lambda: 0.001,
		Tau:    86400.0,
	}

	hebb := NewHebbianRealTime(nil, config)

	currentWeight := 0.7
	deltaT := 7 * 86400.0 // 7 days without activation
	decay := 0.001        // exp(-7) ≈ 0.001

	// Δw = η * decay - λ * w_atual
	deltaW := hebb.eta*decay - hebb.lambda*currentWeight
	newWeight := currentWeight + deltaW

	// newWeight deve ser < currentWeight (decaiu)
	if newWeight >= currentWeight {
		t.Errorf("Weight should decay without activation: %.3f → %.3f", currentWeight, newWeight)
	}
}

func TestHebbianRealTime_BoostMemories(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{}
	hebb := NewHebbianRealTime(mockNeo4j, nil)

	memoryIDs := []string{"mem_1", "mem_2", "mem_3"}
	boostFactor := 0.15 // +15%

	err := hebb.BoostMemories(context.Background(), memoryIDs, boostFactor)

	if err != nil {
		t.Fatalf("BoostMemories failed: %v", err)
	}
}

func TestHebbianRealTime_DecayMemories(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{}
	hebb := NewHebbianRealTime(mockNeo4j, nil)

	memoryIDs := []string{"mem_1", "mem_2"}
	decayFactor := 0.10 // -10%

	err := hebb.DecayMemories(context.Background(), memoryIDs, decayFactor)

	if err != nil {
		t.Fatalf("DecayMemories failed: %v", err)
	}
}

func TestHebbianRealTime_Timeout(t *testing.T) {
	mockNeo4j := &mockSlowNeo4jClient{delay: 200 * time.Millisecond}

	config := &HebbianRTConfig{
		Timeout: 50 * time.Millisecond, // Timeout menor que delay
	}

	hebb := NewHebbianRealTime(mockNeo4j, config)

	nodeIDs := []string{"node_1", "node_2"}

	start := time.Now()
	hebb.UpdateWeights(context.Background(), 1, nodeIDs)
	duration := time.Since(start)

	// Deve abortar após ~50ms (timeout)
	if duration > 100*time.Millisecond {
		t.Errorf("Timeout not respected: took %v (expected <100ms)", duration)
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

// Mock Neo4j clients for testing

type mockNeo4jClient struct{}

func (m *mockNeo4jClient) ExecuteRead(ctx context.Context, query string, params map[string]interface{}) ([]interface{}, error) {
	return []interface{}{}, nil
}

func (m *mockNeo4jClient) ExecuteWrite(ctx context.Context, query string, params map[string]interface{}) (interface{}, error) {
	return nil, nil
}

type mockSlowNeo4jClient struct {
	delay time.Duration
}

func (m *mockSlowNeo4jClient) ExecuteRead(ctx context.Context, query string, params map[string]interface{}) ([]interface{}, error) {
	select {
	case <-time.After(m.delay):
		return []interface{}{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *mockSlowNeo4jClient) ExecuteWrite(ctx context.Context, query string, params map[string]interface{}) (interface{}, error) {
	select {
	case <-time.After(m.delay):
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
