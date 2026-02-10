package cognitive

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// UNIT TESTS: Cognitive Load Orchestrator
// ============================================================================

func TestCalculateCognitiveLoad_Normal(t *testing.T) {
	// Test normal conversation metrics
	metrics := ConversationMetrics{
		TopicDepth:         0.5,
		EmotionalIntensity: 0.4,
		ResponseComplexity: 0.5,
		SessionDuration:    20 * time.Minute,
		InteractionCount:   10,
	}

	load := calculateCognitiveLoad(metrics)

	assert.True(t, load >= 0 && load <= 1, "Load should be 0-1")
	assert.True(t, load < 0.6, "Normal conversation should have moderate load")
}

func TestCalculateCognitiveLoad_HighStress(t *testing.T) {
	// Test high stress conversation
	metrics := ConversationMetrics{
		TopicDepth:         0.9,
		EmotionalIntensity: 0.95,
		ResponseComplexity: 0.8,
		SessionDuration:    60 * time.Minute,
		InteractionCount:   50,
	}

	load := calculateCognitiveLoad(metrics)

	assert.True(t, load >= 0.7, "High stress conversation should have high load")
}

func TestCalculateCognitiveLoad_LowActivity(t *testing.T) {
	// Test low activity conversation
	metrics := ConversationMetrics{
		TopicDepth:         0.2,
		EmotionalIntensity: 0.1,
		ResponseComplexity: 0.2,
		SessionDuration:    5 * time.Minute,
		InteractionCount:   3,
	}

	load := calculateCognitiveLoad(metrics)

	assert.True(t, load < 0.3, "Low activity should have low load")
}

func TestRuminationDetection_SameTopic(t *testing.T) {
	// Test detection of same topic repeated
	interactions := []TopicInteraction{
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-90 * time.Minute)},
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-60 * time.Minute)},
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-30 * time.Minute)},
		{Topic: "death_of_spouse", Timestamp: time.Now()},
	}

	isRuminating := detectRumination(interactions, 2*time.Hour, 3)

	assert.True(t, isRuminating, "Should detect rumination on same topic 4x in 2h")
}

func TestRuminationDetection_DifferentTopics(t *testing.T) {
	// Test no rumination with different topics
	interactions := []TopicInteraction{
		{Topic: "weather", Timestamp: time.Now().Add(-90 * time.Minute)},
		{Topic: "health", Timestamp: time.Now().Add(-60 * time.Minute)},
		{Topic: "family", Timestamp: time.Now().Add(-30 * time.Minute)},
		{Topic: "medication", Timestamp: time.Now()},
	}

	isRuminating := detectRumination(interactions, 2*time.Hour, 3)

	assert.False(t, isRuminating, "Should not detect rumination with varied topics")
}

func TestRuminationDetection_OutsideWindow(t *testing.T) {
	// Test topics repeated but outside time window
	interactions := []TopicInteraction{
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-5 * time.Hour)},
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-4 * time.Hour)},
		{Topic: "death_of_spouse", Timestamp: time.Now().Add(-3 * time.Hour)},
		{Topic: "death_of_spouse", Timestamp: time.Now()},
	}

	isRuminating := detectRumination(interactions, 2*time.Hour, 3)

	// Only 1 interaction in last 2 hours
	assert.False(t, isRuminating, "Should not detect rumination outside time window")
}

func TestFatigueDetection_LongSession(t *testing.T) {
	// Test fatigue after long session
	metrics := SessionMetrics{
		Duration:             90 * time.Minute,
		InteractionCount:     60,
		AverageResponseTime:  1500 * time.Millisecond,
		LateResponseCount:    15,
		EnergyTrend:          -0.3, // Decreasing energy
	}

	fatigueLevel := calculateFatigue(metrics)

	assert.True(t, fatigueLevel > 0.6, "Long session with declining energy should show fatigue")
}

func TestFatigueDetection_FreshSession(t *testing.T) {
	// Test low fatigue in fresh session
	metrics := SessionMetrics{
		Duration:             10 * time.Minute,
		InteractionCount:     5,
		AverageResponseTime:  500 * time.Millisecond,
		LateResponseCount:    0,
		EnergyTrend:          0.1,
	}

	fatigueLevel := calculateFatigue(metrics)

	assert.True(t, fatigueLevel < 0.3, "Fresh session should show low fatigue")
}

func TestActionBlocking_HighLoad(t *testing.T) {
	// Test that complex actions are blocked under high load
	state := CognitiveLoadState{
		CurrentLoad:        0.85,
		FatigueLevel:       0.7,
		EmotionalIntensity: 0.8,
	}

	allowed := isActionAllowed(state, "complex_decision", 0.6)
	assert.False(t, allowed, "Complex action should be blocked under high load")

	allowed = isActionAllowed(state, "simple_response", 0.9)
	assert.True(t, allowed, "Simple action should be allowed even under high load")
}

func TestActionBlocking_NormalLoad(t *testing.T) {
	// Test that all actions are allowed under normal load
	state := CognitiveLoadState{
		CurrentLoad:        0.4,
		FatigueLevel:       0.3,
		EmotionalIntensity: 0.4,
	}

	allowed := isActionAllowed(state, "complex_decision", 0.6)
	assert.True(t, allowed, "Complex action should be allowed under normal load")
}

