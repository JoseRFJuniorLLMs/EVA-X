// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Dual Weight System (DHP)
// Differential Hebbian Plasticity: slow_weight (fixo) + fast_weight (dinamico)
// Baseado em Zenke & Gerstner (2017)
package memory

import (
	"context"
	"fmt"
	"log"
	"math"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// DualWeightSystem gerencia pesos lentos (embedding) + rapidos (Hebb)
type DualWeightSystem struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	slowRatio    float64 // Peso do slow_weight (default: 0.3)
	fastRatio    float64 // Peso do fast_weight (default: 0.7)
}

// DualWeightConfig configuracao do DHP
type DualWeightConfig struct {
	SlowRatio float64 // 0.3 = 30% slow weight
	FastRatio float64 // 0.7 = 70% fast weight
}

// NewDualWeightSystem cria um novo DHP manager
func NewDualWeightSystem(graphAdapter *nietzscheInfra.GraphAdapter, config *DualWeightConfig) *DualWeightSystem {
	if config == nil {
		config = &DualWeightConfig{
			SlowRatio: 0.3,
			FastRatio: 0.7,
		}
	}

	return &DualWeightSystem{
		graphAdapter: graphAdapter,
		slowRatio:    config.SlowRatio,
		fastRatio:    config.FastRatio,
	}
}

// InitializeEdge inicializa uma nova aresta com dual weights
// slow_weight = cosine similarity dos embeddings (fixo)
// fast_weight = 0.5 (inicial, sera atualizado pelo Hebbian RT)
// combined_weight = 0.3 * slow + 0.7 * fast
func (d *DualWeightSystem) InitializeEdge(ctx context.Context, nodeA, nodeB string, embeddingA, embeddingB []float32) error {
	if d.graphAdapter == nil {
		return fmt.Errorf("graph adapter not initialized")
	}

	// Calcular slow_weight (cosine similarity)
	slowWeight := cosineSimilarity(embeddingA, embeddingB)

	// fast_weight inicial
	fastWeight := 0.5

	// combined_weight
	combinedWeight := d.slowRatio*slowWeight + d.fastRatio*fastWeight

	_, err := d.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: nodeA,
		ToNodeID:   nodeB,
		EdgeType:   "ASSOCIADO_COM",
		OnCreateSet: map[string]interface{}{
			"slow_weight":      slowWeight,
			"fast_weight":      fastWeight,
			"weight":           combinedWeight,
			"slow_ratio":       d.slowRatio,
			"fast_ratio":       d.fastRatio,
			"created_at":       nietzscheInfra.NowUnix(),
			"dhp_initialized":  true,
		},
		OnMatchSet: map[string]interface{}{
			"weight": combinedWeight,
		},
	})

	if err == nil {
		nodeAShort := nodeA
		nodeBShort := nodeB
		if len(nodeA) > 8 {
			nodeAShort = nodeA[:8]
		}
		if len(nodeB) > 8 {
			nodeBShort = nodeB[:8]
		}
		log.Printf("[DHP] Initialized edge %s<->%s: slow=%.3f, fast=%.3f, combined=%.3f",
			nodeAShort, nodeBShort, slowWeight, fastWeight, combinedWeight)
	}

	return err
}

// UpdateCombinedWeight recalcula combined_weight apos fast_weight mudar
// combined_weight = slow_ratio * slow_weight + fast_ratio * fast_weight
func (d *DualWeightSystem) UpdateCombinedWeight(ctx context.Context, nodeA, nodeB string) error {
	if d.graphAdapter == nil {
		return fmt.Errorf("graph adapter not initialized")
	}

	// Read edge weights via NQL
	nql := `MATCH (a)-[r:ASSOCIADO_COM]-(b) WHERE a.id = $nodeA AND b.id = $nodeB RETURN r`
	result, err := d.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"nodeA": nodeA,
		"nodeB": nodeB,
	}, "")
	if err != nil {
		return err
	}

	// Get current slow/fast weights, default to 0.5
	slowWeight := 0.5
	fastWeight := 0.5
	if len(result.NodePairs) > 0 {
		edge := result.NodePairs[0]
		if sw, ok := edge.Edge.Content["slow_weight"]; ok {
			if f, ok := sw.(float64); ok {
				slowWeight = f
			}
		}
		if fw, ok := edge.Edge.Content["fast_weight"]; ok {
			if f, ok := fw.(float64); ok {
				fastWeight = f
			}
		}
	}

	// Recalculate combined
	newWeight := d.slowRatio*slowWeight + d.fastRatio*fastWeight

	// Update via MergeEdge
	_, err = d.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: nodeA,
		ToNodeID:   nodeB,
		EdgeType:   "ASSOCIADO_COM",
		OnMatchSet: map[string]interface{}{
			"weight":             newWeight,
			"combined_updated_at": nietzscheInfra.NowUnix(),
		},
	})

	return err
}

