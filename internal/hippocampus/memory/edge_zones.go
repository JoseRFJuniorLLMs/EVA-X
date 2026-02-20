// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Edge Zones Classification
// Classifica arestas em 3 zonas: Consolidated, Emerging, Weak
// Cada zona tem uma ação automática associada
// Fase C do plano de implementação
package memory

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/infrastructure/graph"
)

// EdgeZone representa a zona de uma aresta baseada em seu peso
type EdgeZone string

const (
	// ZoneConsolidated: w > 0.7 - Associações fortes e consolidadas
	// Ação: Preload no contexto Gemini automaticamente
	ZoneConsolidated EdgeZone = "consolidated"

	// ZoneEmerging: 0.3 < w < 0.7 - Associações em formação
	// Ação: Sugerir ao cuidador para revisão
	ZoneEmerging EdgeZone = "emerging"

	// ZoneWeak: w < 0.3 - Associações fracas/decaindo
	// Ação: Candidata a pruning periódico
	ZoneWeak EdgeZone = "weak"
)

// EdgeClassifier classifica arestas em zonas
type EdgeClassifier struct {
	neo4j         *graph.Neo4jClient
	thresholdHigh float64 // 0.7 - Consolidated
	thresholdLow  float64 // 0.3 - Weak
	pruningAge    int     // Dias para prune weak edges
}

// EdgeClassifierConfig configuração do classificador
type EdgeClassifierConfig struct {
	ThresholdHigh float64
	ThresholdLow  float64
	PruningAge    int // Dias
}

// AssociationEdge representa uma aresta com metadata
type AssociationEdge struct {
	NodeA          string    `json:"node_a"`
	NodeB          string    `json:"node_b"`
	NodeAName      string    `json:"node_a_name"`
	NodeBName      string    `json:"node_b_name"`
	Weight         float64   `json:"weight"`
	SlowWeight     float64   `json:"slow_weight"`
	FastWeight     float64   `json:"fast_weight"`
	CoActivations  int       `json:"co_activations"`
	LastActivated  time.Time `json:"last_activated"`
	Zone           EdgeZone  `json:"zone"`
	AgeInDays      int       `json:"age_in_days"`
}

// NewEdgeClassifier cria um novo classificador de arestas
func NewEdgeClassifier(neo4j *graph.Neo4jClient, config *EdgeClassifierConfig) *EdgeClassifier {
	if config == nil {
		config = &EdgeClassifierConfig{
			ThresholdHigh: 0.7,
			ThresholdLow:  0.3,
			PruningAge:    30,
		}
	}

	return &EdgeClassifier{
		neo4j:         neo4j,
		thresholdHigh: config.ThresholdHigh,
		thresholdLow:  config.ThresholdLow,
		pruningAge:    config.PruningAge,
	}
}

// Classify classifica uma aresta baseada em seu peso
func (c *EdgeClassifier) Classify(weight float64) EdgeZone {
	if weight > c.thresholdHigh {
		return ZoneConsolidated
	} else if weight > c.thresholdLow {
		return ZoneEmerging
	}
	return ZoneWeak
}

