// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Edge Zones Classification
// Classifica arestas em 3 zonas: Consolidated, Emerging, Weak
// Cada zona tem uma acao automatica associada
// Fase C do plano de implementacao
package memory

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// EdgeZone representa a zona de uma aresta baseada em seu peso
type EdgeZone string

const (
	// ZoneConsolidated: w > 0.7 - Associacoes fortes e consolidadas
	// Acao: Preload no contexto Gemini automaticamente
	ZoneConsolidated EdgeZone = "consolidated"

	// ZoneEmerging: 0.3 < w < 0.7 - Associacoes em formacao
	// Acao: Sugerir ao cuidador para revisao
	ZoneEmerging EdgeZone = "emerging"

	// ZoneWeak: w < 0.3 - Associacoes fracas/decaindo
	// Acao: Candidata a pruning periodico
	ZoneWeak EdgeZone = "weak"
)

// EdgeClassifier classifica arestas em zonas
type EdgeClassifier struct {
	graphAdapter  *nietzscheInfra.GraphAdapter
	thresholdHigh float64 // 0.7 - Consolidated
	thresholdLow  float64 // 0.3 - Weak
	pruningAge    int     // Dias para prune weak edges
}

// EdgeClassifierConfig configuracao do classificador
type EdgeClassifierConfig struct {
	ThresholdHigh float64
	ThresholdLow  float64
	PruningAge    int // Dias
}

// AssociationEdge representa uma aresta com metadata
type AssociationEdge struct {
	NodeA         string    `json:"node_a"`
	NodeB         string    `json:"node_b"`
	NodeAName     string    `json:"node_a_name"`
	NodeBName     string    `json:"node_b_name"`
	Weight        float64   `json:"weight"`
	SlowWeight    float64   `json:"slow_weight"`
	FastWeight    float64   `json:"fast_weight"`
	CoActivations int       `json:"co_activations"`
	LastActivated time.Time `json:"last_activated"`
	Zone          EdgeZone  `json:"zone"`
	AgeInDays     int       `json:"age_in_days"`
}

// NewEdgeClassifier cria um novo classificador de arestas
func NewEdgeClassifier(graphAdapter *nietzscheInfra.GraphAdapter, config *EdgeClassifierConfig) *EdgeClassifier {
	if config == nil {
		config = &EdgeClassifierConfig{
			ThresholdHigh: 0.7,
			ThresholdLow:  0.3,
			PruningAge:    30,
		}
	}

	return &EdgeClassifier{
		graphAdapter:  graphAdapter,
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

// getPatientEdges helper: BFS from patient, then collect ASSOCIADO_COM edges from neighbors
func (c *EdgeClassifier) getPatientEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	patientNodeID := fmt.Sprintf("%d", patientID)
	neighborIDs, err := c.graphAdapter.Bfs(ctx, patientNodeID, 2, "")
	if err != nil {
		return nil, fmt.Errorf("BFS failed: %w", err)
	}

	var allEdges []AssociationEdge
	seen := make(map[string]bool) // deduplicate edges

	for _, nid := range neighborIDs {
		if nid == patientNodeID {
			continue
		}

		nql := `MATCH (n)-[r:ASSOCIADO_COM]-(m) WHERE n.id = $nodeID RETURN r, n, m`
		result, err := c.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"nodeID": nid,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range result.NodePairs {
			// Deduplicate by edge ID or from+to pair
			edgeKey := pair.From.ID + "-" + pair.To.ID
			reverseKey := pair.To.ID + "-" + pair.From.ID
			if seen[edgeKey] || seen[reverseKey] {
				continue
			}
			seen[edgeKey] = true

			// Skip if either node is the patient node itself
			if pair.From.ID == patientNodeID || pair.To.ID == patientNodeID {
				continue
			}

			edge := AssociationEdge{
				NodeA: pair.From.ID,
				NodeB: pair.To.ID,
			}

			// Node names
			if n, ok := pair.From.Content["name"]; ok {
				edge.NodeAName = fmt.Sprintf("%v", n)
			} else if n, ok := pair.From.Content["content"]; ok {
				edge.NodeAName = fmt.Sprintf("%v", n)
			} else {
				edge.NodeAName = "Unknown"
			}
			if n, ok := pair.To.Content["name"]; ok {
				edge.NodeBName = fmt.Sprintf("%v", n)
			} else if n, ok := pair.To.Content["content"]; ok {
				edge.NodeBName = fmt.Sprintf("%v", n)
			} else {
				edge.NodeBName = "Unknown"
			}

			// Edge weights
			if v, ok := pair.To.Content["weight"]; ok {
				if f, ok := v.(float64); ok {
					edge.Weight = f
				}
			}
			if v, ok := pair.To.Content["slow_weight"]; ok {
				if f, ok := v.(float64); ok {
					edge.SlowWeight = f
				}
			} else {
				edge.SlowWeight = edge.Weight
			}
			if v, ok := pair.To.Content["fast_weight"]; ok {
				if f, ok := v.(float64); ok {
					edge.FastWeight = f
				}
			} else {
				edge.FastWeight = edge.Weight
			}
			if v, ok := pair.To.Content["co_activation_count"]; ok {
				switch cv := v.(type) {
				case float64:
					edge.CoActivations = int(cv)
				case int64:
					edge.CoActivations = int(cv)
				case int:
					edge.CoActivations = cv
				}
			}

			// Last activated
			edge.LastActivated = time.Now()
			if v, ok := pair.To.Content["last_activated"]; ok {
				if f, ok := v.(float64); ok {
					edge.LastActivated = time.Unix(int64(f), 0)
				}
			}
			edge.AgeInDays = int(time.Since(edge.LastActivated).Hours() / 24)

			// Classify
			edge.Zone = c.Classify(edge.Weight)

			allEdges = append(allEdges, edge)
		}
	}

	return allEdges, nil
}

// GetConsolidatedEdges retorna arestas consolidadas para um paciente
// ACAO: Essas arestas sao preloaded no contexto Gemini
func (c *EdgeClassifier) GetConsolidatedEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	allEdges, err := c.getPatientEdges(ctx, patientID)
	if err != nil {
		return nil, err
	}

	edges := make([]AssociationEdge, 0)
	for _, e := range allEdges {
		if e.Weight > c.thresholdHigh {
			e.Zone = ZoneConsolidated
			edges = append(edges, e)
			if len(edges) >= 50 {
				break
			}
		}
	}

	log.Printf("[EDGE_ZONES] Found %d consolidated edges for patient %d", len(edges), patientID)
	return edges, nil
}

