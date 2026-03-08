// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package consolidation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/util"
)

// SynapticPruning implementa poda sinaptica baseada em reforco
// Conexoes nao reforcadas sao eliminadas (20% por ciclo noturno)
// Ciencia: Tononi & Cirelli (2006) - Synaptic Homeostasis Hypothesis
type SynapticPruning struct {
	graphAdapter        *nietzscheInfra.GraphAdapter
	activationThreshold int     // Minimo de ativacoes para sobreviver
	pruningRate         float64 // % de conexoes a remover (0.2 = 20%)
	maxAgeDays          int     // Idade maxima sem reforco antes de poda
	mu                  sync.Mutex
}

// PruningResult resultado de um ciclo de poda
type PruningResult struct {
	CycleTime       time.Time `json:"cycle_time"`
	TotalEdges      int       `json:"total_edges"`
	WeakEdges       int       `json:"weak_edges"`
	PrunedEdges     int       `json:"pruned_edges"`
	ReinforcedEdges int       `json:"reinforced_edges"`
	PruningRate     float64   `json:"pruning_rate_percent"`
	Duration        string    `json:"duration"`
}

// EdgeInfo informacao de uma aresta para analise de poda
type EdgeInfo struct {
	ElementID       string
	SourceID        string
	TargetID        string
	Weight          float64
	ActivationCount int
	LastActivation  time.Time
	AgeDays         int
}

// NewSynapticPruning cria um novo motor de poda sinaptica
func NewSynapticPruning(graphAdapter *nietzscheInfra.GraphAdapter) *SynapticPruning {
	return &SynapticPruning{
		graphAdapter:        graphAdapter,
		activationThreshold: 2,
		pruningRate:         0.20, // 20%
		maxAgeDays:          30,
	}
}

// PruneNightly executa poda noturna para um paciente
// Criterios: conexoes nao ativadas em maxAgeDays OU com activationCount < threshold
func (s *SynapticPruning) PruneNightly(ctx context.Context, patientID int64) (*PruningResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	result := &PruningResult{CycleTime: start}

	log.Printf("[PRUNING] Iniciando poda sinaptica para paciente %d", patientID)

	// 1. Envelhecer conexoes nao ativadas hoje
	agedCount, err := s.ageUnactivatedEdges(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao envelhecer edges: %w", err)
	}

	// 2. Contar total de edges e edges fracos
	totalEdges, weakEdges, err := s.countEdges(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao contar edges: %w", err)
	}

	result.TotalEdges = totalEdges
	result.WeakEdges = weakEdges
	result.ReinforcedEdges = totalEdges - weakEdges

	// 3. Podar conexoes fracas (bottom 20% ou idade > maxAgeDays)
	pruned, err := s.pruneWeakEdges(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao podar edges: %w", err)
	}

	result.PrunedEdges = pruned
	if totalEdges > 0 {
		result.PruningRate = float64(pruned) / float64(totalEdges) * 100.0
	}

	result.Duration = time.Since(start).String()

	log.Printf("[PRUNING] Paciente %d: %d total, %d envelhecidas, %d fracas, %d podadas (%.1f%%) em %s",
		patientID, totalEdges, agedCount, weakEdges, pruned, result.PruningRate, result.Duration)

	return result, nil
}

// ageUnactivatedEdges incrementa idade de conexoes nao ativadas hoje
// Rewritten as Go loop: BFS from patient, then check each neighbor's edges
func (s *SynapticPruning) ageUnactivatedEdges(ctx context.Context, patientID int64) (int, error) {
	if s.graphAdapter == nil {
		return 0, nil
	}

	// Find patient node
	patientResult, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": patientID,
		},
	})
	if err != nil {
		return 0, err
	}

	// BFS from patient with edge types EXPERIENCED|ASSOCIATED_WITH to depth 2
	neighborIDs, err := s.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return 0, err
	}

	aged := 0
	oneDayAgo := nietzscheInfra.DaysAgoUnix(1)

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue
		}

		// For each neighbor, find CO_ACTIVATED/RELATED_TO/ASSOCIATED_WITH edges
		for _, edgeType := range []string{"CO_ACTIVATED", "RELATED_TO", "ASSOCIATED_WITH"} {
			edgeNeighbors, err := s.graphAdapter.BfsWithEdgeType(ctx, nID, edgeType, 1, "")
			if err != nil {
				continue
			}

			for _, targetID := range edgeNeighbors {
				if targetID == nID || targetID == patientResult.NodeID {
					continue
				}
				// Get edge node to check last_activation
				node, err := s.graphAdapter.GetNode(ctx, targetID, "")
				if err != nil {
					continue
				}

				lastActivation := util.ToFloat64(node.Content["last_activation"])
				if lastActivation == 0 || lastActivation < oneDayAgo {
					currentAge := util.ToInt(node.Content["age"])
					_, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
						NodeType: node.NodeType,
						MatchKeys: map[string]interface{}{
							"id": targetID,
						},
						OnMatchSet: map[string]interface{}{
							"age": currentAge + 1,
						},
					})
					if err == nil {
						aged++
					}
				}
			}
		}
	}

	return aged, nil
}

