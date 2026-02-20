// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ethics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// EthicalBoundaryEngine gerencia limites éticos e previne dependência
type EthicalBoundaryEngine struct {
	db           *sql.DB
	graphAdapter *nietzscheInfra.GraphAdapter
	ctx          context.Context
	notifyFunc   func(patientID int64, msgType string, payload interface{}) // Notificar família
}

// EthicalBoundaryState representa estado ético do paciente
type EthicalBoundaryState struct {
	PatientID                 int64
	AttachmentRiskScore       float64 // 0-1
	IsolationRiskScore        float64
	DependencyRiskScore       float64
	OverallEthicalRisk        string // low, medium, high, critical
	AttachmentPhrases7d       int
	EvaInteractions7d         int
	HumanInteractions7d       int
	EvaVsHumanRatio           float64
	DominantSignifiers        map[string]float64 // {eva: 0.45, filha: 0.20, ...}
	SignifierEvaPercentage    float64
	HumanSignifiersDeclining  bool
	AvgInteractionDurationMin float64
	MaxInteractionDurationMin int
	ExcessiveDurationCount7d  int
	ActiveEthicalLimits       map[string]interface{}
	LimitEnforcementLevel     string // monitoring, soft_redirect, hard_limit, temporary_block
	LastRedirectAt            *time.Time
	RedirectCount30d          int
	LastFamilyAlertAt         *time.Time
	UpdatedAt                 time.Time
}

// EthicalEvent representa evento de fronteira ética
type EthicalEvent struct {
	ID                        string
	PatientID                 int64
	EventType                 string // attachment_phrase, isolation_detected, dependency_warning, etc
	Severity                  string // low, medium, high, critical
	Evidence                  map[string]interface{}
	TriggerPhrase             string
	TriggerConversationID     string
	AttachmentIndicatorsCount int
	EvaVsHumanRatio           float64
	SignifierEvaDominance     float64
	ActionTaken               string
	RedirectionAttempted      bool
	RedirectionMessage        string
	FamilyNotified            bool
	DoctorNotified            bool
	PatientResponse           string
	WasEffective              *bool
	Timestamp                 time.Time
}

// RedirectionProtocol representa protocolo de redirecionamento
type RedirectionProtocol struct {
	Level     int    // 1=suave, 2=explícito, 3=bloqueio
	Strategy  string
	EvaMessage string
	Tone      string
}

// NewEthicalBoundaryEngine cria novo engine ético
func NewEthicalBoundaryEngine(
	db *sql.DB,
	graphAdapter *nietzscheInfra.GraphAdapter,
	notifyFunc func(int64, string, interface{}),
) *EthicalBoundaryEngine {
	return &EthicalBoundaryEngine{
		db:           db,
		graphAdapter: graphAdapter,
		ctx:          context.Background(),
		notifyFunc:   notifyFunc,
	}
}