func TestSystemInstructions_HighLoad(t *testing.T) {
	// Test system instructions adapt to high load
	state := CognitiveLoadState{
		CurrentLoad:        0.8,
		FatigueLevel:       0.7,
		EmotionalIntensity: 0.6,
	}

	instructions := generateAdaptiveInstructions(state)

	assert.Contains(t, instructions, "breve", "Should recommend brief responses")
	assert.Contains(t, instructions, "simples", "Should recommend simple language")
}

func TestSystemInstructions_Rumination(t *testing.T) {
	// Test system instructions for rumination
	state := CognitiveLoadState{
		CurrentLoad:    0.7,
		IsRuminating:   true,
		RuminationTopic: "death_of_spouse",
	}

	instructions := generateAdaptiveInstructions(state)

	assert.Contains(t, instructions, "redirecionar", "Should recommend topic redirection")
}

// ============================================================================
// HELPER TYPES FOR TESTING
// ============================================================================

type ConversationMetrics struct {
	TopicDepth         float64
	EmotionalIntensity float64
	ResponseComplexity float64
	SessionDuration    time.Duration
	InteractionCount   int
}

type TopicInteraction struct {
	Topic     string
	Timestamp time.Time
}

type SessionMetrics struct {
	Duration             time.Duration
	InteractionCount     int
	AverageResponseTime  time.Duration
	LateResponseCount    int
	EnergyTrend          float64
}

type CognitiveLoadState struct {
	CurrentLoad        float64
	FatigueLevel       float64
	EmotionalIntensity float64
	IsRuminating       bool
	RuminationTopic    string
}

// ============================================================================
// HELPER FUNCTIONS FOR TESTING
// ============================================================================

func calculateCognitiveLoad(m ConversationMetrics) float64 {
	// Weighted calculation
	load := (m.TopicDepth*0.3 + m.EmotionalIntensity*0.4 + m.ResponseComplexity*0.3)

	// Duration factor (increases load after 30 min)
	durationFactor := float64(m.SessionDuration.Minutes()) / 60.0
	if durationFactor > 1 {
		durationFactor = 1
	}
	load += durationFactor * 0.2

	// Interaction density factor
	interactionsPerHour := float64(m.InteractionCount) / (float64(m.SessionDuration.Minutes()) / 60.0)
	if interactionsPerHour > 30 {
		load += 0.1
	}

	if load > 1 {
		load = 1
	}
	return load
}

func detectRumination(interactions []TopicInteraction, window time.Duration, threshold int) bool {
	now := time.Now()
	topicCounts := make(map[string]int)

	for _, interaction := range interactions {
		if now.Sub(interaction.Timestamp) <= window {
			topicCounts[interaction.Topic]++
		}
	}

	for _, count := range topicCounts {
		if count >= threshold {
			return true
		}
	}
	return false
}

func calculateFatigue(m SessionMetrics) float64 {
	fatigue := 0.0

	// Duration factor
	durationHours := m.Duration.Hours()
	if durationHours > 0.5 {
		fatigue += durationHours * 0.3
	}

	// Response time factor (slower = more fatigued)
	if m.AverageResponseTime > time.Second {
		fatigue += 0.2
	}

	// Late response factor
	lateRatio := float64(m.LateResponseCount) / float64(m.InteractionCount)
	fatigue += lateRatio * 0.3

	// Energy trend factor
	if m.EnergyTrend < 0 {
		fatigue += (-m.EnergyTrend) * 0.3
	}

	if fatigue > 1 {
		fatigue = 1
	}
	return fatigue
}

func isActionAllowed(state CognitiveLoadState, actionType string, threshold float64) bool {
	// Simple actions always allowed
	if actionType == "simple_response" || actionType == "acknowledgment" {
		return true
	}

	// Complex actions blocked under high load
	return state.CurrentLoad < threshold
}

func generateAdaptiveInstructions(state CognitiveLoadState) string {
	instructions := "Diretrizes adaptativas:\n"

	if state.CurrentLoad > 0.7 {
		instructions += "- Use frases curtas e breves\n"
		instructions += "- Linguagem simples e direta\n"
		instructions += "- Evite tópicos complexos\n"
	}

	if state.IsRuminating {
		instructions += "- Tente redirecionar gentilmente para outro assunto\n"
		instructions += "- Não alimente o ciclo de pensamento\n"
	}

	if state.FatigueLevel > 0.6 {
		instructions += "- Sugira uma pausa\n"
		instructions += "- Pergunte se quer continuar depois\n"
	}

	return instructions
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkCognitiveLoadCalculation(b *testing.B) {
	metrics := ConversationMetrics{
		TopicDepth:         0.6,
		EmotionalIntensity: 0.5,
		ResponseComplexity: 0.5,
		SessionDuration:    30 * time.Minute,
		InteractionCount:   20,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calculateCognitiveLoad(metrics)
	}
}

func BenchmarkRuminationDetection(b *testing.B) {
	interactions := make([]TopicInteraction, 100)
	for i := 0; i < 100; i++ {
		interactions[i] = TopicInteraction{
			Topic:     "topic_" + string(rune('a'+i%5)),
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detectRumination(interactions, 2*time.Hour, 3)
	}
}
