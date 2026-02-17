// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ram

import (
	"context"
	"fmt"
	"math"
	"time"
)

// HistoricalValidator (E2) - Valida interpretações contra histórico do paciente
type HistoricalValidator struct {
	retrieval          RetrievalService
	embedder           EmbeddingService
	graphStore         GraphStore
	similarityThreshold float64 // 0.75 default
	maxMemoriesToCheck  int     // 20 default
}

// GraphStore interface para Neo4j
type GraphStore interface {
	GetTemporalMemories(ctx context.Context, patientID int64, startDate, endDate time.Time) ([]Memory, error)
	GetRelatedMemories(ctx context.Context, memoryID int64, depth int) ([]Memory, error)
}

// ValidationResult resultado da validação
type ValidationResult struct {
	ConsistencyScore  float64
	SupportingFacts   []SupportingFact
	Contradictions    []Contradiction
	MemoriesChecked   int
	ValidationMethod  string // embedding_similarity, temporal_consistency, etc
}

// NewHistoricalValidator cria novo validador
func NewHistoricalValidator(retrieval RetrievalService, embedder EmbeddingService, graphStore GraphStore) *HistoricalValidator {
	return &HistoricalValidator{
		retrieval:          retrieval,
		embedder:           embedder,
		graphStore:         graphStore,
		similarityThreshold: 0.75,
		maxMemoriesToCheck:  20,
	}
}

// Validate valida interpretação contra histórico
func (v *HistoricalValidator) Validate(ctx context.Context, patientID int64, interp *Interpretation) (*ValidationResult, error) {
	result := &ValidationResult{
		SupportingFacts:  make([]SupportingFact, 0),
		Contradictions:   make([]Contradiction, 0),
		ValidationMethod: "embedding_similarity",
	}

	// 1. Recuperar memórias relevantes para validação
	memories, err := v.retrieval.RetrieveRelevant(ctx, patientID, interp.Content, v.maxMemoriesToCheck)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve memories: %w", err)
	}

	result.MemoriesChecked = len(memories)

	if len(memories) == 0 {
		// Sem memórias para validar
		result.ConsistencyScore = 0.5 // Neutro
		return result, nil
	}

	// 2. Gerar embedding da interpretação
	interpEmbedding, err := v.embedder.GenerateEmbedding(ctx, interp.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// 3. Comparar com cada memória
	totalSimilarity := 0.0
	supportingCount := 0
	contradictionCount := 0

	for _, memory := range memories {
		memoryEmbedding, err := v.embedder.GenerateEmbedding(ctx, memory.Content)
		if err != nil {
			continue
		}

		similarity := v.cosineSimilarity(interpEmbedding, memoryEmbedding)

		if similarity >= v.similarityThreshold {
			// Supporting fact
			fact := SupportingFact{
				MemoryID:   memory.ID,
				Content:    memory.Content,
				Similarity: similarity,
				Timestamp:  memory.Timestamp,
			}
			result.SupportingFacts = append(result.SupportingFacts, fact)
			supportingCount++
			totalSimilarity += similarity

		} else if similarity < 0.3 && memory.Score > 0.8 {
			// Possível contradição: memória relevante mas semanticamente oposta
			contradiction := Contradiction{
				MemoryID: memory.ID,
				Content:  memory.Content,
				Reason:   fmt.Sprintf("Low semantic similarity (%.2f) with high relevance (%.2f)", similarity, memory.Score),
				Severity: v.calculateContradictionSeverity(similarity, memory.Score),
			}
			result.Contradictions = append(result.Contradictions, contradiction)
			contradictionCount++
		}
	}

	// 4. Calcular consistency score
	if supportingCount > 0 {
		avgSimilarity := totalSimilarity / float64(supportingCount)

		// Boost se tem muitos supporting facts
		supportBoost := math.Min(float64(supportingCount)*0.05, 0.2)

		// Penalty se tem contradições
		contradictionPenalty := float64(contradictionCount) * 0.1

		result.ConsistencyScore = avgSimilarity + supportBoost - contradictionPenalty

	} else {
		// Sem supporting facts
		result.ConsistencyScore = 0.3 // Baixo por padrão
	}

	// Clamp [0, 1]
	if result.ConsistencyScore > 1.0 {
		result.ConsistencyScore = 1.0
	}
	if result.ConsistencyScore < 0.0 {
		result.ConsistencyScore = 0.0
	}

	// 5. Validação temporal adicional (se disponível)
	if v.graphStore != nil {
		temporalScore := v.validateTemporalConsistency(ctx, patientID, interp, memories)
		// Ponderar com score embedding
		result.ConsistencyScore = (result.ConsistencyScore * 0.7) + (temporalScore * 0.3)
	}

	return result, nil
}

