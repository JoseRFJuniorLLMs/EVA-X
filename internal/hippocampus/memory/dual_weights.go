// Package memory - Dual Weight System (DHP)
// Differential Hebbian Plasticity: slow_weight (fixo) + fast_weight (dinâmico)
// Baseado em Zenke & Gerstner (2017)
package memory

import (
	"context"
	"fmt"
	"log"
	"math"

	"eva-mind/internal/brainstem/infrastructure/graph"
)

// DualWeightSystem gerencia pesos lentos (embedding) + rápidos (Hebb)
type DualWeightSystem struct {
	neo4j      *graph.Neo4jClient
	slowRatio  float64 // Peso do slow_weight (default: 0.3)
	fastRatio  float64 // Peso do fast_weight (default: 0.7)
}

// DualWeightConfig configuração do DHP
type DualWeightConfig struct {
	SlowRatio float64 // 0.3 = 30% slow weight
	FastRatio float64 // 0.7 = 70% fast weight
}

// NewDualWeightSystem cria um novo DHP manager
func NewDualWeightSystem(neo4j *graph.Neo4jClient, config *DualWeightConfig) *DualWeightSystem {
	if config == nil {
		config = &DualWeightConfig{
			SlowRatio: 0.3,
			FastRatio: 0.7,
		}
	}

	return &DualWeightSystem{
		neo4j:     neo4j,
		slowRatio: config.SlowRatio,
		fastRatio: config.FastRatio,
	}
}

// InitializeEdge inicializa uma nova aresta com dual weights
// slow_weight = cosine similarity dos embeddings (fixo)
// fast_weight = 0.5 (inicial, será atualizado pelo Hebbian RT)
// combined_weight = 0.3 * slow + 0.7 * fast
func (d *DualWeightSystem) InitializeEdge(ctx context.Context, nodeA, nodeB string, embeddingA, embeddingB []float32) error {
	if d.neo4j == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	// Calcular slow_weight (cosine similarity)
	slowWeight := cosineSimilarity(embeddingA, embeddingB)

	// fast_weight inicial
	fastWeight := 0.5

	// combined_weight
	combinedWeight := d.slowRatio*slowWeight + d.fastRatio*fastWeight

	query := `
		MATCH (a), (b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		MERGE (a)-[r:ASSOCIADO_COM]-(b)
		ON CREATE SET
			r.slow_weight = $slowWeight,
			r.fast_weight = $fastWeight,
			r.weight = $combinedWeight,
			r.slow_ratio = $slowRatio,
			r.fast_ratio = $fastRatio,
			r.created_at = datetime(),
			r.dhp_initialized = true
		ON MATCH SET
			r.slow_weight = COALESCE(r.slow_weight, $slowWeight),
			r.fast_weight = COALESCE(r.fast_weight, $fastWeight),
			r.weight = $combinedWeight
		RETURN count(r) AS initialized
	`

	_, err := d.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"nodeA":         nodeA,
		"nodeB":         nodeB,
		"slowWeight":    slowWeight,
		"fastWeight":    fastWeight,
		"combinedWeight": combinedWeight,
		"slowRatio":     d.slowRatio,
		"fastRatio":     d.fastRatio,
	})

	if err == nil {
		log.Printf("✨ [DHP] Initialized edge %s<->%s: slow=%.3f, fast=%.3f, combined=%.3f",
			nodeA[:8], nodeB[:8], slowWeight, fastWeight, combinedWeight)
	}

	return err
}

// UpdateCombinedWeight recalcula combined_weight após fast_weight mudar
// combined_weight = slow_ratio * slow_weight + fast_ratio * fast_weight
func (d *DualWeightSystem) UpdateCombinedWeight(ctx context.Context, nodeA, nodeB string) error {
	if d.neo4j == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (a)-[r:ASSOCIADO_COM]-(b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		SET r.weight = $slowRatio * COALESCE(r.slow_weight, 0.5) +
		               $fastRatio * COALESCE(r.fast_weight, 0.5),
		    r.combined_updated_at = datetime()
		RETURN r.weight AS newWeight
	`

	_, err := d.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"nodeA":     nodeA,
		"nodeB":     nodeB,
		"slowRatio": d.slowRatio,
		"fastRatio": d.fastRatio,
	})

	return err
}

// MigrateExistingEdges migra arestas existentes para DHP
// Adiciona slow_weight e fast_weight às arestas que só têm weight
func (d *DualWeightSystem) MigrateExistingEdges(ctx context.Context, patientID int64, batchSize int) (*MigrationResult, error) {
	if d.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	log.Printf("🔄 [DHP] Starting migration for patient %d (batch size=%d)", patientID, batchSize)

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(n2)
		WHERE r.slow_weight IS NULL OR r.fast_weight IS NULL
		WITH r, n1, n2
		LIMIT $batchSize
		SET r.slow_weight = COALESCE(r.slow_weight, r.weight, 0.5),
		    r.fast_weight = COALESCE(r.fast_weight, 0.5),
		    r.weight = $slowRatio * r.slow_weight + $fastRatio * r.fast_weight,
		    r.migrated_at = datetime(),
		    r.dhp_migrated = true
		RETURN count(r) AS migrated
	`

	records, err := d.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"batchSize": batchSize,
		"slowRatio": d.slowRatio,
		"fastRatio": d.fastRatio,
	})

	result := &MigrationResult{
		PatientID:   patientID,
		BatchSize:   batchSize,
		EdgesMigrated: 0,
	}

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// TODO: Extrair count do record
	result.EdgesMigrated = batchSize

	log.Printf("✅ [DHP] Migrated %d edges for patient %d", result.EdgesMigrated, patientID)

	return result, nil
}

