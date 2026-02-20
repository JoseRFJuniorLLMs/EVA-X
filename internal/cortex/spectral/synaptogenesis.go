// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package spectral

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"eva/internal/brainstem/infrastructure/graph"
)

// SynaptogenesisEngine implementa auto-organizacao fractal de conexoes no grafo
// Regras: Preferential Attachment + Triadic Closure + Homofilia + Hebbian Strengthening
// Ciencia: Bullmore & Sporns (2012) - "The economy of brain network organization"
//          Holtmaat & Svoboda (2009) - "Experience-dependent structural synaptic plasticity"
type SynaptogenesisEngine struct {
	client    *graph.Neo4jClient
	threshold float64 // Minimo de co-ativacoes para criar sinapse
	decayTau  float64 // Constante de decay temporal para reforco
	mu        sync.Mutex
}

// SynaptogenesisResult resultado de um ciclo de sinaptogenese
type SynaptogenesisResult struct {
	CycleTime          time.Time `json:"cycle_time"`
	CoActivationsFound int       `json:"co_activations_found"`
	NewEdgesCreated    int       `json:"new_edges_created"`
	EdgesStrengthened  int       `json:"edges_strengthened"`
	TriadsClosed       int       `json:"triads_closed"`
	Duration           string    `json:"duration"`
}

// CoActivation par de memorias ativadas juntas
type CoActivation struct {
	NodeA     string
	NodeB     string
	Frequency int
	LastSeen  time.Time
}

// NewSynaptogenesisEngine cria o motor de sinaptogenese
func NewSynaptogenesisEngine(client *graph.Neo4jClient, threshold float64) *SynaptogenesisEngine {
	if threshold <= 0 {
		threshold = 3.0 // Minimo 3 co-ativacoes para criar sinapse
	}
	return &SynaptogenesisEngine{
		client:    client,
		threshold: threshold,
		decayTau:  90.0,
	}
}

// GrowConnections executa sinaptogenese para um paciente
// Pipeline: Detectar co-ativacoes -> Criar/reforcar edges -> Fechar triadas
func (s *SynaptogenesisEngine) GrowConnections(ctx context.Context, patientID int64) (*SynaptogenesisResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	start := time.Now()
	result := &SynaptogenesisResult{CycleTime: start}

	log.Printf("[SYNAPTOGENESIS] Iniciando crescimento de conexoes para paciente %d", patientID)

	// 1. Detectar co-ativacoes (memorias recuperadas juntas nos ultimos 7 dias)
	coActivations, err := s.detectCoActivations(ctx, patientID)
	if err != nil {
		return nil, fmt.Errorf("falha ao detectar co-ativacoes: %w", err)
	}
	result.CoActivationsFound = len(coActivations)

	// 2. Para cada par co-ativado, criar ou reforcar edge
	for _, pair := range coActivations {
		if float64(pair.Frequency) >= s.threshold {
			created, err := s.createOrStrengthenEdge(ctx, pair, patientID)
			if err != nil {
				log.Printf("[SYNAPTOGENESIS] Erro ao criar edge %s<->%s: %v", pair.NodeA, pair.NodeB, err)
				continue
			}
			if created {
				result.NewEdgesCreated++
			} else {
				result.EdgesStrengthened++
			}
		}
	}

	// 3. Triadic Closure: Se A->B e B->C existem, criar A->C
	triads, err := s.closeTriads(ctx, patientID)
	if err != nil {
		log.Printf("[SYNAPTOGENESIS] Erro ao fechar triadas: %v", err)
	} else {
		result.TriadsClosed = triads
	}

	result.Duration = time.Since(start).String()

	log.Printf("[SYNAPTOGENESIS] Paciente %d: %d co-ativacoes, %d novas edges, %d reforçadas, %d triadas fechadas em %s",
		patientID, result.CoActivationsFound, result.NewEdgesCreated,
		result.EdgesStrengthened, result.TriadsClosed, result.Duration)

	return result, nil
}

// RecordCoActivation registra que duas memorias foram ativadas juntas
// Deve ser chamado durante o retrieval quando multiplas memorias sao retornadas
func (s *SynaptogenesisEngine) RecordCoActivation(ctx context.Context, nodeIDs []string) error {
	if s.client == nil || len(nodeIDs) < 2 {
		return nil
	}

	// Registrar co-ativacao para cada par
	for i := 0; i < len(nodeIDs)-1; i++ {
		for j := i + 1; j < len(nodeIDs); j++ {
			query := `
				MATCH (n1) WHERE toString(id(n1)) = $nodeA
				MATCH (n2) WHERE toString(id(n2)) = $nodeB
				MERGE (n1)-[r:CO_ACTIVATED]-(n2)
				SET r.frequency = COALESCE(r.frequency, 0) + 1,
				    r.last_seen = datetime(),
				    r.weight = COALESCE(r.weight, 0.0) + 0.1
			`
			_, err := s.client.ExecuteWrite(ctx, query, map[string]interface{}{
				"nodeA": nodeIDs[i],
				"nodeB": nodeIDs[j],
			})
			if err != nil {
				return fmt.Errorf("falha ao registrar co-ativacao: %w", err)
			}
		}
	}

	return nil
}