// GetConsolidatedEdges retorna arestas consolidadas para um paciente
// AÇÃO: Essas arestas são preloaded no contexto Gemini
func (c *EdgeClassifier) GetConsolidatedEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE r.weight > $threshold
		  AND n1 <> p AND n2 <> p
		RETURN
			toString(id(n1)) AS nodeA,
			toString(id(n2)) AS nodeB,
			COALESCE(n1.name, n1.content, 'Unknown') AS nodeAName,
			COALESCE(n2.name, n2.content, 'Unknown') AS nodeBName,
			r.weight AS weight,
			COALESCE(r.slow_weight, r.weight) AS slowWeight,
			COALESCE(r.fast_weight, r.weight) AS fastWeight,
			COALESCE(r.co_activation_count, 0) AS coActivations,
			COALESCE(r.last_activated, datetime()) AS lastActivated
		ORDER BY r.weight DESC
		LIMIT 50
	`

	records, err := c.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"threshold": c.thresholdHigh,
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	edges := make([]AssociationEdge, 0)

	for _, rec := range records {
		edge := AssociationEdge{Zone: ZoneConsolidated}
		if v, ok := rec.Get("nodeA"); ok {
			edge.NodeA, _ = v.(string)
		}
		if v, ok := rec.Get("nodeB"); ok {
			edge.NodeB, _ = v.(string)
		}
		if v, ok := rec.Get("nodeAName"); ok {
			edge.NodeAName, _ = v.(string)
		}
		if v, ok := rec.Get("nodeBName"); ok {
			edge.NodeBName, _ = v.(string)
		}
		if v, ok := rec.Get("weight"); ok {
			edge.Weight, _ = v.(float64)
		}
		if v, ok := rec.Get("slowWeight"); ok {
			edge.SlowWeight, _ = v.(float64)
		}
		if v, ok := rec.Get("fastWeight"); ok {
			edge.FastWeight, _ = v.(float64)
		}
		if v, ok := rec.Get("coActivations"); ok {
			edge.CoActivations = int(v.(int64))
		}
		if v, ok := rec.Get("lastActivated"); ok {
			if t, ok := v.(time.Time); ok {
				edge.LastActivated = t
				edge.AgeInDays = int(time.Since(t).Hours() / 24)
			}
		}
		edges = append(edges, edge)
	}

	log.Printf("✅ [EDGE_ZONES] Found %d consolidated edges for patient %d", len(edges), patientID)

	return edges, nil
}

// GetEmergingEdges retorna arestas emergentes para um paciente
// AÇÃO: Sugerir ao cuidador para confirmação/rejeição
func (c *EdgeClassifier) GetEmergingEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE r.weight > $thresholdLow
		  AND r.weight <= $thresholdHigh
		  AND n1 <> p AND n2 <> p
		RETURN
			toString(id(n1)) AS nodeA,
			toString(id(n2)) AS nodeB,
			COALESCE(n1.name, n1.content, 'Unknown') AS nodeAName,
			COALESCE(n2.name, n2.content, 'Unknown') AS nodeBName,
			r.weight AS weight,
			COALESCE(r.slow_weight, r.weight) AS slowWeight,
			COALESCE(r.fast_weight, r.weight) AS fastWeight,
			COALESCE(r.co_activation_count, 0) AS coActivations,
			COALESCE(r.last_activated, datetime()) AS lastActivated
		ORDER BY r.co_activation_count DESC
		LIMIT 30
	`

	records, err := c.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId":     patientID,
		"thresholdLow":  c.thresholdLow,
		"thresholdHigh": c.thresholdHigh,
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	edges := make([]AssociationEdge, 0)

	for _, rec := range records {
		edge := AssociationEdge{Zone: ZoneEmerging}
		if v, ok := rec.Get("nodeA"); ok {
			edge.NodeA, _ = v.(string)
		}
		if v, ok := rec.Get("nodeB"); ok {
			edge.NodeB, _ = v.(string)
		}
		if v, ok := rec.Get("nodeAName"); ok {
			edge.NodeAName, _ = v.(string)
		}
		if v, ok := rec.Get("nodeBName"); ok {
			edge.NodeBName, _ = v.(string)
		}
		if v, ok := rec.Get("weight"); ok {
			edge.Weight, _ = v.(float64)
		}
		if v, ok := rec.Get("slowWeight"); ok {
			edge.SlowWeight, _ = v.(float64)
		}
		if v, ok := rec.Get("fastWeight"); ok {
			edge.FastWeight, _ = v.(float64)
		}
		if v, ok := rec.Get("coActivations"); ok {
			edge.CoActivations = int(v.(int64))
		}
		if v, ok := rec.Get("lastActivated"); ok {
			if t, ok := v.(time.Time); ok {
				edge.LastActivated = t
				edge.AgeInDays = int(time.Since(t).Hours() / 24)
			}
		}
		edges = append(edges, edge)
	}

	log.Printf("✅ [EDGE_ZONES] Found %d emerging edges for patient %d", len(edges), patientID)

	return edges, nil
}

