// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package lacan

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"eva/internal/brainstem/infrastructure/graph"
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
	client *graph.Neo4jClient
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
func NewTemporalDecayService(client *graph.Neo4jClient, tauDays float64) *TemporalDecayService {
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

	// Cypher com decay temporal: peso = frequency * e^(-days_since_last / tau)
	query := `
		MATCH (s:Significante {idoso_id: $idosoId})
		WHERE s.frequency >= 2
		WITH s,
		     s.frequency AS freq,
		     duration.inDays(s.last_occurrence, datetime()).days AS days_since_last,
		     duration.inDays(s.first_occurrence, datetime()).days AS days_since_first
		WITH s, freq, days_since_last, days_since_first,
		     freq * exp(-1.0 * days_since_last / $tau) AS decayed_weight
		RETURN s.word AS word,
		       freq AS raw_frequency,
		       decayed_weight,
		       days_since_first,
		       days_since_last
		ORDER BY decayed_weight DESC
		LIMIT $limit
	`

	records, err := td.client.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
		"tau":     td.tau,
		"limit":   topN,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar significantes com decay: %w", err)
	}

	var result []DecayedSignifier
	for _, record := range records {
		word, _ := record.Get("word")
		rawFreq, _ := record.Get("raw_frequency")
		decayed, _ := record.Get("decayed_weight")
		daysSinceFirst, _ := record.Get("days_since_first")
		daysSinceLast, _ := record.Get("days_since_last")

		ds := DecayedSignifier{
			Word:         word.(string),
			RawFrequency: int(rawFreq.(int64)),
		}

		if dw, ok := decayed.(float64); ok {
			ds.DecayedWeight = dw
		}
		if dsf, ok := daysSinceFirst.(int64); ok {
			ds.DaysSinceFirst = int(dsf)
		}
		if dsl, ok := daysSinceLast.(int64); ok {
			ds.DaysSinceLast = int(dsl)
		}

		result = append(result, ds)
	}

	return result, nil
}

// GetDecayedRelations retorna relacoes com peso temporal
func (td *TemporalDecayService) GetDecayedRelations(ctx context.Context, idosoID int64, topN int) ([]DecayedRelation, error) {
	if td.client == nil {
		return []DecayedRelation{}, nil
	}

	query := `
		MATCH (p:Person {id: $idosoId})-[r]->(target)
		WHERE r.created_at IS NOT NULL
		WITH target, r, type(r) AS rel_type,
		     COALESCE(r.weight, 1.0) AS raw_weight,
		     duration.inDays(r.created_at, datetime()).days AS age_days
		WITH target, rel_type, raw_weight, age_days,
		     raw_weight * exp(-1.0 * age_days / $tau) AS decayed_weight
		RETURN target.id AS target_id,
		       COALESCE(target.name, target.word, toString(target.id)) AS target_name,
		       rel_type,
		       raw_weight,
		       decayed_weight,
		       age_days
		ORDER BY decayed_weight DESC
		LIMIT $limit
	`

	records, err := td.client.ExecuteRead(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
		"tau":     td.tau,
		"limit":   topN,
	})
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar relacoes com decay: %w", err)
	}

	var result []DecayedRelation
	for _, record := range records {
		targetID, _ := record.Get("target_id")
		targetName, _ := record.Get("target_name")
		relType, _ := record.Get("rel_type")
		rawWeight, _ := record.Get("raw_weight")
		decayedWeight, _ := record.Get("decayed_weight")
		ageDays, _ := record.Get("age_days")

		dr := DecayedRelation{
			RelationType: relType.(string),
		}

		if id, ok := targetID.(int64); ok {
			dr.TargetID = id
		}
		if name, ok := targetName.(string); ok {
			dr.TargetName = name
		}
		if rw, ok := rawWeight.(float64); ok {
			dr.RawWeight = rw
		}
		if dw, ok := decayedWeight.(float64); ok {
			dr.DecayedWeight = dw
		}
		if ad, ok := ageDays.(int64); ok {
			dr.AgeInDays = int(ad)
		}

		result = append(result, dr)
	}

	return result, nil
}

// PruneDecayedConnections remove conexoes com peso temporal abaixo de threshold
// Limpa o grafo de relacoes irrelevantes (muito antigas + baixa frequencia)
func (td *TemporalDecayService) PruneDecayedConnections(ctx context.Context, idosoID int64, minWeight float64) (int64, error) {
	if td.client == nil {
		return 0, nil
	}

	query := `
		MATCH (p:Person {id: $idosoId})-[r]->(target)
		WHERE r.created_at IS NOT NULL
		  AND COALESCE(r.weight, 1.0) * exp(-1.0 * duration.inDays(r.created_at, datetime()).days / $tau) < $minWeight
		WITH r, count(*) AS total
		DELETE r
		RETURN total
	`

	result, err := td.client.ExecuteWrite(ctx, query, map[string]interface{}{
		"idosoId":   idosoID,
		"tau":       td.tau,
		"minWeight": minWeight,
	})
	if err != nil {
		return 0, fmt.Errorf("falha ao podar conexoes: %w", err)
	}

	log.Printf("[DECAY] Podadas conexoes com peso < %.4f para paciente %d", minWeight, idosoID)
	_ = result
	return 0, nil
}

// RefreshSignifierDecay recalcula o decayed_weight de todos os significantes
// e armazena no proprio no para queries rapidas
func (td *TemporalDecayService) RefreshSignifierDecay(ctx context.Context, idosoID int64) error {
	if td.client == nil {
		return nil
	}

	query := `
		MATCH (s:Significante {idoso_id: $idosoId})
		WHERE s.last_occurrence IS NOT NULL
		WITH s,
		     duration.inDays(s.last_occurrence, datetime()).days AS days_since
		SET s.decayed_weight = s.frequency * exp(-1.0 * days_since / $tau),
		    s.decay_tau = $tau,
		    s.decay_updated_at = datetime()
	`

	_, err := td.client.ExecuteWrite(ctx, query, map[string]interface{}{
		"idosoId": idosoID,
		"tau":     td.tau,
	})
	if err != nil {
		return fmt.Errorf("falha ao atualizar decay: %w", err)
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