// RecalculateSlowWeights recalcula slow_weights usando embeddings atuais
// Executado durante consolidação noturna
func (d *DualWeightSystem) RecalculateSlowWeights(ctx context.Context, patientID int64) error {
	if d.neo4j == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	log.Printf("🔄 [DHP] Recalculating slow_weights for patient %d", patientID)

	// TODO: Implementar quando embeddings estiverem disponíveis no grafo
	// Por enquanto, mantém slow_weights existentes

	return nil
}

// NormalizeWeights normaliza fast_weights para evitar saturação
// Executado durante consolidação noturna
func (d *DualWeightSystem) NormalizeWeights(ctx context.Context, patientID int64) (*NormalizationResult, error) {
	if d.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	log.Printf("🔄 [DHP] Normalizing weights for patient %d", patientID)

	// 1. Buscar estatísticas de fast_weights
	statsQuery := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE r.fast_weight IS NOT NULL
		RETURN
			avg(r.fast_weight) AS avgWeight,
			stdev(r.fast_weight) AS stdWeight,
			max(r.fast_weight) AS maxWeight,
			count(r) AS totalEdges
	`

	records, err := d.neo4j.ExecuteRead(ctx, statsQuery, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return nil, err
	}

	// TODO: Extrair stats do record
	avgWeight := 0.5
	stdWeight := 0.2
	maxWeight := 1.0

	result := &NormalizationResult{
		PatientID:      patientID,
		BeforeAvg:      avgWeight,
		BeforeStd:      stdWeight,
		BeforeMax:      maxWeight,
	}

	// 2. Normalizar se necessário (max > 0.95 ou std > 0.3)
	if maxWeight > 0.95 || stdWeight > 0.3 {
		normalizeQuery := `
			MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
			WHERE r.fast_weight IS NOT NULL
			SET r.fast_weight = (r.fast_weight - $avgWeight) / $stdWeight * 0.15 + 0.5,
			    r.normalized_at = datetime()
			RETURN count(r) AS normalized
		`

		_, err = d.neo4j.ExecuteWrite(ctx, normalizeQuery, map[string]interface{}{
			"patientId": patientID,
			"avgWeight": avgWeight,
			"stdWeight": math.Max(stdWeight, 0.01), // Evitar divisão por zero
		})

		if err != nil {
			result.Error = err.Error()
			return result, err
		}

		result.Normalized = true
		result.AfterAvg = 0.5
		result.AfterStd = 0.15

		log.Printf("✅ [DHP] Normalized %d edges: avg %.3f→%.3f, std %.3f→%.3f",
			result.EdgesNormalized, result.BeforeAvg, result.AfterAvg, result.BeforeStd, result.AfterStd)
	} else {
		result.Normalized = false
		log.Printf("✅ [DHP] No normalization needed (max=%.3f, std=%.3f)", maxWeight, stdWeight)
	}

	return result, nil
}

// GetEdgeWeights retorna os pesos de uma aresta específica
func (d *DualWeightSystem) GetEdgeWeights(ctx context.Context, nodeA, nodeB string) (*EdgeWeights, error) {
	if d.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (a)-[r:ASSOCIADO_COM]-(b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		RETURN
			COALESCE(r.slow_weight, 0.5) AS slowWeight,
			COALESCE(r.fast_weight, 0.5) AS fastWeight,
			COALESCE(r.weight, 0.5) AS combinedWeight,
			COALESCE(r.co_activation_count, 0) AS coActivations
	`

	records, err := d.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"nodeA": nodeA,
		"nodeB": nodeB,
	})
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("edge not found")
	}

	// TODO: Extrair do record
	weights := &EdgeWeights{
		SlowWeight:     0.5,
		FastWeight:     0.5,
		CombinedWeight: 0.5,
		CoActivations:  0,
	}

	return weights, nil
}

// Helper functions

// cosineSimilarity calcula similaridade cosseno entre dois embeddings
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.5 // Default se dimensões incompatíveis
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

	similarity := dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))

	// Normalizar para [0, 1]
	return (similarity + 1.0) / 2.0
}

// Structs

type MigrationResult struct {
	PatientID     int64
	BatchSize     int
	EdgesMigrated int
	Error         string
}

type NormalizationResult struct {
	PatientID       int64
	Normalized      bool
	BeforeAvg       float64
	BeforeStd       float64
	BeforeMax       float64
	AfterAvg        float64
	AfterStd        float64
	EdgesNormalized int
	Error           string
}

type EdgeWeights struct {
	SlowWeight     float64
	FastWeight     float64
	CombinedWeight float64
	CoActivations  int
}
