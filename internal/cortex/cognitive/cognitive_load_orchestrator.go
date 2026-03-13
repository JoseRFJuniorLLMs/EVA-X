// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package cognitive

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// CognitiveLoadOrchestrator gerencia carga cognitiva do paciente
type CognitiveLoadOrchestrator struct {
	db    *database.DB
	cache *nietzscheInfra.CacheStore
	ctx   context.Context
}

// InteractionLoad representa carga de uma interacao
type InteractionLoad struct {
	PatientID           int64
	InteractionType     string // therapeutic, entertainment, clinical, educational, emergency
	EmotionalIntensity  float64 // 0-1
	CognitiveComplexity float64 // 0-1
	DurationSeconds     int
	FatigueIndicators   map[string]interface{}
	TopicsDiscussed     []string
	LacanianSignifiers  []string
	SessionID           string
	VoiceEnergyScore    *float64
	SpeechRateWPM       *int
	PauseFrequency      *float64
}

// CognitiveLoadState representa estado atual de carga
type CognitiveLoadState struct {
	PatientID             int64
	CurrentLoadScore      float64 // 0-1
	Load24h               float64
	Load7d                float64
	InteractionsCount24h  int
	TherapeuticCount24h   int
	HighIntensityCount24h int
	LastInteractionAt     *time.Time
	LastHighIntensityAt   *time.Time
	LastRestPeriodStart   *time.Time
	RuminationDetected    bool
	RuminationTopic       string
	RuminationCount24h    int
	EmotionalSaturation   bool
	FatigueLevel          string // none, mild, moderate, severe
	ActiveRestrictions    map[string]interface{}
	RestrictionReason     string
	RestrictionUntil      *time.Time
	UpdatedAt             time.Time
}

// LoadDecision representa decisao do orquestrador
type LoadDecision struct {
	PatientID                 int64
	CurrentLoad               float64
	TriggerEvent              string
	DecisionType              string // block, allow, redirect, reduce_frequency, suggest_rest
	BlockedActions            []string
	AllowedActions            []string
	RedirectSuggestion        string
	SystemInstructionOverride string
	ToneAdjustment            string
}

// NewCognitiveLoadOrchestrator cria novo orquestrador
func NewCognitiveLoadOrchestrator(db *database.DB, cacheStore *nietzscheInfra.CacheStore) *CognitiveLoadOrchestrator {
	return &CognitiveLoadOrchestrator{
		db:    db,
		cache: cacheStore,
		ctx:   context.Background(),
	}
}

// RecordInteraction registra uma interacao e atualiza carga
func (clo *CognitiveLoadOrchestrator) RecordInteraction(load InteractionLoad) error {
	// 1. Calcular carga da interacao
	interactionLoad := clo.calculateInteractionLoad(load)

	// 2. Buscar estado atual (necessario para atualizacao incremental)
	_, err := clo.GetCurrentState(load.PatientID)
	if err != nil {
		// If error is not "not found", return it
		return fmt.Errorf("erro ao buscar estado: %w", err)
	}

	// 3. Inserir interacao no historico
	fatigueJSON, _ := json.Marshal(load.FatigueIndicators)
	topicsJSON, _ := json.Marshal(load.TopicsDiscussed)
	signifiersJSON, _ := json.Marshal(load.LacanianSignifiers)

	content := map[string]interface{}{
		"patient_id":                load.PatientID,
		"timestamp":                time.Now().Format(time.RFC3339),
		"interaction_type":         load.InteractionType,
		"emotional_intensity":      load.EmotionalIntensity,
		"cognitive_complexity":     load.CognitiveComplexity,
		"duration_seconds":         load.DurationSeconds,
		"patient_fatigue_indicators": string(fatigueJSON),
		"topics_discussed":         string(topicsJSON),
		"lacanian_signifiers":      string(signifiersJSON),
		"session_id":               load.SessionID,
		"cumulative_load_24h":      interactionLoad,
	}

	if load.VoiceEnergyScore != nil {
		content["voice_energy_score"] = *load.VoiceEnergyScore
	}
	if load.SpeechRateWPM != nil {
		content["speech_rate_wpm"] = *load.SpeechRateWPM
	}
	if load.PauseFrequency != nil {
		content["pause_frequency"] = *load.PauseFrequency
	}

	_, err = clo.db.Insert(clo.ctx, "interaction_cognitive_load", content)
	if err != nil {
		return fmt.Errorf("erro ao inserir interacao: %w", err)
	}

	// 4. Atualizar estado acumulado
	err = clo.updateStateAfterInteraction(load.PatientID, load, interactionLoad)
	if err != nil {
		return fmt.Errorf("erro ao atualizar estado: %w", err)
	}

	// 5. Atualizar cache NietzscheDB
	err = clo.updateNietzscheDBCache(load.PatientID)
	if err != nil {
		log.Printf("[COGNITIVE] Erro ao atualizar cache: %v", err)
	}

	// 6. Verificar se precisa tomar decisao
	updatedState, _ := clo.GetCurrentState(load.PatientID)
	if clo.shouldTakeAction(updatedState) {
		decision := clo.MakeDecision(updatedState)
		err = clo.applyDecision(decision)
		if err != nil {
			log.Printf("[COGNITIVE] Erro ao aplicar decisao: %v", err)
		}
	}

	return nil
}

