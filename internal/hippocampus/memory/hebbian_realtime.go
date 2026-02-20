// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memory - Hebbian Real-Time Updates
// "Neurons that fire together, wire together" — Donald Hebb, 1949
// Atualiza pesos de arestas APÓS cada query (não apenas consolidação noturna)
// Com safeguards: decay rate, timeout, normalização periódica
package memory

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"eva/internal/brainstem/infrastructure/graph"
)

// HebbianRealTime atualiza pesos de arestas em tempo real
// após cada RetrieveHybrid() para reforçar associações co-ativadas
type HebbianRealTime struct {
	neo4j   *graph.Neo4jClient
	eta     float64       // Learning rate (default: 0.01)
	lambda  float64       // Decay rate - SAFEGUARD (default: 0.001)
	tau     float64       // Time constant em segundos (default: 86400 = 1 day)
	timeout time.Duration // Timeout para goroutine (default: 100ms)
}

// Config para HebbianRealTime
type HebbianRTConfig struct {
	Eta     float64       // Learning rate
	Lambda  float64       // Decay rate (safeguard contra saturação)
	Tau     float64       // Time constant (1 day = 86400s)
	Timeout time.Duration // Timeout goroutine
}

// NewHebbianRealTime cria um novo Hebbian Real-Time updater
func NewHebbianRealTime(neo4j *graph.Neo4jClient, config *HebbianRTConfig) *HebbianRealTime {
	if config == nil {
		config = &HebbianRTConfig{
			Eta:     0.01,
			Lambda:  0.001,
			Tau:     86400.0,
			Timeout: 100 * time.Millisecond,
		}
	}

	return &HebbianRealTime{
		neo4j:   neo4j,
		eta:     config.Eta,
		lambda:  config.Lambda,
		tau:     config.Tau,
		timeout: config.Timeout,
	}
}

// UpdateWeights atualiza pesos das arestas entre nós co-ativados
// Roda em goroutine (não bloqueia resposta ao usuário)
// Fórmula: Δw = η·decay(Δt) - λ·w_atual
func (h *HebbianRealTime) UpdateWeights(ctx context.Context, patientID int64, nodeIDs []string) error {
	if h.neo4j == nil || len(nodeIDs) < 2 {
		return nil
	}

	// Context com timeout (safeguard)
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	startTime := time.Now()
	updatedCount := 0

	// Para cada par de nós co-ativados
	for i := 0; i < len(nodeIDs)-1; i++ {
		for j := i + 1; j < len(nodeIDs); j++ {
			err := h.updatePairWeight(ctx, nodeIDs[i], nodeIDs[j])
			if err != nil {
				log.Printf("⚠️ [HEBBIAN_RT] Failed to update pair %s<->%s: %v", nodeIDs[i], nodeIDs[j], err)
				continue
			}
			updatedCount++
		}
	}

	duration := time.Since(startTime)
	log.Printf("✅ [HEBBIAN_RT] Updated %d edges in %v (patient=%d, nodes=%d)",
		updatedCount, duration, patientID, len(nodeIDs))

	return nil
}

// updatePairWeight atualiza peso de uma aresta entre dois nós
func (h *HebbianRealTime) updatePairWeight(ctx context.Context, nodeA, nodeB string) error {
	// 1. Buscar aresta existente e calcular Δt
	query := `
		MATCH (a)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		RETURN toString(id(r)) AS relID,
		       COALESCE(r.fast_weight, r.weight, 0.5) AS currentWeight,
		       COALESCE(r.last_activated, datetime()) AS lastActivated
		LIMIT 1
	`

	records, err := h.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"nodeA": nodeA,
		"nodeB": nodeB,
	})
	if err != nil {
		return fmt.Errorf("neo4j read failed: %w", err)
	}

	var currentWeight float64
	var lastActivated time.Time
	var relExists bool

	if len(records) > 0 {
		relExists = true
		rec := records[0]
		if v, ok := rec.Get("currentWeight"); ok {
			currentWeight, _ = v.(float64)
		}
		if v, ok := rec.Get("lastActivated"); ok {
			if t, ok := v.(time.Time); ok {
				lastActivated = t
			}
		}
	} else {
		// Aresta não existe, será criada
		relExists = false
		currentWeight = 0.5
		lastActivated = time.Now()
	}

	// 2. Calcular Δt desde última ativação
	deltaT := time.Since(lastActivated).Seconds()

	// 3. Calcular decay exponencial
	decay := math.Exp(-deltaT / h.tau)

	// 4. Calcular Δw com LTP + LTD + regularização
	// Δw = η * decay - λ * w_atual
	deltaW := h.eta*decay - h.lambda*currentWeight

	// 5. Novo peso = peso atual + Δw
	newWeight := currentWeight + deltaW

	// Safeguard: limitar entre 0.0 e 1.0
	newWeight = math.Max(0.0, math.Min(1.0, newWeight))

	// 6. Atualizar ou criar aresta no Neo4j
	if relExists {
		err = h.updateExistingEdge(ctx, nodeA, nodeB, newWeight)
	} else {
		err = h.createNewEdge(ctx, nodeA, nodeB, newWeight)
	}

	if err != nil {
		return err
	}

	log.Printf("   🔄 [HEBBIAN_RT] %s<->%s: %.3f → %.3f (Δw=%.4f, decay=%.3f)",
		nodeA[:8], nodeB[:8], currentWeight, newWeight, deltaW, decay)

	return nil
}