// GetEmergingEdges retorna arestas emergentes para um paciente
// ACAO: Sugerir ao cuidador para confirmacao/rejeicao
func (c *EdgeClassifier) GetEmergingEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	allEdges, err := c.getPatientEdges(ctx, patientID)
	if err != nil {
		return nil, err
	}

	edges := make([]AssociationEdge, 0)
	for _, e := range allEdges {
		if e.Weight > c.thresholdLow && e.Weight <= c.thresholdHigh {
			e.Zone = ZoneEmerging
			edges = append(edges, e)
			if len(edges) >= 30 {
				break
			}
		}
	}

	log.Printf("[EDGE_ZONES] Found %d emerging edges for patient %d", len(edges), patientID)
	return edges, nil
}

// GetWeakEdges retorna arestas fracas candidatas a pruning
// ACAO: Prune se idade > pruningAge dias
func (c *EdgeClassifier) GetWeakEdges(ctx context.Context, patientID int64) ([]AssociationEdge, error) {
	if c.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	allEdges, err := c.getPatientEdges(ctx, patientID)
	if err != nil {
		return nil, err
	}

	edges := make([]AssociationEdge, 0)
	for _, e := range allEdges {
		if e.Weight <= c.thresholdLow && e.AgeInDays > c.pruningAge {
			e.Zone = ZoneWeak
			edges = append(edges, e)
			if len(edges) >= 100 {
				break
			}
		}
	}

	log.Printf("[EDGE_ZONES] Found %d weak edges (candidates for pruning) for patient %d", len(edges), patientID)
	return edges, nil
}

// PruneWeakEdges remove arestas fracas antigas
// Executado periodicamente (consolidacao noturna)
func (c *EdgeClassifier) PruneWeakEdges(ctx context.Context, patientID int64) (*PruningResult, error) {
	if c.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	log.Printf("[EDGE_ZONES] Starting pruning for patient %d (threshold=%.2f, age>%d days)",
		patientID, c.thresholdLow, c.pruningAge)

	result := &PruningResult{
		PatientID:   patientID,
		Threshold:   c.thresholdLow,
		PruningAge:  c.pruningAge,
		EdgesPruned: 0,
		Timestamp:   time.Now(),
	}

	weakEdges, err := c.GetWeakEdges(ctx, patientID)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	pruned := 0
	for _, edge := range weakEdges {
		// Find the edge ID by querying
		nql := `MATCH (a)-[r:ASSOCIADO_COM]-(b) WHERE a.id = $nodeA AND b.id = $nodeB RETURN r LIMIT 1`
		qr, err := c.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"nodeA": edge.NodeA,
			"nodeB": edge.NodeB,
		}, "")
		if err != nil || len(qr.NodePairs) == 0 {
			continue
		}

		edgeID := qr.NodePairs[0].From.ID
		if err := c.graphAdapter.DeleteEdge(ctx, edgeID, ""); err == nil {
			pruned++
		}
	}

	result.EdgesPruned = pruned
	log.Printf("[EDGE_ZONES] Pruned %d weak edges for patient %d", result.EdgesPruned, patientID)

	return result, nil
}

// GetZoneStatistics retorna estatisticas das zonas para um paciente
func (c *EdgeClassifier) GetZoneStatistics(ctx context.Context, patientID int64) (*ZoneStatistics, error) {
	if c.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	allEdges, err := c.getPatientEdges(ctx, patientID)
	if err != nil {
		return nil, err
	}

	stats := &ZoneStatistics{
		PatientID: patientID,
		Timestamp: time.Now(),
	}

	var sumWeight, maxWeight, minWeight float64
	minWeight = 1.0
	if len(allEdges) == 0 {
		minWeight = 0.0
	}

	for _, e := range allEdges {
		switch {
		case e.Weight > c.thresholdHigh:
			stats.ConsolidatedCount++
		case e.Weight > c.thresholdLow:
			stats.EmergingCount++
		default:
			stats.WeakCount++
		}

		sumWeight += e.Weight
		if e.Weight > maxWeight {
			maxWeight = e.Weight
		}
		if e.Weight < minWeight {
			minWeight = e.Weight
		}
	}

	if len(allEdges) > 0 {
		stats.AvgWeight = sumWeight / float64(len(allEdges))
	}
	stats.MaxWeight = maxWeight
	stats.MinWeight = minWeight

	return stats, nil
}

// Structs

// PruningResult resultado de uma operacao de pruning
type PruningResult struct {
	PatientID   int64
	Threshold   float64
	PruningAge  int
	EdgesPruned int
	Timestamp   time.Time
	Error       string
}

// ZoneStatistics estatisticas das zonas de um paciente
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
