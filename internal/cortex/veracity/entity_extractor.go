// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package veracity

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// EntityType categorias de entidades extraidas
type EntityType string

const (
	EntityPerson     EntityType = "person"
	EntityDate       EntityType = "date"
	EntityLocation   EntityType = "location"
	EntityQuantity   EntityType = "quantity"
	EntityMedication EntityType = "medication"
	EntitySymptom    EntityType = "symptom"
	EntityEvent      EntityType = "event"
)

// ExtractedEntity representa uma entidade extraida de uma afirmacao
type ExtractedEntity struct {
	Type       EntityType `json:"type"`
	Value      string     `json:"value"`
	Normalized string     `json:"normalized"` // forma normalizada para comparacao
	Context    string     `json:"context"`    // frase ou trecho onde aparece
}

// TurnEntities entidades extraidas de um turno de conversacao
type TurnEntities struct {
	UserID    int64              `json:"user_id"`
	Statement string             `json:"statement"`
	Entities  []ExtractedEntity  `json:"entities"`
	Timestamp time.Time          `json:"timestamp"`
}

// EntityContradiction contradição detectada entre entidades de turnos diferentes
type EntityContradiction struct {
	CurrentEntity  ExtractedEntity `json:"current_entity"`
	PreviousEntity ExtractedEntity `json:"previous_entity"`
	CurrentStmt    string          `json:"current_statement"`
	PreviousStmt   string          `json:"previous_statement"`
	Confidence     float64         `json:"confidence"`
	Explanation    string          `json:"explanation"`
}

// EntityExtractor extrai entidades usando Gemini (NER via LLM)
type EntityExtractor struct {
	apiKey string
	model  string

	// historico de entidades por usuario (in-memory, ultimos N turnos)
	mu      sync.RWMutex
	history map[int64][]TurnEntities // userID -> turnos recentes
	maxTurns int
}

// NewEntityExtractor cria um novo extrator de entidades
func NewEntityExtractor(apiKey, model string) *EntityExtractor {
	if model == "" {
		model = "gemini-2.0-flash" // modelo leve para NER, NAO o modelo de audio
	}
	return &EntityExtractor{
		apiKey:   apiKey,
		model:    model,
		history:  make(map[int64][]TurnEntities),
		maxTurns: 20, // manter ultimos 20 turnos por usuario
	}
}

// ExtractEntities extrai entidades de uma afirmacao usando Gemini
func (e *EntityExtractor) ExtractEntities(ctx context.Context, statement string) ([]ExtractedEntity, error) {
	if e.apiKey == "" || strings.TrimSpace(statement) == "" {
		return e.fallbackExtract(statement), nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(e.apiKey))
	if err != nil {
		log.Printf("[EntityExtractor] Gemini client failed, using fallback: %v", err)
		return e.fallbackExtract(statement), nil
	}
	defer client.Close()

	model := client.GenerativeModel(e.model)
	model.SetTemperature(0.1)

	prompt := fmt.Sprintf(`Extraia todas as entidades nomeadas do texto abaixo.
Categorias: person, date, location, quantity, medication, symptom, event.

Para cada entidade retorne:
- "type": categoria (person|date|location|quantity|medication|symptom|event)
- "value": texto exato como aparece
- "normalized": forma normalizada em minusculas, sem artigos (ex: "paracetamol 500mg", "dr silva", "2026-03-10")
- "context": trecho da frase que da contexto ao uso da entidade

Se nao houver entidades, retorne [].

Texto: "%s"

Responda APENAS o JSON array, sem markdown nem explicacoes.`, statement)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("[EntityExtractor] Gemini extraction failed, using fallback: %v", err)
		return e.fallbackExtract(statement), nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return e.fallbackExtract(statement), nil
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var extracted []ExtractedEntity
	if err := json.Unmarshal([]byte(responseText), &extracted); err != nil {
		log.Printf("[EntityExtractor] Failed to parse LLM response: %v (response: %s)", err, responseText)
		return e.fallbackExtract(statement), nil
	}

	// Normalizar tipos validos
	for i := range extracted {
		extracted[i].Type = normalizeEntityType(extracted[i].Type)
		if extracted[i].Normalized == "" {
			extracted[i].Normalized = strings.ToLower(strings.TrimSpace(extracted[i].Value))
		}
	}

	log.Printf("[EntityExtractor] Extracted %d entities from statement via Gemini", len(extracted))
	return extracted, nil
}

