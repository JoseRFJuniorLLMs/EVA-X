package memory

import (
	"context"
	"testing"
	"time"
)

func TestEntityResolver_CosineSimilarity(t *testing.T) {
	resolver := &EntityResolver{}

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
			a:        []float32{0.9, 0.1, 0.0},
			b:        []float32{0.8, 0.2, 0.0},
			expected: 0.997, // Alta similaridade
			delta:    0.01,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0},
			b:        []float32{-1.0, 0.0},
			expected: -1.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := resolver.cosineSimilarity(tt.a, tt.b)
			if similarity < tt.expected-tt.delta || similarity > tt.expected+tt.delta {
				t.Errorf("Similarity: expected ~%.3f, got %.3f", tt.expected, similarity)
			}
		})
	}
}

func TestEntityResolver_FindDuplicateEntities(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{
		entities: []map[string]interface{}{
			{
				"node_id":   "entity_1",
				"name":      "Maria",
				"type":      "person",
				"frequency": 10,
			},
			{
				"node_id":   "entity_2",
				"name":      "Dona Maria",
				"type":      "person",
				"frequency": 5,
			},
			{
				"node_id":   "entity_3",
				"name":      "café",
				"type":      "concept",
				"frequency": 8,
			},
		},
	}

	mockEmbedder := &mockEmbeddingService{
		embeddings: map[string][]float32{
			"Maria":      {0.9, 0.1, 0.0},
			"Dona Maria": {0.85, 0.15, 0.0}, // Similar a Maria
			"café":       {0.1, 0.9, 0.0},   // Diferente
		},
	}

	resolver := NewEntityResolver(mockNeo4j, mockEmbedder, nil)
	resolver.SetSimilarityThreshold(0.85)

	// Executar
	candidates, err := resolver.FindDuplicateEntities(context.Background(), 123)

	if err != nil {
		t.Fatalf("FindDuplicateEntities failed: %v", err)
	}

	// Verificar: deve encontrar 1 candidato (Maria vs Dona Maria)
	if len(candidates) != 1 {
		t.Errorf("Expected 1 candidate, got %d", len(candidates))
	}

	if len(candidates) > 0 {
		candidate := candidates[0]

		// Verificar que target é o mais frequente
		if candidate.TargetName != "Maria" {
			t.Errorf("Target should be 'Maria' (more frequent), got '%s'", candidate.TargetName)
		}

		if candidate.SourceName != "Dona Maria" {
			t.Errorf("Source should be 'Dona Maria', got '%s'", candidate.SourceName)
		}

		if candidate.Similarity < 0.85 {
			t.Errorf("Similarity should be >= 0.85, got %.2f", candidate.Similarity)
		}

		if candidate.Confidence != "high" && candidate.Confidence != "medium" {
			t.Errorf("Confidence should be high/medium, got '%s'", candidate.Confidence)
		}
	}
}

func TestEntityResolver_ResolveEntityName(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{
		entities: []map[string]interface{}{
			{
				"node_id":   "entity_1",
				"name":      "Maria Silva",
				"type":      "person",
				"frequency": 10,
			},
		},
	}

	mockEmbedder := &mockEmbeddingService{
		embeddings: map[string][]float32{
			"Maria Silva":     {0.9, 0.1, 0.0},
			"minha mãe Maria": {0.88, 0.12, 0.0}, // Similar
			"João":            {0.1, 0.9, 0.0},   // Diferente
		},
	}

	resolver := NewEntityResolver(mockNeo4j, mockEmbedder, nil)
	resolver.SetSimilarityThreshold(0.85)

	tests := []struct {
		name           string
		input          string
		expectedOutput string
		expectedMatch  bool
	}{
		{
			name:           "exact match",
			input:          "Maria Silva",
			expectedOutput: "Maria Silva",
			expectedMatch:  true,
		},
		{
			name:           "fuzzy match",
			input:          "minha mãe Maria",
			expectedOutput: "Maria Silva", // Resolve para nome canônico
			expectedMatch:  true,
		},
		{
			name:           "no match",
			input:          "João",
			expectedOutput: "João", // Mantém original
			expectedMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, matched, err := resolver.ResolveEntityName(context.Background(), 123, tt.input)

			if err != nil {
				t.Fatalf("ResolveEntityName failed: %v", err)
			}

			if resolved != tt.expectedOutput {
				t.Errorf("Expected '%s', got '%s'", tt.expectedOutput, resolved)
			}

			if matched != tt.expectedMatch {
				t.Errorf("Expected match=%v, got %v", tt.expectedMatch, matched)
			}
		})
	}
}

func TestEntityResolver_MergeSingleEntity(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{
		mergeSuccess: true,
		edgesMoved:   3,
	}

	resolver := &EntityResolver{
		neo4j: mockNeo4j,
	}

	candidate := MergeCandidate{
		SourceID:   "entity_2",
		TargetID:   "entity_1",
		SourceName: "Dona Maria",
		TargetName: "Maria",
		Similarity: 0.92,
	}

	result := resolver.mergeSingleEntity(context.Background(), 123, candidate)

	if !result.Success {
		t.Errorf("Merge should succeed, got error: %v", result.Error)
	}

	if result.EdgesMoved != 3 {
		t.Errorf("Expected 3 edges moved, got %d", result.EdgesMoved)
	}

	if result.SourceID != candidate.SourceID {
		t.Errorf("SourceID mismatch")
	}

	if result.TargetID != candidate.TargetID {
		t.Errorf("TargetID mismatch")
	}
}

