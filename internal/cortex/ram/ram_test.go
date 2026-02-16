package ram

import (
	"context"
	"testing"
	"time"
)

func TestRAMEngine_Process(t *testing.T) {
	mockLLM := &mockLLMService{
		responses: []string{
			"Interpretação 1: O paciente pergunta sobre café da manhã de ontem",
			"Interpretação 2: O paciente quer saber se tomou café recentemente",
			"Interpretação 3: O paciente está confuso sobre quando foi a última vez que tomou café",
		},
	}

	mockEmbedder := &mockEmbeddingService{}
	mockRetrieval := &mockRetrievalService{
		memories: []Memory{
			{ID: 1, Content: "Tomei café com Maria ontem", Score: 0.9},
			{ID: 2, Content: "Gosto muito de café pela manhã", Score: 0.7},
		},
	}

	generator := NewInterpretationGenerator(mockLLM, mockEmbedder, mockRetrieval)
	validator := NewHistoricalValidator(mockRetrieval, mockEmbedder, nil)
	feedback := NewFeedbackLoop(nil, nil, nil)

	config := DefaultRAMConfig()
	engine := NewRAMEngine(generator, validator, feedback, config)

	// Test: processar query
	response, err := engine.Process(context.Background(), 123, "O que eu tomei ontem?", "Paciente com Alzheimer")

	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	if response == nil {
		t.Fatal("Response is nil")
	}

	if len(response.Interpretations) != 3 {
		t.Errorf("Expected 3 interpretations, got %d", len(response.Interpretations))
	}

	if response.BestInterpretation == nil {
		t.Error("BestInterpretation is nil")
	}

	if response.Confidence == 0.0 {
		t.Error("Confidence should not be 0")
	}
}

func TestRAMEngine_CalculateCombinedScore(t *testing.T) {
	engine := &RAMEngine{
		config: DefaultRAMConfig(),
	}

	tests := []struct {
		name                string
		plausibility        float64
		historical          float64
		confidence          float64
		contradictionsCount int
		expectedRange       [2]float64
	}{
		{
			name:          "high scores no contradictions",
			plausibility:  0.9,
			historical:    0.8,
			confidence:    0.85,
			expectedRange: [2]float64{0.8, 0.9},
		},
		{
			name:                "high scores with contradictions",
			plausibility:        0.9,
			historical:          0.8,
			confidence:          0.85,
			contradictionsCount: 2,
			expectedRange:       [2]float64{0.6, 0.8},
		},
		{
			name:          "low scores",
			plausibility:  0.3,
			historical:    0.4,
			confidence:    0.5,
			expectedRange: [2]float64{0.3, 0.5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interp := Interpretation{
				PlausibilityScore: tt.plausibility,
				HistoricalScore:   tt.historical,
				Confidence:        tt.confidence,
				Contradictions:    make([]Contradiction, tt.contradictionsCount),
			}

			score := engine.calculateCombinedScore(&interp)

			if score < tt.expectedRange[0] || score > tt.expectedRange[1] {
				t.Errorf("Score %.2f out of expected range [%.2f, %.2f]", score, tt.expectedRange[0], tt.expectedRange[1])
			}
		})
	}
}

func TestRAMEngine_ShouldRequireReview(t *testing.T) {
	engine := &RAMEngine{
		config: DefaultRAMConfig(),
	}

	tests := []struct {
		name               string
		interpretations    []Interpretation
		expectReview       bool
		expectedReasonPart string
	}{
		{
			name: "high confidence - no review",
			interpretations: []Interpretation{
				{CombinedScore: 0.85, Contradictions: []Contradiction{}},
			},
			expectReview: false,
		},
		{
			name: "low confidence - requires review",
			interpretations: []Interpretation{
				{CombinedScore: 0.4, Contradictions: []Contradiction{}},
			},
			expectReview:       true,
			expectedReasonPart: "low_confidence",
		},
		{
			name: "high severity contradiction - requires review",
			interpretations: []Interpretation{
				{
					CombinedScore: 0.85,
					Contradictions: []Contradiction{
						{Severity: "high"},
					},
				},
			},
			expectReview:       true,
			expectedReasonPart: "contradiction",
		},
		{
			name: "ambiguous scores - requires review",
			interpretations: []Interpretation{
				{CombinedScore: 0.75},
				{CombinedScore: 0.72},
			},
			expectReview:       true,
			expectedReasonPart: "ambiguous",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requiresReview, reason := engine.shouldRequireReview(tt.interpretations)

			if requiresReview != tt.expectReview {
				t.Errorf("Expected requiresReview=%v, got %v", tt.expectReview, requiresReview)
			}

			if tt.expectReview && tt.expectedReasonPart != "" {
				if len(reason) == 0 {
					t.Error("Expected reason but got empty string")
				}
			}
		})
	}
}