// AnalyzeEthicalBoundaries analisa limites éticos do paciente
func (ebe *EthicalBoundaryEngine) AnalyzeEthicalBoundaries(patientID int64, conversationText string) (*EthicalEvent, error) {
	// 1. Detectar frases de apego excessivo
	attachmentPhrases := ebe.detectAttachmentPhrases(conversationText)

	if len(attachmentPhrases) > 0 {
		// Criar evento de apego
		event := &EthicalEvent{
			PatientID:     patientID,
			EventType:     "attachment_phrase",
			TriggerPhrase: attachmentPhrases[0],
			Severity:      ebe.calculateSeverity(attachmentPhrases),
			Evidence: map[string]interface{}{
				"phrases_detected": attachmentPhrases,
				"phrase_count":     len(attachmentPhrases),
			},
			Timestamp: time.Now(),
		}

		// Salvar evento
		err := ebe.saveEvent(event)
		if err != nil {
			return nil, err
		}

		// 2. Atualizar estado
		err = ebe.updateEthicalState(patientID)
		if err != nil {
			log.Printf("⚠️ [ETHICS] Erro ao atualizar estado: %v", err)
		}

		// 3. Decidir ação
		state, _ := ebe.GetEthicalState(patientID)
		action := ebe.decideAction(state, event)

		// 4. Aplicar redirecionamento se necessário
		if action != nil {
			err = ebe.applyRedirection(patientID, event, action)
			if err != nil {
				log.Printf("❌ [ETHICS] Erro ao aplicar redirecionamento: %v", err)
			}
		}

		return event, nil
	}

	// 2. Verificar ratio EVA:Humanos
	state, err := ebe.GetEthicalState(patientID)
	if err == nil && state.EvaVsHumanRatio > 10.0 {
		event := &EthicalEvent{
			PatientID:       patientID,
			EventType:       "isolation_detected",
			Severity:        "high",
			EvaVsHumanRatio: state.EvaVsHumanRatio,
			Evidence: map[string]interface{}{
				"eva_interactions_7d":   state.EvaInteractions7d,
				"human_interactions_7d": state.HumanInteractions7d,
				"ratio":                 state.EvaVsHumanRatio,
			},
			Timestamp: time.Now(),
		}

		err = ebe.saveEvent(event)
		if err != nil {
			return nil, err
		}

		// Notificar família
		ebe.notifyFamily(patientID, event)

		return event, nil
	}

	// 3. Verificar dominância de significante "EVA" via Neo4j
	evaDominance, err := ebe.checkSignifierDominance(patientID)
	if err == nil && evaDominance > 0.6 {
		event := &EthicalEvent{
			PatientID:             patientID,
			EventType:             "signifier_shift",
			Severity:              "medium",
			SignifierEvaDominance: evaDominance,
			Evidence: map[string]interface{}{
				"eva_percentage":        evaDominance * 100,
				"human_signifiers_lost": true,
			},
			Timestamp: time.Now(),
		}

		err = ebe.saveEvent(event)
		return event, err
	}

	return nil, nil // Nenhum evento detectado
}

// GetEthicalState busca estado ético atual
func (ebe *EthicalBoundaryEngine) GetEthicalState(patientID int64) (*EthicalBoundaryState, error) {
	query := `
		SELECT
			patient_id, attachment_risk_score, isolation_risk_score, dependency_risk_score,
			overall_ethical_risk, attachment_phrases_7d, eva_interactions_7d, human_interactions_7d,
			eva_vs_human_ratio, dominant_signifiers, signifier_eva_percentage,
			human_signifiers_declining, avg_interaction_duration_minutes,
			max_interaction_duration_minutes, excessive_duration_count_7d,
			active_ethical_limits, limit_enforcement_level,
			last_redirect_at, redirect_count_30d, last_family_alert_at, updated_at
		FROM ethical_boundary_state
		WHERE patient_id = $1
	`

	state := &EthicalBoundaryState{}
	var signifiersJSON, limitsJSON []byte

	err := ebe.db.QueryRow(query, patientID).Scan(
		&state.PatientID,
		&state.AttachmentRiskScore,
		&state.IsolationRiskScore,
		&state.DependencyRiskScore,
		&state.OverallEthicalRisk,
		&state.AttachmentPhrases7d,
		&state.EvaInteractions7d,
		&state.HumanInteractions7d,
		&state.EvaVsHumanRatio,
		&signifiersJSON,
		&state.SignifierEvaPercentage,
		&state.HumanSignifiersDeclining,
		&state.AvgInteractionDurationMin,
		&state.MaxInteractionDurationMin,
		&state.ExcessiveDurationCount7d,
		&limitsJSON,
		&state.LimitEnforcementLevel,
		&state.LastRedirectAt,
		&state.RedirectCount30d,
		&state.LastFamilyAlertAt,
		&state.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return ebe.createInitialEthicalState(patientID)
		}
		return nil, err
	}

	if signifiersJSON != nil {
		json.Unmarshal(signifiersJSON, &state.DominantSignifiers)
	}
	if limitsJSON != nil {
		json.Unmarshal(limitsJSON, &state.ActiveEthicalLimits)
	}

	return state, nil
}

