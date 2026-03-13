// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package veracity

import (
	"context"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/lacan"
	"eva/internal/cortex/transnar"
	"fmt"
	"log"
	"strings"
	"time"
)

// NegationPattern represents a negation keyword and its semantic weight
type NegationPattern struct {
	Pattern string
	Weight  float64 // higher = stronger negation
}

// LieDetector motor de detecção de inconsistências
type LieDetector struct {
	graph        *nietzscheInfra.GraphAdapter
	lacanService *lacan.SignifierService
	transnar     *transnar.Engine
	ner          *EntityExtractor
}

// NewLieDetector cria um novo detector
func NewLieDetector(
	graphAdapter *nietzscheInfra.GraphAdapter,
	lacanService *lacan.SignifierService,
	transnarEngine *transnar.Engine,
	apiKey string,
) *LieDetector {
	return &LieDetector{
		graph:        graphAdapter,
		lacanService: lacanService,
		transnar:     transnarEngine,
		ner:          NewEntityExtractor(apiKey, ""),
	}
}

// Detect detecta todas as inconsistências em uma afirmação
func (d *LieDetector) Detect(
	ctx context.Context,
	userID int64,
	statement string,
) []Inconsistency {

	inconsistencies := []Inconsistency{}

	log.Printf("[LieDetector] Analisando: '%s'", statement)

	// 0. NER: Extrair entidades via Gemini e comparar com historico
	entities, err := d.ner.ExtractEntities(ctx, statement)
	if err != nil {
		log.Printf("[LieDetector] NER extraction error (non-fatal): %v", err)
	} else if len(entities) > 0 {
		log.Printf("[LieDetector] NER: %d entities extracted", len(entities))

		// Detectar contradicoes entre entidades do turno atual e historico
		contradictions, err := d.ner.DetectEntityContradictions(ctx, userID, statement, entities)
		if err != nil {
			log.Printf("[LieDetector] Entity contradiction detection error: %v", err)
		}
		for _, c := range contradictions {
			inconsistencies = append(inconsistencies, Inconsistency{
				Type:       DirectContradiction,
				Confidence: c.Confidence,
				Statement:  statement,
				GraphEvidence: []Evidence{
					{
						Fact:      fmt.Sprintf("Anterior: '%s' | Atual: '%s'", c.PreviousEntity.Value, c.CurrentEntity.Value),
						Timestamp: time.Now(),
						Source:    "NER Entity Comparison",
						Metadata: map[string]interface{}{
							"entity_type":    c.CurrentEntity.Type,
							"previous_stmt":  c.PreviousStmt,
							"current_stmt":   c.CurrentStmt,
						},
					},
				},
				Reasoning: c.Explanation,
				Severity:  severityFromConfidence(c.Confidence),
				Timestamp: time.Now(),
			})
			log.Printf("[LieDetector] NER contradiction: %s (%.0f%%)",
				c.Explanation, c.Confidence*100)
		}

		// Registrar turno no historico APOS verificacao
		d.ner.RecordTurn(userID, statement, entities)
	}

	// 1. Verificar contradições diretas (com NER integrado)
	if contradiction := d.checkDirectContradiction(ctx, userID, statement); contradiction != nil {
		inconsistencies = append(inconsistencies, *contradiction)
		log.Printf("[LieDetector] Contradicao direta detectada: %.0f%% confianca",
			contradiction.Confidence*100)
	}

	// 2. Verificar inconsistências temporais
	if temporal := d.checkTemporalInconsistency(ctx, userID, statement); temporal != nil {
		inconsistencies = append(inconsistencies, *temporal)
		log.Printf("[LieDetector] Inconsistencia temporal: %.0f%% confianca",
			temporal.Confidence*100)
	}

	// 3. Verificar inconsistências emocionais
	if emotional := d.checkEmotionalInconsistency(ctx, userID, statement); emotional != nil {
		inconsistencies = append(inconsistencies, *emotional)
		log.Printf("[LieDetector] Inconsistencia emocional: %.0f%% confianca",
			emotional.Confidence*100)
	}

	// 4. Verificar gaps narrativos
	if gap := d.checkNarrativeGap(ctx, userID, statement); gap != nil {
		inconsistencies = append(inconsistencies, *gap)
		log.Printf("[LieDetector] Gap narrativo: %.0f%% confianca",
			gap.Confidence*100)
	}

	// 5. Verificar mudanças comportamentais
	if behavioral := d.checkBehavioralChange(ctx, userID, statement); behavioral != nil {
		inconsistencies = append(inconsistencies, *behavioral)
		log.Printf("[LieDetector] Mudanca comportamental: %.0f%% confianca",
			behavioral.Confidence*100)
	}

	if len(inconsistencies) == 0 {
		log.Printf("[LieDetector] Nenhuma inconsistencia detectada")
	}

	return inconsistencies
}