func TestInterpretationGenerator_EstimateConfidence(t *testing.T) {
	generator := &InterpretationGenerator{}

	memories := []Memory{
		{Content: "Tomei café com Maria"},
		{Content: "Maria é minha filha"},
	}

	tests := []struct {
		text     string
		expected [2]float64 // min, max
	}{
		{
			text:     "café com Maria",
			expected: [2]float64{0.6, 1.0},
		},
		{
			text:     "João e Pedro",
			expected: [2]float64{0.3, 0.5},
		},
	}

	for _, tt := range tests {
		confidence := generator.estimateConfidence(tt.text, memories)
		if confidence < tt.expected[0] || confidence > tt.expected[1] {
			t.Errorf("Confidence %.2f out of range [%.2f, %.2f]", confidence, tt.expected[0], tt.expected[1])
		}
	}
}

func TestHistoricalValidator_CosineSimilarity(t *testing.T) {
	validator := &HistoricalValidator{}

	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float64
		delta    float64
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{0.0, 1.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "similar vectors",
			a:        []float32{0.9, 0.1},
			b:        []float32{0.8, 0.2},
			expected: 0.997,
			delta:    0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := validator.cosineSimilarity(tt.a, tt.b)
			if similarity < tt.expected-tt.delta || similarity > tt.expected+tt.delta {
				t.Errorf("Similarity: expected ~%.3f, got %.3f", tt.expected, similarity)
			}
		})
	}
}

func TestFeedbackLoop_Apply(t *testing.T) {
	mockHebbian := &mockHebbianRT{
		boostCalled: false,
		decayCalled: false,
	}

	feedbackLoop := NewFeedbackLoop(mockHebbian, nil, nil)

	// Test: feedback positivo
	feedback := &Feedback{
		PatientID:        123,
		InterpretationID: "interp_1",
		Correct:          true,
		Timestamp:        time.Now(),
	}

	err := feedbackLoop.Apply(context.Background(), feedback)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if !mockHebbian.boostCalled {
		t.Error("BoostWeight should have been called for correct feedback")
	}

	// Test: feedback negativo
	mockHebbian.boostCalled = false
	mockHebbian.decayCalled = false

	feedback.Correct = false

	err = feedbackLoop.Apply(context.Background(), feedback)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if !mockHebbian.decayCalled {
		t.Error("DecayWeight should have been called for incorrect feedback")
	}
}

func TestFeedbackLoop_GetStats(t *testing.T) {
	mockDB := &mockDatabase{
		feedbacks: []Feedback{
			{Correct: true, NodesAffected: []string{"node_1", "node_2"}},
			{Correct: true, NodesAffected: []string{"node_1"}},
			{Correct: false, NodesAffected: []string{"node_2"}},
		},
	}

	feedbackLoop := NewFeedbackLoop(nil, nil, mockDB)

	stats, err := feedbackLoop.GetStats(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if stats.TotalFeedbacks != 3 {
		t.Errorf("Expected 3 total feedbacks, got %d", stats.TotalFeedbacks)
	}

	if stats.CorrectCount != 2 {
		t.Errorf("Expected 2 correct, got %d", stats.CorrectCount)
	}

	if stats.IncorrectCount != 1 {
		t.Errorf("Expected 1 incorrect, got %d", stats.IncorrectCount)
	}

	expectedAccuracy := 2.0 / 3.0
	if stats.AccuracyRate < expectedAccuracy-0.01 || stats.AccuracyRate > expectedAccuracy+0.01 {
		t.Errorf("Expected accuracy ~%.2f, got %.2f", expectedAccuracy, stats.AccuracyRate)
	}
}

// Mock implementations

type mockLLMService struct {
	responses []string
}

func (m *mockLLMService) GenerateText(ctx context.Context, prompt string, temperature float64) (string, error) {
	if len(m.responses) > 0 {
		return m.responses[0], nil
	}
	return "Mock response", nil
}

func (m *mockLLMService) GenerateMultiple(ctx context.Context, prompt string, n int, temperature float64) ([]string, error) {
	if len(m.responses) >= n {
		return m.responses[:n], nil
	}
	return m.responses, nil
}

type mockEmbeddingService struct{}

func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return []float32{0.5, 0.5, 0.0}, nil
}

type mockRetrievalService struct {
	memories []Memory
}

func (m *mockRetrievalService) RetrieveRelevant(ctx context.Context, patientID int64, query string, k int) ([]Memory, error) {
	return m.memories, nil
}

type mockHebbianRT struct {
	boostCalled bool
	decayCalled bool
}

func (m *mockHebbianRT) UpdateWeights(ctx context.Context, patientID int64, nodeIDs []string) error {
	return nil
}

func (m *mockHebbianRT) BoostWeight(ctx context.Context, sourceID, targetID string, factor float64) error {
	m.boostCalled = true
	return nil
}

func (m *mockHebbianRT) DecayWeight(ctx context.Context, sourceID, targetID string, factor float64) error {
	m.decayCalled = true
	return nil
}

type mockDatabase struct {
	feedbacks []Feedback
}

func (m *mockDatabase) StoreFeedback(ctx context.Context, feedback *Feedback) error {
	return nil
}

func (m *mockDatabase) GetFeedbackHistory(ctx context.Context, patientID int64, limit int) ([]Feedback, error) {
	return m.feedbacks, nil
}
