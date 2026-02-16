package self

import (
	"context"
	"fmt"
	"math"
)

// SemanticDeduplicator detecta duplicatas usando similaridade de embeddings
type SemanticDeduplicator struct {
	embeddingService  EmbeddingService
	similarityThreshold float64 // 0.88 = muito similar
}

// EmbeddingService interface para gerar embeddings
type EmbeddingService interface {
	GetEmbedding(ctx context.Context, text string) ([]float32, error)
	GetEmbeddingBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// DuplicateCheckResult resultado da verificação de duplicata
type DuplicateCheckResult struct {
	IsDuplicate      bool
	ExistingMemoryID string
	Similarity       float64
	ShouldReinforce  bool // true se deve reforçar ao invés de criar novo
}

// NewSemanticDeduplicator cria o deduplicador
func NewSemanticDeduplicator(embeddingService EmbeddingService, threshold float64) *SemanticDeduplicator {
	if threshold == 0 {
		threshold = 0.88 // Default: 88% de similaridade
	}

	return &SemanticDeduplicator{
		embeddingService:    embeddingService,
		similarityThreshold: threshold,
	}
}

// CheckDuplicate verifica se uma memória é duplicata
func (sd *SemanticDeduplicator) CheckDuplicate(
	ctx context.Context,
	newMemoryText string,
	existingMemories []ExistingMemory,
) (*DuplicateCheckResult, error) {

	if len(existingMemories) == 0 {
		return &DuplicateCheckResult{
			IsDuplicate: false,
		}, nil
	}

	// Gera embedding da nova memória
	newEmbedding, err := sd.embeddingService.GetEmbedding(ctx, newMemoryText)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding: %w", err)
	}

	// Compara com memórias existentes
	var mostSimilar ExistingMemory
	var maxSimilarity float64 = 0.0

	for _, existing := range existingMemories {
		similarity := cosineSimilarity(newEmbedding, existing.Embedding)
		if similarity > maxSimilarity {
			maxSimilarity = similarity
			mostSimilar = existing
		}
	}

	// Decide se é duplicata
	isDuplicate := maxSimilarity >= sd.similarityThreshold

	return &DuplicateCheckResult{
		IsDuplicate:      isDuplicate,
		ExistingMemoryID: mostSimilar.ID,
		Similarity:       maxSimilarity,
		ShouldReinforce:  isDuplicate, // Se é duplicata, reforça ao invés de criar
	}, nil
}

// ExistingMemory representa uma memória já armazenada
type ExistingMemory struct {
	ID        string
	Content   string
	Embedding []float32
	ReinforcementCount int
}

// cosineSimilarity calcula similaridade coseno entre dois vetores
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// FindDuplicatesInBatch encontra duplicatas em um lote de memórias
func (sd *SemanticDeduplicator) FindDuplicatesInBatch(
	ctx context.Context,
	newMemories []string,
	existingMemories []ExistingMemory,
) ([]DuplicateCheckResult, error) {

	// Gera embeddings em batch (mais eficiente)
	newEmbeddings, err := sd.embeddingService.GetEmbeddingBatch(ctx, newMemories)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embeddings em batch: %w", err)
	}

	results := make([]DuplicateCheckResult, len(newMemories))

	for i, newEmb := range newEmbeddings {
		var mostSimilar ExistingMemory
		var maxSimilarity float64 = 0.0

		// Compara com todas as memórias existentes
		for _, existing := range existingMemories {
			similarity := cosineSimilarity(newEmb, existing.Embedding)
			if similarity > maxSimilarity {
				maxSimilarity = similarity
				mostSimilar = existing
			}
		}

		isDuplicate := maxSimilarity >= sd.similarityThreshold

		results[i] = DuplicateCheckResult{
			IsDuplicate:      isDuplicate,
			ExistingMemoryID: mostSimilar.ID,
			Similarity:       maxSimilarity,
			ShouldReinforce:  isDuplicate,
		}
	}

	return results, nil
}