// severityFromConfidence mapeia confianca para gravidade
func severityFromConfidence(confidence float64) Severity {
	switch {
	case confidence >= 0.9:
		return SeverityCritical
	case confidence >= 0.8:
		return SeverityHigh
	case confidence >= 0.6:
		return SeverityMedium
	default:
		return SeverityLow
	}
}

// GetExtractedEntities retorna as entidades extraidas do ultimo turno (para uso externo)
func (d *LieDetector) GetExtractedEntities(userID int64) []TurnEntities {
	return d.ner.GetHistory(userID)
}

// checkDirectContradiction verifica contradições diretas usando NER + grafo
func (d *LieDetector) checkDirectContradiction(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar padrões de negação absoluta (com pesos)
	negationPatterns := []NegationPattern{
		{"nunca", 1.0},
		{"jamais", 1.0},
		{"não tomei", 0.9},
		{"não fiz", 0.9},
		{"não senti", 0.8},
		{"não tenho", 0.8},
		{"não tive", 0.8},
		{"nao tomei", 0.9},
		{"nao fiz", 0.9},
		{"nenhum", 0.7},
		{"nada", 0.7},
	}

	maxWeight := 0.0
	for _, np := range negationPatterns {
		if strings.Contains(strings.ToLower(statement), np.Pattern) {
			if np.Weight > maxWeight {
				maxWeight = np.Weight
			}
		}
	}

	if maxWeight == 0 {
		return nil // Sem negacao absoluta
	}

	// NER: Extrair entidades da afirmacao atual para buscar no grafo
	entities, err := d.ner.ExtractEntities(ctx, statement)
	if err != nil {
		log.Printf("[LieDetector] NER error in contradiction check: %v", err)
	}

	// Buscar entidades extraidas no grafo
	for _, ent := range entities {
		searchTerm := ent.Normalized
		if searchTerm == "" {
			searchTerm = strings.ToLower(ent.Value)
		}

		evidence := d.queryGraphForEntity(ctx, userID, searchTerm)
		if len(evidence) > 0 {
			confidence := 0.85 * maxWeight
			return &Inconsistency{
				Type:          DirectContradiction,
				Confidence:    confidence,
				Statement:     statement,
				GraphEvidence: evidence,
				Reasoning: fmt.Sprintf(
					"Paciente nega '%s' (%s), mas ha registro no grafo",
					ent.Value, ent.Type,
				),
				Severity:  severityFromConfidence(confidence),
				Timestamp: time.Now(),
			}
		}
	}

	// Fallback: buscar palavras-chave comuns se NER nao retornou entidades
	if len(entities) == 0 {
		keywords := []string{"remedio", "medicamento", "dor", "medico", "consulta"}
		for _, keyword := range keywords {
			if strings.Contains(strings.ToLower(statement), keyword) {
				evidence := d.queryGraphForEntity(ctx, userID, keyword)
				if len(evidence) > 0 {
					return &Inconsistency{
						Type:          DirectContradiction,
						Confidence:    0.75,
						Statement:     statement,
						GraphEvidence: evidence,
						Reasoning:     "Afirmacao contradiz registro no grafo (fallback keyword match)",
						Severity:      SeverityMedium,
						Timestamp:     time.Now(),
					}
				}
			}
		}
	}

	return nil
}