// RecordTurn registra as entidades de um turno no historico
func (e *EntityExtractor) RecordTurn(userID int64, statement string, entities []ExtractedEntity) {
	e.mu.Lock()
	defer e.mu.Unlock()

	turn := TurnEntities{
		UserID:    userID,
		Statement: statement,
		Entities:  entities,
		Timestamp: time.Now(),
	}

	turns := e.history[userID]
	turns = append(turns, turn)

	// Manter apenas os ultimos maxTurns
	if len(turns) > e.maxTurns {
		turns = turns[len(turns)-e.maxTurns:]
	}
	e.history[userID] = turns
}

// GetHistory retorna o historico de entidades de um usuario
func (e *EntityExtractor) GetHistory(userID int64) []TurnEntities {
	e.mu.RLock()
	defer e.mu.RUnlock()

	turns := e.history[userID]
	// Retornar copia para evitar race conditions
	result := make([]TurnEntities, len(turns))
	copy(result, turns)
	return result
}

// DetectEntityContradictions compara entidades do turno atual com historico
func (e *EntityExtractor) DetectEntityContradictions(
	ctx context.Context,
	userID int64,
	currentStatement string,
	currentEntities []ExtractedEntity,
) ([]EntityContradiction, error) {

	history := e.GetHistory(userID)
	if len(history) == 0 || len(currentEntities) == 0 {
		return nil, nil
	}

	// Fase 1: Detectar contradicoes obvias por correspondencia de entidade
	var contradictions []EntityContradiction

	for _, currentEnt := range currentEntities {
		for _, turn := range history {
			for _, prevEnt := range turn.Entities {
				if c := compareEntities(currentEnt, prevEnt, currentStatement, turn.Statement); c != nil {
					contradictions = append(contradictions, *c)
				}
			}
		}
	}

	// Fase 2: Se temos API key e ha entidades medicas/criticas, usar Gemini para analise profunda
	if e.apiKey != "" && hasCriticalEntities(currentEntities) && len(history) > 0 {
		geminiContradictions := e.detectContradictionsViaGemini(ctx, currentStatement, currentEntities, history)
		contradictions = mergeContradictions(contradictions, geminiContradictions)
	}

	return contradictions, nil
}

// detectContradictionsViaGemini usa o LLM para detectar contradicoes semanticas
func (e *EntityExtractor) detectContradictionsViaGemini(
	ctx context.Context,
	currentStatement string,
	currentEntities []ExtractedEntity,
	history []TurnEntities,
) []EntityContradiction {

	client, err := genai.NewClient(ctx, option.WithAPIKey(e.apiKey))
	if err != nil {
		log.Printf("[EntityExtractor] Gemini client failed for contradiction detection: %v", err)
		return nil
	}
	defer client.Close()

	model := client.GenerativeModel(e.model)
	model.SetTemperature(0.1)

	// Construir contexto do historico (ultimos 5 turnos relevantes)
	var historyText strings.Builder
	maxHistory := 5
	start := 0
	if len(history) > maxHistory {
		start = len(history) - maxHistory
	}
	for i := start; i < len(history); i++ {
		turn := history[i]
		historyText.WriteString(fmt.Sprintf("- [%s] \"%s\"\n",
			turn.Timestamp.Format("15:04"), turn.Statement))
	}

	prompt := fmt.Sprintf(`Analise a afirmacao atual em relacao ao historico de conversacao.
Identifique CONTRADICOES FACTUAIS (nao mudancas de opiniao).

Historico recente:
%s

Afirmacao atual: "%s"

Para cada contradicao encontrada, retorne um JSON array com:
- "current_value": o que foi dito agora
- "previous_value": o que foi dito antes que contradiz
- "confidence": 0.0 a 1.0
- "explanation": explicacao breve da contradicao

Se nao houver contradicoes, retorne [].
Responda APENAS o JSON array, sem markdown.`, historyText.String(), currentStatement)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Printf("[EntityExtractor] Gemini contradiction detection failed: %v", err)
		return nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var detected []struct {
		CurrentValue  string  `json:"current_value"`
		PreviousValue string  `json:"previous_value"`
		Confidence    float64 `json:"confidence"`
		Explanation   string  `json:"explanation"`
	}

	if err := json.Unmarshal([]byte(responseText), &detected); err != nil {
		log.Printf("[EntityExtractor] Failed to parse contradiction response: %v", err)
		return nil
	}

	var contradictions []EntityContradiction
	for _, d := range detected {
		contradictions = append(contradictions, EntityContradiction{
			CurrentEntity:  ExtractedEntity{Value: d.CurrentValue},
			PreviousEntity: ExtractedEntity{Value: d.PreviousValue},
			CurrentStmt:    currentStatement,
			Confidence:     d.Confidence,
			Explanation:    d.Explanation,
		})
	}

	return contradictions
}

