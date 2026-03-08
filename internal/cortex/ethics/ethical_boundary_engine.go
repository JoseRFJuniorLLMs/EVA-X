// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ethics

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

// EthicalBoundaryEngine gerencia limites eticos e previne dependencia
type EthicalBoundaryEngine struct {
	db           *database.DB
	graphAdapter *nietzscheInfra.GraphAdapter
	ctx          context.Context
	notifyFunc   func(patientID int64, msgType string, payload interface{}) // Notificar familia
}

// EthicalBoundaryState representa estado etico do paciente
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

// EthicalEvent representa evento de fronteira etica
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
	Level      int    // 1=suave, 2=explicito, 3=bloqueio
	Strategy   string
	EvaMessage string
	Tone       string
}

// NewEthicalBoundaryEngine cria novo engine etico
func NewEthicalBoundaryEngine(
	db *database.DB,
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

// AnalyzeEthicalBoundaries analisa limites eticos do paciente
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
			log.Printf("[ETHICS] Erro ao atualizar estado: %v", err)
		}

		// 3. Decidir acao
		state, _ := ebe.GetEthicalState(patientID)
		action := ebe.decideAction(state, event)

		// 4. Aplicar redirecionamento se necessario
		if action != nil {
			err = ebe.applyRedirection(patientID, event, action)
			if err != nil {
				log.Printf("[ETHICS] Erro ao aplicar redirecionamento: %v", err)
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

		// Notificar familia
		ebe.notifyFamily(patientID, event)

		return event, nil
	}

	// 3. Verificar dominancia de significante "EVA" via NietzscheDB graph
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

// GetEthicalState busca estado etico atual
func (ebe *EthicalBoundaryEngine) GetEthicalState(patientID int64) (*EthicalBoundaryState, error) {
	rows, err := ebe.db.QueryByLabel(ebe.ctx, "ethical_boundary_state",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return ebe.createInitialEthicalState(patientID)
	}

	m := rows[0]
	state := &EthicalBoundaryState{
		PatientID:                 database.GetInt64(m, "patient_id"),
		AttachmentRiskScore:       database.GetFloat64(m, "attachment_risk_score"),
		IsolationRiskScore:        database.GetFloat64(m, "isolation_risk_score"),
		DependencyRiskScore:       database.GetFloat64(m, "dependency_risk_score"),
		OverallEthicalRisk:        database.GetString(m, "overall_ethical_risk"),
		AttachmentPhrases7d:       int(database.GetInt64(m, "attachment_phrases_7d")),
		EvaInteractions7d:         int(database.GetInt64(m, "eva_interactions_7d")),
		HumanInteractions7d:       int(database.GetInt64(m, "human_interactions_7d")),
		EvaVsHumanRatio:           database.GetFloat64(m, "eva_vs_human_ratio"),
		SignifierEvaPercentage:    database.GetFloat64(m, "signifier_eva_percentage"),
		HumanSignifiersDeclining:  database.GetBool(m, "human_signifiers_declining"),
		AvgInteractionDurationMin: database.GetFloat64(m, "avg_interaction_duration_minutes"),
		MaxInteractionDurationMin: int(database.GetInt64(m, "max_interaction_duration_minutes")),
		ExcessiveDurationCount7d:  int(database.GetInt64(m, "excessive_duration_count_7d")),
		LimitEnforcementLevel:     database.GetString(m, "limit_enforcement_level"),
		LastRedirectAt:            database.GetTimePtr(m, "last_redirect_at"),
		RedirectCount30d:          int(database.GetInt64(m, "redirect_count_30d")),
		LastFamilyAlertAt:         database.GetTimePtr(m, "last_family_alert_at"),
		UpdatedAt:                 database.GetTime(m, "updated_at"),
	}

	// Unmarshal JSON fields
	if sigJSON := database.GetString(m, "dominant_signifiers"); sigJSON != "" {
		json.Unmarshal([]byte(sigJSON), &state.DominantSignifiers)
	}
	if limitsJSON := database.GetString(m, "active_ethical_limits"); limitsJSON != "" {
		json.Unmarshal([]byte(limitsJSON), &state.ActiveEthicalLimits)
	}

	return state, nil
}

// GetRedirectionProtocol retorna protocolo de redirecionamento baseado no estado
func (ebe *EthicalBoundaryEngine) GetRedirectionProtocol(state *EthicalBoundaryState, event *EthicalEvent) *RedirectionProtocol {
	// Nivel 1: Validacao + Redirecionamento Suave
	if state.AttachmentPhrases7d <= 2 && state.EvaVsHumanRatio < 10 {
		return &RedirectionProtocol{
			Level:    1,
			Strategy: "validation_redirect",
			EvaMessage: fmt.Sprintf(
				"Fico feliz que goste de conversar comigo! Mas sabe quem seria legal voce ligar hoje? %s.",
				ebe.getSuggestedFamilyMember(state.PatientID),
			),
			Tone: "gentle",
		}
	}

	// Nivel 2: Limite Explicito
	if state.AttachmentPhrases7d >= 3 || state.EvaVsHumanRatio >= 10 {
		return &RedirectionProtocol{
			Level:    2,
			Strategy: "explicit_limit",
			EvaMessage: "Eu estou aqui pra te ajudar, mas nao posso substituir as pessoas que te amam de verdade. " +
				"Que tal a gente combinar: voce liga pra sua familia hoje e amanha conversamos de novo?",
			Tone: "firm",
		}
	}

	// Nivel 3: Bloqueio Temporario
	if state.AttachmentPhrases7d >= 5 || state.EvaVsHumanRatio >= 15 {
		return &RedirectionProtocol{
			Level:      3,
			Strategy:   "temporary_block",
			EvaMessage: "Vou dar um tempo para voce ter mais contato com sua familia. Estarei disponivel apenas para emergencias.",
			Tone:       "professional",
		}
	}

	return nil
}

// detectAttachmentPhrases detecta frases de apego excessivo
func (ebe *EthicalBoundaryEngine) detectAttachmentPhrases(text string) []string {
	lowerText := strings.ToLower(text)
	attachmentIndicators := []string{
		"voce e minha unica",
		"voce e meu unico",
		"prefiro voce do que",
		"prefiro falar com voce",
		"nao preciso de ninguem alem de voce",
		"voce e melhor que",
		"so voce me entende",
		"ninguem me entende como voce",
		"voce e tudo pra mim",
		"nao sei o que faria sem voce",
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

// checkSignifierDominance verifica dominancia de "EVA" nos significantes lacanianos via NietzscheDB
func (ebe *EthicalBoundaryEngine) checkSignifierDominance(patientID int64) (float64, error) {
	// NQL query para buscar significantes das ultimas 2 semanas
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

// updateEthicalState atualiza estado etico do paciente
func (ebe *EthicalBoundaryEngine) updateEthicalState(patientID int64) error {
	// Contar frases de apego ultimos 7 dias
	// Query NietzscheDB for attachment events in last 7 days
	cutoff7d := time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339)
	attachmentPhrases7d, _ := ebe.db.Count(ebe.ctx, "ethical_boundary_events",
		" AND n.patient_id = $pid AND n.event_type = $etype AND n.timestamp > $cutoff",
		map[string]interface{}{
			"pid":    patientID,
			"etype":  "attachment_phrase",
			"cutoff": cutoff7d,
		},
	)

	// Contar interacoes EVA vs Humanos (ultimos 7 dias)
	evaInteractions7d, _ := ebe.db.Count(ebe.ctx, "interaction_cognitive_load",
		" AND n.patient_id = $pid AND n.timestamp > $cutoff",
		map[string]interface{}{
			"pid":    patientID,
			"cutoff": cutoff7d,
		},
	)

	humanInteractions7d := 2 // Placeholder - deve vir de call logs ou mensagens familia

	var evaVsHumanRatio float64
	if humanInteractions7d > 0 {
		evaVsHumanRatio = float64(evaInteractions7d) / float64(humanInteractions7d)
	} else {
		evaVsHumanRatio = 999 // Infinito (sem interacao humana)
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

	// Upsert via Update (matchKeys on patient_id); if not found, Insert
	updates := map[string]interface{}{
		"attachment_risk_score":  attachmentRisk,
		"isolation_risk_score":   isolationRisk,
		"dependency_risk_score":  dependencyRisk,
		"overall_ethical_risk":   overallRisk,
		"attachment_phrases_7d":  attachmentPhrases7d,
		"eva_interactions_7d":    evaInteractions7d,
		"human_interactions_7d":  humanInteractions7d,
		"eva_vs_human_ratio":     evaVsHumanRatio,
		"updated_at":             time.Now().Format(time.RFC3339),
	}

	// Try update first
	err := ebe.db.Update(ebe.ctx, "ethical_boundary_state",
		map[string]interface{}{"patient_id": patientID},
		updates,
	)
	if err != nil {
		// If update fails (node doesn't exist), insert
		updates["patient_id"] = patientID
		_, err = ebe.db.Insert(ebe.ctx, "ethical_boundary_state", updates)
	}

	return err
}

// saveEvent salva evento etico no banco
func (ebe *EthicalBoundaryEngine) saveEvent(event *EthicalEvent) error {
	evidenceJSON, _ := json.Marshal(event.Evidence)

	content := map[string]interface{}{
		"patient_id":                  event.PatientID,
		"event_type":                  event.EventType,
		"severity":                    event.Severity,
		"evidence":                    string(evidenceJSON),
		"trigger_phrase":              event.TriggerPhrase,
		"trigger_conversation_id":     event.TriggerConversationID,
		"attachment_indicators_count": event.AttachmentIndicatorsCount,
		"eva_vs_human_ratio":          event.EvaVsHumanRatio,
		"signifier_eva_dominance":     event.SignifierEvaDominance,
		"action_taken":                "pending",
		"redirection_attempted":       false,
		"redirection_message":         "",
		"family_notified":             false,
		"doctor_notified":             false,
		"timestamp":                   time.Now().Format(time.RFC3339),
	}

	eventID, err := ebe.db.Insert(ebe.ctx, "ethical_boundary_events", content)
	if err != nil {
		return err
	}

	event.ID = fmt.Sprintf("%d", eventID)
	log.Printf("[ETHICS] Evento etico criado: %s (Severidade: %s) para paciente %d", event.EventType, event.Severity, event.PatientID)

	return nil
}

// decideAction decide acao baseada no estado e evento
func (ebe *EthicalBoundaryEngine) decideAction(state *EthicalBoundaryState, event *EthicalEvent) *RedirectionProtocol {
	return ebe.GetRedirectionProtocol(state, event)
}

// applyRedirection aplica protocolo de redirecionamento
func (ebe *EthicalBoundaryEngine) applyRedirection(patientID int64, event *EthicalEvent, protocol *RedirectionProtocol) error {
	// Salvar redirecionamento
	content := map[string]interface{}{
		"patient_id":        patientID,
		"event_id":          event.ID,
		"trigger_reason":    event.TriggerPhrase,
		"severity_level":    event.Severity,
		"redirection_level": protocol.Level,
		"strategy_used":     protocol.Strategy,
		"eva_message":       protocol.EvaMessage,
		"tone":              protocol.Tone,
		"created_at":        time.Now().Format(time.RFC3339),
	}

	_, err := ebe.db.Insert(ebe.ctx, "ethical_redirections", content)
	if err != nil {
		return err
	}

	// Notificar familia se nivel >= 2
	if protocol.Level >= 2 {
		ebe.notifyFamily(patientID, event)
	}

	log.Printf("[ETHICS] Redirecionamento aplicado: Nivel %d para paciente %d", protocol.Level, patientID)

	return nil
}

// notifyFamily notifica familia sobre evento etico
func (ebe *EthicalBoundaryEngine) notifyFamily(patientID int64, event *EthicalEvent) {
	if ebe.notifyFunc != nil {
		ebe.notifyFunc(patientID, "ethical_boundary_alert", map[string]interface{}{
			"event_type": event.EventType,
			"severity":   event.Severity,
			"message":    "Atencao: Detectado padrao de dependencia emocional. Recomendamos aumentar contato humano.",
			"timestamp":  event.Timestamp,
		})

		// Atualizar flag no banco
		if event.ID != "" {
			ebe.db.Update(ebe.ctx, "ethical_boundary_events",
				map[string]interface{}{"id": event.ID},
				map[string]interface{}{
					"family_notified":              true,
					"family_notification_sent_at":  time.Now().Format(time.RFC3339),
				},
			)
		}

		log.Printf("[ETHICS] Familia notificada sobre evento critico (paciente %d)", patientID)
	}
}

// Helper: Criar estado etico inicial
func (ebe *EthicalBoundaryEngine) createInitialEthicalState(patientID int64) (*EthicalBoundaryState, error) {
	content := map[string]interface{}{
		"patient_id":             patientID,
		"attachment_risk_score":  0.0,
		"isolation_risk_score":   0.0,
		"dependency_risk_score":  0.0,
		"overall_ethical_risk":   "low",
		"created_at":             time.Now().Format(time.RFC3339),
		"updated_at":             time.Now().Format(time.RFC3339),
	}

	_, err := ebe.db.Insert(ebe.ctx, "ethical_boundary_state", content)
	if err != nil {
		return nil, err
	}

	return &EthicalBoundaryState{
		PatientID:          patientID,
		OverallEthicalRisk: "low",
	}, nil
}

// Helper: Sugerir membro da familia
func (ebe *EthicalBoundaryEngine) getSuggestedFamilyMember(patientID int64) string {
	// Query NietzscheDB for family member
	rows, err := ebe.db.QueryByLabel(ebe.ctx, "familiares",
		" AND n.idoso_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil || len(rows) == 0 {
		return "sua familia"
	}

	familyName := database.GetString(rows[0], "nome")
	if familyName == "" {
		return "sua familia"
	}

	return familyName
}