// checkTemporalInconsistency verifica inconsistências temporais
func (d *LieDetector) checkTemporalInconsistency(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar marcadores temporais
	temporalMarkers := map[string]time.Duration{
		"ontem":          -24 * time.Hour,
		"hoje":           0,
		"semana passada": -7 * 24 * time.Hour,
		"mês passado":    -30 * 24 * time.Hour,
	}

	for marker, expectedDuration := range temporalMarkers {
		if strings.Contains(strings.ToLower(statement), marker) {
			// Buscar eventos recentes no grafo
			evidence := d.queryRecentEvents(ctx, userID, 30) // Últimos 30 dias

			if len(evidence) > 0 {
				// Verificar se a diferença temporal é significativa
				expectedTime := time.Now().Add(expectedDuration)
				actualTime := evidence[0].Timestamp

				diff := expectedTime.Sub(actualTime).Hours() / 24 // Dias

				if diff > 2 || diff < -2 { // Diferença > 2 dias
					return &Inconsistency{
						Type:          TemporalInconsistency,
						Confidence:    0.70, // Média - memória pode ser imprecisa
						Statement:     statement,
						GraphEvidence: evidence,
						Reasoning:     "Diferença temporal significativa detectada",
						Severity:      SeverityMedium,
						Timestamp:     time.Now(),
					}
				}
			}
		}
	}

	return nil
}

// checkEmotionalInconsistency verifica negação de emoções
func (d *LieDetector) checkEmotionalInconsistency(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar negação de emoções
	emotionNegations := map[string]string{
		"não tenho medo":       "medo",
		"não estou triste":     "tristeza",
		"não me sinto só":      "solidão",
		"não estou ansioso":    "ansiedade",
		"não estou preocupado": "preocupação",
	}

	for negation, emotion := range emotionNegations {
		if strings.Contains(strings.ToLower(statement), negation) {
			// Buscar significantes emocionais no histórico
			signifiers, err := d.lacanService.GetKeySignifiers(ctx, userID, 20)
			if err != nil {
				log.Printf("[LieDetector] Erro ao buscar significantes: %v", err)
				return nil
			}

			// Verificar se a emoção negada está no histórico
			for _, sig := range signifiers {
				if strings.Contains(strings.ToLower(sig.Word), emotion) && sig.Frequency >= 3 {
					// Emoção negada mas presente no histórico!
					return &Inconsistency{
						Type:       EmotionalInconsistency,
						Confidence: 0.80,
						Statement:  statement,
						GraphEvidence: []Evidence{
							{
								Fact:      sig.Word + " mencionado " + fmt.Sprintf("%d", sig.Frequency) + "x",
								Timestamp: sig.LastOccurrence,
								Source:    "Lacan Signifier Tracking",
							},
						},
						Reasoning: "Emoção negada mas presente no histórico de significantes",
						Severity:  SeverityMedium,
						Timestamp: time.Now(),
					}
				}
			}
		}
	}

	return nil
}

// checkNarrativeGap verifica omissões importantes
func (d *LieDetector) checkNarrativeGap(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar menções a eventos médicos específicos
	medicalKeywords := []string{"consulta", "médico", "exame", "hospital", "internação", "diagnóstico"}

	hasMedicalReference := false
	for _, keyword := range medicalKeywords {
		if strings.Contains(strings.ToLower(statement), keyword) {
			hasMedicalReference = true
			break
		}
	}

	if !hasMedicalReference {
		return nil // Não é sobre evento médico
	}

	// Buscar eventos médicos recentes no grafo (últimos 30 dias)
	evidence := d.queryRecentEvents(ctx, userID, 30)

	if len(evidence) == 0 {
		return nil // Sem eventos recentes para comparar
	}

	// Verificar se há eventos médicos no grafo que conflitam com a afirmação
	// Padrões de negação ou omissão: paciente nega consulta mas há registro
	negationPatterns := []string{
		"não fui", "não tive", "não fiz", "nunca fui",
		"não consultei", "não precisei", "sem consulta",
	}

	hasNegation := false
	for _, pattern := range negationPatterns {
		if strings.Contains(strings.ToLower(statement), pattern) {
			hasNegation = true
			break
		}
	}

	if !hasNegation {
		return nil // Sem negação, não há gap narrativo detectável
	}

	// Filtrar evidências que sejam de eventos médicos
	medicalEvidence := []Evidence{}
	for _, ev := range evidence {
		factLower := strings.ToLower(ev.Fact)
		for _, keyword := range medicalKeywords {
			if strings.Contains(factLower, keyword) {
				medicalEvidence = append(medicalEvidence, ev)
				break
			}
		}
	}

	if len(medicalEvidence) > 0 {
		return &Inconsistency{
			Type:          NarrativeGap,
			Confidence:    0.75,
			Statement:     statement,
			GraphEvidence: medicalEvidence,
			Reasoning:     fmt.Sprintf("Paciente nega evento médico, mas há %d registro(s) nos últimos 30 dias", len(medicalEvidence)),
			Severity:      SeverityMedium,
			Timestamp:     time.Now(),
		}
	}

	return nil
}