// GetCurrentState busca estado atual de carga
func (clo *CognitiveLoadOrchestrator) GetCurrentState(patientID int64) (*CognitiveLoadState, error) {
	// Tentar buscar do cache primeiro
	cacheKey := fmt.Sprintf("cognitive_load:%d:state", patientID)
	cached, err := clo.cache.Get(clo.ctx, cacheKey)
	if err == nil {
		var state CognitiveLoadState
		if json.Unmarshal([]byte(cached), &state) == nil {
			return &state, nil
		}
	}

	// Buscar do NietzscheDB
	rows, err := clo.db.QueryByLabel(clo.ctx, "cognitive_load_state",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		// Criar estado inicial
		return clo.createInitialState(patientID)
	}

	m := rows[0]
	state := &CognitiveLoadState{
		PatientID:             database.GetInt64(m, "patient_id"),
		CurrentLoadScore:      database.GetFloat64(m, "current_load_score"),
		Load24h:               database.GetFloat64(m, "load_24h"),
		Load7d:                database.GetFloat64(m, "load_7d"),
		InteractionsCount24h:  int(database.GetInt64(m, "interactions_count_24h")),
		TherapeuticCount24h:   int(database.GetInt64(m, "therapeutic_count_24h")),
		HighIntensityCount24h: int(database.GetInt64(m, "high_intensity_count_24h")),
		LastInteractionAt:     database.GetTimePtr(m, "last_interaction_at"),
		LastHighIntensityAt:   database.GetTimePtr(m, "last_high_intensity_at"),
		LastRestPeriodStart:   database.GetTimePtr(m, "last_rest_period_start"),
		RuminationDetected:    database.GetBool(m, "rumination_detected"),
		RuminationTopic:       database.GetString(m, "rumination_topic"),
		RuminationCount24h:    int(database.GetInt64(m, "rumination_count_24h")),
		EmotionalSaturation:   database.GetBool(m, "emotional_saturation"),
		FatigueLevel:          database.GetString(m, "fatigue_level"),
		RestrictionReason:     database.GetString(m, "restriction_reason"),
		RestrictionUntil:      database.GetTimePtr(m, "restriction_until"),
		UpdatedAt:             database.GetTime(m, "updated_at"),
	}

	// Unmarshal JSON fields
	if restrictionsJSON := database.GetString(m, "active_restrictions"); restrictionsJSON != "" {
		json.Unmarshal([]byte(restrictionsJSON), &state.ActiveRestrictions)
	}

	return state, nil
}

