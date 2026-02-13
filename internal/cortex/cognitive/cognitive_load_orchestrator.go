package cognitive

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/lib/pq"
)

// CognitiveLoadOrchestrator gerencia carga cognitiva do paciente
type CognitiveLoadOrchestrator struct {
	db    *sql.DB
	redis *redis.Client
	ctx   context.Context
}

// InteractionLoad representa carga de uma interação
type InteractionLoad struct {
	PatientID            int64
	InteractionType      string // therapeutic, entertainment, clinical, educational, emergency
	EmotionalIntensity   float64 // 0-1
	CognitiveComplexity  float64 // 0-1
	DurationSeconds      int
	FatigueIndicators    map[string]interface{}
	TopicsDiscussed      []string
	LacanianSignifiers   []string
	SessionID            string
	VoiceEnergyScore     *float64
	SpeechRateWPM        *int
	PauseFrequency       *float64
}

// CognitiveLoadState representa estado atual de carga
type CognitiveLoadState struct {
	PatientID                int64
	CurrentLoadScore         float64 // 0-1
	Load24h                  float64
	Load7d                   float64
	InteractionsCount24h     int
	TherapeuticCount24h      int
	HighIntensityCount24h    int
	LastInteractionAt        *time.Time
	LastHighIntensityAt      *time.Time
	LastRestPeriodStart      *time.Time
	RuminationDetected       bool
	RuminationTopic          string
	RuminationCount24h       int
	EmotionalSaturation      bool
	FatigueLevel             string // none, mild, moderate, severe
	ActiveRestrictions       map[string]interface{}
	RestrictionReason        string
	RestrictionUntil         *time.Time
	UpdatedAt                time.Time
}

// LoadDecision representa decisão do orquestrador
type LoadDecision struct {
	PatientID                int64
	CurrentLoad              float64
	TriggerEvent             string
	DecisionType             string // block, allow, redirect, reduce_frequency, suggest_rest
	BlockedActions           []string
	AllowedActions           []string
	RedirectSuggestion       string
	SystemInstructionOverride string
	ToneAdjustment           string
}

// NewCognitiveLoadOrchestrator cria novo orquestrador
func NewCognitiveLoadOrchestrator(db *sql.DB, redisClient *redis.Client) *CognitiveLoadOrchestrator {
	return &CognitiveLoadOrchestrator{
		db:    db,
		redis: redisClient,
		ctx:   context.Background(),
	}
}