// ClusterSimilarMemories agrupa memórias similares
func (sd *SemanticDeduplicator) ClusterSimilarMemories(
	memories []ExistingMemory,
	threshold float64,
) [][]ExistingMemory {

	if threshold == 0 {
		threshold = sd.similarityThreshold
	}

	// Algoritmo de clustering simples: greedy
	var clusters [][]ExistingMemory
	used := make(map[string]bool)

	for _, mem := range memories {
		if used[mem.ID] {
			continue
		}

		// Cria novo cluster com esta memória
		cluster := []ExistingMemory{mem}
		used[mem.ID] = true

		// Encontra memórias similares
		for _, other := range memories {
			if used[other.ID] {
				continue
			}

			similarity := cosineSimilarity(mem.Embedding, other.Embedding)
			if similarity >= threshold {
				cluster = append(cluster, other)
				used[other.ID] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// GetSimilarMemories retorna memórias similares a um texto
func (sd *SemanticDeduplicator) GetSimilarMemories(
	ctx context.Context,
	queryText string,
	memories []ExistingMemory,
	topK int,
) ([]SimilarMemory, error) {

	// Gera embedding da query
	queryEmbedding, err := sd.embeddingService.GetEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar embedding da query: %w", err)
	}

	// Calcula similaridade com todas as memórias
	type scoredMemory struct {
		memory     ExistingMemory
		similarity float64
	}

	scored := make([]scoredMemory, 0, len(memories))
	for _, mem := range memories {
		sim := cosineSimilarity(queryEmbedding, mem.Embedding)
		scored = append(scored, scoredMemory{mem, sim})
	}

	// Ordena por similaridade (bubble sort simples, OK para poucos itens)
	for i := 0; i < len(scored)-1; i++ {
		for j := 0; j < len(scored)-i-1; j++ {
			if scored[j].similarity < scored[j+1].similarity {
				scored[j], scored[j+1] = scored[j+1], scored[j]
			}
		}
	}

	// Retorna top K
	if topK > len(scored) {
		topK = len(scored)
	}

	results := make([]SimilarMemory, topK)
	for i := 0; i < topK; i++ {
		results[i] = SimilarMemory{
			ID:         scored[i].memory.ID,
			Content:    scored[i].memory.Content,
			Similarity: scored[i].similarity,
			Reinforcement: scored[i].memory.ReinforcementCount,
		}
	}

	return results, nil
}

// SimilarMemory representa uma memória similar encontrada
type SimilarMemory struct {
	ID            string
	Content       string
	Similarity    float64
	Reinforcement int
}

// CalculateDiversityScore calcula quão diversas são as memórias
func (sd *SemanticDeduplicator) CalculateDiversityScore(memories []ExistingMemory) float64 {
	if len(memories) <= 1 {
		return 1.0 // Totalmente diverso (ou vazio)
	}

	// Calcula similaridade média entre todas as memórias
	var totalSimilarity float64
	var comparisons int

	for i := 0; i < len(memories)-1; i++ {
		for j := i + 1; j < len(memories); j++ {
			sim := cosineSimilarity(memories[i].Embedding, memories[j].Embedding)
			totalSimilarity += sim
			comparisons++
		}
	}

	avgSimilarity := totalSimilarity / float64(comparisons)

	// Diversidade é o inverso da similaridade média
	// Se similaridade média é 0.9 (muito similar), diversidade é 0.1 (pouco diverso)
	// Se similaridade média é 0.2 (pouco similar), diversidade é 0.8 (muito diverso)
	return 1.0 - avgSimilarity
}

// SuggestMemoryConsolidation sugere quais memórias podem ser consolidadas
func (sd *SemanticDeduplicator) SuggestMemoryConsolidation(
	memories []ExistingMemory,
) []ConsolidationSuggestion {

	clusters := sd.ClusterSimilarMemories(memories, 0.85) // Threshold mais alto para consolidação

	var suggestions []ConsolidationSuggestion

	for _, cluster := range clusters {
		if len(cluster) >= 3 { // Só sugere se houver 3+ memórias similares
			// Ordena por reinforcement count
			for i := 0; i < len(cluster)-1; i++ {
				for j := 0; j < len(cluster)-i-1; j++ {
					if cluster[j].ReinforcementCount < cluster[j+1].ReinforcementCount {
						cluster[j], cluster[j+1] = cluster[j+1], cluster[j]
					}
				}
			}

			suggestion := ConsolidationSuggestion{
				MemoryIDs:     make([]string, len(cluster)),
				KeepMemoryID:  cluster[0].ID, // Mantém a mais reforçada
				ClusterSize:   len(cluster),
				AvgSimilarity: calculateAvgSimilarity(cluster),
			}

			for i, mem := range cluster {
				suggestion.MemoryIDs[i] = mem.ID
			}

			suggestions = append(suggestions, suggestion)
		}
	}

	return suggestions
}

// ConsolidationSuggestion sugere consolidação de memórias similares
type ConsolidationSuggestion struct {
	MemoryIDs     []string
	KeepMemoryID  string
	ClusterSize   int
	AvgSimilarity float64
}

// calculateAvgSimilarity calcula similaridade média em um cluster
func calculateAvgSimilarity(cluster []ExistingMemory) float64 {
	if len(cluster) <= 1 {
		return 1.0
	}

	var totalSim float64
	var comparisons int

	for i := 0; i < len(cluster)-1; i++ {
		for j := i + 1; j < len(cluster); j++ {
			sim := cosineSimilarity(cluster[i].Embedding, cluster[j].Embedding)
			totalSim += sim
			comparisons++
		}
	}

	return totalSim / float64(comparisons)
}

// GetDeduplicationStats retorna estatísticas sobre deduplicação
func (sd *SemanticDeduplicator) GetDeduplicationStats(
	duplicateChecks []DuplicateCheckResult,
) map[string]interface{} {

	totalChecks := len(duplicateChecks)
	duplicatesFound := 0
	var avgSimilarity float64

	for _, check := range duplicateChecks {
		if check.IsDuplicate {
			duplicatesFound++
		}
		avgSimilarity += check.Similarity
	}

	if totalChecks > 0 {
		avgSimilarity /= float64(totalChecks)
	}

	return map[string]interface{}{
		"total_checks":       totalChecks,
		"duplicates_found":   duplicatesFound,
		"duplicate_rate":     float64(duplicatesFound) / float64(totalChecks) * 100,
		"avg_similarity":     avgSimilarity,
		"threshold":          sd.similarityThreshold,
	}
}