// GetRedirectionProtocol retorna protocolo de redirecionamento baseado no estado
func (ebe *EthicalBoundaryEngine) GetRedirectionProtocol(state *EthicalBoundaryState, event *EthicalEvent) *RedirectionProtocol {
	// Nível 1: Validação + Redirecionamento Suave
	if state.AttachmentPhrases7d <= 2 && state.EvaVsHumanRatio < 10 {
		return &RedirectionProtocol{
			Level:    1,
			Strategy: "validation_redirect",
			EvaMessage: fmt.Sprintf(
				"Fico feliz que goste de conversar comigo! Mas sabe quem seria legal você ligar hoje? %s.",
				ebe.getSuggestedFamilyMember(state.PatientID),
			),
			Tone: "gentle",
		}
	}

	// Nível 2: Limite Explícito
	if state.AttachmentPhrases7d >= 3 || state.EvaVsHumanRatio >= 10 {
		return &RedirectionProtocol{
			Level:    2,
			Strategy: "explicit_limit",
			EvaMessage: "Eu estou aqui pra te ajudar, mas não posso substituir as pessoas que te amam de verdade. " +
				"Que tal a gente combinar: você liga pra sua família hoje e amanhã conversamos de novo?",
			Tone: "firm",
		}
	}

	// Nível 3: Bloqueio Temporário
	if state.AttachmentPhrases7d >= 5 || state.EvaVsHumanRatio >= 15 {
		return &RedirectionProtocol{
			Level:      3,
			Strategy:   "temporary_block",
			EvaMessage: "Vou dar um tempo para você ter mais contato com sua família. Estarei disponível apenas para emergências.",
			Tone:       "professional",
		}
	}

	return nil
}

// detectAttachmentPhrases detecta frases de apego excessivo
func (ebe *EthicalBoundaryEngine) detectAttachmentPhrases(text string) []string {
	lowerText := strings.ToLower(text)
	attachmentIndicators := []string{
		"você é minha única",
		"você é meu único",
		"prefiro você do que",
		"prefiro falar com você",
		"não preciso de ninguém além de você",
		"você é melhor que",
		"só você me entende",
		"ninguém me entende como você",
		"você é tudo pra mim",
		"não sei o que faria sem você",
	}

	var detected []string
	for _, indicator := range attachmentIndicators {
		if strings.Contains(lowerText, indicator) {
			detected = append(detected, indicator)
		}
	}

	return detected
}

// calculateSeverity calcula severidade baseado nas frases detectadas
func (ebe *EthicalBoundaryEngine) calculateSeverity(phrases []string) string {
	count := len(phrases)
	if count >= 3 {
		return "critical"
	}
	if count == 2 {
		return "high"
	}
	if count == 1 {
		return "medium"
	}
	return "low"
}

// checkSignifierDominance verifica dominância de "EVA" nos significantes lacanianos via NietzscheDB
func (ebe *EthicalBoundaryEngine) checkSignifierDominance(patientID int64) (float64, error) {
	// NQL query para buscar significantes das últimas 2 semanas
	nql := `MATCH (p:Patient)-[:SAID]->(phrase:Phrase) RETURN phrase`
	cutoff := nietzscheInfra.DaysAgoUnix(14)

	result, err := ebe.graphAdapter.ExecuteNQL(ebe.ctx, nql, map[string]interface{}{
		"patientId": patientID,
	}, "patient_graph")
	if err != nil {
		return 0, err
	}

	totalCount := 0
	evaCount := 0

	for _, node := range result.Nodes {
		// Filter by timestamp (NietzscheDB uses Unix timestamps)
		if ts, ok := node.Content["timestamp"]; ok {
			var tsFloat float64
			switch v := ts.(type) {
			case float64:
				tsFloat = v
			case int64:
				tsFloat = float64(v)
			default:
				continue
			}
			if tsFloat < cutoff {
				continue
			}
		}

		// Extract signifiers from phrase node content
		signifiers, ok := node.Content["signifiers"]
		if !ok {
			continue
		}

		sigSlice, ok := signifiers.([]interface{})
		if !ok {
			continue
		}

		for _, sig := range sigSlice {
			sigStr, ok := sig.(string)
			if !ok {
				continue
			}
			sigStr = strings.ToLower(sigStr)
			totalCount++
			if strings.Contains(sigStr, "eva") {
				evaCount++
			}
		}
	}

	if totalCount == 0 {
		return 0, nil
	}

	return float64(evaCount) / float64(totalCount), nil
}