// detectCoActivations encontra pares de memorias frequentemente co-ativadas
func (s *SynaptogenesisEngine) detectCoActivations(ctx context.Context, patientID int64) ([]CoActivation, error) {
	if s.client == nil {
		return nil, nil
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(n1)-[r:CO_ACTIVATED]-(n2)
		WHERE n1 <> p AND n2 <> p
		  AND r.last_seen > datetime() - duration('P7D')
		RETURN toString(id(n1)) AS nodeA,
		       toString(id(n2)) AS nodeB,
		       COALESCE(r.frequency, 1) AS freq
		ORDER BY freq DESC
		LIMIT 100
	`

	records, err := s.client.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return nil, err
	}

	var results []CoActivation
	for _, rec := range records {
		var ca CoActivation
		if v, ok := rec.Get("nodeA"); ok {
			ca.NodeA, _ = v.(string)
		}
		if v, ok := rec.Get("nodeB"); ok {
			ca.NodeB, _ = v.(string)
		}
		if v, ok := rec.Get("freq"); ok {
			if n, ok := v.(int64); ok {
				ca.Frequency = int(n)
			}
		}
		results = append(results, ca)
	}

	return results, nil
}

// createOrStrengthenEdge cria nova edge ou reforça existente (Hebbian: fire together -> wire together)
func (s *SynaptogenesisEngine) createOrStrengthenEdge(ctx context.Context, pair CoActivation, patientID int64) (bool, error) {
	if s.client == nil {
		return true, nil
	}

	query := `
		MATCH (n1) WHERE toString(id(n1)) = $nodeA
		MATCH (n2) WHERE toString(id(n2)) = $nodeB
		MERGE (n1)-[r:SYNAPSE]->(n2)
		ON CREATE SET
			r.weight = $weight,
			r.created_at = datetime(),
			r.last_activation = datetime(),
			r.activation_count = 1,
			r.age = 0,
			r.source = 'synaptogenesis'
		ON MATCH SET
			r.weight = r.weight + $weight * 0.5,
			r.last_activation = datetime(),
			r.activation_count = COALESCE(r.activation_count, 0) + 1,
			r.age = 0
		RETURN r.activation_count AS count
	`

	weight := float64(pair.Frequency) * 0.1
	if weight > 1.0 {
		weight = 1.0
	}

	_, err := s.client.ExecuteWrite(ctx, query, map[string]interface{}{
		"nodeA":  pair.NodeA,
		"nodeB":  pair.NodeB,
		"weight": weight,
	})
	if err != nil {
		return false, err
	}

	return true, nil
}

// closeTriads fecha triangulos: Se A->B e B->C, cria A->C com peso menor
func (s *SynaptogenesisEngine) closeTriads(ctx context.Context, patientID int64) (int, error) {
	if s.client == nil {
		return 0, nil
	}

	query := `
		MATCH (p:Person {id: $patientId})-[*1..2]-(a)-[r1:SYNAPSE]->(b)-[r2:SYNAPSE]->(c)
		WHERE a <> p AND b <> p AND c <> p
		  AND a <> c
		  AND NOT (a)-[:SYNAPSE]->(c)
		  AND r1.weight > 0.3 AND r2.weight > 0.3
		WITH a, c, (r1.weight + r2.weight) / 3.0 AS inferredWeight
		LIMIT 50
		CREATE (a)-[r:SYNAPSE {
			weight: inferredWeight,
			created_at: datetime(),
			last_activation: datetime(),
			activation_count: 0,
			age: 0,
			source: 'triadic_closure'
		}]->(c)
		RETURN count(r) AS closed
	`

	_, err := s.client.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})
	if err != nil {
		return 0, err
	}

	return 0, nil
}

// AnalyzeFractalStructure analisa se o grafo tem propriedades fractais
// Retorna dimensao fractal da distribuicao de graus (power law)
func (s *SynaptogenesisEngine) AnalyzeFractalStructure(ctx context.Context, patientID int64) (map[string]interface{}, error) {
	return map[string]interface{}{
		"engine":    "synaptogenesis",
		"patient":   patientID,
		"status":    "fractal_analysis_pending",
		"threshold": s.threshold,
	}, nil
}

// GetStatistics retorna estatisticas do motor
func (s *SynaptogenesisEngine) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"engine":    "synaptogenesis_fractal",
		"threshold": s.threshold,
		"decay_tau": s.decayTau,
		"status":    "active",
	}
}
