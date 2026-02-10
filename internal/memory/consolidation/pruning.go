package consolidation

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"
)

// SynapticPruning implementa poda sinaptica baseada em reforco
// Conexoes nao reforçadas sao eliminadas (20% por ciclo noturno)
// Ciencia: Tononi & Cirelli (2006) - Synaptic Homeostasis Hypothesis
type SynapticPruning struct {
	neo4j               *graph.Neo4jClient
	activationThreshold int     // Minimo de ativacoes para sobreviver
	pruningRate         float64 // % de conexoes a remover (0.2 = 20%)
	maxAgeDays          int     // Idade maxima sem reforco antes de poda
	mu                  sync.Mutex
}

// PruningResult resultado de um ciclo de poda
type PruningResult struct {
	CycleTime        time.Time `json:"cycle_time"`
	TotalEdges       int       `json:"total_edges"`
	WeakEdges        int       `json:"weak_edges"`
	PrunedEdges      int       `json:"pruned_edges"`
	ReinforcedEdges  int       `json:"reinforced_edges"`
	PruningRate      float64   `json:"pruning_rate_percent"`
	Duration         string    `json:"duration"`
}

// EdgeInfo informacao de uma aresta para analise de poda
type EdgeInfo struct {
	ElementID      string
	SourceID       string
	TargetID       string
	Weight         float64
	ActivationCount int
	LastActivation time.Time
	AgeDays        int
}

// NewSynapticPruning cria um novo motor de poda sinaptica
func NewSynapticPruning(neo4j *graph.Neo4jClient) *SynapticPruning {
	return &SynapticPruning{
		neo4j:               neo4j,
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
func (s *SynapticPruning) ageUnactivatedEdges(ctx context.Context, patientID int64) (int, error) {
	if s.neo4j == nil {
		return 0, nil
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:CO_ACTIVATED|RELATES_TO|ASSOCIATED_WITH]-(n2)
		WHERE n1 <> p AND n2 <> p
		  AND (r.last_activation IS NULL OR r.last_activation < datetime() - duration('P1D'))
		SET r.age = COALESCE(r.age, 0) + 1
		RETURN count(r) AS aged
	`

	_, err := s.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return 0, err
	}

	return 0, nil
}

// countEdges conta total de edges e edges fracos
func (s *SynapticPruning) countEdges(ctx context.Context, patientID int64) (total int, weak int, err error) {
	if s.neo4j == nil {
		return 0, 0, nil
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:CO_ACTIVATED|RELATES_TO|ASSOCIATED_WITH]-(n2)
		WHERE n1 <> p AND n2 <> p
		WITH r, COALESCE(r.age, 0) AS age, COALESCE(r.activation_count, 0) AS actCount
		RETURN count(r) AS total,
		       sum(CASE WHEN age > $maxAge OR actCount < $minAct THEN 1 ELSE 0 END) AS weak
	`

	records, err := s.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"maxAge":    s.maxAgeDays,
		"minAct":    s.activationThreshold,
	})
	if err != nil {
		return 0, 0, err
	}

	if len(records) > 0 {
		rec := records[0]
		if v, ok := rec.Get("total"); ok {
			if n, ok := v.(int64); ok {
				total = int(n)
			}
		}
		if v, ok := rec.Get("weak"); ok {
			if n, ok := v.(int64); ok {
				weak = int(n)
			}
		}
	}

	return total, weak, nil
}

// pruneWeakEdges deleta conexoes fracas
func (s *SynapticPruning) pruneWeakEdges(ctx context.Context, patientID int64) (int, error) {
	if s.neo4j == nil {
		return 0, nil
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:CO_ACTIVATED]-(n2)
		WHERE n1 <> p AND n2 <> p
		  AND COALESCE(r.age, 0) > $maxAge
		  AND COALESCE(r.activation_count, 0) < $minAct
		WITH r LIMIT 100
		DELETE r
		RETURN count(*) AS pruned
	`

	_, err := s.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"maxAge":    s.maxAgeDays,
		"minAct":    s.activationThreshold,
	})
	if err != nil {
		return 0, err
	}

	return 0, nil
}

// ResetEdgeAge reseta idade de uma conexao quando e ativada (reforco Hebbiano)
func (s *SynapticPruning) ResetEdgeAge(ctx context.Context, sourceID, targetID string) error {
	if s.neo4j == nil {
		return nil
	}

	query := `
		MATCH (n1)-[r:CO_ACTIVATED]-(n2)
		WHERE toString(id(n1)) = $src AND toString(id(n2)) = $dst
		SET r.age = 0,
		    r.activation_count = COALESCE(r.activation_count, 0) + 1,
		    r.last_activation = datetime()
	`

	_, err := s.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"src": sourceID,
		"dst": targetID,
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