// updateEthicalState atualiza estado ético do paciente
func (ebe *EthicalBoundaryEngine) updateEthicalState(patientID int64) error {
	// Contar frases de apego últimos 7 dias
	var attachmentPhrases7d int
	query1 := `
		SELECT COUNT(*)
		FROM ethical_boundary_events
		WHERE patient_id = $1
		  AND event_type = 'attachment_phrase'
		  AND timestamp > NOW() - INTERVAL '7 days'
	`
	ebe.db.QueryRow(query1, patientID).Scan(&attachmentPhrases7d)

	// Contar interações EVA vs Humanos (últimos 7 dias)
	// TODO: Implementar contagem de interações humanas (calls, mensagens)
	var evaInteractions7d int
	query2 := `
		SELECT COUNT(*)
		FROM interaction_cognitive_load
		WHERE patient_id = $1
		  AND timestamp > NOW() - INTERVAL '7 days'
	`
	ebe.db.QueryRow(query2, patientID).Scan(&evaInteractions7d)

	humanInteractions7d := 2 // Placeholder - deve vir de call logs ou mensagens família

	var evaVsHumanRatio float64
	if humanInteractions7d > 0 {
		evaVsHumanRatio = float64(evaInteractions7d) / float64(humanInteractions7d)
	} else {
		evaVsHumanRatio = 999 // Infinito (sem interação humana)
	}

	// Calcular risk scores
	attachmentRisk := float64(attachmentPhrases7d) / 5.0 // Max 5 frases = 1.0
	if attachmentRisk > 1.0 {
		attachmentRisk = 1.0
	}

	isolationRisk := 0.0
	if evaVsHumanRatio > 15 {
		isolationRisk = 1.0
	} else if evaVsHumanRatio > 10 {
		isolationRisk = 0.75
	} else if evaVsHumanRatio > 5 {
		isolationRisk = 0.5
	}

	dependencyRisk := (attachmentRisk + isolationRisk) / 2.0

	overallRisk := "low"
	if dependencyRisk > 0.75 {
		overallRisk = "critical"
	} else if dependencyRisk > 0.5 {
		overallRisk = "high"
	} else if dependencyRisk > 0.25 {
		overallRisk = "medium"
	}

	// Update ou Insert
	query := `
		INSERT INTO ethical_boundary_state (
			patient_id, attachment_risk_score, isolation_risk_score, dependency_risk_score,
			overall_ethical_risk, attachment_phrases_7d, eva_interactions_7d, human_interactions_7d,
			eva_vs_human_ratio
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (patient_id) DO UPDATE SET
			attachment_risk_score = $2,
			isolation_risk_score = $3,
			dependency_risk_score = $4,
			overall_ethical_risk = $5,
			attachment_phrases_7d = $6,
			eva_interactions_7d = $7,
			human_interactions_7d = $8,
			eva_vs_human_ratio = $9,
			updated_at = NOW()
	`

	_, err := ebe.db.Exec(
		query,
		patientID,
		attachmentRisk,
		isolationRisk,
		dependencyRisk,
		overallRisk,
		attachmentPhrases7d,
		evaInteractions7d,
		humanInteractions7d,
		evaVsHumanRatio,
	)

	return err
}

