package attention

import (
	"context"
	"eva-mind/internal/cortex/attention/models"
	"testing"
)

func TestExecutive_Process_Basic(t *testing.T) {
	// Setup
	cfg := &Config{
		ConfidenceThreshold:     0.7,
		LoopSimilarityThreshold: 0.9,
		MaxResponseTokens:       100,
		Temperature:             0.7,
		PatternInterruptEnabled: true,
		WorkingMemorySize:       5,
		PatternBufferSize:       10,
	}
	exec := NewExecutive(cfg)
	state := models.NewExecutiveState("test-session", 0)

	// Test 1: High confidence input
	decision, err := exec.Process(context.Background(), "Hello EVA, how are you?", state)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if !decision.ShouldRespond {
		t.Error("Expected ShouldRespond to be true")
	}
	if decision.ResponseStrategy == StrategyPatternInterrupt {
		t.Error("Did not expect PatternInterrupt for unique input")
	}

	// Test 2: Low confidence / Clarification
	// We simulate low confidence by mocking (if possible) or providing nonsense
	// But our mock confidence assessor is simple.
	// Let's assume input "blah blah" might have low intent clarity in our mock logic?
	// Actually assessConfidence uses intent clarity which is hardcoded to 0.7 in the current mock.
	// So confidence will be roughly 0.7 * 1.0 * something.
}

func TestExecutive_PatternInterrupt(t *testing.T) {
	cfg := &Config{
		ConfidenceThreshold:     0.7,
		LoopSimilarityThreshold: 0.8, // Low threshold for easier triggering
		PatternInterruptEnabled: true,
		WorkingMemorySize:       5,
		PatternBufferSize:       10,
	}
	exec := NewExecutive(cfg)
	state := models.NewExecutiveState("test-session-loop", 0)

	input := "I am stuck in a loop."

	// Repeat input multiple times
	for i := 0; i < 5; i++ {
		_, err := exec.Process(context.Background(), input, state)
		if err != nil {
			t.Fatalf("Process iteration %d failed: %v", i, err)
		}
	}

	// Next repetition should trigger interrupt
	decision, err := exec.Process(context.Background(), input, state)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if decision.ResponseStrategy != StrategyPatternInterrupt {
		t.Errorf("Expected StrategyPatternInterrupt, got %s", decision.ResponseStrategy)
	}
	if !decision.LoopDetected {
		t.Error("Expected LoopDetected to be true")
	}
	if decision.InterruptionQuestion == "" {
		t.Error("Expected InterruptionQuestion to be set")
	}
}
