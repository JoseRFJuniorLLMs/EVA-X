// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Hebbian Real-Time Updates
// "Neurons that fire together, wire together" -- Donald Hebb, 1949
// Atualiza pesos de arestas APOS cada query (nao apenas consolidacao noturna)
// Com safeguards: decay rate, timeout, normalizacao periodica
package memory

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// HebbianRealTime atualiza pesos de arestas em tempo real
// apos cada RetrieveHybrid() para reforcar associacoes co-ativadas
type HebbianRealTime struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	eta          float64       // Learning rate (default: 0.01)
	lambda       float64       // Decay rate - SAFEGUARD (default: 0.001)
	tau          float64       // Time constant em segundos (default: 86400 = 1 day)
	timeout      time.Duration // Timeout para goroutine (default: 100ms)
}

// Config para HebbianRealTime
type HebbianRTConfig struct {
	Eta     float64       // Learning rate
	Lambda  float64       // Decay rate (safeguard contra saturacao)
	Tau     float64       // Time constant (1 day = 86400s)
	Timeout time.Duration // Timeout goroutine
}

// NewHebbianRealTime cria um novo Hebbian Real-Time updater
func NewHebbianRealTime(graphAdapter *nietzscheInfra.GraphAdapter, config *HebbianRTConfig) *HebbianRealTime {
	if config == nil {
		config = &HebbianRTConfig{
			Eta:     0.01,
			Lambda:  0.001,
			Tau:     86400.0,
			Timeout: 100 * time.Millisecond,
		}
	}

	return &HebbianRealTime{
		graphAdapter: graphAdapter,
		eta:          config.Eta,
		lambda:       config.Lambda,
		tau:          config.Tau,
		timeout:      config.Timeout,
	}
}

// UpdateWeights atualiza pesos das arestas entre nos co-ativados
// Roda em goroutine (nao bloqueia resposta ao usuario)
// Formula: dw = n*decay(dt) - l*w_atual
func (h *HebbianRealTime) UpdateWeights(ctx context.Context, patientID int64, nodeIDs []string) error {
	if h.graphAdapter == nil || len(nodeIDs) < 2 {
		return nil
	}

	// Context com timeout (safeguard)
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	startTime := time.Now()
	updatedCount := 0

	// Para cada par de nos co-ativados
	for i := 0; i < len(nodeIDs)-1; i++ {
		for j := i + 1; j < len(nodeIDs); j++ {
			err := h.updatePairWeight(ctx, nodeIDs[i], nodeIDs[j])
			if err != nil {
				log.Printf("[HEBBIAN_RT] Failed to update pair %s<->%s: %v", nodeIDs[i], nodeIDs[j], err)
				continue
			}
			updatedCount++
		}
	}

	duration := time.Since(startTime)
	log.Printf("[HEBBIAN_RT] Updated %d edges in %v (patient=%d, nodes=%d)",
		updatedCount, duration, patientID, len(nodeIDs))

	return nil
}

// updatePairWeight atualiza peso de uma aresta entre dois nos
func (h *HebbianRealTime) updatePairWeight(ctx context.Context, nodeA, nodeB string) error {
	// 1. Buscar aresta existente via NQL
	nql := `MATCH (a)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(b) WHERE a.id = $nodeA AND b.id = $nodeB RETURN r LIMIT 1`
	result, err := h.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"nodeA": nodeA,
		"nodeB": nodeB,
	}, "")
	if err != nil {
		return fmt.Errorf("graph read failed: %w", err)
	}

	var currentWeight float64
	var lastActivated time.Time
	var relExists bool

	if len(result.NodePairs) > 0 {
		relExists = true
		edgeContent := result.NodePairs[0].To.Content
		currentWeight = 0.5
		if v, ok := edgeContent["fast_weight"]; ok {
			if f, ok := v.(float64); ok {
				currentWeight = f
			}
		} else if v, ok := edgeContent["weight"]; ok {
			if f, ok := v.(float64); ok {
				currentWeight = f
			}
		}
		lastActivated = time.Now() // default
		if v, ok := edgeContent["last_activated"]; ok {
			if f, ok := v.(float64); ok {
				lastActivated = time.Unix(int64(f), 0)
			}
		}
	} else {
		// Aresta nao existe, sera criada
		relExists = false
		currentWeight = 0.5
		lastActivated = time.Now()
	}

	// 2. Calcular dt desde ultima ativacao
	deltaT := time.Since(lastActivated).Seconds()

	// 3. Calcular decay exponencial
	decay := math.Exp(-deltaT / h.tau)

	// 4. Calcular dw com LTP + LTD + regularizacao
	// dw = eta * decay - lambda * w_atual
	deltaW := h.eta*decay - h.lambda*currentWeight

	// 5. Novo peso = peso atual + dw
	newWeight := currentWeight + deltaW

	// Safeguard: limitar entre 0.0 e 1.0
	newWeight = math.Max(0.0, math.Min(1.0, newWeight))

	// 6. Atualizar ou criar aresta
	if relExists {
		err = h.updateExistingEdge(ctx, nodeA, nodeB, newWeight)
	} else {
		err = h.createNewEdge(ctx, nodeA, nodeB, newWeight)
	}

	if err != nil {
		return err
	}

	nodeAShort := nodeA
	nodeBShort := nodeB
	if len(nodeA) > 8 {
		nodeAShort = nodeA[:8]
	}
	if len(nodeB) > 8 {
		nodeBShort = nodeB[:8]
	}
	log.Printf("   [HEBBIAN_RT] %s<->%s: %.3f -> %.3f (dw=%.4f, decay=%.3f)",
		nodeAShort, nodeBShort, currentWeight, newWeight, deltaW, decay)

	return nil
}