// GetWeakEdges retorna arestas fracas candidatas a pruning
// AÇÃO: Prune se idade > pruningAge dias
func (c *EdgeClassifier) GetWeakEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE r.weight <= $threshold
		  AND n1 <> p AND n2 <> p
		  AND duration.between(COALESCE(r.last_activated, r.created_at, datetime()), datetime()).days > $pruningAge
		RETURN
			toString(id(n1)) AS nodeA,
			toString(id(n2)) AS nodeB,
			COALESCE(n1.name, n1.content, 'Unknown') AS nodeAName,
			COALESCE(n2.name, n2.content, 'Unknown') AS nodeBName,
			r.weight AS weight,
			duration.between(COALESCE(r.last_activated, r.created_at, datetime()), datetime()).days AS ageInDays
		ORDER BY ageInDays DESC
		LIMIT 100
	`

	records, err := c.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId":  patientID,
		"threshold":  c.thresholdLow,
		"pruningAge": c.pruningAge,
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	edges := make([]AssociationEdge, 0)

	for _, rec := range records {
		edge := AssociationEdge{Zone: ZoneWeak}
		if v, ok := rec.Get("nodeA"); ok {
			edge.NodeA, _ = v.(string)
		}
		if v, ok := rec.Get("nodeB"); ok {
			edge.NodeB, _ = v.(string)
		}
		if v, ok := rec.Get("nodeAName"); ok {
			edge.NodeAName, _ = v.(string)
		}
		if v, ok := rec.Get("nodeBName"); ok {
			edge.NodeBName, _ = v.(string)
		}
		if v, ok := rec.Get("weight"); ok {
			edge.Weight, _ = v.(float64)
		}
		if v, ok := rec.Get("ageInDays"); ok {
			edge.AgeInDays = int(v.(int64))
		}
		edges = append(edges, edge)
	}

	log.Printf("✅ [EDGE_ZONES] Found %d weak edges (candidates for pruning) for patient %d", len(edges), patientID)

	return edges, nil
}

// PruneWeakEdges remove arestas fracas antigas
// Executado periodicamente (consolidação noturna)
func (c *EdgeClassifier) PruneWeakEdges(ctx context.Context, patientID int64) (*PruningResult, error) {
	if c.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	log.Printf("🗑️ [EDGE_ZONES] Starting pruning for patient %d (threshold=%.2f, age>%d days)",
		patientID, c.thresholdLow, c.pruningAge)

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE r.weight <= $threshold
		  AND duration.between(COALESCE(r.last_activated, r.created_at, datetime()), datetime()).days > $pruningAge
		WITH r, n1, n2
		DELETE r
		RETURN count(r) AS pruned
	`

	records, err := c.neo4j.ExecuteWriteAndReturn(ctx, query, map[string]interface{}{
		"patientId":  patientID,
		"threshold":  c.thresholdLow,
		"pruningAge": c.pruningAge,
	})

	result := &PruningResult{
		PatientID:   patientID,
		Threshold:   c.thresholdLow,
		PruningAge:  c.pruningAge,
		EdgesPruned: 0,
		Timestamp:   time.Now(),
	}

	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	if len(records) > 0 {
		if v, ok := records[0].Get("pruned"); ok {
			result.EdgesPruned = int(v.(int64))
		}
	}

	log.Printf("✅ [EDGE_ZONES] Pruned %d weak edges for patient %d", result.EdgesPruned, patientID)

	return result, nil
}

// GetZoneStatistics retorna estatísticas das zonas para um paciente
func (c *EdgeClassifier) GetZoneStatistics(ctx context.Context, patientID int64) (*ZoneStatistics, error) {
	if c.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM]-(n2)
		WHERE n1 <> p AND n2 <> p
		WITH
			count(CASE WHEN r.weight > $thresholdHigh THEN 1 END) AS consolidated,
			count(CASE WHEN r.weight > $thresholdLow AND r.weight <= $thresholdHigh THEN 1 END) AS emerging,
			count(CASE WHEN r.weight <= $thresholdLow THEN 1 END) AS weak,
			avg(r.weight) AS avgWeight,
			max(r.weight) AS maxWeight,
			min(r.weight) AS minWeight
		RETURN consolidated, emerging, weak, avgWeight, maxWeight, minWeight
	`

	records, err := c.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId":     patientID,
		"thresholdHigh": c.thresholdHigh,
		"thresholdLow":  c.thresholdLow,
	})
	if err != nil {
		return nil, err
	}

	stats := &ZoneStatistics{
		PatientID: patientID,
		Timestamp: time.Now(),
	}

	if len(records) > 0 {
		rec := records[0]
		if v, ok := rec.Get("consolidated"); ok {
			stats.ConsolidatedCount = int(v.(int64))
		}
		if v, ok := rec.Get("emerging"); ok {
			stats.EmergingCount = int(v.(int64))
		}
		if v, ok := rec.Get("weak"); ok {
			stats.WeakCount = int(v.(int64))
		}
		if v, ok := rec.Get("avgWeight"); ok {
			if f, ok := v.(float64); ok {
				stats.AvgWeight = f
			}
		}
		if v, ok := rec.Get("maxWeight"); ok {
			if f, ok := v.(float64); ok {
				stats.MaxWeight = f
			}
		}
		if v, ok := rec.Get("minWeight"); ok {
			if f, ok := v.(float64); ok {
				stats.MinWeight = f
			}
		}
	}

	return stats, nil
}

// Structs

// PruningResult resultado de uma operação de pruning
type PruningResult struct {
	PatientID   int64
	Threshold   float64
	PruningAge  int
	EdgesPruned int
	Timestamp   time.Time
	Error       string
}

// ZoneStatistics estatísticas das zonas de um paciente
type ZoneStatistics struct {
	PatientID         int64
	ConsolidatedCount int
	EmergingCount     int
	WeakCount         int
	TotalEdges        int
	AvgWeight         float64
	MaxWeight         float64
	MinWeight         float64
	Timestamp         time.Time
}

// String implementa fmt.Stringer
func (s *ZoneStatistics) String() string {
	s.TotalEdges = s.ConsolidatedCount + s.EmergingCount + s.WeakCount
	return fmt.Sprintf("EdgeZones (patient=%d): %d consolidated, %d emerging, %d weak | avg=%.3f, max=%.3f, min=%.3f",
		s.PatientID, s.ConsolidatedCount, s.EmergingCount, s.WeakCount, s.AvgWeight, s.MaxWeight, s.MinWeight)
}
