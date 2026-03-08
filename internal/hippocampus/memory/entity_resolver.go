// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// GraphClient interface para operações no grafo NietzscheDB (entity resolution)
type GraphClient interface {
	ExecuteQuery(ctx context.Context, query string, params map[string]interface{}) ([]map[string]interface{}, error)
}

// EntityResolver resolve variações de nomes de entidades no grafo
// Exemplo: "Maria", "Dona Maria", "minha mãe Maria" → mesmo nó
// Usa embedding similarity (NÃO SRC - conforme mente.md)
type EntityResolver struct {
	graph           GraphClient
	embedder        EmbeddingService
	similarityThreshold float64 // 0.85 default
	minNameLength   int         // 3 chars minimum
	batchSize       int         // 50 nodes per batch
	cache           EntityCache
}

// EntityCandidate representa um candidato para merge
type EntityCandidate struct {
	NodeID       string
	Name         string
	Type         string // person, place, concept
	Embedding    []float32
	Frequency    int       // Quantas vezes foi mencionado
	LastSeen     time.Time
	Aliases      []string  // Variações conhecidas
}

// MergeCandidate representa dois nós que devem ser merged
type MergeCandidate struct {
	SourceID       string
	TargetID       string
	SourceName     string
	TargetName     string
	Similarity     float64
	Confidence     string // high, medium, low
	ReasonCode     string // embedding_match, exact_alias, fuzzy_match
}

// MergeResult resultado do merge de entidades
type MergeResult struct {
	SourceID       string
	TargetID       string
	EdgesMoved     int
	PropertiesMerged int
	Success        bool
	Error          error
}

// ResolutionStats estatísticas de resolução
type ResolutionStats struct {
	TotalNodes       int
	CandidatesFound  int
	MergesPerformed  int
	EdgesConsolidated int
	Duration         time.Duration
}

// EntityCache cache de embeddings para evitar recomputação
type EntityCache interface {
	Get(ctx context.Context, key string) ([]float32, error)
	Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error
}

// NewEntityResolver cria novo resolver
func NewEntityResolver(graph GraphClient, embedder EmbeddingService, cache EntityCache) *EntityResolver {
	return &EntityResolver{
		graph:           graph,
		embedder:        embedder,
		similarityThreshold: 0.85, // Threshold conservador
		minNameLength:   3,
		batchSize:       50,
		cache:           cache,
	}
}

// FindDuplicateEntities encontra entidades duplicadas para um paciente
func (r *EntityResolver) FindDuplicateEntities(ctx context.Context, patientID int64) ([]MergeCandidate, error) {
	// 1. Obter todas as entidades do grafo do paciente
	entities, err := r.getPatientEntities(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("failed to get entities: %w", err)
	}

	if len(entities) == 0 {
		return nil, nil
	}

	// 2. Gerar embeddings para todas as entidades
	if err := r.generateEmbeddings(ctx, entities); err != nil {
		return nil, fmt.Errorf("failed to generate embeddings: %w", err)
	}

	// 3. Comparar pares de entidades
	candidates := make([]MergeCandidate, 0)

	for i := 0; i < len(entities); i++ {
		for j := i + 1; j < len(entities); j++ {
			entity1 := entities[i]
			entity2 := entities[j]

			// Skip se tipos diferentes (pessoa != lugar)
			if entity1.Type != entity2.Type {
				continue
			}

			// Calcular similaridade
			similarity := r.cosineSimilarity(entity1.Embedding, entity2.Embedding)

			if similarity >= r.similarityThreshold {
				// Decidir qual é source e qual é target
				// Target = mais frequente (preserva o nó mais usado)
				source, target := entity1, entity2
				if entity2.Frequency > entity1.Frequency {
					source, target = entity2, entity1
				}

				candidate := MergeCandidate{
					SourceID:   source.NodeID,
					TargetID:   target.NodeID,
					SourceName: source.Name,
					TargetName: target.Name,
					Similarity: similarity,
					Confidence: r.calculateConfidence(similarity),
					ReasonCode: "embedding_match",
				}

				candidates = append(candidates, candidate)
			}
		}
	}

	// 4. Ordenar por similarity descendente
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Similarity > candidates[j].Similarity
	})

	return candidates, nil
}