// compareEntities compara duas entidades para detectar contradicao direta
func compareEntities(current, previous ExtractedEntity, currentStmt, prevStmt string) *EntityContradiction {
	// So comparar entidades do mesmo tipo
	if current.Type != previous.Type {
		return nil
	}

	// Ignorar se os valores normalizados sao identicos
	if current.Normalized == previous.Normalized {
		return nil
	}

	// Detectar contradicoes por tipo
	switch current.Type {
	case EntityMedication:
		// Mesmo medicamento com dosagens diferentes ou negacao
		if medicationOverlap(current.Normalized, previous.Normalized) {
			return &EntityContradiction{
				CurrentEntity:  current,
				PreviousEntity: previous,
				CurrentStmt:    currentStmt,
				PreviousStmt:   prevStmt,
				Confidence:     0.80,
				Explanation:    fmt.Sprintf("Informacao sobre medicamento divergente: '%s' vs '%s'", current.Value, previous.Value),
			}
		}

	case EntityQuantity:
		// Mesma grandeza com valores diferentes
		if quantityContext(current.Context, previous.Context) {
			return &EntityContradiction{
				CurrentEntity:  current,
				PreviousEntity: previous,
				CurrentStmt:    currentStmt,
				PreviousStmt:   prevStmt,
				Confidence:     0.75,
				Explanation:    fmt.Sprintf("Quantidade divergente: '%s' vs '%s'", current.Value, previous.Value),
			}
		}

	case EntityDate:
		// Mesmo evento com datas diferentes
		if dateContextOverlap(current.Context, previous.Context) {
			return &EntityContradiction{
				CurrentEntity:  current,
				PreviousEntity: previous,
				CurrentStmt:    currentStmt,
				PreviousStmt:   prevStmt,
				Confidence:     0.70,
				Explanation:    fmt.Sprintf("Data divergente para evento similar: '%s' vs '%s'", current.Value, previous.Value),
			}
		}
	}

	return nil
}

// medicationOverlap verifica se dois textos de medicamento se referem ao mesmo remedio
func medicationOverlap(a, b string) bool {
	// Extrair nome base do medicamento (primeira palavra significativa)
	aBase := extractMedBase(a)
	bBase := extractMedBase(b)

	if aBase == "" || bBase == "" {
		return false
	}

	return strings.Contains(aBase, bBase) || strings.Contains(bBase, aBase)
}

// extractMedBase extrai nome base de um medicamento
func extractMedBase(med string) string {
	med = strings.ToLower(strings.TrimSpace(med))
	// Remover dosagens e formas
	for _, suffix := range []string{"mg", "ml", "comprimido", "gotas", "capsulas", "capsula"} {
		parts := strings.Split(med, suffix)
		if len(parts) > 0 {
			med = strings.TrimSpace(parts[0])
		}
	}
	// Pegar primeira palavra (nome do principio ativo)
	parts := strings.Fields(med)
	if len(parts) > 0 {
		return parts[0]
	}
	return med
}