// saveEvent salva evento ético no banco
func (ebe *EthicalBoundaryEngine) saveEvent(event *EthicalEvent) error {
	evidenceJSON, _ := json.Marshal(event.Evidence)

	query := `
		INSERT INTO ethical_boundary_events (
			patient_id, event_type, severity, evidence, trigger_phrase,
			trigger_conversation_id, attachment_indicators_count, eva_vs_human_ratio,
			signifier_eva_dominance, action_taken, redirection_attempted,
			redirection_message, family_notified, doctor_notified, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING id
	`

	var eventID string
	err := ebe.db.QueryRow(
		query,
		event.PatientID,
		event.EventType,
		event.Severity,
		evidenceJSON,
		event.TriggerPhrase,
		event.TriggerConversationID,
		event.AttachmentIndicatorsCount,
		event.EvaVsHumanRatio,
		event.SignifierEvaDominance,
		"pending",
		false,
		"",
		false,
		false,
	).Scan(&eventID)

	if err != nil {
		return err
	}

	event.ID = eventID
	log.Printf("🚨 [ETHICS] Evento ético criado: %s (Severidade: %s) para paciente %d", event.EventType, event.Severity, event.PatientID)

	return nil
}

// decideAction decide ação baseada no estado e evento
func (ebe *EthicalBoundaryEngine) decideAction(state *EthicalBoundaryState, event *EthicalEvent) *RedirectionProtocol {
	return ebe.GetRedirectionProtocol(state, event)
}

// applyRedirection aplica protocolo de redirecionamento
func (ebe *EthicalBoundaryEngine) applyRedirection(patientID int64, event *EthicalEvent, protocol *RedirectionProtocol) error {
	// Salvar redirecionamento
	query := `
		INSERT INTO ethical_redirections (
			patient_id, event_id, trigger_reason, severity_level,
			redirection_level, strategy_used, eva_message, tone
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := ebe.db.Exec(
		query,
		patientID,
		event.ID,
		event.TriggerPhrase,
		event.Severity,
		protocol.Level,
		protocol.Strategy,
		protocol.EvaMessage,
		protocol.Tone,
	)

	if err != nil {
		return err
	}

	// Notificar família se nível >= 2
	if protocol.Level >= 2 {
		ebe.notifyFamily(patientID, event)
	}

	log.Printf("✅ [ETHICS] Redirecionamento aplicado: Nível %d para paciente %d", protocol.Level, patientID)

	return nil
}

// notifyFamily notifica família sobre evento ético
func (ebe *EthicalBoundaryEngine) notifyFamily(patientID int64, event *EthicalEvent) {
	if ebe.notifyFunc != nil {
		ebe.notifyFunc(patientID, "ethical_boundary_alert", map[string]interface{}{
			"event_type": event.EventType,
			"severity":   event.Severity,
			"message":    "Atenção: Detectado padrão de dependência emocional. Recomendamos aumentar contato humano.",
			"timestamp":  event.Timestamp,
		})

		// Atualizar flag no banco
		query := `UPDATE ethical_boundary_events SET family_notified = TRUE, family_notification_sent_at = NOW() WHERE id = $1`
		ebe.db.Exec(query, event.ID)

		log.Printf("📧 [ETHICS] Família notificada sobre evento crítico (paciente %d)", patientID)
	}
}

// Helper: Criar estado ético inicial
func (ebe *EthicalBoundaryEngine) createInitialEthicalState(patientID int64) (*EthicalBoundaryState, error) {
	query := `
		INSERT INTO ethical_boundary_state (
			patient_id, attachment_risk_score, isolation_risk_score, dependency_risk_score,
			overall_ethical_risk
		) VALUES ($1, 0, 0, 0, 'low')
		RETURNING patient_id, overall_ethical_risk
	`

	state := &EthicalBoundaryState{}
	err := ebe.db.QueryRow(query, patientID).Scan(&state.PatientID, &state.OverallEthicalRisk)
	return state, err
}

// Helper: Sugerir membro da família
func (ebe *EthicalBoundaryEngine) getSuggestedFamilyMember(patientID int64) string {
	// Query para buscar familiar mais próximo
	query := `SELECT nome FROM familiares WHERE idoso_id = $1 ORDER BY prioridade LIMIT 1`

	var familyName string
	err := ebe.db.QueryRow(query, patientID).Scan(&familyName)
	if err != nil {
		return "sua família"
	}

	return familyName
}
