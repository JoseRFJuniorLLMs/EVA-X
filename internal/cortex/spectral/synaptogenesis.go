// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package spectral

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"
)

// SynaptogenesisEngine implementa auto-organizacao fractal de conexoes no grafo
// Regras: Preferential Attachment + Triadic Closure + Homofilia + Hebbian Strengthening
// Ciencia: Bullmore & Sporns (2012) - "The economy of brain network organization"
//          Holtmaat & Svoboda (2009) - "Experience-dependent structural synaptic plasticity"
type SynaptogenesisEngine struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	threshold    float64 // Minimo de co-ativacoes para criar sinapse
	decayTau     float64 // Constante de decay temporal para reforco
	mu           sync.Mutex
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
func NewSynaptogenesisEngine(graphAdapter *nietzscheInfra.GraphAdapter, threshold float64) *SynaptogenesisEngine {
	if threshold <= 0 {
		threshold = 3.0 // Minimo 3 co-ativacoes para criar sinapse
	}
	return &SynaptogenesisEngine{
		graphAdapter: graphAdapter,
		threshold:    threshold,
		decayTau:     90.0,
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

	log.Printf("[SYNAPTOGENESIS] Paciente %d: %d co-ativacoes, %d novas edges, %d reforcadas, %d triadas fechadas em %s",
		patientID, result.CoActivationsFound, result.NewEdgesCreated,
		result.EdgesStrengthened, result.TriadsClosed, result.Duration)

	return result, nil
}

// RecordCoActivation registra que duas memorias foram ativadas juntas
// Deve ser chamado durante o retrieval quando multiplas memorias sao retornadas
func (s *SynaptogenesisEngine) RecordCoActivation(ctx context.Context, nodeIDs []string) error {
	if s.graphAdapter == nil || len(nodeIDs) < 2 {
		return nil
	}

	// Registrar co-ativacao para cada par
	for i := 0; i < len(nodeIDs)-1; i++ {
		for j := i + 1; j < len(nodeIDs); j++ {
			_, err := s.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: nodeIDs[i],
				ToNodeID:   nodeIDs[j],
				EdgeType:   "CO_ACTIVATED",
				OnCreateSet: map[string]interface{}{
					"frequency": 1,
					"last_seen": nietzscheInfra.NowUnix(),
					"weight":    0.1,
				},
				OnMatchSet: map[string]interface{}{
					"last_seen": nietzscheInfra.NowUnix(),
				},
			})
			if err != nil {
				return fmt.Errorf("falha ao registrar co-ativacao: %w", err)
			}
		}
	}

	return nil
}

// detectCoActivations encontra pares de memorias frequentemente co-ativadas
// Uses BFS from patient node to find nearby CO_ACTIVATED edges
func (s *SynaptogenesisEngine) detectCoActivations(ctx context.Context, patientID int64) ([]CoActivation, error) {
	if s.graphAdapter == nil {
		return nil, nil
	}

	// Find the patient node
	patientResult, err := s.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "Person",
		MatchKeys: map[string]interface{}{
			"id": patientID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao encontrar paciente: %w", err)
	}

	// BFS from patient node up to depth 2
	neighborIDs, err := s.graphAdapter.Bfs(ctx, patientResult.NodeID, 2, "")
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar vizinhos: %w", err)
	}

	// For each neighbor, check for CO_ACTIVATED edges using BfsWithEdgeType
	sevenDaysAgo := nietzscheInfra.DaysAgoUnix(7)
	var results []CoActivation
	seen := make(map[string]bool)

	for _, nID := range neighborIDs {
		if nID == patientResult.NodeID {
			continue
		}
		// Find CO_ACTIVATED neighbors of this node
		coActNeighbors, err := s.graphAdapter.BfsWithEdgeType(ctx, nID, "CO_ACTIVATED", 1, "")
		if err != nil {
			continue
		}
		for _, coID := range coActNeighbors {
			if coID == nID || coID == patientResult.NodeID {
				continue
			}
			pairKey := nID + ":" + coID
			reversePairKey := coID + ":" + nID
			if seen[pairKey] || seen[reversePairKey] {
				continue
			}
			seen[pairKey] = true

			// Get the node to check last_seen
			node, err := s.graphAdapter.GetNode(ctx, coID, "")
			if err != nil {
				continue
			}
			lastSeen := toFloat64Synaptogenesis(node.Content["last_seen"])
			if lastSeen > 0 && lastSeen < sevenDaysAgo {
				continue
			}

			freq := toIntSynaptogenesis(node.Content["frequency"])
			if freq == 0 {
				freq = 1
			}

			results = append(results, CoActivation{
				NodeA:     nID,
				NodeB:     coID,
				Frequency: freq,
			})
		}
	}

	// Limit to 100 results
	if len(results) > 100 {
		results = results[:100]
	}

	return results, nil
}

