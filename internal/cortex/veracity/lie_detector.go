// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package veracity

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/cortex/lacan"
	"eva-mind/internal/cortex/transnar"
	"log"
	"strings"
	"time"
)

// LieDetector motor de detecção de inconsistências
type LieDetector struct {
	neo4j        *graph.Neo4jClient
	lacanService *lacan.SignifierService
	transnar     *transnar.Engine
}

// NewLieDetector cria um novo detector
func NewLieDetector(
	neo4j *graph.Neo4jClient,
	lacanService *lacan.SignifierService,
	transnarEngine *transnar.Engine,
) *LieDetector {
	return &LieDetector{
		neo4j:        neo4j,
		lacanService: lacanService,
		transnar:     transnarEngine,
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

	// 1. Verificar contradições diretas
	if contradiction := d.checkDirectContradiction(ctx, userID, statement); contradiction != nil {
		inconsistencies = append(inconsistencies, *contradiction)
		log.Printf("[LieDetector] ⚠️ Contradição direta detectada: %.0f%% confiança",
			contradiction.Confidence*100)
	}

	// 2. Verificar inconsistências temporais
	if temporal := d.checkTemporalInconsistency(ctx, userID, statement); temporal != nil {
		inconsistencies = append(inconsistencies, *temporal)
		log.Printf("[LieDetector] ⏰ Inconsistência temporal: %.0f%% confiança",
			temporal.Confidence*100)
	}

	// 3. Verificar inconsistências emocionais
	if emotional := d.checkEmotionalInconsistency(ctx, userID, statement); emotional != nil {
		inconsistencies = append(inconsistencies, *emotional)
		log.Printf("[LieDetector] 😔 Inconsistência emocional: %.0f%% confiança",
			emotional.Confidence*100)
	}

	// 4. Verificar gaps narrativos
	if gap := d.checkNarrativeGap(ctx, userID, statement); gap != nil {
		inconsistencies = append(inconsistencies, *gap)
		log.Printf("[LieDetector] 📖 Gap narrativo: %.0f%% confiança",
			gap.Confidence*100)
	}

	// 5. Verificar mudanças comportamentais
	if behavioral := d.checkBehavioralChange(ctx, userID, statement); behavioral != nil {
		inconsistencies = append(inconsistencies, *behavioral)
		log.Printf("[LieDetector] 🔄 Mudança comportamental: %.0f%% confiança",
			behavioral.Confidence*100)
	}

	if len(inconsistencies) == 0 {
		log.Printf("[LieDetector] ✅ Nenhuma inconsistência detectada")
	}

	return inconsistencies
}

// checkDirectContradiction verifica contradições diretas
func (d *LieDetector) checkDirectContradiction(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar padrões de negação absoluta
	negationPatterns := []string{
		"nunca", "jamais", "não tomei", "não fiz",
		"não senti", "não tenho", "não tive",
	}

	hasNegation := false
	for _, pattern := range negationPatterns {
		if strings.Contains(strings.ToLower(statement), pattern) {
			hasNegation = true
			break
		}
	}

	if !hasNegation {
		return nil // Sem negação absoluta
	}

	// Extrair possíveis entidades mencionadas
	// TODO: Implementar NER (Named Entity Recognition)
	// Por ora, buscar palavras-chave comuns

	keywords := []string{"remédio", "medicamento", "dor", "médico", "consulta"}

	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(statement), keyword) {
			// Buscar no grafo se há registro dessa entidade
			evidence := d.queryGraphForEntity(ctx, userID, keyword)

			if len(evidence) > 0 {
				// Contradição encontrada!
				return &Inconsistency{
					Type:          DirectContradiction,
					Confidence:    0.85, // Alta confiança
					Statement:     statement,
					GraphEvidence: evidence,
					Reasoning:     "Afirmação contradiz registro no grafo",
					Severity:      SeverityHigh,
					Timestamp:     time.Now(),
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
								Fact:      sig.Word + " mencionado " + string(rune(sig.Frequency)) + "x",
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

	// Detectar perguntas sobre eventos específicos
	if !strings.Contains(strings.ToLower(statement), "consulta") &&
		!strings.Contains(strings.ToLower(statement), "médico") {
		return nil // Não é sobre consulta médica
	}

	// Buscar consultas recentes com diagnósticos graves
	// TODO: Implementar query específica
	// Por ora, retornar nil

	return nil
}

// checkBehavioralChange verifica mudanças de padrão
func (d *LieDetector) checkBehavioralChange(
	ctx context.Context,
	userID int64,
	statement string,
) *Inconsistency {

	// Detectar afirmações sobre comportamentos
	if !strings.Contains(strings.ToLower(statement), "tomei") &&
		!strings.Contains(strings.ToLower(statement), "fiz") {
		return nil
	}

	// TODO: Implementar análise de padrões comportamentais
	// Requer histórico de horários e frequências

	return nil
}

// queryGraphForEntity busca entidade no grafo
func (d *LieDetector) queryGraphForEntity(
	ctx context.Context,
	userID int64,
	entity string,
) []Evidence {

	// Query genérica para buscar menções da entidade
	query := `
		MATCH (p:Person {id: $userId})-[r]->(n)
		WHERE toLower(n.nome) CONTAINS toLower($entity)
		  OR toLower(type(r)) CONTAINS toLower($entity)
		RETURN type(r) as relacao, n.nome as entidade, r.timestamp as timestamp
		ORDER BY r.timestamp DESC
		LIMIT 5
	`

	params := map[string]interface{}{
		"userId": userID,
		"entity": entity,
	}

	records, err := d.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		log.Printf("[LieDetector] Erro ao buscar entidade: %v", err)
		return []Evidence{}
	}

	evidence := []Evidence{}
	for _, record := range records {
		relacao, _ := record.Get("relacao")
		entidade, _ := record.Get("entidade")
		timestamp, _ := record.Get("timestamp")

		evidence = append(evidence, Evidence{
			Fact:      relacao.(string) + " " + entidade.(string),
			Timestamp: timestamp.(time.Time),
			Source:    "Neo4j Graph",
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

	query := `
		MATCH (p:Person {id: $userId})-[r]->(n)
		WHERE r.timestamp > datetime() - duration({days: $days})
		RETURN type(r) as tipo, n.nome as nome, r.timestamp as timestamp
		ORDER BY r.timestamp DESC
		LIMIT 10
	`

	params := map[string]interface{}{
		"userId": userID,
		"days":   days,
	}

	records, err := d.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return []Evidence{}
	}

	evidence := []Evidence{}
	for _, record := range records {
		tipo, _ := record.Get("tipo")
		nome, _ := record.Get("nome")
		timestamp, _ := record.Get("timestamp")

		evidence = append(evidence, Evidence{
			Fact:      tipo.(string) + ": " + nome.(string),
			Timestamp: timestamp.(time.Time),
			Source:    "Neo4j Recent Events",
		})
	}

	return evidence
}