// quantityContext verifica se dois contextos se referem a mesma grandeza
func quantityContext(ctxA, ctxB string) bool {
	ctxA = strings.ToLower(ctxA)
	ctxB = strings.ToLower(ctxB)

	// Palavras-chave que indicam mesma grandeza
	keywords := []string{
		"pressao", "peso", "temperatura", "glicose", "glicemia",
		"dor", "vezes", "horas", "dias", "dose",
	}

	for _, kw := range keywords {
		if strings.Contains(ctxA, kw) && strings.Contains(ctxB, kw) {
			return true
		}
	}
	return false
}

// dateContextOverlap verifica se dois contextos de data se referem ao mesmo evento
func dateContextOverlap(ctxA, ctxB string) bool {
	ctxA = strings.ToLower(ctxA)
	ctxB = strings.ToLower(ctxB)

	eventKeywords := []string{
		"consulta", "exame", "internacao", "cirurgia", "acidente",
		"diagnostico", "nascimento", "inicio", "tratamento",
	}

	for _, kw := range eventKeywords {
		if strings.Contains(ctxA, kw) && strings.Contains(ctxB, kw) {
			return true
		}
	}
	return false
}

// hasCriticalEntities verifica se ha entidades criticas (medicas) que justificam analise LLM
func hasCriticalEntities(entities []ExtractedEntity) bool {
	for _, e := range entities {
		switch e.Type {
		case EntityMedication, EntitySymptom, EntityQuantity:
			return true
		}
	}
	return false
}

// mergeContradictions junta contradicoes sem duplicatas
func mergeContradictions(a, b []EntityContradiction) []EntityContradiction {
	if len(b) == 0 {
		return a
	}

	result := make([]EntityContradiction, len(a))
	copy(result, a)

	for _, bc := range b {
		isDuplicate := false
		for _, ac := range a {
			if strings.Contains(ac.Explanation, bc.Explanation) ||
				(ac.CurrentEntity.Value == bc.CurrentEntity.Value &&
					ac.PreviousEntity.Value == bc.PreviousEntity.Value) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			result = append(result, bc)
		}
	}
	return result
}

// normalizeEntityType normaliza tipo de entidade
func normalizeEntityType(t EntityType) EntityType {
	switch t {
	case EntityPerson, EntityDate, EntityLocation, EntityQuantity,
		EntityMedication, EntitySymptom, EntityEvent:
		return t
	default:
		return EntityEvent // fallback
	}
}

// fallbackExtract extracao baseada em regras simples (sem LLM)
func (e *EntityExtractor) fallbackExtract(statement string) []ExtractedEntity {
	lower := strings.ToLower(statement)
	var entities []ExtractedEntity

	// Medicamentos comuns
	medications := []string{
		"paracetamol", "dipirona", "ibuprofeno", "amoxicilina",
		"omeprazol", "losartana", "metformina", "atenolol",
		"captopril", "insulina", "remedio", "medicamento",
		"comprimido", "antibiotico", "analgesico", "anti-inflamatorio",
	}
	for _, med := range medications {
		if strings.Contains(lower, med) {
			entities = append(entities, ExtractedEntity{
				Type:       EntityMedication,
				Value:      med,
				Normalized: med,
				Context:    statement,
			})
		}
	}

	// Sintomas comuns
	symptoms := []string{
		"dor", "febre", "tosse", "nausea", "vomito", "tontura",
		"cansaco", "falta de ar", "diarreia", "coceira",
		"inchaco", "sangramento", "insonia", "ansiedade",
	}
	for _, sym := range symptoms {
		if strings.Contains(lower, sym) {
			entities = append(entities, ExtractedEntity{
				Type:       EntitySymptom,
				Value:      sym,
				Normalized: sym,
				Context:    statement,
			})
		}
	}

	// Marcadores temporais
	dateMarkers := []string{
		"ontem", "hoje", "semana passada", "mes passado",
		"ano passado", "segunda", "terca", "quarta",
		"quinta", "sexta", "sabado", "domingo",
	}
	for _, dm := range dateMarkers {
		if strings.Contains(lower, dm) {
			entities = append(entities, ExtractedEntity{
				Type:       EntityDate,
				Value:      dm,
				Normalized: dm,
				Context:    statement,
			})
		}
	}

	return entities
}