// MakeDecision decide acoes baseado no estado
func (clo *CognitiveLoadOrchestrator) MakeDecision(state *CognitiveLoadState) *LoadDecision {
	decision := &LoadDecision{
		PatientID:   state.PatientID,
		CurrentLoad: state.CurrentLoadScore,
	}

	// Regra 1: Carga alta + intensidade emocional recente
	if state.CurrentLoadScore > 0.7 && state.LastHighIntensityAt != nil {
		timeSinceHighIntensity := time.Since(*state.LastHighIntensityAt)
		if timeSinceHighIntensity < 3*time.Hour {
			decision.DecisionType = "block"
			decision.TriggerEvent = "high_load_with_recent_high_intensity"
			decision.BlockedActions = []string{
				"apply_phq9", "apply_gad7", "apply_cssrs",
				"deep_therapy", "reminiscence_therapy",
			}
			decision.AllowedActions = []string{
				"play_music", "read_newspaper", "light_jokes", "weather_chat",
			}
			decision.RedirectSuggestion = "Vamos relaxar um pouco? Que tal ouvir musica ou conversar sobre coisas leves?"
			decision.SystemInstructionOverride = clo.generateSystemInstructionForHighLoad(state)
			decision.ToneAdjustment = "lighter"
			return decision
		}
	}

	// Regra 2: Ruminacao detectada
	if state.RuminationDetected && state.RuminationCount24h >= 3 {
		decision.DecisionType = "redirect"
		decision.TriggerEvent = "rumination_detected"
		decision.BlockedActions = []string{"transnar_deep_analysis"}
		decision.RedirectSuggestion = fmt.Sprintf(
			"Percebo que '%s' esta te preocupando muito. Vamos pensar em outra coisa?",
			state.RuminationTopic,
		)
		decision.SystemInstructionOverride = fmt.Sprintf(`
RESTRICAO ATIVA: Ruminacao detectada
TOPICO EM LOOP: %s

COMPORTAMENTO OBRIGATORIO:
- NAO aprofundar topico "%s"
- Validar sentimento brevemente
- Redirecionar suavemente para outros assuntos
- Sugerir atividades praticas ou distrativas
		`, state.RuminationTopic, state.RuminationTopic)
		decision.ToneAdjustment = "gentle_redirect"
		return decision
	}

	// Regra 3: Saturacao emocional
	if state.EmotionalSaturation {
		decision.DecisionType = "reduce_frequency"
		decision.TriggerEvent = "emotional_saturation"
		decision.BlockedActions = []string{"all_therapeutic_tools"}
		decision.RedirectSuggestion = "Voce ja conversou bastante hoje. Que tal descansar um pouco?"
		decision.SystemInstructionOverride = `
RESTRICAO ATIVA: Saturacao emocional
- Aumentar tempo entre respostas proativas
- Apenas responder se paciente iniciar
- Tom muito leve e breve
- Sugerir descanso
		`
		return decision
	}

	// Regra 4: Muitas interacoes no dia
	if state.InteractionsCount24h > 15 {
		decision.DecisionType = "suggest_rest"
		decision.TriggerEvent = "high_interaction_count"
		decision.RedirectSuggestion = "Voce ja conversou bastante hoje. Vamos dar um tempo?"
		decision.SystemInstructionOverride = `
RESTRICAO ATIVA: Alto numero de interacoes
- Reduzir proatividade
- Respostas mais breves
- Incentivar descanso
		`
		return decision
	}

	// Sem restricoes - permitir tudo
	decision.DecisionType = "allow"
	return decision
}

// GetSystemInstructionOverride retorna system instruction adaptativa
func (clo *CognitiveLoadOrchestrator) GetSystemInstructionOverride(patientID int64) (string, error) {
	state, err := clo.GetCurrentState(patientID)
	if err != nil {
		return "", err
	}

	if state.CurrentLoadScore < 0.5 {
		return "", nil // Sem restricoes
	}

	decision := clo.MakeDecision(state)
	return decision.SystemInstructionOverride, nil
}

