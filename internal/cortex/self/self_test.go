package self

import (
	"context"
	"testing"
)

// MockEmbeddingService para testes
type MockEmbeddingService struct {
	embeddings map[string][]float32
}

func NewMockEmbeddingService() *MockEmbeddingService {
	return &MockEmbeddingService{
		embeddings: make(map[string][]float32),
	}
}

func (m *MockEmbeddingService) GetEmbedding(ctx context.Context, text string) ([]float32, error) {
	// Simula embeddings baseados no comprimento do texto
	// Textos similares terão embeddings similares
	embedding := make([]float32, 10)
	for i := range embedding {
		embedding[i] = float32(len(text)%10) / 10.0
	}
	return embedding, nil
}

func (m *MockEmbeddingService) GetEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		emb, err := m.GetEmbedding(ctx, text)
		if err != nil {
			return nil, err
		}
		embeddings[i] = emb
	}
	return embeddings, nil
}

// TestCosineSimilarity testa cálculo de similaridade coseno
func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		vec1     []float32
		vec2     []float32
		expected float64
		tolerance float64
	}{
		{
			name:      "vetores idênticos",
			vec1:      []float32{1.0, 0.0, 0.0},
			vec2:      []float32{1.0, 0.0, 0.0},
			expected:  1.0,
			tolerance: 0.01,
		},
		{
			name:      "vetores ortogonais",
			vec1:      []float32{1.0, 0.0},
			vec2:      []float32{0.0, 1.0},
			expected:  0.0,
			tolerance: 0.01,
		},
		{
			name:      "vetores similares",
			vec1:      []float32{1.0, 1.0},
			vec2:      []float32{1.0, 0.9},
			expected:  0.99,
			tolerance: 0.02,
		},
		{
			name:      "vetores opostos",
			vec1:      []float32{1.0, 0.0},
			vec2:      []float32{-1.0, 0.0},
			expected:  -1.0,
			tolerance: 0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.vec1, tt.vec2)
			diff := result - tt.expected
			if diff < 0 {
				diff = -diff
			}
			if diff > tt.tolerance {
				t.Errorf("cosineSimilarity() = %v, esperado %v (±%v)", result, tt.expected, tt.tolerance)
			}
		})
	}
}

// TestSemanticDeduplicator testa deduplicação semântica
func TestSemanticDeduplicator(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)

	ctx := context.Background()

	t.Run("sem memórias existentes - não é duplicata", func(t *testing.T) {
		result, err := dedup.CheckDuplicate(ctx, "nova memória", []ExistingMemory{})
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
		if result.IsDuplicate {
			t.Error("não deveria ser duplicata quando não há memórias existentes")
		}
	})

	t.Run("memória similar - é duplicata", func(t *testing.T) {
		existing := []ExistingMemory{
			{
				ID:      "mem1",
				Content: "nova memória",
				Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			},
		}

		result, err := dedup.CheckDuplicate(ctx, "nova memória", existing)
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}

		// Textos idênticos devem ter alta similaridade
		if result.Similarity < 0.9 {
			t.Errorf("similaridade muito baixa para textos idênticos: %v", result.Similarity)
		}
	})
}