func TestEntityResolver_CalculateConfidence(t *testing.T) {
	resolver := &EntityResolver{}

	tests := []struct {
		similarity float64
		expected   string
	}{
		{0.99, "high"},
		{0.95, "high"},
		{0.94, "medium"},
		{0.90, "medium"},
		{0.89, "low"},
		{0.85, "low"},
	}

	for _, tt := range tests {
		confidence := resolver.calculateConfidence(tt.similarity)
		if confidence != tt.expected {
			t.Errorf("Similarity %.2f: expected '%s', got '%s'", tt.similarity, tt.expected, confidence)
		}
	}
}

func TestEntityResolver_AutoResolve_DryRun(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{
		entities: []map[string]interface{}{
			{
				"node_id":   "entity_1",
				"name":      "Maria",
				"type":      "person",
				"frequency": 10,
			},
			{
				"node_id":   "entity_2",
				"name":      "Dona Maria",
				"type":      "person",
				"frequency": 5,
			},
		},
	}

	mockEmbedder := &mockEmbeddingService{
		embeddings: map[string][]float32{
			"Maria":      {0.9, 0.1, 0.0},
			"Dona Maria": {0.88, 0.12, 0.0},
		},
	}

	resolver := NewEntityResolver(mockNeo4j, mockEmbedder, nil)

	// DryRun = true (não executa merges)
	stats, err := resolver.AutoResolve(context.Background(), 123, true)

	if err != nil {
		t.Fatalf("AutoResolve failed: %v", err)
	}

	if stats.CandidatesFound != 1 {
		t.Errorf("Expected 1 candidate, got %d", stats.CandidatesFound)
	}

	if stats.MergesPerformed != 0 {
		t.Errorf("DryRun should not perform merges, got %d", stats.MergesPerformed)
	}

	if stats.Duration == 0 {
		t.Error("Duration should be measured")
	}
}

func TestEntityResolver_AutoResolve_Execute(t *testing.T) {
	mockNeo4j := &mockNeo4jClient{
		entities: []map[string]interface{}{
			{
				"node_id":   "entity_1",
				"name":      "Maria",
				"type":      "person",
				"frequency": 10,
			},
			{
				"node_id":   "entity_2",
				"name":      "Dona Maria",
				"type":      "person",
				"frequency": 5,
			},
		},
		mergeSuccess: true,
		edgesMoved:   3,
	}

	mockEmbedder := &mockEmbeddingService{
		embeddings: map[string][]float32{
			"Maria":      {0.9, 0.1, 0.0},
			"Dona Maria": {0.88, 0.12, 0.0},
		},
	}

	resolver := NewEntityResolver(mockNeo4j, mockEmbedder, nil)

	// DryRun = false (executa merges)
	stats, err := resolver.AutoResolve(context.Background(), 123, false)

	if err != nil {
		t.Fatalf("AutoResolve failed: %v", err)
	}

	if stats.CandidatesFound != 1 {
		t.Errorf("Expected 1 candidate, got %d", stats.CandidatesFound)
	}

	if stats.MergesPerformed != 1 {
		t.Errorf("Expected 1 merge, got %d", stats.MergesPerformed)
	}

	if stats.EdgesConsolidated != 3 {
		t.Errorf("Expected 3 edges consolidated, got %d", stats.EdgesConsolidated)
	}
}

func TestEntityResolver_ThresholdConfiguration(t *testing.T) {
	resolver := NewEntityResolver(nil, nil, nil)

	// Default threshold
	if resolver.GetSimilarityThreshold() != 0.85 {
		t.Errorf("Default threshold should be 0.85, got %.2f", resolver.GetSimilarityThreshold())
	}

	// Set new threshold
	resolver.SetSimilarityThreshold(0.90)
	if resolver.GetSimilarityThreshold() != 0.90 {
		t.Errorf("Threshold should be 0.90, got %.2f", resolver.GetSimilarityThreshold())
	}

	// Invalid threshold (should be ignored)
	resolver.SetSimilarityThreshold(1.5)
	if resolver.GetSimilarityThreshold() != 0.90 {
		t.Errorf("Invalid threshold should be ignored, got %.2f", resolver.GetSimilarityThreshold())
	}
}

// Mock implementations

type mockNeo4jClient struct {
	entities     []map[string]interface{}
	mergeSuccess bool
	edgesMoved   int
}

func (m *mockNeo4jClient) ExecuteQuery(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error) {
	// Detectar tipo de query
	if contains(query, "MATCH (p:Patient)") && contains(query, "MENTIONS") {
		// Query de getPatientEntities
		return m.entities, nil
	}

	if contains(query, "DETACH DELETE source") {
		// Query de merge
		if m.mergeSuccess {
			return []map[string]interface{}{
				{"edges_moved": m.edgesMoved},
			}, nil
		}
	}

	return nil, nil
}

type mockEmbeddingService struct {
	embeddings map[string][]float32
}

func (m *mockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if emb, ok := m.embeddings[text]; ok {
		return emb, nil
	}
	// Default embedding
	return []float32{0.5, 0.5, 0.0}, nil
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr))
}