// updateExistingEdge atualiza aresta existente
func (h *HebbianRealTime) updateExistingEdge(ctx context.Context, nodeA, nodeB string, newWeight float64) error {
	_, err := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: nodeA,
		ToNodeID:   nodeB,
		EdgeType:   "ASSOCIADO_COM",
		OnMatchSet: map[string]interface{}{
			"fast_weight":             newWeight,
			"co_activation_count_increment": 1,
			"last_activated":          nietzscheInfra.NowUnix(),
			"hebbian_rt_updated_at":   nietzscheInfra.NowUnix(),
		},
	})
	return err
}

// createNewEdge cria nova aresta (se nao existia)
func (h *HebbianRealTime) createNewEdge(ctx context.Context, nodeA, nodeB string, initialWeight float64) error {
	combinedWeight := 0.3*0.5 + 0.7*initialWeight

	_, err := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: nodeA,
		ToNodeID:   nodeB,
		EdgeType:   "ASSOCIADO_COM",
		OnCreateSet: map[string]interface{}{
			"fast_weight":          initialWeight,
			"slow_weight":          0.5,
			"weight":               combinedWeight,
			"co_activation_count":  1,
			"created_at":           nietzscheInfra.NowUnix(),
			"last_activated":       nietzscheInfra.NowUnix(),
			"hebbian_rt_created":   true,
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
		log.Printf("   [HEBBIAN_RT] Created new edge %s<->%s (weight=%.3f)",
			nodeAShort, nodeBShort, initialWeight)
	}

	return err
}

// BoostMemories aumenta pesos de memorias especificas (feedback positivo)
// Usado quando cuidador confirma que interpretacao estava correta
func (h *HebbianRealTime) BoostMemories(ctx context.Context, memoryIDs []string, boostFactor float64) error {
	if h.graphAdapter == nil || len(memoryIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	boosted := 0
	for _, memID := range memoryIDs {
		// Find edges connected to this memory node
		nql := `MATCH (m)-[r]-(other) WHERE m.id = $memID RETURN r, other`
		result, err := h.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"memID": memID,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range result.NodePairs {
			currentWeight := 0.5
			if v, ok := pair.To.Content["fast_weight"]; ok {
				if f, ok := v.(float64); ok {
					currentWeight = f
				}
			} else if v, ok := pair.To.Content["weight"]; ok {
				if f, ok := v.(float64); ok {
					currentWeight = f
				}
			}

			newWeight := currentWeight * (1.0 + boostFactor)
			if newWeight > 1.0 {
				newWeight = 1.0
			}

			_, err := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: pair.From.ID,
				ToNodeID:   pair.To.ID,
				EdgeType:   "Association",
				OnMatchSet: map[string]interface{}{
					"fast_weight":              newWeight,
					"feedback_boost_at":        nietzscheInfra.NowUnix(),
					"feedback_boost_count_increment": 1,
				},
			})
			if err == nil {
				boosted++
			}
		}
	}

	log.Printf("[HEBBIAN_RT] Boosted %d edges across %d memories by %.1f%% (positive feedback)",
		boosted, len(memoryIDs), boostFactor*100)

	return nil
}

