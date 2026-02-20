// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// TemporalDecayService adiciona envelhecimento as conexoes do grafo de conhecimento
// Relacoes antigas perdem peso via e^(-t/tau), simulando esquecimento humano (curva de Ebbinghaus)
//
// Sem decay: todas as conexoes tem peso igual independente da idade
// Com decay: informacao recente pesa mais que informacao antiga
//
// Parametro tau controla a velocidade de esquecimento:
// - tau = 30 dias: memorias perdem ~63% do peso em 1 mes (esquecimento rapido)
// - tau = 90 dias: memorias perdem ~63% do peso em 3 meses (moderado)
// - tau = 365 dias: memorias perdem ~63% do peso em 1 ano (esquecimento lento)
type TemporalDecayService struct {
	client *nietzscheInfra.GraphAdapter
	tau    float64 // Constante de tempo em dias (default: 90)
}

// DecayedSignifier significante com peso ajustado pelo tempo
type DecayedSignifier struct {
	Word          string  `json:"word"`
	RawFrequency  int     `json:"raw_frequency"`
	DecayedWeight float64 `json:"decayed_weight"`
	DaysSinceFirst int    `json:"days_since_first"`
	DaysSinceLast  int    `json:"days_since_last"`
}

// DecayedRelation relacao no grafo com peso temporal
type DecayedRelation struct {
	TargetID      int64   `json:"target_id"`
	TargetName    string  `json:"target_name"`
	RelationType  string  `json:"relation_type"`
	RawWeight     float64 `json:"raw_weight"`
	DecayedWeight float64 `json:"decayed_weight"`
	AgeInDays     int     `json:"age_in_days"`
}

// NewTemporalDecayService cria servico com tau configuravel
func NewTemporalDecayService(client *nietzscheInfra.GraphAdapter, tauDays float64) *TemporalDecayService {
	if tauDays <= 0 {
		tauDays = 90 // Default: 3 meses
	}
	return &TemporalDecayService{
		client: client,
		tau:    tauDays,
	}
}