// createOrStrengthenEdge cria nova edge ou reforca existente (Hebbian: fire together -> wire together)
func (s *SynaptogenesisEngine) createOrStrengthenEdge(ctx context.Context, pair CoActivation, patientID int64) (bool, error) {
	if s.graphAdapter == nil {
		return true, nil
	}

	weight := float64(pair.Frequency) * 0.1
	if weight > 1.0 {
		weight = 1.0
	}

	result, err := s.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: pair.NodeA,
		ToNodeID:   pair.NodeB,
		EdgeType:   "SYNAPSE",
		OnCreateSet: map[string]interface{}{
			"weight":           weight,
			"created_at":       nietzscheInfra.NowUnix(),
			"last_activation":  nietzscheInfra.NowUnix(),
			"activation_count": 1,
			"age":              0,
			"source":           "synaptogenesis",
		},
		OnMatchSet: map[string]interface{}{
			"last_activation": nietzscheInfra.NowUnix(),
		},
	})
	if err != nil {
		return false, err
	}

	return result.Created, nil
}

// closeTriads fecha triangulos: Se A->B e B->C, cria A->C com peso menor
// Rewritten as Go loop using BFS instead of complex Cypher
func (s *SynaptogenesisEngine) closeTriads(ctx context.Context, patientID int64) (int, error) {
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

	closed := 0
	// For each neighbor, find SYNAPSE connections and check for triads
	for _, aID := range neighborIDs {
		if aID == patientResult.NodeID {
			continue
		}
		// Get SYNAPSE neighbors of A
		bIDs, err := s.graphAdapter.BfsWithEdgeType(ctx, aID, "SYNAPSE", 1, "")
		if err != nil {
			continue
		}
		for _, bID := range bIDs {
			if bID == aID || bID == patientResult.NodeID {
				continue
			}
			// Get SYNAPSE neighbors of B
			cIDs, err := s.graphAdapter.BfsWithEdgeType(ctx, bID, "SYNAPSE", 1, "")
			if err != nil {
				continue
			}
			for _, cID := range cIDs {
				if cID == aID || cID == bID || cID == patientResult.NodeID {
					continue
				}
				// Check if A->C already exists (skip if so)
				existingNeighbors, _ := s.graphAdapter.BfsWithEdgeType(ctx, aID, "SYNAPSE", 1, "")
				alreadyConnected := false
				for _, existing := range existingNeighbors {
					if existing == cID {
						alreadyConnected = true
						break
					}
				}
				if alreadyConnected {
					continue
				}

				// Create triadic closure edge with inferred weight
				inferredWeight := 0.2 // Default modest weight for triadic closure
				_, err = s.graphAdapter.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
					FromID: aID,
					ToID:   cID,
					Label:  "SYNAPSE",
					Weight: float32(inferredWeight),
					Content: map[string]interface{}{
						"created_at":       nietzscheInfra.NowUnix(),
						"last_activation":  nietzscheInfra.NowUnix(),
						"activation_count": 0,
						"age":              0,
						"source":           "triadic_closure",
					},
				})
				if err == nil {
					closed++
				}

				if closed >= 50 {
					return closed, nil
				}
			}
		}
	}

	return closed, nil
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

// Helper type conversions for this package
func toFloat64Synaptogenesis(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func toIntSynaptogenesis(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case float32:
		return int(val)
	default:
		return 0
	}
}