// Helper: Calcular carga de uma interacao
func (clo *CognitiveLoadOrchestrator) calculateInteractionLoad(load InteractionLoad) float64 {
	// Formula de carga: intensidade emocional (40%) + complexidade cognitiva (30%) + duracao (30%)
	emotionalComponent := load.EmotionalIntensity * 0.4
	cognitiveComponent := load.CognitiveComplexity * 0.3

	// Normalizar duracao (0-60 min)
	durationMinutes := float64(load.DurationSeconds) / 60.0
	durationNormalized := durationMinutes / 60.0 // 60 min = 1.0
	if durationNormalized > 1.0 {
		durationNormalized = 1.0
	}
	durationComponent := durationNormalized * 0.3

	return emotionalComponent + cognitiveComponent + durationComponent
}

// Helper: Verificar se deve tomar acao
func (clo *CognitiveLoadOrchestrator) shouldTakeAction(state *CognitiveLoadState) bool {
	return state.CurrentLoadScore > 0.7 ||
		state.RuminationDetected ||
		state.EmotionalSaturation ||
		state.InteractionsCount24h > 15
}

// Helper: Atualizar estado apos interacao
func (clo *CognitiveLoadOrchestrator) updateStateAfterInteraction(patientID int64, load InteractionLoad, interactionLoad float64) error {
	// Buscar ou criar estado
	state, err := clo.GetCurrentState(patientID)
	if err != nil {
		return err
	}

	// Calcular nova carga 24h (weighted average)
	newLoad24h := (state.Load24h*0.9 + interactionLoad*0.1) // Decay + nova interacao

	// Incrementar contadores
	newInteractionsCount := state.InteractionsCount24h + 1
	newTherapeuticCount := state.TherapeuticCount24h
	if load.InteractionType == "therapeutic" {
		newTherapeuticCount++
	}

	newHighIntensityCount := state.HighIntensityCount24h
	var newLastHighIntensityAt *time.Time
	if load.EmotionalIntensity > 0.8 {
		newHighIntensityCount++
		now := time.Now()
		newLastHighIntensityAt = &now
	} else {
		newLastHighIntensityAt = state.LastHighIntensityAt
	}

	// Detectar ruminacao (mesmo topico 3x)
	ruminationDetected := clo.detectRumination(patientID, load.TopicsDiscussed, load.LacanianSignifiers)

	// Detectar saturacao emocional
	emotionalSaturation := newLoad24h > 0.8 && newTherapeuticCount >= 3

	// Calcular fadiga
	fatigueLevel := clo.calculateFatigueLevel(newLoad24h, newInteractionsCount, load.FatigueIndicators)

	now := time.Now().Format(time.RFC3339)

	updates := map[string]interface{}{
		"current_load_score":      newLoad24h,
		"load_24h":                newLoad24h,
		"interactions_count_24h":  newInteractionsCount,
		"therapeutic_count_24h":   newTherapeuticCount,
		"high_intensity_count_24h": newHighIntensityCount,
		"last_interaction_at":     now,
		"rumination_detected":     ruminationDetected,
		"rumination_count_24h":    clo.countRumination24h(patientID, load.TopicsDiscussed, load.LacanianSignifiers),
		"emotional_saturation":    emotionalSaturation,
		"fatigue_level":           fatigueLevel,
		"updated_at":              now,
	}

	if newLastHighIntensityAt != nil {
		updates["last_high_intensity_at"] = newLastHighIntensityAt.Format(time.RFC3339)
	}

	// Compute 7-day load for both update and insert paths
	updates["load_7d"] = clo.calculateLoad7d(patientID)

	// Try update first
	err = clo.db.Update(clo.ctx, "cognitive_load_state",
		map[string]interface{}{"patient_id": patientID},
		updates,
	)
	if err != nil {
		// If update fails, insert
		updates["patient_id"] = patientID
		_, err = clo.db.Insert(clo.ctx, "cognitive_load_state", updates)
	}

	return err
}

