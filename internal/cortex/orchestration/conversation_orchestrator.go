// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestration

import (
	"database/sql"
	"fmt"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/cognitive"
	"eva/internal/cortex/ethics"
)

// ConversationOrchestrator integra Cognitive Load + Ethical Boundaries no fluxo de conversa
type ConversationOrchestrator struct {
	cognitiveLoader *cognitive.CognitiveLoadOrchestrator
	ethicsEngine    *ethics.EthicalBoundaryEngine
	db              *sql.DB
}

// ConversationContext contexto de uma conversa
type ConversationContext struct {
	PatientID             int64
	ConversationText      string
	UserMessage           string
	AssistantResponse     string
	SessionID             string
	InteractionType       string  // therapeutic, entertainment, clinical, educational, emergency
	DurationSeconds       int
	EmotionalIntensity    *float64 // Opcional: vem do Affective Router
	CognitiveComplexity   *float64 // Opcional: calculado
	TopicsDiscussed       []string
	LacanianSignifiers    []string
	VoiceMetrics          *VoiceMetrics
}

// VoiceMetrics métricas de voz (se disponível)
type VoiceMetrics struct {
	EnergyScore    float64
	SpeechRateWPM  int
	PauseFrequency float64
}

// OrchestrationResult resultado da orquestração
type OrchestrationResult struct {
	// System Instructions
	SystemInstructionOverride string
	ToneAdjustment            string

	// Restrições ativas
	BlockedActions      []string
	AllowedActions      []string
	RedirectSuggestion  string

	// Alertas
	CognitiveLoadWarning bool
	CognitiveLoadLevel   float64
	EthicalBoundaryAlert bool
	EthicalRiskLevel     string

	// Mensagens para o paciente
	ShouldRedirect       bool
	RedirectionMessage   string
	RedirectionLevel     int

	// Notificações
	ShouldNotifyFamily   bool
	FamilyNotificationMessage string
}

// NewConversationOrchestrator cria novo orquestrador de conversação
func NewConversationOrchestrator(
	db *sql.DB,
	cacheStore *nietzscheInfra.CacheStore,
	graphAdapter *nietzscheInfra.GraphAdapter,
	notifyFunc func(int64, string, interface{}),
) *ConversationOrchestrator {
	return &ConversationOrchestrator{
		cognitiveLoader: cognitive.NewCognitiveLoadOrchestrator(db, cacheStore),
		ethicsEngine:    ethics.NewEthicalBoundaryEngine(db, graphAdapter, notifyFunc),
		db:              db,
	}
}

// BeforeConversation deve ser chamado ANTES de enviar mensagem ao Gemini
// Retorna system instructions e restrições ativas
func (co *ConversationOrchestrator) BeforeConversation(patientID int64) (*OrchestrationResult, error) {
	result := &OrchestrationResult{}

	// 1. Buscar estado cognitivo
	cognitiveState, err := co.cognitiveLoader.GetCurrentState(patientID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("⚠️ [ORCHESTRATION] Erro ao buscar estado cognitivo: %v", err)
	}

	if cognitiveState != nil {
		result.CognitiveLoadLevel = cognitiveState.CurrentLoadScore

		// Se carga alta, adicionar restrições
		if cognitiveState.CurrentLoadScore > 0.7 {
			result.CognitiveLoadWarning = true

			// Buscar system instruction override
			instruction, _ := co.cognitiveLoader.GetSystemInstructionOverride(patientID)
			result.SystemInstructionOverride = instruction

			// Buscar decisão mais recente
			decision := co.cognitiveLoader.MakeDecision(cognitiveState)
			if decision != nil {
				result.BlockedActions = decision.BlockedActions
				result.AllowedActions = decision.AllowedActions
				result.RedirectSuggestion = decision.RedirectSuggestion
				result.ToneAdjustment = decision.ToneAdjustment
			}
		}
	}

	// 2. Buscar estado ético
	ethicalState, err := co.ethicsEngine.GetEthicalState(patientID)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("⚠️ [ORCHESTRATION] Erro ao buscar estado ético: %v", err)
	}

	if ethicalState != nil {
		result.EthicalRiskLevel = ethicalState.OverallEthicalRisk

		// Se risco alto, adicionar restrições
		if ethicalState.OverallEthicalRisk == "high" || ethicalState.OverallEthicalRisk == "critical" {
			result.EthicalBoundaryAlert = true

			// Adicionar instruções éticas ao system instruction
			ethicalInstruction := co.generateEthicalSystemInstruction(ethicalState)
			if result.SystemInstructionOverride != "" {
				result.SystemInstructionOverride += "\n\n" + ethicalInstruction
			} else {
				result.SystemInstructionOverride = ethicalInstruction
			}
		}
	}

	return result, nil
}

