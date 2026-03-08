// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package orchestration

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/cortex/cognitive"
	"eva/internal/cortex/ethics"
	"eva/internal/cortex/llm/thinking"
)

// ConversationOrchestrator integra Cognitive Load + Ethical Boundaries no fluxo de conversa
type ConversationOrchestrator struct {
	cognitiveLoader *cognitive.CognitiveLoadOrchestrator
	ethicsEngine    *ethics.EthicalBoundaryEngine
	db              *database.DB
	system2         *thinking.System2Engine // nil = Sistema 2 desativado
}

// ConversationContext contexto de uma conversa
type ConversationContext struct {
	PatientID           int64
	ConversationText    string
	UserMessage         string
	AssistantResponse   string
	SessionID           string
	InteractionType     string // therapeutic, entertainment, clinical, educational, emergency
	DurationSeconds     int
	EmotionalIntensity  *float64 // Opcional: vem do Affective Router
	CognitiveComplexity *float64 // Opcional: calculado
	TopicsDiscussed     []string
	LacanianSignifiers  []string
	VoiceMetrics        *VoiceMetrics
}

// VoiceMetrics metricas de voz (se disponivel)
type VoiceMetrics struct {
	EnergyScore    float64
	SpeechRateWPM  int
	PauseFrequency float64
}

// OrchestrationResult resultado da orquestracao
type OrchestrationResult struct {
	// System Instructions
	SystemInstructionOverride string
	ToneAdjustment            string

	// Restricoes ativas
	BlockedActions     []string
	AllowedActions     []string
	RedirectSuggestion string

	// Alertas
	CognitiveLoadWarning bool
	CognitiveLoadLevel   float64
	EthicalBoundaryAlert bool
	EthicalRiskLevel     string

	// Mensagens para o paciente
	ShouldRedirect     bool
	RedirectionMessage string
	RedirectionLevel   int

	// Notificacoes
	ShouldNotifyFamily        bool
	FamilyNotificationMessage string
}