// MigrateExistingEdges migra arestas existentes para DHP
// Adiciona slow_weight e fast_weight as arestas que so tem weight
func (d *DualWeightSystem) MigrateExistingEdges(ctx context.Context, patientID int64, batchSize int) (*MigrationResult, error) {
	if d.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	log.Printf("[DHP] Starting migration for patient %d (batch size=%d)", patientID, batchSize)

	result := &MigrationResult{
		PatientID:     patientID,
		BatchSize:     batchSize,
		EdgesMigrated: 0,
	}

	// Find patient node first
	patientNodeID := fmt.Sprintf("%d", patientID)

	// BFS from patient to find nearby nodes (depth 2)
	neighborIDs, err := d.graphAdapter.Bfs(ctx, patientNodeID, 2, "")
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	// For each pair of neighbors, check and migrate ASSOCIADO_COM/CO_ACTIVATED edges
	migrated := 0
	for _, nid := range neighborIDs {
		if migrated >= batchSize {
			break
		}

		// Query edges from this node
		nql := `MATCH (n)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(m) WHERE n.id = $nodeID AND r.slow_weight IS NULL RETURN r, m LIMIT $limit`
		edgeResult, err := d.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"nodeID": nid,
			"limit":  batchSize - migrated,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range edgeResult.NodePairs {
			existingWeight := 0.5
			if w, ok := pair.Edge.Content["weight"]; ok {
				if f, ok := w.(float64); ok {
					existingWeight = f
				}
			}

			slowW := existingWeight
			fastW := 0.5
			combinedW := d.slowRatio*slowW + d.fastRatio*fastW

			_, err := d.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: pair.From.ID,
				ToNodeID:   pair.To.ID,
				EdgeType:   pair.Edge.Label,
				OnMatchSet: map[string]interface{}{
					"slow_weight":   slowW,
					"fast_weight":   fastW,
					"weight":        combinedW,
					"migrated_at":   nietzscheInfra.NowUnix(),
					"dhp_migrated":  true,
				},
			})
			if err == nil {
				migrated++
			}
		}
	}

	result.EdgesMigrated = migrated
	log.Printf("[DHP] Migrated %d edges for patient %d", result.EdgesMigrated, patientID)

	return result, nil
}

// RecalculateSlowWeights recalcula slow_weights usando embeddings atuais
// Executado durante consolidacao noturna
func (d *DualWeightSystem) RecalculateSlowWeights(ctx context.Context, patientID int64) error {
	if d.graphAdapter == nil {
		return fmt.Errorf("graph adapter not initialized")
	}

	log.Printf("[DHP] Recalculating slow_weights for patient %d", patientID)

	// TODO: Implementar quando embeddings estiverem disponiveis no grafo
	// Por enquanto, mantem slow_weights existentes

	return nil
}