// validateTemporalConsistency valida consistência temporal
func (v *HistoricalValidator) validateTemporalConsistency(ctx context.Context, patientID int64, interp *Interpretation, memories []Memory) float64 {
	// Verificar se interpretação é consistente com linha temporal

	if len(memories) < 2 {
		return 0.5 // Neutro
	}

	// Ordenar memórias por timestamp
	sortedMemories := make([]Memory, len(memories))
	copy(sortedMemories, memories)

	// Bubble sort simples (poucos elementos)
	for i := 0; i < len(sortedMemories)-1; i++ {
		for j := 0; j < len(sortedMemories)-i-1; j++ {
			if sortedMemories[j].Timestamp.After(sortedMemories[j+1].Timestamp) {
				sortedMemories[j], sortedMemories[j+1] = sortedMemories[j+1], sortedMemories[j]
			}
		}
	}

	// Verificar se há eventos contraditórios na linha temporal
	// Exemplo: "João morreu em 2020" mas interpretação menciona João em 2021
	// TODO: Implementar lógica temporal mais sofisticada

	// Por enquanto, retornar score baseado em recência
	mostRecentMemory := sortedMemories[len(sortedMemories)-1]
	daysSinceLastMemory := time.Since(mostRecentMemory.Timestamp).Hours() / 24

	// Score mais alto se memórias são recentes
	temporalScore := 0.5
	if daysSinceLastMemory < 7 {
		temporalScore = 0.9 // Muito recente
	} else if daysSinceLastMemory < 30 {
		temporalScore = 0.7 // Recente
	} else if daysSinceLastMemory < 90 {
		temporalScore = 0.6 // Moderado
	}

	return temporalScore
}

// calculateContradictionSeverity calcula severidade da contradição
func (v *HistoricalValidator) calculateContradictionSeverity(similarity float64, relevance float64) string {
	// Alta relevância + baixa similaridade = contradição grave
	if relevance > 0.9 && similarity < 0.2 {
		return "high"
	} else if relevance > 0.7 && similarity < 0.3 {
		return "medium"
	}
	return "low"
}

// cosineSimilarity calcula similaridade de cosseno
func (v *HistoricalValidator) cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ValidateBatch valida múltiplas interpretações em batch
func (v *HistoricalValidator) ValidateBatch(ctx context.Context, patientID int64, interpretations []Interpretation) ([]*ValidationResult, error) {
	results := make([]*ValidationResult, len(interpretations))

	for i := range interpretations {
		result, err := v.Validate(ctx, patientID, &interpretations[i])
		if err != nil {
			return nil, fmt.Errorf("failed to validate interpretation %s: %w", interpretations[i].ID, err)
		}
		results[i] = result
	}

	return results, nil
}

// SetSimilarityThreshold ajusta threshold
func (v *HistoricalValidator) SetSimilarityThreshold(threshold float64) {
	if threshold >= 0.0 && threshold <= 1.0 {
		v.similarityThreshold = threshold
	}
}

// SetMaxMemoriesToCheck ajusta limite de memórias
func (v *HistoricalValidator) SetMaxMemoriesToCheck(max int) {
	if max > 0 {
		v.maxMemoriesToCheck = max
	}
}

// GetStats retorna estatísticas do validador
func (v *HistoricalValidator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"similarity_threshold":  v.similarityThreshold,
		"max_memories_to_check": v.maxMemoriesToCheck,
	}
}