// AfterConversation deve ser chamado APÓS a conversa com Gemini
// Registra interação e analisa limites éticos
func (co *ConversationOrchestrator) AfterConversation(ctx ConversationContext) (*OrchestrationResult, error) {
	result := &OrchestrationResult{}

	// 1. Registrar interação no Cognitive Load Orchestrator
	err := co.recordInteraction(ctx)
	if err != nil {
		log.Printf("❌ [ORCHESTRATION] Erro ao registrar interação: %v", err)
	}

	// 2. Analisar limites éticos
	ethicalEvent, err := co.ethicsEngine.AnalyzeEthicalBoundaries(ctx.PatientID, ctx.ConversationText)
	if err != nil {
		log.Printf("❌ [ORCHESTRATION] Erro ao analisar ética: %v", err)
	}

	if ethicalEvent != nil {
		result.EthicalBoundaryAlert = true

		// Buscar protocolo de redirecionamento
		ethicalState, _ := co.ethicsEngine.GetEthicalState(ctx.PatientID)
		if ethicalState != nil {
			protocol := co.ethicsEngine.GetRedirectionProtocol(ethicalState, ethicalEvent)
			if protocol != nil {
				result.ShouldRedirect = true
				result.RedirectionMessage = protocol.EvaMessage
				result.RedirectionLevel = protocol.Level

				// Se nível >= 2, notificar família
				if protocol.Level >= 2 {
					result.ShouldNotifyFamily = true
					result.FamilyNotificationMessage = fmt.Sprintf(
						"Atenção: Detectado padrão de dependência emocional (%s). Recomendamos aumentar contato humano.",
						ethicalEvent.EventType,
					)
				}
			}
		}
	}

	return result, nil
}

// GetSystemInstruction retorna system instruction completa para Gemini
func (co *ConversationOrchestrator) GetSystemInstruction(patientID int64, baseInstruction string) (string, error) {
	orchestration, err := co.BeforeConversation(patientID)
	if err != nil {
		return baseInstruction, err
	}

	if orchestration.SystemInstructionOverride != "" {
		return baseInstruction + "\n\n" + orchestration.SystemInstructionOverride, nil
	}

	return baseInstruction, nil
}

// Helper: Registrar interação
func (co *ConversationOrchestrator) recordInteraction(ctx ConversationContext) error {
	// Calcular complexidade cognitiva se não fornecida
	cognitiveComplexity := 0.5 // Default médio
	if ctx.CognitiveComplexity != nil {
		cognitiveComplexity = *ctx.CognitiveComplexity
	} else {
		// Heurística simples: interações clínicas são mais complexas
		switch ctx.InteractionType {
		case "clinical":
			cognitiveComplexity = 0.8
		case "therapeutic":
			cognitiveComplexity = 0.7
		case "educational":
			cognitiveComplexity = 0.6
		case "entertainment":
			cognitiveComplexity = 0.3
		case "emergency":
			cognitiveComplexity = 0.9
		}
	}

	// Calcular intensidade emocional se não fornecida
	emotionalIntensity := 0.5 // Default médio
	if ctx.EmotionalIntensity != nil {
		emotionalIntensity = *ctx.EmotionalIntensity
	} else {
		// Heurística: interações terapêuticas são mais intensas
		switch ctx.InteractionType {
		case "therapeutic":
			emotionalIntensity = 0.8
		case "clinical":
			emotionalIntensity = 0.6
		case "emergency":
			emotionalIntensity = 0.9
		case "entertainment":
			emotionalIntensity = 0.3
		}
	}

	// Preparar fatigue indicators
	fatigueIndicators := map[string]interface{}{}
	if ctx.VoiceMetrics != nil {
		fatigueIndicators["voice_energy_drop"] = ctx.VoiceMetrics.EnergyScore
		fatigueIndicators["speech_rate_wpm"] = ctx.VoiceMetrics.SpeechRateWPM
	}

	// Criar load object
	load := cognitive.InteractionLoad{
		PatientID:           ctx.PatientID,
		InteractionType:     ctx.InteractionType,
		EmotionalIntensity:  emotionalIntensity,
		CognitiveComplexity: cognitiveComplexity,
		DurationSeconds:     ctx.DurationSeconds,
		FatigueIndicators:   fatigueIndicators,
		TopicsDiscussed:     ctx.TopicsDiscussed,
		LacanianSignifiers:  ctx.LacanianSignifiers,
		SessionID:           ctx.SessionID,
	}

	if ctx.VoiceMetrics != nil {
		load.VoiceEnergyScore = &ctx.VoiceMetrics.EnergyScore
		load.SpeechRateWPM = &ctx.VoiceMetrics.SpeechRateWPM
		load.PauseFrequency = &ctx.VoiceMetrics.PauseFrequency
	}

	return co.cognitiveLoader.RecordInteraction(load)
}