// MergeEntities executa merge de entidades duplicadas
func (r *EntityResolver) MergeEntities(ctx context.Context, patientID int64, candidates []MergeCandidate) ([]MergeResult, error) {
	results := make([]MergeResult, 0, len(candidates))

	for _, candidate := range candidates {
		result := r.mergeSingleEntity(ctx, patientID, candidate)
		results = append(results, result)

		if !result.Success {
			// Log error mas continua com outros merges
			fmt.Printf("Failed to merge %s -> %s: %v\n", candidate.SourceID, candidate.TargetID, result.Error)
		}
	}

	return results, nil
}

// AutoResolve encontra e merge automaticamente entidades duplicadas
func (r *EntityResolver) AutoResolve(ctx context.Context, patientID int64, dryRun bool) (*ResolutionStats, error) {
	startTime := time.Now()

	// 1. Encontrar candidatos
	candidates, err := r.FindDuplicateEntities(ctx, patientID)
	if err != nil {
		return nil, err
	}

	stats := &ResolutionStats{
		CandidatesFound: len(candidates),
	}

	if dryRun {
		stats.Duration = time.Since(startTime)
		return stats, nil
	}

	// 2. Executar merges
	results, err := r.MergeEntities(ctx, patientID, candidates)
	if err != nil {
		return nil, err
	}

	// 3. Compilar estatísticas
	for _, result := range results {
		if result.Success {
			stats.MergesPerformed++
			stats.EdgesConsolidated += result.EdgesMoved
		}
	}

	stats.Duration = time.Since(startTime)
	return stats, nil
}

// ResolveEntityName resolve variações de nome em tempo real
// Usa durante inserção de novas memórias
func (r *EntityResolver) ResolveEntityName(ctx context.Context, patientID int64, entityName string) (string, bool, error) {
	// 1. Gerar embedding do nome
	embedding, err := r.getOrGenerateEmbedding(ctx, entityName)
	if err != nil {
		return entityName, false, err
	}

	// 2. Buscar entidades similares no grafo
	entities, err := r.getPatientEntities(ctx, patientID)
	if err != nil {
		return entityName, false, err
	}

	// 3. Encontrar match mais próximo
	var bestMatch *EntityCandidate
	var bestSimilarity float64 = 0.0

	for _, entity := range entities {
		if len(entity.Embedding) == 0 {
			continue
		}

		similarity := r.cosineSimilarity(embedding, entity.Embedding)
		if similarity > bestSimilarity && similarity >= r.similarityThreshold {
			bestSimilarity = similarity
			bestMatch = &entity
		}
	}

	// 4. Retornar nome canônico se encontrado
	if bestMatch != nil {
		return bestMatch.Name, true, nil // Nome canônico encontrado
	}

	return entityName, false, nil // Nome original (nova entidade)
}

// getPatientEntities obtém todas as entidades do grafo do paciente
func (r *EntityResolver) getPatientEntities(ctx context.Context, patientID int64) ([]EntityCandidate, error) {
	query := `
		MATCH (p:Patient {id: $patientID})-[:HAS_MEMORY]->(m:Memory)-[:MENTIONS]->(e:Entity)
		RETURN
			e.id AS node_id,
			e.name AS name,
			e.type AS type,
			e.embedding AS embedding,
			COUNT(DISTINCT m) AS frequency,
			MAX(m.created_at) AS last_seen
		ORDER BY frequency DESC
	`

	params := map[string]interface{}{
		"patientID": patientID,
	}

	records, err := r.graph.ExecuteQuery(ctx, query, params)
	if err != nil {
		return nil, err
	}

	entities := make([]EntityCandidate, 0)
	for _, record := range records {
		entity := EntityCandidate{
			NodeID:    getStringField(record, "node_id"),
			Name:      getStringField(record, "name"),
			Type:      getStringField(record, "type"),
			Frequency: getIntField(record, "frequency"),
		}

		// Parse embedding se existir
		if embeddingData, ok := record["embedding"].([]interface{}); ok {
			entity.Embedding = parseEmbedding(embeddingData)
		}

		// Parse last_seen
		if lastSeenStr, ok := record["last_seen"].(string); ok {
			entity.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr)
		}

		// Filtrar nomes muito curtos
		if len(entity.Name) >= r.minNameLength {
			entities = append(entities, entity)
		}
	}

	return entities, nil
}