// TestClusterSimilarMemories testa agrupamento de memórias
func TestClusterSimilarMemories(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)

	memories := []ExistingMemory{
		{
			ID:        "mem1",
			Content:   "aprendi sobre empatia",
			Embedding: []float32{0.9, 0.1, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		},
		{
			ID:        "mem2",
			Content:   "aprendi sobre empatia também",
			Embedding: []float32{0.9, 0.1, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		},
		{
			ID:        "mem3",
			Content:   "aprendi sobre ansiedade",
			Embedding: []float32{0.1, 0.9, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
		},
	}

	clusters := dedup.ClusterSimilarMemories(memories, 0.95)

	if len(clusters) < 2 {
		t.Errorf("esperava pelo menos 2 clusters, got %d", len(clusters))
	}
}

// TestGetSimilarMemories testa busca de memórias similares
func TestGetSimilarMemories(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)
	ctx := context.Background()

	memories := []ExistingMemory{
		{
			ID:        "mem1",
			Content:   "empatia é importante",
			Embedding: []float32{0.9, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 5,
		},
		{
			ID:        "mem2",
			Content:   "ansiedade pode ser gerenciada",
			Embedding: []float32{0.1, 0.9, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 3,
		},
		{
			ID:        "mem3",
			Content:   "escutar é fundamental",
			Embedding: []float32{0.8, 0.2, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 8,
		},
	}

	results, err := dedup.GetSimilarMemories(ctx, "empatia", memories, 2)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("esperava 2 resultados, got %d", len(results))
	}

	// Primeiro resultado deve ter maior similaridade
	if len(results) >= 2 && results[0].Similarity < results[1].Similarity {
		t.Error("resultados não estão ordenados por similaridade")
	}
}

// TestCalculateDiversityScore testa cálculo de diversidade
func TestCalculateDiversityScore(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)

	t.Run("memórias idênticas - baixa diversidade", func(t *testing.T) {
		memories := []ExistingMemory{
			{ID: "1", Embedding: []float32{1.0, 0.0}},
			{ID: "2", Embedding: []float32{1.0, 0.0}},
			{ID: "3", Embedding: []float32{1.0, 0.0}},
		}

		score := dedup.CalculateDiversityScore(memories)
		if score > 0.1 {
			t.Errorf("esperava baixa diversidade (<0.1), got %v", score)
		}
	})

	t.Run("memórias ortogonais - alta diversidade", func(t *testing.T) {
		memories := []ExistingMemory{
			{ID: "1", Embedding: []float32{1.0, 0.0, 0.0}},
			{ID: "2", Embedding: []float32{0.0, 1.0, 0.0}},
			{ID: "3", Embedding: []float32{0.0, 0.0, 1.0}},
		}

		score := dedup.CalculateDiversityScore(memories)
		if score < 0.8 {
			t.Errorf("esperava alta diversidade (>0.8), got %v", score)
		}
	})
}

// TestSuggestMemoryConsolidation testa sugestões de consolidação
func TestSuggestMemoryConsolidation(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)

	memories := []ExistingMemory{
		{
			ID:                 "mem1",
			Content:            "empatia é chave",
			Embedding:          []float32{0.9, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 10,
		},
		{
			ID:                 "mem2",
			Content:            "empatia importa",
			Embedding:          []float32{0.9, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 5,
		},
		{
			ID:                 "mem3",
			Content:            "ser empático ajuda",
			Embedding:          []float32{0.9, 0.1, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0, 0.0},
			ReinforcementCount: 3,
		},
	}

	suggestions := dedup.SuggestMemoryConsolidation(memories)

	if len(suggestions) != 1 {
		t.Errorf("esperava 1 sugestão de consolidação, got %d", len(suggestions))
	}

	if len(suggestions) > 0 {
		// Deve manter a memória mais reforçada (mem1 com count=10)
		if suggestions[0].KeepMemoryID != "mem1" {
			t.Errorf("deveria manter mem1 (mais reforçada), got %s", suggestions[0].KeepMemoryID)
		}

		if suggestions[0].ClusterSize != 3 {
			t.Errorf("esperava cluster de 3 memórias, got %d", suggestions[0].ClusterSize)
		}
	}
}

// TestGetDeduplicationStats testa estatísticas
func TestGetDeduplicationStats(t *testing.T) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)

	checks := []DuplicateCheckResult{
		{IsDuplicate: true, Similarity: 0.95},
		{IsDuplicate: true, Similarity: 0.92},
		{IsDuplicate: false, Similarity: 0.65},
		{IsDuplicate: false, Similarity: 0.70},
	}

	stats := dedup.GetDeduplicationStats(checks)

	totalChecks, ok := stats["total_checks"].(int)
	if !ok || totalChecks != 4 {
		t.Errorf("total_checks incorreto: %v", stats["total_checks"])
	}

	duplicatesFound, ok := stats["duplicates_found"].(int)
	if !ok || duplicatesFound != 2 {
		t.Errorf("duplicates_found incorreto: %v", stats["duplicates_found"])
	}

	duplicateRate, ok := stats["duplicate_rate"].(float64)
	if !ok || duplicateRate != 50.0 {
		t.Errorf("duplicate_rate incorreto: %v", stats["duplicate_rate"])
	}
}

// TestMemoryType valida tipos de memória
func TestMemoryType(t *testing.T) {
	validTypes := []MemoryType{
		MemoryTypeLesson,
		MemoryTypePattern,
		MemoryTypeMetaInsight,
		MemoryTypeSelfCritique,
		MemoryTypeEmotionalRule,
	}

	for _, mt := range validTypes {
		if string(mt) == "" {
			t.Errorf("MemoryType vazio: %v", mt)
		}
	}
}

// TestAbstractionLevel valida níveis de abstração
func TestAbstractionLevel(t *testing.T) {
	validLevels := []AbstractionLevel{
		AbstractionConcrete,
		AbstractionTactical,
		AbstractionStrategic,
		AbstractionPhilosophical,
	}

	for _, al := range validLevels {
		if string(al) == "" {
			t.Errorf("AbstractionLevel vazio: %v", al)
		}
	}
}

// TestEvaSelfInitialization testa inicialização de personalidade
func TestEvaSelfInitialization(t *testing.T) {
	self := EvaSelf{
		Openness:          0.85,
		Conscientiousness: 0.75,
		Extraversion:      0.40,
		Agreeableness:     0.88,
		Neuroticism:       0.15,
		PrimaryType:       2,
		Wing:              1,
		SelfDescription:   "Guardiã digital",
		CoreValues:        []string{"empatia", "privacidade"},
	}

	// Valida Big Five (devem estar entre 0 e 1)
	traits := []float64{
		self.Openness,
		self.Conscientiousness,
		self.Extraversion,
		self.Agreeableness,
		self.Neuroticism,
	}

	for i, trait := range traits {
		if trait < 0.0 || trait > 1.0 {
			t.Errorf("Trait %d fora do range [0,1]: %v", i, trait)
		}
	}

	// Valida Enneagram (1-9)
	if self.PrimaryType < 1 || self.PrimaryType > 9 {
		t.Errorf("PrimaryType inválido: %d", self.PrimaryType)
	}

	if self.Wing < 1 || self.Wing > 9 {
		t.Errorf("Wing inválido: %d", self.Wing)
	}
}

// BenchmarkCosineSimilarity benchmark do cálculo de similaridade
func BenchmarkCosineSimilarity(b *testing.B) {
	vec1 := make([]float32, 768) // Tamanho típico de embedding
	vec2 := make([]float32, 768)

	for i := range vec1 {
		vec1[i] = float32(i) / 768.0
		vec2[i] = float32(i+1) / 768.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cosineSimilarity(vec1, vec2)
	}
}

// BenchmarkCheckDuplicate benchmark da verificação de duplicatas
func BenchmarkCheckDuplicate(b *testing.B) {
	mockEmbedding := NewMockEmbeddingService()
	dedup := NewSemanticDeduplicator(mockEmbedding, 0.88)
	ctx := context.Background()

	// Cria 100 memórias existentes
	existing := make([]ExistingMemory, 100)
	for i := range existing {
		existing[i] = ExistingMemory{
			ID:        string(rune(i)),
			Content:   "test memory",
			Embedding: []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dedup.CheckDuplicate(ctx, "nova memória", existing)
	}
}