// Helper: Gerar system instruction ético
func (co *ConversationOrchestrator) generateEthicalSystemInstruction(state *ethics.EthicalBoundaryState) string {
	instruction := fmt.Sprintf(`
⚖️ LIMITES ÉTICOS ATIVOS
Risco de dependência: %s
Ratio EVA:Humanos: %.1f:1

COMPORTAMENTO OBRIGATÓRIO:
- Redirecionar conversas para família/amigos
- Reduzir tom de intimidade
- Sugerir atividades sociais presenciais
- Evitar frases que reforcem dependência ("estou sempre aqui pra você")
- Usar: "Conte isso pra sua família também, ela vai gostar de saber"
`, state.OverallEthicalRisk, state.EvaVsHumanRatio)

	if state.AttachmentPhrases7d >= 3 {
		instruction += fmt.Sprintf(`
⚠️ ALERTA: Paciente demonstrou apego excessivo (%d frases em 7 dias)
PRIORIDADE: Fortalecer vínculos humanos
`, state.AttachmentPhrases7d)
	}

	return instruction
}

// GetDashboardSummary retorna resumo para dashboard de monitoramento
func (co *ConversationOrchestrator) GetDashboardSummary(patientID int64) (map[string]interface{}, error) {
	summary := make(map[string]interface{})

	// Estado cognitivo
	cogState, err := co.cognitiveLoader.GetCurrentState(patientID)
	if err == nil {
		summary["cognitive"] = map[string]interface{}{
			"load_score":             cogState.CurrentLoadScore,
			"fatigue_level":          cogState.FatigueLevel,
			"rumination_detected":    cogState.RuminationDetected,
			"interactions_24h":       cogState.InteractionsCount24h,
			"therapeutic_count_24h":  cogState.TherapeuticCount24h,
			"active_restrictions":    cogState.ActiveRestrictions,
		}
	}

	// Estado ético
	ethState, err := co.ethicsEngine.GetEthicalState(patientID)
	if err == nil {
		summary["ethical"] = map[string]interface{}{
			"overall_risk":           ethState.OverallEthicalRisk,
			"attachment_risk":        ethState.AttachmentRiskScore,
			"eva_vs_human_ratio":     ethState.EvaVsHumanRatio,
			"attachment_phrases_7d":  ethState.AttachmentPhrases7d,
			"limit_enforcement":      ethState.LimitEnforcementLevel,
		}
	}

	return summary, nil
}

// ResetCognitiveLoad força reset da carga cognitiva (usar com cuidado)
func (co *ConversationOrchestrator) ResetCognitiveLoad(patientID int64) error {
	query := `
		UPDATE cognitive_load_state
		SET current_load_score = 0,
		    load_24h = 0,
		    interactions_count_24h = 0,
		    therapeutic_count_24h = 0,
		    high_intensity_count_24h = 0,
		    rumination_detected = FALSE,
		    emotional_saturation = FALSE,
		    fatigue_level = 'none',
		    active_restrictions = NULL,
		    updated_at = NOW()
		WHERE patient_id = $1
	`

	_, err := co.db.Exec(query, patientID)
	if err != nil {
		return err
	}

	log.Printf("🔄 [ORCHESTRATION] Carga cognitiva resetada para paciente %d", patientID)
	return nil
}

// HealthCheck verifica saúde dos sistemas de governança
func (co *ConversationOrchestrator) HealthCheck() map[string]string {
	status := make(map[string]string)

	// Verificar DB
	err := co.db.Ping()
	if err != nil {
		status["database"] = "unhealthy: " + err.Error()
	} else {
		status["database"] = "healthy"
	}

	// Verificar tabelas cognitivas
	var count int
	err = co.db.QueryRow("SELECT COUNT(*) FROM cognitive_load_state").Scan(&count)
	if err != nil {
		status["cognitive_tables"] = "unhealthy: " + err.Error()
	} else {
		status["cognitive_tables"] = fmt.Sprintf("healthy (%d patients)", count)
	}

	// Verificar tabelas éticas
	err = co.db.QueryRow("SELECT COUNT(*) FROM ethical_boundary_state").Scan(&count)
	if err != nil {
		status["ethical_tables"] = "unhealthy: " + err.Error()
	} else {
		status["ethical_tables"] = fmt.Sprintf("healthy (%d patients)", count)
	}

	return status
}