// Helper: Detectar ruminacao
func (clo *CognitiveLoadOrchestrator) detectRumination(patientID int64, topics []string, signifiers []string) bool {
	// Query: mesmo topico ou significante 3x nas ultimas 2h
	if len(topics) == 0 && len(signifiers) == 0 {
		return false
	}

	cutoff := time.Now().Add(-2 * time.Hour).Format(time.RFC3339)

	// Query all recent interactions and check topic overlap in Go
	rows, err := clo.db.QueryByLabel(clo.ctx, "interaction_cognitive_load",
		" AND n.patient_id = $pid AND n.timestamp > $cutoff",
		map[string]interface{}{
			"pid":    patientID,
			"cutoff": cutoff,
		},
		0,
	)
	if err != nil {
		return false
	}

	// Count how many past interactions share topics or signifiers
	matchCount := 0
	for _, m := range rows {
		// Check topics overlap
		storedTopicsStr := database.GetString(m, "topics_discussed")
		storedSignifiersStr := database.GetString(m, "lacanian_signifiers")

		hasOverlap := false

		// Check topic overlap
		for _, topic := range topics {
			if strings.Contains(storedTopicsStr, topic) {
				hasOverlap = true
				break
			}
		}

		// Check signifier overlap
		if !hasOverlap {
			for _, sig := range signifiers {
				if strings.Contains(storedSignifiersStr, sig) {
					hasOverlap = true
					break
				}
			}
		}

		if hasOverlap {
			matchCount++
		}
	}

	return matchCount >= 3
}

// Helper: Calcular nivel de fadiga
func (clo *CognitiveLoadOrchestrator) calculateFatigueLevel(load24h float64, interactionsCount int, fatigueIndicators map[string]interface{}) string {
	if load24h > 0.8 || interactionsCount > 20 {
		return "severe"
	}
	if load24h > 0.6 || interactionsCount > 15 {
		return "moderate"
	}
	if load24h > 0.4 || interactionsCount > 10 {
		return "mild"
	}
	return "none"
}

// Helper: Criar estado inicial
func (clo *CognitiveLoadOrchestrator) createInitialState(patientID int64) (*CognitiveLoadState, error) {
	content := map[string]interface{}{
		"patient_id":           patientID,
		"current_load_score":   0.0,
		"load_24h":             0.0,
		"load_7d":              0.0,
		"interactions_count_24h": 0,
		"created_at":           time.Now().Format(time.RFC3339),
		"updated_at":           time.Now().Format(time.RFC3339),
	}

	_, err := clo.db.Insert(clo.ctx, "cognitive_load_state", content)
	if err != nil {
		return nil, err
	}

	return &CognitiveLoadState{
		PatientID:        patientID,
		CurrentLoadScore: 0,
		Load24h:          0,
		Load7d:           0,
	}, nil
}

// Helper: Gerar system instruction para carga alta
func (clo *CognitiveLoadOrchestrator) generateSystemInstructionForHighLoad(state *CognitiveLoadState) string {
	return fmt.Sprintf(`
CARGA COGNITIVA ATUAL DO PACIENTE: ALTA (%.2f/1.0)
ULTIMAS INTERACOES: %d total, %d terapeuticas nas ultimas 24h

RESTRICOES ATIVAS:
- NAO aprofundar temas emocionais
- NAO aplicar escalas clinicas (PHQ-9, GAD-7, C-SSRS)
- PRIORIZAR: entretenimento leve, humor, musica, conversas descontraidas
- TOM: leve, descontraido, sem peso emocional

Se paciente insistir em topico pesado:
- Validar sentimento brevemente
- Redirecionar suavemente: "Entendo... vamos pensar nisso amanha com mais calma?"
- Sugerir atividade leve: musica, noticias, piada
	`, state.CurrentLoadScore, state.InteractionsCount24h, state.TherapeuticCount24h)
}

