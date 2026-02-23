// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package evolution

import (
	"context"
	"fmt"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// DreamService manages speculative graph exploration via NietzscheDB's Dream Engine.
// Dreams are transient simulations that diffuse energy from a seed node with stochastic
// noise, detecting energy spikes, curvature anomalies, and latent connections.
// Results can be accepted (APPLY) or discarded (REJECT) without side effects.
type DreamService struct {
	client *nietzscheInfra.GraphAdapter
}

// NewDreamService creates a new dream exploration service.
func NewDreamService(client *nietzscheInfra.GraphAdapter) *DreamService {
	return &DreamService{client: client}
}

// DreamCycleResult holds the outcome of a complete dream cycle
// (exploration + evaluation + commit/reject decision).
type DreamCycleResult struct {
	DreamID    string `json:"dream_id"`
	Collection string `json:"collection"`
	SeedNodeID string `json:"seed_node_id"`
	EventCount int    `json:"event_count"`
	NodeCount  int    `json:"node_count"`
	Accepted   bool   `json:"accepted"`
	Reason     string `json:"reason"`
}

// RunDreamCycle executes a full speculative exploration cycle on a collection:
//  1. Starts a dream from the given seed node
//  2. Evaluates whether the dream results are beneficial
//  3. Commits or rejects based on evaluation
func (d *DreamService) RunDreamCycle(ctx context.Context, collection string, seedNodeID string) (*DreamCycleResult, error) {
	ndb := d.client.Client()
	if ndb == nil {
		return nil, fmt.Errorf("nietzsche client not available")
	}

	log.Printf("[DREAM] Iniciando ciclo de sonho especulativo na colecao: %s (seed: %s)", collection, seedNodeID)

	// 1. Start speculative exploration (depth=5, noise=0.05)
	dreamResult, err := ndb.StartDream(ctx, collection, seedNodeID, 5, 0.05)
	if err != nil {
		log.Printf("[DREAM] Erro ao iniciar sonho: %v", err)
		return nil, fmt.Errorf("dream cycle start: %w", err)
	}

	result := &DreamCycleResult{
		DreamID:    dreamResult.DreamID,
		Collection: collection,
		SeedNodeID: seedNodeID,
		EventCount: len(dreamResult.Events),
		NodeCount:  len(dreamResult.Nodes),
	}

	if dreamResult.DreamID == "" {
		result.Reason = "no dream ID returned (empty exploration)"
		log.Printf("[DREAM] Sonho vazio retornado para seed %s", seedNodeID)
		return result, nil
	}

	// 2. Evaluate whether dream is beneficial
	beneficial, reason := d.EvaluateDream(ctx, collection, dreamResult)
	result.Reason = reason

	// 3. Commit or reject
	if beneficial {
		err = ndb.ApplyDream(ctx, collection, dreamResult.DreamID)
		if err != nil {
			log.Printf("[DREAM] Erro ao aplicar sonho %s: %v", dreamResult.DreamID, err)
			return result, fmt.Errorf("dream cycle commit: %w", err)
		}
		result.Accepted = true
		log.Printf("[DREAM] Sonho %s APLICADO: %s", dreamResult.DreamID, reason)
	} else {
		err = ndb.RejectDream(ctx, collection, dreamResult.DreamID)
		if err != nil {
			log.Printf("[DREAM] Erro ao rejeitar sonho %s: %v", dreamResult.DreamID, err)
			return result, fmt.Errorf("dream cycle reject: %w", err)
		}
		log.Printf("[DREAM] Sonho %s REJEITADO: %s", dreamResult.DreamID, reason)
	}

	return result, nil
}

// EvaluateDream checks if dream results are beneficial for the graph.
// It combines heuristic event counting with neural MCTS validation.
func (d *DreamService) EvaluateDream(ctx context.Context, collection string, dream *nietzscheInfra.DreamResult) (bool, string) {
	if dream == nil {
		return false, "nil dream result"
	}

	if dream.DreamID == "" {
		return false, "empty dream ID"
	}

	eventCount := len(dream.Events)
	nodeCount := len(dream.Nodes)

	// Reject if no events were detected (exploration found nothing interesting)
	if eventCount == 0 {
		return false, fmt.Sprintf("no events detected (%d nodes explored)", nodeCount)
	}

	// Neural Validation (Phase 5):
	// Use MCTS to verify if the seed node has a high-value action in the "dreamt" state.
	// We use "clinical_reasoner" model as the primary evaluator.
	mctsRes, err := d.client.MctsSearch(ctx, "clinical_reasoner", dream.SeedNodeID, 100, collection)
	if err != nil {
		log.Printf("[DREAM] MCTS validation failed: %v. Falling back to heuristic evaluation.", err)
		// Fallback to heuristic: if nodes were affected, it's beneficial
		if nodeCount > 0 {
			return true, fmt.Sprintf("%d events detected across %d nodes (heuristic fallback)", eventCount, nodeCount)
		}
		return false, "MCTS failed and no nodes affected"
	}

	// If MCTS finds a high-confidence action (> 0.6), the dream has revealed a useful clinical path.
	if mctsRes.Value > 0.6 {
		return true, fmt.Sprintf("MCTS verified value %.2f (best action: %s) with %d events", mctsRes.Value, mctsRes.BestActionID, eventCount)
	}

	// If MCTS value is low, but we have many events, still accept (curiosity-driven)
	if eventCount > 3 {
		return true, fmt.Sprintf("accepted on curiosity (events: %d) despite low MCTS value %.2f", eventCount, mctsRes.Value)
	}

	return false, fmt.Sprintf("insufficient value revealed: MCTS %.2f, events %d", mctsRes.Value, eventCount)
}

// CommitDream applies a pending dream, persisting energy changes to the graph.
func (d *DreamService) CommitDream(ctx context.Context, collection string, dreamID string) error {
	ndb := d.client.Client()
	if ndb == nil {
		return fmt.Errorf("nietzsche client not available")
	}

	return ndb.ApplyDream(ctx, collection, dreamID)
}

// RejectDream discards a pending dream without modifying the graph.
func (d *DreamService) RejectDream(ctx context.Context, collection string, dreamID string) error {
	ndb := d.client.Client()
	if ndb == nil {
		return fmt.Errorf("nietzsche client not available")
	}

	return ndb.RejectDream(ctx, collection, dreamID)
}

// ShowPendingDreams lists all pending dream sessions for a collection.
func (d *DreamService) ShowPendingDreams(ctx context.Context, collection string) ([]map[string]interface{}, error) {
	ndb := d.client.Client()
	if ndb == nil {
		return nil, fmt.Errorf("nietzsche client not available")
	}

	return ndb.ShowDreams(ctx, collection)
}

// RunDreamFromHighEnergy finds high-energy nodes in a collection and runs
// dream cycles from them. This is useful for autonomous exploration of
// promising graph regions during REM consolidation.
func (d *DreamService) RunDreamFromHighEnergy(ctx context.Context, collection string, energyThreshold float64, maxDreams int) ([]*DreamCycleResult, error) {
	log.Printf("[DREAM] Buscando nos de alta energia (>%.2f) na colecao %s para sonho autonomo", energyThreshold, collection)

	// Find high-energy nodes via NQL
	nql := fmt.Sprintf(`MATCH (n) WHERE n.energy > %.4f RETURN n ORDER BY n.energy DESC LIMIT %d`, energyThreshold, maxDreams)
	queryResult, err := d.client.ExecuteNQL(ctx, nql, nil, collection)
	if err != nil {
		return nil, fmt.Errorf("dream high-energy query: %w", err)
	}

	if len(queryResult.Nodes) == 0 {
		log.Printf("[DREAM] Nenhum no com energia > %.2f encontrado", energyThreshold)
		return nil, nil
	}

	log.Printf("[DREAM] Encontrados %d nos de alta energia, iniciando sonhos", len(queryResult.Nodes))

	var results []*DreamCycleResult
	for _, node := range queryResult.Nodes {
		if node.ID == "" {
			continue
		}

		result, err := d.RunDreamCycle(ctx, collection, node.ID)
		if err != nil {
			log.Printf("[DREAM] Erro no sonho para no %s: %v", node.ID, err)
			continue
		}
		results = append(results, result)
	}

	log.Printf("[DREAM] Ciclo de sonhos autonomos concluido: %d sonhos executados", len(results))
	return results, nil
}

// CleanupPendingDreams rejects all pending dream sessions for a collection.
// Useful for garbage collection after interrupted cycles.
func (d *DreamService) CleanupPendingDreams(ctx context.Context, collection string) (int, error) {
	ndb := d.client.Client()
	if ndb == nil {
		return 0, fmt.Errorf("nietzsche client not available")
	}

	dreams, err := ndb.ShowDreams(ctx, collection)
	if err != nil {
		return 0, fmt.Errorf("dream cleanup list: %w", err)
	}

	rejected := 0
	for _, dream := range dreams {
		dreamID, ok := dream["dream_id"].(string)
		if !ok || dreamID == "" {
			continue
		}

		if err := ndb.RejectDream(ctx, collection, dreamID); err != nil {
			log.Printf("[DREAM] Erro ao rejeitar sonho pendente %s: %v", dreamID, err)
			continue
		}
		rejected++
	}

	if rejected > 0 {
		log.Printf("[DREAM] Limpeza: %d sonhos pendentes rejeitados na colecao %s", rejected, collection)
	}

	return rejected, nil
}