// checkBehavioralChange verifica mudanças de padrão
func (d *LieDetector) checkBehavioralChange(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar afirmações sobre comportamentos habituais
	behaviorKeywords := []string{
		"tomei", "fiz", "sempre", "todo dia", "toda semana",
		"exercício", "caminhada", "medicamento", "remédio",
	}

	hasBehaviorClaim := false
	for _, keyword := range behaviorKeywords {
		if strings.Contains(strings.ToLower(statement), keyword) {
			hasBehaviorClaim = true
			break
		}
	}

	if !hasBehaviorClaim {
		return nil
	}

	// Buscar eventos recentes (últimos 7 dias) e histórico (últimos 30 dias)
	recentEvents := d.queryRecentEvents(ctx, userID, 7)
	historicalEvents := d.queryRecentEvents(ctx, userID, 30)

	if len(historicalEvents) == 0 {
		return nil // Sem histórico para comparar
	}

	// Calcular frequência: eventos nos últimos 7 dias vs média semanal dos últimos 30 dias
	recentCount := float64(len(recentEvents))
	// Normalizar histórico para frequência semanal (30 dias ~ 4.3 semanas)
	historicalWeeklyAvg := float64(len(historicalEvents)) / 4.3

	// Evitar divisão por zero
	if historicalWeeklyAvg < 0.5 {
		return nil // Histórico insuficiente
	}

	// Calcular diferença percentual
	var diffPercent float64
	if historicalWeeklyAvg > 0 {
		diffPercent = ((recentCount - historicalWeeklyAvg) / historicalWeeklyAvg) * 100
	}

	// Se diferença > 50% (aumento ou diminuição), sinalizar mudança comportamental
	if diffPercent > 50 || diffPercent < -50 {
		direction := "aumento"
		if diffPercent < 0 {
			direction = "diminuição"
		}

		return &Inconsistency{
			Type:          BehavioralChange,
			Confidence:    0.70,
			Statement:     statement,
			GraphEvidence: recentEvents,
			Reasoning: fmt.Sprintf(
				"Mudança comportamental significativa detectada: %s de %.0f%% na frequência de eventos (%.1f/semana recente vs %.1f/semana histórico)",
				direction, diffPercent, recentCount, historicalWeeklyAvg,
			),
			Severity:  SeverityMedium,
			Timestamp: time.Now(),
		}
	}

	return nil
}

// queryGraphForEntity busca entidade no grafo
func (d *LieDetector) queryGraphForEntity(
	ctx context.Context,
	userID int64,
	entity string,
) []Evidence {
	if d.graph == nil {
		return []Evidence{}
	}

	nql := `MATCH (p:Person {id: $userId})-[r]->(n) WHERE toLower(n.nome) CONTAINS toLower($entity) RETURN n LIMIT 5`
	result, err := d.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"userId": fmt.Sprintf("%d", userID),
		"entity": entity,
	}, "")
	if err != nil {
		log.Printf("[LieDetector] Erro ao buscar entidade: %v", err)
		return []Evidence{}
	}

	evidence := []Evidence{}
	for _, node := range result.Nodes {
		nome := fmt.Sprintf("%v", node.Content["nome"])
		evidence = append(evidence, Evidence{
			Fact:      "RELATED: " + nome,
			Timestamp: time.Now(),
			Source:    "NietzscheDB Graph",
		})
	}
	return evidence
}

// queryRecentEvents busca eventos recentes
func (d *LieDetector) queryRecentEvents(
	ctx context.Context,
	userID int64,
	days int,
) []Evidence {
	if d.graph == nil {
		return []Evidence{}
	}

	cutoff := nietzscheInfra.DaysAgoUnix(days)
	nql := `MATCH (p:Person {id: $userId})-[r]->(n) WHERE n.timestamp > $cutoff RETURN n LIMIT 10`
	result, err := d.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"userId": fmt.Sprintf("%d", userID),
		"cutoff": cutoff,
	}, "")
	if err != nil {
		return []Evidence{}
	}

	evidence := []Evidence{}
	for _, node := range result.Nodes {
		nome := fmt.Sprintf("%v", node.Content["nome"])
		evidence = append(evidence, Evidence{
			Fact:      "EVENT: " + nome,
			Timestamp: time.Now(),
			Source:    "NietzscheDB Recent Events",
		})
	}
	return evidence
}