// DecayMemories diminui pesos de memorias especificas (feedback negativo)
// Usado quando cuidador indica que interpretacao estava incorreta
func (h *HebbianRealTime) DecayMemories(ctx context.Context, memoryIDs []string, decayFactor float64) error {
	if h.graphAdapter == nil || len(memoryIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	decayed := 0
	for _, memID := range memoryIDs {
		// Find edges connected to this memory node
		nql := `MATCH (m)-[r]-(other) WHERE m.id = $memID RETURN r, other`
		result, err := h.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"memID": memID,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range result.NodePairs {
			currentWeight := 0.5
			if v, ok := pair.To.Content["fast_weight"]; ok {
				if f, ok := v.(float64); ok {
					currentWeight = f
				}
			} else if v, ok := pair.To.Content["weight"]; ok {
				if f, ok := v.(float64); ok {
					currentWeight = f
				}
			}

			newWeight := currentWeight * (1.0 - decayFactor)
			// Only decay if weight stays above 0.1
			if newWeight < 0.1 {
				continue
			}

			_, err := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: pair.From.ID,
				ToNodeID:   pair.To.ID,
				EdgeType:   "Association",
				OnMatchSet: map[string]interface{}{
					"fast_weight":              newWeight,
					"feedback_decay_at":        nietzscheInfra.NowUnix(),
					"feedback_decay_count_increment": 1,
				},
			})
			if err == nil {
				decayed++
			}
		}
	}

	log.Printf("[HEBBIAN_RT] Decayed %d edges across %d memories by %.1f%% (negative feedback)",
		decayed, len(memoryIDs), decayFactor*100)

	return nil
}

// GetStatistics retorna estatisticas do Hebbian Real-Time
func (h *HebbianRealTime) GetStatistics(ctx context.Context, patientID int64) (*HebbianRTStats, error) {
	if h.graphAdapter == nil {
		return nil, fmt.Errorf("graph adapter not initialized")
	}

	// BFS from patient node (depth 2) to find nearby nodes
	patientNodeID := fmt.Sprintf("%d", patientID)
	neighborIDs, err := h.graphAdapter.Bfs(ctx, patientNodeID, 2, "")
	if err != nil {
		return nil, err
	}

	stats := &HebbianRTStats{
		PatientID: patientID,
		Timestamp: time.Now(),
	}

	var totalEdges int
	var sumWeight, maxWeight, minWeight float64
	var totalCoActivations int
	minWeight = math.MaxFloat64
	first := true

	for _, nid := range neighborIDs {
		nql := `MATCH (n)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(m) WHERE n.id = $nodeID AND r.hebbian_rt_updated_at IS NOT NULL RETURN r`
		result, err := h.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"nodeID": nid,
		}, "")
		if err != nil {
			continue
		}

		for _, pair := range result.NodePairs {
			fw := 0.5
			if v, ok := pair.To.Content["fast_weight"]; ok {
				if f, ok := v.(float64); ok {
					fw = f
				}
			}
			totalEdges++
			sumWeight += fw
			if fw > maxWeight {
				maxWeight = fw
			}
			if fw < minWeight {
				minWeight = fw
			}
			first = false

			if v, ok := pair.To.Content["co_activation_count"]; ok {
				switch c := v.(type) {
				case float64:
					totalCoActivations += int(c)
				case int64:
					totalCoActivations += int(c)
				case int:
					totalCoActivations += c
				}
			}
		}
	}

	if first {
		minWeight = 0
	}

	stats.TotalEdges = totalEdges
	if totalEdges > 0 {
		stats.AvgWeight = sumWeight / float64(totalEdges)
	}
	stats.MaxWeight = maxWeight
	stats.MinWeight = minWeight
	stats.TotalCoActivations = totalCoActivations

	return stats, nil
}

// HebbianRTStats estatisticas do Hebbian Real-Time
type HebbianRTStats struct {
	PatientID          int64
	TotalEdges         int
	AvgWeight          float64
	MaxWeight          float64
	MinWeight          float64
	TotalCoActivations int
	Timestamp          time.Time
}

// String implementa fmt.Stringer
func (s *HebbianRTStats) String() string {
	return fmt.Sprintf("HebbianRT Stats (patient=%d): %d edges, avg=%.3f, max=%.3f, min=%.3f, coactivations=%d",
		s.PatientID, s.TotalEdges, s.AvgWeight, s.MaxWeight, s.MinWeight, s.TotalCoActivations)
}