// GetDecayedSignifiers retorna significantes com peso temporal
// Em vez de ORDER BY frequency, usa ORDER BY frequency * e^(-age/tau)
func (td *TemporalDecayService) GetDecayedSignifiers(ctx context.Context, idosoID int64, topN int) ([]DecayedSignifier, error) {
	if td.client == nil {
		return []DecayedSignifier{}, nil
	}

	// Query all significantes for this patient with frequency >= 2
	nql := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId AND s.frequency >= 2 RETURN s`
	result, err := td.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar significantes com decay: %w", err)
	}

	now := time.Now()
	var items []DecayedSignifier

	for _, node := range result.Nodes {
		ds := DecayedSignifier{}

		if w, ok := node.Content["word"].(string); ok {
			ds.Word = w
		}
		if f, ok := node.Content["frequency"].(float64); ok {
			ds.RawFrequency = int(f)
		}

		// Calculate days since last and first occurrence
		if lastOcc, ok := node.Content["last_occurrence"].(float64); ok {
			lastTime := time.Unix(int64(lastOcc), 0)
			ds.DaysSinceLast = int(now.Sub(lastTime).Hours() / 24)
		}
		if firstOcc, ok := node.Content["first_occurrence"].(float64); ok {
			firstTime := time.Unix(int64(firstOcc), 0)
			ds.DaysSinceFirst = int(now.Sub(firstTime).Hours() / 24)
		}

		// Calculate decayed weight: frequency * e^(-days_since_last / tau)
		ds.DecayedWeight = float64(ds.RawFrequency) * exponentialDecay(float64(ds.DaysSinceLast), td.tau)

		items = append(items, ds)
	}

	// Sort by decayed weight DESC
	sort.Slice(items, func(i, j int) bool {
		return items[i].DecayedWeight > items[j].DecayedWeight
	})

	// Limit to topN
	if len(items) > topN {
		items = items[:topN]
	}

	return items, nil
}

// GetDecayedRelations retorna relacoes com peso temporal
func (td *TemporalDecayService) GetDecayedRelations(ctx context.Context, idosoID int64, topN int) ([]DecayedRelation, error) {
	if td.client == nil {
		return []DecayedRelation{}, nil
	}

	// Find Person node first
	nql := `MATCH (p:Person) WHERE p.id = $idosoId RETURN p`
	personResult, err := td.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil || len(personResult.Nodes) == 0 {
		return []DecayedRelation{}, err
	}

	personID := personResult.Nodes[0].ID

	// BFS from person, depth 1 to get all directly connected nodes
	connectedIDs, err := td.client.Bfs(ctx, personID, 1, "")
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar relacoes com decay: %w", err)
	}

	now := time.Now()
	var items []DecayedRelation

	for _, targetID := range connectedIDs {
		if targetID == personID {
			continue // skip self
		}

		targetNode, err := td.client.GetNode(ctx, targetID, "")
		if err != nil {
			continue
		}

		dr := DecayedRelation{
			RelationType: targetNode.NodeType,
		}

		if id, ok := targetNode.Content["id"].(float64); ok {
			dr.TargetID = int64(id)
		}
		if name, ok := targetNode.Content["name"].(string); ok {
			dr.TargetName = name
		} else if word, ok := targetNode.Content["word"].(string); ok {
			dr.TargetName = word
		} else {
			dr.TargetName = targetID
		}

		// Get raw weight
		dr.RawWeight = 1.0
		if w, ok := targetNode.Content["weight"].(float64); ok {
			dr.RawWeight = w
		}

		// Calculate age from created_at
		if createdAt := targetNode.CreatedAt; createdAt > 0 {
			created := time.Unix(createdAt, 0)
			dr.AgeInDays = int(now.Sub(created).Hours() / 24)
		}

		// Calculate decayed weight
		dr.DecayedWeight = dr.RawWeight * exponentialDecay(float64(dr.AgeInDays), td.tau)

		items = append(items, dr)
	}

	// Sort by decayed weight DESC
	sort.Slice(items, func(i, j int) bool {
		return items[i].DecayedWeight > items[j].DecayedWeight
	})

	// Limit to topN
	if len(items) > topN {
		items = items[:topN]
	}

	return items, nil
}

// PruneDecayedConnections remove conexoes com peso temporal abaixo de threshold
// Limpa o grafo de relacoes irrelevantes (muito antigas + baixa frequencia)
func (td *TemporalDecayService) PruneDecayedConnections(ctx context.Context, idosoID int64, minWeight float64) (int64, error) {
	if td.client == nil {
		return 0, nil
	}

	// Find Person node
	nql := `MATCH (p:Person) WHERE p.id = $idosoId RETURN p`
	personResult, err := td.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil || len(personResult.Nodes) == 0 {
		return 0, err
	}

	personID := personResult.Nodes[0].ID

	// BFS to find connected nodes
	connectedIDs, err := td.client.Bfs(ctx, personID, 1, "")
	if err != nil {
		return 0, fmt.Errorf("falha ao podar conexoes: %w", err)
	}

	now := time.Now()
	var pruned int64

	for _, targetID := range connectedIDs {
		if targetID == personID {
			continue
		}

		targetNode, err := td.client.GetNode(ctx, targetID, "")
		if err != nil {
			continue
		}

		rawWeight := 1.0
		if w, ok := targetNode.Content["weight"].(float64); ok {
			rawWeight = w
		}

		ageDays := 0
		if createdAt := targetNode.CreatedAt; createdAt > 0 {
			created := time.Unix(createdAt, 0)
			ageDays = int(now.Sub(created).Hours() / 24)
		}

		decayedWeight := rawWeight * exponentialDecay(float64(ageDays), td.tau)

		if decayedWeight < minWeight {
			// Delete the node (and its edges will be cleaned up)
			if err := td.client.DeleteNode(ctx, targetID, ""); err == nil {
				pruned++
			}
		}
	}

	log.Printf("[DECAY] Podadas %d conexoes com peso < %.4f para paciente %d", pruned, minWeight, idosoID)
	return pruned, nil
}

// RefreshSignifierDecay recalcula o decayed_weight de todos os significantes
// e armazena no proprio no para queries rapidas
func (td *TemporalDecayService) RefreshSignifierDecay(ctx context.Context, idosoID int64) error {
	if td.client == nil {
		return nil
	}

	// Query all significantes for this patient
	nql := `MATCH (s:Significante) WHERE s.idoso_id = $idosoId RETURN s`
	result, err := td.client.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return fmt.Errorf("falha ao atualizar decay: %w", err)
	}

	now := time.Now()
	for _, node := range result.Nodes {
		frequency := 1.0
		if f, ok := node.Content["frequency"].(float64); ok {
			frequency = f
		}

		daysSince := 0.0
		if lastOcc, ok := node.Content["last_occurrence"].(float64); ok {
			lastTime := time.Unix(int64(lastOcc), 0)
			daysSince = now.Sub(lastTime).Hours() / 24
		}

		decayedWeight := frequency * exponentialDecay(daysSince, td.tau)

		// Update the node with decayed_weight
		_, err := td.client.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType: "Significante",
			MatchKeys: map[string]interface{}{
				"idoso_id": idosoID,
				"word":     node.Content["word"],
			},
			OnMatchSet: map[string]interface{}{
				"decayed_weight":   decayedWeight,
				"decay_tau":        td.tau,
				"decay_updated_at": nietzscheInfra.NowUnix(),
			},
		})
		if err != nil {
			log.Printf("[DECAY] Erro ao atualizar decay do significante %v: %v", node.Content["word"], err)
		}
	}

	log.Printf("[DECAY] Decay atualizado para significantes do paciente %d (tau=%.0f dias)", idosoID, td.tau)
	return nil
}

// GetMemoryRelevanceScore calcula score de relevancia temporal de uma memoria
// Combina importancia base com decay temporal
func (td *TemporalDecayService) GetMemoryRelevanceScore(importance float64, createdAt time.Time) float64 {
	daysSince := time.Since(createdAt).Hours() / 24
	decayFactor := exponentialDecay(daysSince, td.tau)
	return importance * decayFactor
}

// exponentialDecay calcula e^(-t/tau)
func exponentialDecay(t, tau float64) float64 {
	if tau <= 0 {
		return 1.0
	}
	return math.Exp(-t / tau)
}