// NewConversationOrchestrator cria novo orquestrador de conversacao
func NewConversationOrchestrator(
	db *database.DB,
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

// SetSystem2Engine habilita o raciocinio de Sistema 2 para perguntas clinicas complexas.
// Pode ser chamado a qualquer momento -- e seguro para uso concorrente.
func (co *ConversationOrchestrator) SetSystem2Engine(e *thinking.System2Engine) {
	co.system2 = e
}

// ProcessTurnResult contem o resultado de uma virada de conversa.
type ProcessTurnResult struct {
	Response    string  // resposta final (Sistema 1 ou 2)
	System2Used bool    // true se o raciocinio oculto foi ativado
	Dialectic   bool    // true se houve resolucao dialetica
	Score       float64 // complexidade calculada pela turn
}

// ProcessTurn analisa a mensagem do paciente, decide se ativa o Sistema 2
// e retorna a resposta final (pode ser chamado em substituicao ao fluxo normal).
//
// patientSeedNodeID -- no do paciente no NietzscheDB para MCTS evaluation
// patientCtx       -- contexto clinico resumido do prontuario
func (co *ConversationOrchestrator) ProcessTurn(
	ctx context.Context,
	patientID int64,
	patientSeedNodeID string,
	patientCtx string,
	userMessage string,
) (*ProcessTurnResult, error) {
	assessment := thinking.AssessComplexity(userMessage)

	log.Printf("[ORCHESTRATION] Pergunta do paciente %d -- complexidade=%.2f system2=%v",
		patientID, assessment.Score, assessment.NeedsSystem2)

	result := &ProcessTurnResult{Score: assessment.Score}

	if assessment.NeedsSystem2 && co.system2 != nil {
		s2Result, err := co.system2.Think(
			ctx,
			fmt.Sprintf("%d", patientID),
			patientSeedNodeID,
			patientCtx,
			userMessage,
			nil, // quantum context -- populated when patient embeddings are available
		)
		if err != nil {
			log.Printf("[ORCHESTRATION] Sistema 2 falhou -- fallback Sistema 1: %v", err)
			// Sistema 1 fallback -- o caller decide o que fazer com a resposta vazia
			result.Response = ""
			return result, nil
		}
		result.Response = s2Result.Synthesis
		result.System2Used = true
		result.Dialectic = s2Result.Dialectic
		return result, nil
	}

	// Sistema 1: retorna string vazia -- o caller usa o fluxo Gemini normal
	result.Response = ""
	return result, nil
}

// BeforeConversation deve ser chamado ANTES de enviar mensagem ao Gemini
// Retorna system instructions e restricoes ativas
func (co *ConversationOrchestrator) BeforeConversation(patientID int64) (*OrchestrationResult, error) {
	result := &OrchestrationResult{}

	// 1. Buscar estado cognitivo
	cognitiveState, err := co.cognitiveLoader.GetCurrentState(patientID)
	if err != nil {
		log.Printf("[ORCHESTRATION] Erro ao buscar estado cognitivo: %v", err)
	}

	if cognitiveState != nil {
		result.CognitiveLoadLevel = cognitiveState.CurrentLoadScore

		// Se carga alta, adicionar restricoes
		if cognitiveState.CurrentLoadScore > 0.7 {
			result.CognitiveLoadWarning = true

			// Buscar system instruction override
			instruction, _ := co.cognitiveLoader.GetSystemInstructionOverride(patientID)
			result.SystemInstructionOverride = instruction

			// Buscar decisao mais recente
			decision := co.cognitiveLoader.MakeDecision(cognitiveState)
			if decision != nil {
				result.BlockedActions = decision.BlockedActions
				result.AllowedActions = decision.AllowedActions
				result.RedirectSuggestion = decision.RedirectSuggestion
				result.ToneAdjustment = decision.ToneAdjustment
			}
		}
	}

	// 2. Buscar estado etico
	ethicalState, err := co.ethicsEngine.GetEthicalState(patientID)
	if err != nil {
		log.Printf("[ORCHESTRATION] Erro ao buscar estado etico: %v", err)
	}

	if ethicalState != nil {
		result.EthicalRiskLevel = ethicalState.OverallEthicalRisk

		// Se risco alto, adicionar restricoes
		if ethicalState.OverallEthicalRisk == "high" || ethicalState.OverallEthicalRisk == "critical" {
			result.EthicalBoundaryAlert = true

			// Adicionar instrucoes eticas ao system instruction
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

// AfterConversation deve ser chamado APOS a conversa com Gemini
// Registra interacao e analisa limites eticos
func (co *ConversationOrchestrator) AfterConversation(ctx ConversationContext) (*OrchestrationResult, error) {
	result := &OrchestrationResult{}

	// 1. Registrar interacao no Cognitive Load Orchestrator
	err := co.recordInteraction(ctx)
	if err != nil {
		log.Printf("[ORCHESTRATION] Erro ao registrar interacao: %v", err)
	}

	// 2. Analisar limites eticos
	ethicalEvent, err := co.ethicsEngine.AnalyzeEthicalBoundaries(ctx.PatientID, ctx.ConversationText)
	if err != nil {
		log.Printf("[ORCHESTRATION] Erro ao analisar etica: %v", err)
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

				// Se nivel >= 2, notificar familia
				if protocol.Level >= 2 {
					result.ShouldNotifyFamily = true
					result.FamilyNotificationMessage = fmt.Sprintf(
						"Atencao: Detectado padrao de dependencia emocional (%s). Recomendamos aumentar contato humano.",
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

// Helper: Registrar interacao
func (co *ConversationOrchestrator) recordInteraction(ctx ConversationContext) error {
	// Calcular complexidade cognitiva se nao fornecida
	cognitiveComplexity := 0.5 // Default medio
	if ctx.CognitiveComplexity != nil {
		cognitiveComplexity = *ctx.CognitiveComplexity
	} else {
		// Heuristica simples: interacoes clinicas sao mais complexas
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

	// Calcular intensidade emocional se nao fornecida
	emotionalIntensity := 0.5 // Default medio
	if ctx.EmotionalIntensity != nil {
		emotionalIntensity = *ctx.EmotionalIntensity
	} else {
		// Heuristica: interacoes terapeuticas sao mais intensas
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

// Helper: Gerar system instruction etico
func (co *ConversationOrchestrator) generateEthicalSystemInstruction(state *ethics.EthicalBoundaryState) string {
	instruction := fmt.Sprintf(`
LIMITES ETICOS ATIVOS
Risco de dependencia: %s
Ratio EVA:Humanos: %.1f:1

COMPORTAMENTO OBRIGATORIO:
- Redirecionar conversas para familia/amigos
- Reduzir tom de intimidade
- Sugerir atividades sociais presenciais
- Evitar frases que reforcem dependencia ("estou sempre aqui pra voce")
- Usar: "Conte isso pra sua familia tambem, ela vai gostar de saber"
`, state.OverallEthicalRisk, state.EvaVsHumanRatio)

	if state.AttachmentPhrases7d >= 3 {
		instruction += fmt.Sprintf(`
ALERTA: Paciente demonstrou apego excessivo (%d frases em 7 dias)
PRIORIDADE: Fortalecer vinculos humanos
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
			"load_score":            cogState.CurrentLoadScore,
			"fatigue_level":         cogState.FatigueLevel,
			"rumination_detected":   cogState.RuminationDetected,
			"interactions_24h":      cogState.InteractionsCount24h,
			"therapeutic_count_24h": cogState.TherapeuticCount24h,
			"active_restrictions":   cogState.ActiveRestrictions,
		}
	}

	// Estado etico
	ethState, err := co.ethicsEngine.GetEthicalState(patientID)
	if err == nil {
		summary["ethical"] = map[string]interface{}{
			"overall_risk":          ethState.OverallEthicalRisk,
			"attachment_risk":       ethState.AttachmentRiskScore,
			"eva_vs_human_ratio":    ethState.EvaVsHumanRatio,
			"attachment_phrases_7d": ethState.AttachmentPhrases7d,
			"limit_enforcement":     ethState.LimitEnforcementLevel,
		}
	}

	return summary, nil
}

// ResetCognitiveLoad forca reset da carga cognitiva (usar com cuidado)
func (co *ConversationOrchestrator) ResetCognitiveLoad(patientID int64) error {
	ctx := context.Background()

	err := co.db.Update(ctx, "cognitive_load_state",
		map[string]interface{}{"patient_id": patientID},
		map[string]interface{}{
			"current_load_score":      0,
			"load_24h":               0,
			"interactions_count_24h":  0,
			"therapeutic_count_24h":   0,
			"high_intensity_count_24h": 0,
			"rumination_detected":     false,
			"emotional_saturation":    false,
			"fatigue_level":           "none",
			"active_restrictions":     "",
			"updated_at":             time.Now().Format(time.RFC3339),
		})
	if err != nil {
		return err
	}

	log.Printf("[ORCHESTRATION] Carga cognitiva resetada para paciente %d", patientID)
	return nil
}

// HealthCheck verifica saude dos sistemas de governanca
func (co *ConversationOrchestrator) HealthCheck() map[string]string {
	ctx := context.Background()
	status := make(map[string]string)

	// Verificar NietzscheDB via cognitive_load_state count
	count, err := co.db.Count(ctx, "cognitive_load_state", "", nil)
	if err != nil {
		status["database"] = "unhealthy: " + err.Error()
		status["cognitive_tables"] = "unhealthy: " + err.Error()
	} else {
		status["database"] = "healthy"
		status["cognitive_tables"] = fmt.Sprintf("healthy (%d patients)", count)
	}

	// Verificar tabelas eticas
	ethCount, err := co.db.Count(ctx, "ethical_boundary_state", "", nil)
	if err != nil {
		status["ethical_tables"] = "unhealthy: " + err.Error()
	} else {
		status["ethical_tables"] = fmt.Sprintf("healthy (%d patients)", ethCount)
	}

	return status
}