// NormalizeWeights normaliza fast_weights para evitar saturacao
// Executado durante consolidacao noturna
func (d *DualWeightSystem) NormalizeWeights(ctx context.Context, patientID int64) (*NormalizationResult, error) {
	if d.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	log.Printf("[DHP] Normalizing weights for patient %d", patientID)

	// 1. Find patient neighbors and collect fast_weight stats
	patientNodeID := fmt.Sprintf("%d", patientID)
	neighborIDs, err := d.graphAdapter.Bfs(ctx, patientNodeID, 2, "")
	if err != nil {
		return nil, err
	}

	var weights []float64
	type edgeInfo struct {
		fromID, toID, edgeType string
		fastWeight             float64
	}
	var edges []edgeInfo

	for _, nid := range neighborIDs {
		nql := `MATCH (n)-[r:ASSOCIADO_COM]-(m) WHERE n.id = $nodeID AND r.fast_weight IS NOT NULL RETURN r, m`
		edgeResult, err := d.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"nodeID": nid,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range edgeResult.NodePairs {
			fw := 0.5
			if f, ok := pair.Edge.Content["fast_weight"]; ok {
				if v, ok := f.(float64); ok {
					fw = v
				}
			}
			weights = append(weights, fw)
			edges = append(edges, edgeInfo{
				fromID:     pair.From.ID,
				toID:       pair.To.ID,
				edgeType:   pair.Edge.Label,
				fastWeight: fw,
			})
		}
	}

	// Calculate statistics
	avgWeight := 0.5
	stdWeight := 0.2
	maxWeight := 1.0

	if len(weights) > 0 {
		sum := 0.0
		for _, w := range weights {
			sum += w
		}
		avgWeight = sum / float64(len(weights))

		maxWeight = 0.0
		for _, w := range weights {
			if w > maxWeight {
				maxWeight = w
			}
		}

		variance := 0.0
		for _, w := range weights {
			variance += (w - avgWeight) * (w - avgWeight)
		}
		stdWeight = math.Sqrt(variance / float64(len(weights)))
	}

	result := &NormalizationResult{
		PatientID: patientID,
		BeforeAvg: avgWeight,
		BeforeStd: stdWeight,
		BeforeMax: maxWeight,
	}

	// 2. Normalizar se necessario (max > 0.95 ou std > 0.3)
	if maxWeight > 0.95 || stdWeight > 0.3 {
		safeStd := math.Max(stdWeight, 0.01)
		normalized := 0
		for _, e := range edges {
			newFastWeight := (e.fastWeight-avgWeight)/safeStd*0.15 + 0.5
			_, err := d.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: e.fromID,
				ToNodeID:   e.toID,
				EdgeType:   e.edgeType,
				OnMatchSet: map[string]interface{}{
					"fast_weight":   newFastWeight,
					"normalized_at": nietzscheInfra.NowUnix(),
				},
			})
			if err == nil {
				normalized++
			}
		}

		result.Normalized = true
		result.AfterAvg = 0.5
		result.AfterStd = 0.15
		result.EdgesNormalized = normalized

		log.Printf("[DHP] Normalized %d edges: avg %.3f->%.3f, std %.3f->%.3f",
			result.EdgesNormalized, result.BeforeAvg, result.AfterAvg, result.BeforeStd, result.AfterStd)
	} else {
		result.Normalized = false
		log.Printf("[DHP] No normalization needed (max=%.3f, std=%.3f)", maxWeight, stdWeight)
	}

	return result, nil
}

// GetEdgeWeights retorna os pesos de uma aresta especifica
func (d *DualWeightSystem) GetEdgeWeights(ctx context.Context, nodeA, nodeB string) (*EdgeWeights, error) {
	if d.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	nql := `MATCH (a)-[r:ASSOCIADO_COM]-(b) WHERE a.id = $nodeA AND b.id = $nodeB RETURN r LIMIT 1`
	result, err := d.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"nodeA": nodeA,
		"nodeB": nodeB,
	}, "")
	if err != nil {
		return nil, err
	}

	if len(result.NodePairs) == 0 {
		return nil, fmt.Errorf("edge not found")
	}

	edge := result.NodePairs[0].Edge
	weights := &EdgeWeights{
		SlowWeight:     0.5,
		FastWeight:     0.5,
		CombinedWeight: 0.5,
		CoActivations:  0,
	}

	if v, ok := edge.Content["slow_weight"]; ok {
		if f, ok := v.(float64); ok {
			weights.SlowWeight = f
		}
	}
	if v, ok := edge.Content["fast_weight"]; ok {
		if f, ok := v.(float64); ok {
			weights.FastWeight = f
		}
	}
	if v, ok := edge.Content["weight"]; ok {
		if f, ok := v.(float64); ok {
			weights.CombinedWeight = f
		}
	}
	if v, ok := edge.Content["co_activation_count"]; ok {
		switch c := v.(type) {
		case float64:
			weights.CoActivations = int(c)
		case int64:
			weights.CoActivations = int(c)
		case int:
			weights.CoActivations = c
		}
	}

	return weights, nil
}

// Helper functions

// cosineSimilarity calcula similaridade cosseno entre dois embeddings
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0.5 // Default se dimensoes incompativeis
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