// countEdges conta total de edges e edges fracos
// Rewritten as Go loop: BFS from patient, inspect edge metadata
func (s *SynapticPruning) countEdges(ctx context.Context, patientID int64) (total int, weak int, err error) {
	if s.graphAdapter == nil {
		return 0, 0, nil
	}

	// Find patient node
	patientResult, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": patientID,
		},
	})
	if err != nil {
		return 0, 0, err
	}

	// BFS from patient to depth 2
	neighborIDs, err := s.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return 0, 0, err
	}

	seen := make(map[string]bool)

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue
		}

		for _, edgeType := range []string{"CO_ACTIVATED", "RELATES_TO", "ASSOCIATED_WITH"} {
			edgeNeighbors, err := s.graphAdapter.BfsWithEdgeType(ctx, nID, edgeType, 1, "")
			if err != nil {
				continue
			}

			for _, targetID := range edgeNeighbors {
				if targetID == nID || targetID == patientResult.NodeID {
					continue
				}
				edgeKey := nID + "-" + targetID + "-" + edgeType
				if seen[edgeKey] {
					continue
				}
				seen[edgeKey] = true

				total++

				// Check if this edge is weak
				node, err := s.graphAdapter.GetNode(ctx, targetID, "")
				if err != nil {
					continue
				}
				age := util.ToInt(node.Content["age"])
				actCount := util.ToInt(node.Content["activation_count"])

				if age > s.maxAgeDays || actCount < s.activationThreshold {
					weak++
				}
			}
		}
	}

	return total, weak, nil
}

// pruneWeakEdges deleta conexoes fracas
// Rewritten as Go loop: BFS from patient, find CO_ACTIVATED edges that are old and unused, delete them
func (s *SynapticPruning) pruneWeakEdges(ctx context.Context, patientID int64) (int, error) {
	if s.graphAdapter == nil {
		return 0, nil
	}

	// Find patient node
	patientResult, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": patientID,
		},
	})
	if err != nil {
		return 0, err
	}

	// BFS from patient to depth 2
	neighborIDs, err := s.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return 0, err
	}

	pruned := 0
	seen := make(map[string]bool)

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue
		}

		coActNeighbors, err := s.graphAdapter.BfsWithEdgeType(ctx, nID, "CO_ACTIVATED", 1, "")
		if err != nil {
			continue
		}

		for _, targetID := range coActNeighbors {
			if targetID == nID || targetID == patientResult.NodeID {
				continue
			}
			edgeKey := nID + "-" + targetID
			reverseKey := targetID + "-" + nID
			if seen[edgeKey] || seen[reverseKey] {
				continue
			}
			seen[edgeKey] = true

			node, err := s.graphAdapter.GetNode(ctx, targetID, "")
			if err != nil {
				continue
			}

			age := util.ToInt(node.Content["age"])
			actCount := util.ToInt(node.Content["activation_count"])

			if age > s.maxAgeDays && actCount < s.activationThreshold {
				// Delete the edge (using edge ID if available, or the node-based approach)
				err := s.graphAdapter.DeleteEdge(ctx, targetID, "")
				if err == nil {
					pruned++
				}
			}

			if pruned >= 100 {
				return pruned, nil
			}
		}
	}

	return pruned, nil
}

// ResetEdgeAge reseta idade de uma conexao quando e ativada (reforco Hebbiano)
func (s *SynapticPruning) ResetEdgeAge(ctx context.Context, sourceID, targetID string) error {
	if s.graphAdapter == nil {
		return nil
	}

	_, err := s.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: sourceID,
		ToNodeID:   targetID,
		EdgeType:   "CO_ACTIVATED",
		OnMatchSet: map[string]interface{}{
			"age":             0,
			"last_activation": nietzscheInfra.NowUnix(),
		},
	})

	return err
}

// GetStatistics retorna estatisticas do motor de poda
func (s *SynapticPruning) GetStatistics() map[string]any {
	return map[string]any{
		"engine":               "synaptic_pruning",
		"activation_threshold": s.activationThreshold,
		"pruning_rate":         s.pruningRate,
		"max_age_days":         s.maxAgeDays,
		"status":               "active",
	}
}