// RecordInteraction registra uma interação e atualiza carga
func (clo *CognitiveLoadOrchestrator) RecordInteraction(load InteractionLoad) error {
	// 1. Calcular carga da interação
	interactionLoad := clo.calculateInteractionLoad(load)

	// 2. Buscar estado atual (necessário para atualização incremental)
	_, err := clo.GetCurrentState(load.PatientID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("erro ao buscar estado: %w", err)
	}

	// 3. Inserir interação no histórico
	fatigueJSON, _ := json.Marshal(load.FatigueIndicators)

	query := `
		INSERT INTO interaction_cognitive_load (
			patient_id, timestamp, interaction_type, emotional_intensity, cognitive_complexity,
			duration_seconds, patient_fatigue_indicators, topics_discussed, lacanian_signifiers,
			session_id, voice_energy_score, speech_rate_wpm, pause_frequency, cumulative_load_24h
		) VALUES ($1, NOW(), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err = clo.db.Exec(
		query,
		load.PatientID,
		load.InteractionType,
		load.EmotionalIntensity,
		load.CognitiveComplexity,
		load.DurationSeconds,
		fatigueJSON,
		pq.Array(load.TopicsDiscussed),
		pq.Array(load.LacanianSignifiers),
		load.SessionID,
		load.VoiceEnergyScore,
		load.SpeechRateWPM,
		load.PauseFrequency,
		interactionLoad,
	)
	if err != nil {
		return fmt.Errorf("erro ao inserir interação: %w", err)
	}

	// 4. Atualizar estado acumulado
	err = clo.updateStateAfterInteraction(load.PatientID, load, interactionLoad)
	if err != nil {
		return fmt.Errorf("erro ao atualizar estado: %w", err)
	}

	// 5. Atualizar cache Redis
	err = clo.updateRedisCache(load.PatientID)
	if err != nil {
		log.Printf("⚠️ [COGNITIVE] Erro ao atualizar Redis: %v", err)
	}

	// 6. Verificar se precisa tomar decisão
	updatedState, _ := clo.GetCurrentState(load.PatientID)
	if clo.shouldTakeAction(updatedState) {
		decision := clo.MakeDecision(updatedState)
		err = clo.applyDecision(decision)
		if err != nil {
			log.Printf("❌ [COGNITIVE] Erro ao aplicar decisão: %v", err)
		}
	}

	return nil
}

// GetCurrentState busca estado atual de carga
func (clo *CognitiveLoadOrchestrator) GetCurrentState(patientID int64) (*CognitiveLoadState, error) {
	// Tentar buscar do Redis primeiro (cache)
	cacheKey := fmt.Sprintf("cognitive_load:%d:state", patientID)
	cached, err := clo.redis.Get(clo.ctx, cacheKey).Result()
	if err == nil {
		var state CognitiveLoadState
		if json.Unmarshal([]byte(cached), &state) == nil {
			return &state, nil
		}
	}

	// Buscar do PostgreSQL
	query := `
		SELECT
			patient_id, current_load_score, load_24h, load_7d,
			interactions_count_24h, therapeutic_count_24h, high_intensity_count_24h,
			last_interaction_at, last_high_intensity_at, last_rest_period_start,
			rumination_detected, rumination_topic, rumination_count_24h,
			emotional_saturation, fatigue_level, active_restrictions,
			restriction_reason, restriction_until, updated_at
		FROM cognitive_load_state
		WHERE patient_id = $1
	`

	state := &CognitiveLoadState{}
	var restrictionsJSON []byte

	err = clo.db.QueryRow(query, patientID).Scan(
		&state.PatientID,
		&state.CurrentLoadScore,
		&state.Load24h,
		&state.Load7d,
		&state.InteractionsCount24h,
		&state.TherapeuticCount24h,
		&state.HighIntensityCount24h,
		&state.LastInteractionAt,
		&state.LastHighIntensityAt,
		&state.LastRestPeriodStart,
		&state.RuminationDetected,
		&state.RuminationTopic,
		&state.RuminationCount24h,
		&state.EmotionalSaturation,
		&state.FatigueLevel,
		&restrictionsJSON,
		&state.RestrictionReason,
		&state.RestrictionUntil,
		&state.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			// Criar estado inicial
			return clo.createInitialState(patientID)
		}
		return nil, err
	}

	if restrictionsJSON != nil {
		json.Unmarshal(restrictionsJSON, &state.ActiveRestrictions)
	}

	return state, nil
}

// MakeDecision decide ações baseado no estado
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
			decision.RedirectSuggestion = "Vamos relaxar um pouco? Que tal ouvir música ou conversar sobre coisas leves?"
			decision.SystemInstructionOverride = clo.generateSystemInstructionForHighLoad(state)
			decision.ToneAdjustment = "lighter"
			return decision
		}
	}

	// Regra 2: Ruminação detectada
	if state.RuminationDetected && state.RuminationCount24h >= 3 {
		decision.DecisionType = "redirect"
		decision.TriggerEvent = "rumination_detected"
		decision.BlockedActions = []string{"transnar_deep_analysis"}
		decision.RedirectSuggestion = fmt.Sprintf(
			"Percebo que '%s' está te preocupando muito. Vamos pensar em outra coisa?",
			state.RuminationTopic,
		)
		decision.SystemInstructionOverride = fmt.Sprintf(`
RESTRIÇÃO ATIVA: Ruminação detectada
TÓPICO EM LOOP: %s

COMPORTAMENTO OBRIGATÓRIO:
- NÃO aprofundar tópico "%s"
- Validar sentimento brevemente
- Redirecionar suavemente para outros assuntos
- Sugerir atividades práticas ou distrativas
		`, state.RuminationTopic, state.RuminationTopic)
		decision.ToneAdjustment = "gentle_redirect"
		return decision
	}

	// Regra 3: Saturação emocional
	if state.EmotionalSaturation {
		decision.DecisionType = "reduce_frequency"
		decision.TriggerEvent = "emotional_saturation"
		decision.BlockedActions = []string{"all_therapeutic_tools"}
		decision.RedirectSuggestion = "Você já conversou bastante hoje. Que tal descansar um pouco?"
		decision.SystemInstructionOverride = `
RESTRIÇÃO ATIVA: Saturação emocional
- Aumentar tempo entre respostas proativas
- Apenas responder se paciente iniciar
- Tom muito leve e breve
- Sugerir descanso
		`
		return decision
	}

	// Regra 4: Muitas interações no dia
	if state.InteractionsCount24h > 15 {
		decision.DecisionType = "suggest_rest"
		decision.TriggerEvent = "high_interaction_count"
		decision.RedirectSuggestion = "Você já conversou bastante hoje. Vamos dar um tempo?"
		decision.SystemInstructionOverride = `
RESTRIÇÃO ATIVA: Alto número de interações
- Reduzir proatividade
- Respostas mais breves
- Incentivar descanso
		`
		return decision
	}

	// Sem restrições - permitir tudo
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
		return "", nil // Sem restrições
	}

	decision := clo.MakeDecision(state)
	return decision.SystemInstructionOverride, nil
}

// Helper: Calcular carga de uma interação
func (clo *CognitiveLoadOrchestrator) calculateInteractionLoad(load InteractionLoad) float64 {
	// Fórmula de carga: intensidade emocional (40%) + complexidade cognitiva (30%) + duração (30%)
	emotionalComponent := load.EmotionalIntensity * 0.4
	cognitiveComponent := load.CognitiveComplexity * 0.3

	// Normalizar duração (0-60 min)
	durationMinutes := float64(load.DurationSeconds) / 60.0
	durationNormalized := durationMinutes / 60.0 // 60 min = 1.0
	if durationNormalized > 1.0 {
		durationNormalized = 1.0
	}
	durationComponent := durationNormalized * 0.3

	return emotionalComponent + cognitiveComponent + durationComponent
}

// Helper: Verificar se deve tomar ação
func (clo *CognitiveLoadOrchestrator) shouldTakeAction(state *CognitiveLoadState) bool {
	return state.CurrentLoadScore > 0.7 ||
		state.RuminationDetected ||
		state.EmotionalSaturation ||
		state.InteractionsCount24h > 15
}

// Helper: Atualizar estado após interação
func (clo *CognitiveLoadOrchestrator) updateStateAfterInteraction(patientID int64, load InteractionLoad, interactionLoad float64) error {
	// Buscar ou criar estado
	state, err := clo.GetCurrentState(patientID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Calcular nova carga 24h (weighted average)
	newLoad24h := (state.Load24h*0.9 + interactionLoad*0.1) // Decay + nova interação

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

	// Detectar ruminação (mesmo tópico 3x)
	ruminationDetected := clo.detectRumination(patientID, load.TopicsDiscussed, load.LacanianSignifiers)

	// Detectar saturação emocional
	emotionalSaturation := newLoad24h > 0.8 && newTherapeuticCount >= 3

	// Calcular fadiga
	fatigueLevel := clo.calculateFatigueLevel(newLoad24h, newInteractionsCount, load.FatigueIndicators)

	// Update ou Insert
	query := `
		INSERT INTO cognitive_load_state (
			patient_id, current_load_score, load_24h, load_7d,
			interactions_count_24h, therapeutic_count_24h, high_intensity_count_24h,
			last_interaction_at, last_high_intensity_at,
			rumination_detected, rumination_count_24h,
			emotional_saturation, fatigue_level
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), $8, $9, $10, $11, $12)
		ON CONFLICT (patient_id) DO UPDATE SET
			current_load_score = $2,
			load_24h = $3,
			interactions_count_24h = $5,
			therapeutic_count_24h = $6,
			high_intensity_count_24h = $7,
			last_interaction_at = NOW(),
			last_high_intensity_at = COALESCE($8, cognitive_load_state.last_high_intensity_at),
			rumination_detected = $9,
			rumination_count_24h = $10,
			emotional_saturation = $11,
			fatigue_level = $12,
			updated_at = NOW()
	`

	_, err = clo.db.Exec(
		query,
		patientID,
		newLoad24h,
		newLoad24h,
		0, // TODO: calcular load_7d
		newInteractionsCount,
		newTherapeuticCount,
		newHighIntensityCount,
		newLastHighIntensityAt,
		ruminationDetected,
		0, // TODO: contar ruminação
		emotionalSaturation,
		fatigueLevel,
	)

	return err
}

// Helper: Detectar ruminação
func (clo *CognitiveLoadOrchestrator) detectRumination(patientID int64, topics []string, signifiers []string) bool {
	// Query: mesmo tópico ou significante 3x nas últimas 2h
	if len(topics) == 0 && len(signifiers) == 0 {
		return false
	}

	query := `
		SELECT COUNT(DISTINCT id)
		FROM interaction_cognitive_load
		WHERE patient_id = $1
		  AND timestamp > NOW() - INTERVAL '2 hours'
		  AND (topics_discussed && $2 OR lacanian_signifiers && $3)
	`

	var count int
	err := clo.db.QueryRow(query, patientID, pq.Array(topics), pq.Array(signifiers)).Scan(&count)
	if err != nil {
		return false
	}

	return count >= 3
}

// Helper: Calcular nível de fadiga
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
	query := `
		INSERT INTO cognitive_load_state (patient_id, current_load_score, load_24h, load_7d)
		VALUES ($1, 0, 0, 0)
		RETURNING patient_id, current_load_score, load_24h, load_7d, interactions_count_24h
	`

	state := &CognitiveLoadState{}
	err := clo.db.QueryRow(query, patientID).Scan(
		&state.PatientID,
		&state.CurrentLoadScore,
		&state.Load24h,
		&state.Load7d,
		&state.InteractionsCount24h,
	)

	return state, err
}

// Helper: Gerar system instruction para carga alta
func (clo *CognitiveLoadOrchestrator) generateSystemInstructionForHighLoad(state *CognitiveLoadState) string {
	return fmt.Sprintf(`