// updateExistingEdge atualiza aresta existente
func (h *HebbianRealTime) updateExistingEdge(ctx context.Context, nodeA, nodeB string, newWeight float64) error {
	query := `
		MATCH (a)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		SET r.fast_weight = $newWeight,
		    r.co_activation_count = COALESCE(r.co_activation_count, 0) + 1,
		    r.last_activated = datetime(),
		    r.hebbian_rt_updated_at = datetime()
		RETURN count(r) AS updated
	`

	_, err := h.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"nodeA":     nodeA,
		"nodeB":     nodeB,
		"newWeight": newWeight,
	})

	return err
}

// createNewEdge cria nova aresta (se não existia)
func (h *HebbianRealTime) createNewEdge(ctx context.Context, nodeA, nodeB string, initialWeight float64) error {
	query := `
		MATCH (a), (b)
		WHERE toString(id(a)) = $nodeA AND toString(id(b)) = $nodeB
		MERGE (a)-[r:ASSOCIADO_COM]-(b)
		ON CREATE SET
			r.fast_weight = $initialWeight,
			r.slow_weight = 0.5,
			r.weight = 0.3 * r.slow_weight + 0.7 * r.fast_weight,
			r.co_activation_count = 1,
			r.created_at = datetime(),
			r.last_activated = datetime(),
			r.hebbian_rt_created = true
		RETURN count(r) AS created
	`

	_, err := h.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"nodeA":         nodeA,
		"nodeB":         nodeB,
		"initialWeight": initialWeight,
	})

	if err == nil {
		log.Printf("   ✨ [HEBBIAN_RT] Created new edge %s<->%s (weight=%.3f)",
			nodeA[:8], nodeB[:8], initialWeight)
	}

	return err
}

// BoostMemories aumenta pesos de memórias específicas (feedback positivo)
// Usado quando cuidador confirma que interpretação estava correta
func (h *HebbianRealTime) BoostMemories(ctx context.Context, memoryIDs []string, boostFactor float64) error {
	if h.neo4j == nil || len(memoryIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	query := `
		UNWIND $memoryIDs AS memID
		MATCH (m:Memory)-[r]-(other)
		WHERE toString(id(m)) = memID
		SET r.fast_weight = COALESCE(r.fast_weight, r.weight, 0.5) * (1.0 + $boost),
		    r.feedback_boost_at = datetime(),
		    r.feedback_boost_count = COALESCE(r.feedback_boost_count, 0) + 1
		RETURN count(r) AS boosted
	`

	_, err := h.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"memoryIDs": memoryIDs,
		"boost":     boostFactor,
	})

	if err == nil {
		log.Printf("⬆️ [HEBBIAN_RT] Boosted %d memories by %.1f%% (positive feedback)",
			len(memoryIDs), boostFactor*100)
	}

	return err
}

// DecayMemories diminui pesos de memórias específicas (feedback negativo)
// Usado quando cuidador indica que interpretação estava incorreta
func (h *HebbianRealTime) DecayMemories(ctx context.Context, memoryIDs []string, decayFactor float64) error {
	if h.neo4j == nil || len(memoryIDs) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	query := `
		UNWIND $memoryIDs AS memID
		MATCH (m:Memory)-[r]-(other)
		WHERE toString(id(m)) = memID
		SET r.fast_weight = COALESCE(r.fast_weight, r.weight, 0.5) * (1.0 - $decay),
		    r.feedback_decay_at = datetime(),
		    r.feedback_decay_count = COALESCE(r.feedback_decay_count, 0) + 1
		WHERE r.fast_weight > 0.1
		RETURN count(r) AS decayed
	`

	_, err := h.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"memoryIDs": memoryIDs,
		"decay":     decayFactor,
	})

	if err == nil {
		log.Printf("⬇️ [HEBBIAN_RT] Decayed %d memories by %.1f%% (negative feedback)",
			len(memoryIDs), decayFactor*100)
	}

	return err
}

// GetStatistics retorna estatísticas do Hebbian Real-Time
func (h *HebbianRealTime) GetStatistics(ctx context.Context, patientID int64) (*HebbianRTStats, error) {
	if h.neo4j == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:ASSOCIADO_COM|CO_ACTIVATED]-(n2)
		WHERE r.hebbian_rt_updated_at IS NOT NULL
		RETURN
			count(r) AS totalEdges,
			avg(r.fast_weight) AS avgWeight,
			max(r.fast_weight) AS maxWeight,
			min(r.fast_weight) AS minWeight,
			sum(r.co_activation_count) AS totalCoActivations
	`

	records, err := h.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return nil, err
	}

	stats := &HebbianRTStats{
		PatientID: patientID,
		Timestamp: time.Now(),
	}

	if len(records) > 0 {
		rec := records[0]
		if v, ok := rec.Get("totalEdges"); ok {
			stats.TotalEdges = int(v.(int64))
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
		if v, ok := rec.Get("totalCoActivations"); ok {
			stats.TotalCoActivations = int(v.(int64))
		}
	}

	return stats, nil
}

// HebbianRTStats estatísticas do Hebbian Real-Time
type HebbianRTStats struct {
	PatientID           int64
	TotalEdges          int
	AvgWeight           float64
	MaxWeight           float64
	MinWeight           float64
	TotalCoActivations  int
	Timestamp           time.Time
}

// String implementa fmt.Stringer
func (s *HebbianRTStats) String() string {
	return fmt.Sprintf("HebbianRT Stats (patient=%d): %d edges, avg=%.3f, max=%.3f, min=%.3f, coactivations=%d",
		s.PatientID, s.TotalEdges, s.AvgWeight, s.MaxWeight, s.MinWeight, s.TotalCoActivations)
}