// generateEmbeddings gera embeddings para entidades que não têm
func (r *EntityResolver) generateEmbeddings(ctx context.Context, entities []EntityCandidate) error {
	for i := range entities {
		if len(entities[i].Embedding) == 0 {
			embedding, err := r.getOrGenerateEmbedding(ctx, entities[i].Name)
			if err != nil {
				return fmt.Errorf("failed to generate embedding for %s: %w", entities[i].Name, err)
			}
			entities[i].Embedding = embedding
		}
	}
	return nil
}

// getOrGenerateEmbedding obtém embedding do cache ou gera novo
func (r *EntityResolver) getOrGenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	cacheKey := fmt.Sprintf("entity_emb:%s", strings.ToLower(text))

	// Tentar cache primeiro
	if r.cache != nil {
		if cached, err := r.cache.Get(ctx, cacheKey); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	// Gerar novo embedding
	embedding, err := r.embedder.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	// Salvar no cache (TTL 24h)
	if r.cache != nil {
		_ = r.cache.Set(ctx, cacheKey, embedding, 24*time.Hour)
	}

	return embedding, nil
}

// mergeSingleEntity executa merge de uma única entidade
func (r *EntityResolver) mergeSingleEntity(ctx context.Context, patientID int64, candidate MergeCandidate) MergeResult {
	result := MergeResult{
		SourceID: candidate.SourceID,
		TargetID: candidate.TargetID,
	}

	// Cypher para merge:
	// 1. Mover todas as arestas de source para target
	// 2. Adicionar source.name como alias em target
	// 3. Deletar source node
	query := `
		MATCH (source:Entity {id: $sourceID})
		MATCH (target:Entity {id: $targetID})

		// Mover todas as arestas de source para target
		WITH source, target
		MATCH (source)-[r]->(other)
		WHERE NOT (target)-[:SAME_TYPE]->(other)
		CREATE (target)-[r2:SAME_TYPE]->(other)
		SET r2 = properties(r)
		DELETE r

		WITH source, target, COUNT(r) AS edges_moved

		// Adicionar alias
		SET target.aliases = COALESCE(target.aliases, []) + [source.name]

		// Atualizar frequência
		SET target.frequency = COALESCE(target.frequency, 0) + COALESCE(source.frequency, 0)

		// Deletar source
		DETACH DELETE source

		RETURN edges_moved
	`

	params := map[string]interface{}{
		"sourceID": candidate.SourceID,
		"targetID": candidate.TargetID,
	}

	records, err := r.graph.ExecuteQuery(ctx, query, params)
	if err != nil {
		result.Error = err
		return result
	}

	if len(records) > 0 {
		result.EdgesMoved = getIntField(records[0], "edges_moved")
		result.PropertiesMerged = 2 // aliases + frequency
		result.Success = true
	}

	return result
}

// cosineSimilarity calcula similaridade de cosseno entre dois embeddings
func (r *EntityResolver) cosineSimilarity(a, b []float32) float64 {
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

// calculateConfidence determina nível de confiança do match
func (r *EntityResolver) calculateConfidence(similarity float64) string {
	if similarity >= 0.95 {
		return "high"
	} else if similarity >= 0.90 {
		return "medium"
	}
	return "low"
}

// Helper functions

func getStringField(record map[string]interface{}, field string) string {
	if val, ok := record[field].(string); ok {
		return val
	}
	return ""
}

func getIntField(record map[string]interface{}, field string) int {
	if val, ok := record[field].(int64); ok {
		return int(val)
	}
	if val, ok := record[field].(int); ok {
		return val
	}
	return 0
}

func parseEmbedding(data []interface{}) []float32 {
	embedding := make([]float32, len(data))
	for i, v := range data {
		if f, ok := v.(float64); ok {
			embedding[i] = float32(f)
		}
	}
	return embedding
}

// SetSimilarityThreshold configura threshold de similaridade
func (r *EntityResolver) SetSimilarityThreshold(threshold float64) {
	if threshold >= 0.0 && threshold <= 1.0 {
		r.similarityThreshold = threshold
	}
}

// GetSimilarityThreshold retorna threshold atual
func (r *EntityResolver) GetSimilarityThreshold() float64 {
	return r.similarityThreshold
}