// Helper: Aplicar decisao
func (clo *CognitiveLoadOrchestrator) applyDecision(decision *LoadDecision) error {
	// Salvar decisao no banco
	blockedJSON, _ := json.Marshal(decision.BlockedActions)
	allowedJSON, _ := json.Marshal(decision.AllowedActions)

	content := map[string]interface{}{
		"patient_id":                decision.PatientID,
		"current_load":              decision.CurrentLoad,
		"trigger_event":             decision.TriggerEvent,
		"decision_type":             decision.DecisionType,
		"blocked_actions":           string(blockedJSON),
		"allowed_actions":           string(allowedJSON),
		"redirect_suggestion":       decision.RedirectSuggestion,
		"system_instruction_override": decision.SystemInstructionOverride,
		"tone_adjustment":           decision.ToneAdjustment,
		"created_at":                time.Now().Format(time.RFC3339),
	}

	_, err := clo.db.Insert(clo.ctx, "cognitive_load_decisions", content)
	if err != nil {
		return err
	}

	log.Printf("[COGNITIVE] Decisao aplicada para paciente %d: %s", decision.PatientID, decision.DecisionType)
	return nil
}

// Helper: Atualizar cache
func (clo *CognitiveLoadOrchestrator) updateNietzscheDBCache(patientID int64) error {
	state, err := clo.GetCurrentState(patientID)
	if err != nil {
		return err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("cognitive_load:%d:state", patientID)
	return clo.cache.Set(clo.ctx, cacheKey, string(stateJSON), 5*time.Minute)
}

// countRumination24h counts how many interactions in the last 24h share topics
// or Lacanian signifiers with the current interaction, indicating rumination.
func (clo *CognitiveLoadOrchestrator) countRumination24h(patientID int64, topics []string, signifiers []string) int {
	if len(topics) == 0 && len(signifiers) == 0 {
		return 0
	}

	cutoff := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	rows, err := clo.db.QueryByLabel(clo.ctx, "interaction_cognitive_load",
		" AND n.patient_id = $pid AND n.timestamp > $cutoff",
		map[string]interface{}{
			"pid":    patientID,
			"cutoff": cutoff,
		},
		0,
	)
	if err != nil {
		log.Printf("[COGNITIVE] Erro ao contar ruminacao 24h: %v", err)
		return 0
	}

	matchCount := 0
	for _, m := range rows {
		storedTopicsStr := database.GetString(m, "topics_discussed")
		storedSignifiersStr := database.GetString(m, "lacanian_signifiers")

		hasOverlap := false
		for _, topic := range topics {
			if strings.Contains(storedTopicsStr, topic) {
				hasOverlap = true
				break
			}
		}
		if !hasOverlap {
			for _, sig := range signifiers {
				if strings.Contains(storedSignifiersStr, sig) {
					hasOverlap = true
					break
				}
			}
		}
		if hasOverlap {
			matchCount++
		}
	}

	return matchCount
}

// calculateLoad7d computes the average cognitive load over the last 7 days
// by querying all interaction records and averaging their cumulative_load_24h values.
func (clo *CognitiveLoadOrchestrator) calculateLoad7d(patientID int64) float64 {
	cutoff := time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)

	rows, err := clo.db.QueryByLabel(clo.ctx, "interaction_cognitive_load",
		" AND n.patient_id = $pid AND n.timestamp > $cutoff",
		map[string]interface{}{
			"pid":    patientID,
			"cutoff": cutoff,
		},
		0,
	)
	if err != nil {
		log.Printf("[COGNITIVE] Erro ao calcular load_7d: %v", err)
		return 0
	}

	if len(rows) == 0 {
		return 0
	}

	totalLoad := 0.0
	for _, m := range rows {
		totalLoad += database.GetFloat64(m, "cumulative_load_24h")
	}

	avg := totalLoad / float64(len(rows))
	if avg > 1.0 {
		avg = 1.0
	}
	return avg
}