CARGA COGNITIVA ATUAL DO PACIENTE: ALTA (%.2f/1.0)
ÚLTIMAS INTERAÇÕES: %d total, %d terapêuticas nas últimas 24h

RESTRIÇÕES ATIVAS:
- NÃO aprofundar temas emocionais
- NÃO aplicar escalas clínicas (PHQ-9, GAD-7, C-SSRS)
- PRIORIZAR: entretenimento leve, humor, música, conversas descontraídas
- TOM: leve, descontraído, sem peso emocional

Se paciente insistir em tópico pesado:
- Validar sentimento brevemente
- Redirecionar suavemente: "Entendo... vamos pensar nisso amanhã com mais calma?"
- Sugerir atividade leve: música, notícias, piada
	`, state.CurrentLoadScore, state.InteractionsCount24h, state.TherapeuticCount24h)
}

// Helper: Aplicar decisão
func (clo *CognitiveLoadOrchestrator) applyDecision(decision *LoadDecision) error {
	// Salvar decisão no banco
	blockedJSON, _ := json.Marshal(decision.BlockedActions)
	allowedJSON, _ := json.Marshal(decision.AllowedActions)

	query := `
		INSERT INTO cognitive_load_decisions (
			patient_id, current_load, trigger_event, decision_type,
			blocked_actions, allowed_actions, redirect_suggestion,
			system_instruction_override, tone_adjustment
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := clo.db.Exec(
		query,
		decision.PatientID,
		decision.CurrentLoad,
		decision.TriggerEvent,
		decision.DecisionType,
		blockedJSON,
		allowedJSON,
		decision.RedirectSuggestion,
		decision.SystemInstructionOverride,
		decision.ToneAdjustment,
	)

	if err != nil {
		return err
	}

	log.Printf("✅ [COGNITIVE] Decisão aplicada para paciente %d: %s", decision.PatientID, decision.DecisionType)
	return nil
}

// Helper: Atualizar cache Redis
func (clo *CognitiveLoadOrchestrator) updateRedisCache(patientID int64) error {
	state, err := clo.GetCurrentState(patientID)
	if err != nil {
		return err
	}

	stateJSON, err := json.Marshal(state)
	if err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("cognitive_load:%d:state", patientID)
	return clo.redis.Set(clo.ctx, cacheKey, stateJSON, 5*time.Minute).Err()
}
