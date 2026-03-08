// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package persona

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// PERSONA MANAGER (SPRINT 5)
// ============================================================================
// Gerencia personas configuráveis (Companion, Clinical, Emergency, Educator)

type PersonaManager struct {
	db *database.DB
}

func NewPersonaManager(db *database.DB) *PersonaManager {
	return &PersonaManager{db: db}
}

// ============================================================================
// PERSONA DEFINITION
// ============================================================================

type PersonaDefinition struct {
	ID                           string    `json:"id"`
	PersonaCode                  string    `json:"persona_code"`
	PersonaName                  string    `json:"persona_name"`
	Description                  string    `json:"description"`
	VoiceID                      string    `json:"voice_id"`
	Tone                         string    `json:"tone"`
	EmotionalDepth               float64   `json:"emotional_depth"`
	NarrativeFreedom             float64   `json:"narrative_freedom"`
	MaxSessionDurationMinutes    int       `json:"max_session_duration_minutes"`
	MaxDailyInteractions         int       `json:"max_daily_interactions"`
	MaxIntimacyLevel             float64   `json:"max_intimacy_level"`
	RequireProfessionalOversight bool      `json:"require_professional_oversight"`
	CanOverridePatientRefusal    bool      `json:"can_override_patient_refusal"`
	AllowedTools                 []string  `json:"allowed_tools"`
	ProhibitedTools              []string  `json:"prohibited_tools"`
	AllowedTopics                []string  `json:"allowed_topics"`
	ProhibitedTopics             []string  `json:"prohibited_topics"`
	SystemInstructionTemplate    string    `json:"system_instruction_template"`
	Priorities                   []string  `json:"priorities"`
	IsActive                     bool      `json:"is_active"`
	CreatedAt                    time.Time `json:"created_at"`
}

// PersonaSession representa uma sessão ativa de persona
type PersonaSession struct {
	ID                 string     `json:"id"`
	PatientID          int64      `json:"patient_id"`
	PersonaCode        string     `json:"persona_code"`
	StartedAt          time.Time  `json:"started_at"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	DurationSeconds    *int       `json:"duration_seconds,omitempty"`
	TriggerReason      string     `json:"trigger_reason"`
	TriggeredBy        string     `json:"triggered_by"`
	ToolsUsed          []string   `json:"tools_used"`
	BoundaryViolations int        `json:"boundary_violations"`
	EscalationRequired bool       `json:"escalation_required"`
	Status             string     `json:"status"`
}

// ============================================================================
// ATIVAÇÃO DE PERSONA
// ============================================================================

// ActivatePersona ativa uma persona para um paciente
func (pm *PersonaManager) ActivatePersona(
	patientID int64,
	personaCode string,
	triggerReason string,
	triggeredBy string,
) (*PersonaSession, error) {

	log.Printf("🎭 [PERSONA] Ativando persona '%s' para paciente %d", personaCode, patientID)
	ctx := context.Background()

	// 1. Verificar se persona existe
	exists, err := pm.personaExists(personaCode)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("persona '%s' não existe", personaCode)
	}

	// 2. Desativar persona anterior (se houver)
	err = pm.deactivateCurrentPersona(patientID)
	if err != nil {
		log.Printf("⚠️ [PERSONA] Erro ao desativar persona anterior: %v", err)
	}

	// 3. Criar nova sessão
	now := time.Now()
	content := map[string]interface{}{
		"patient_id":     patientID,
		"persona_code":   personaCode,
		"trigger_reason": triggerReason,
		"triggered_by":   triggeredBy,
		"status":         "active",
		"started_at":     now.Format(time.RFC3339),
	}

	newID, err := pm.db.Insert(ctx, "persona_sessions", content)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar sessão: %w", err)
	}

	session := &PersonaSession{
		ID:            fmt.Sprintf("%d", newID),
		PatientID:     patientID,
		PersonaCode:   personaCode,
		TriggerReason: triggerReason,
		TriggeredBy:   triggeredBy,
		Status:        "active",
		StartedAt:     now,
	}

	log.Printf("✅ [PERSONA] Persona '%s' ativada (session_id: %s)", personaCode, session.ID)

	return session, nil
}

// GetCurrentPersona retorna persona ativa de um paciente
func (pm *PersonaManager) GetCurrentPersona(patientID int64) (*PersonaSession, error) {
	ctx := context.Background()

	rows, err := pm.db.QueryByLabel(ctx, "persona_sessions",
		" AND n.patient_id = $pid AND n.status = $status",
		map[string]interface{}{
			"pid":    patientID,
			"status": "active",
		}, 0)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, nil // Nenhuma persona ativa
	}

	// Find the most recent session by started_at
	var latest map[string]interface{}
	var latestTime time.Time
	for _, row := range rows {
		t := database.GetTime(row, "started_at")
		if latest == nil || t.After(latestTime) {
			latest = row
			latestTime = t
		}
	}

	session := mapToSession(latest)
	return session, nil
}

// deactivateCurrentPersona desativa persona ativa atual
func (pm *PersonaManager) deactivateCurrentPersona(patientID int64) error {
	ctx := context.Background()

	// Find all active sessions for this patient
	rows, err := pm.db.QueryByLabel(ctx, "persona_sessions",
		" AND n.patient_id = $pid AND n.status = $status",
		map[string]interface{}{
			"pid":    patientID,
			"status": "active",
		}, 0)
	if err != nil {
		return err
	}

	now := time.Now().Format(time.RFC3339)
	for _, row := range rows {
		id := database.GetInt64(row, "id")
		if id == 0 {
			continue
		}
		err := pm.db.Update(ctx, "persona_sessions",
			map[string]interface{}{"id": id},
			map[string]interface{}{
				"status":   "completed",
				"ended_at": now,
			})
		if err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// PERMISSÕES DE FERRAMENTAS
// ============================================================================

// IsToolAllowed verifica se uma ferramenta é permitida para a persona atual
func (pm *PersonaManager) IsToolAllowed(patientID int64, toolName string) (bool, string) {
	// 1. Obter persona atual
	session, err := pm.GetCurrentPersona(patientID)
	if err != nil {
		return false, fmt.Sprintf("erro ao obter persona: %v", err)
	}

	if session == nil {
		// Sem persona ativa = usar default (companion)
		session = &PersonaSession{PersonaCode: "companion"}
	}

	// 2. Verificar permissão via definição da persona
	definition, err := pm.GetPersonaDefinition(session.PersonaCode)
	if err != nil {
		return false, fmt.Sprintf("erro ao verificar permissão: %v", err)
	}

	// Check prohibited tools first
	for _, t := range definition.ProhibitedTools {
		if strings.EqualFold(t, toolName) {
			return false, fmt.Sprintf("ferramenta '%s' não permitida para persona '%s'", toolName, session.PersonaCode)
		}
	}

	// If allowed_tools is defined, check if tool is in the list
	if len(definition.AllowedTools) > 0 {
		for _, t := range definition.AllowedTools {
			if strings.EqualFold(t, toolName) {
				return true, ""
			}
		}
		return false, fmt.Sprintf("ferramenta '%s' não permitida para persona '%s'", toolName, session.PersonaCode)
	}

	return true, ""
}

// RecordToolUsage registra uso de ferramenta
func (pm *PersonaManager) RecordToolUsage(patientID int64, toolName string) error {
	session, err := pm.GetCurrentPersona(patientID)
	if err != nil || session == nil {
		return fmt.Errorf("nenhuma persona ativa")
	}

	ctx := context.Background()

	// Append tool to the tools_used list
	toolsUsed := append(session.ToolsUsed, toolName)

	return pm.db.Update(ctx, "persona_sessions",
		map[string]interface{}{"id": database.GetInt64(mapFromSession(session), "id")},
		map[string]interface{}{
			"tools_used": toolsUsed,
		})
}

// ============================================================================
// VIOLAÇÕES DE LIMITES
// ============================================================================

// RecordBoundaryViolation registra violação de limite
func (pm *PersonaManager) RecordBoundaryViolation(patientID int64, violationRule string) error {
	session, err := pm.GetCurrentPersona(patientID)
	if err != nil || session == nil {
		return fmt.Errorf("nenhuma persona ativa")
	}

	ctx := context.Background()

	sessionID := sessionIDToInt64(session.ID)

	// Get current node to read existing violated_rules
	node, err := pm.db.GetNodeByID(ctx, "persona_sessions", sessionID)
	if err != nil || node == nil {
		return fmt.Errorf("sessão não encontrada")
	}

	violatedRules := getStringSlice(node, "violated_rules")
	violatedRules = append(violatedRules, violationRule)

	err = pm.db.Update(ctx, "persona_sessions",
		map[string]interface{}{"id": sessionID},
		map[string]interface{}{
			"boundary_violations": session.BoundaryViolations + 1,
			"violated_rules":      violatedRules,
		})

	if err == nil {
		log.Printf("⚠️ [PERSONA] Violação de limite registrada: %s (session: %s)", violationRule, session.ID)
	}

	return err
}

// CheckSessionLimits verifica se sessão excedeu limites
func (pm *PersonaManager) CheckSessionLimits(patientID int64) (bool, []string) {
	session, err := pm.GetCurrentPersona(patientID)
	if err != nil || session == nil {
		return false, nil
	}

	// Obter definição da persona
	definition, err := pm.GetPersonaDefinition(session.PersonaCode)
	if err != nil {
		return false, nil
	}

	violations := []string{}

	// Verificar duração da sessão
	if definition.MaxSessionDurationMinutes > 0 {
		duration := time.Since(session.StartedAt).Minutes()
		if duration > float64(definition.MaxSessionDurationMinutes) {
			violations = append(violations, fmt.Sprintf("max_session_duration (%.0f > %d min)", duration, definition.MaxSessionDurationMinutes))
		}
	}

	// Verificar interações diárias
	if definition.MaxDailyInteractions > 0 {
		count, _ := pm.countDailyInteractions(patientID)
		if count > definition.MaxDailyInteractions {
			violations = append(violations, fmt.Sprintf("max_daily_interactions (%d > %d)", count, definition.MaxDailyInteractions))
		}
	}

	return len(violations) > 0, violations
}

func (pm *PersonaManager) countDailyInteractions(patientID int64) (int, error) {
	ctx := context.Background()

	// Query all sessions for this patient, then filter by time in Go
	rows, err := pm.db.QueryByLabel(ctx, "persona_sessions",
		" AND n.patient_id = $pid",
		map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-24 * time.Hour)
	count := 0
	for _, row := range rows {
		startedAt := database.GetTime(row, "started_at")
		if startedAt.After(cutoff) {
			count++
		}
	}

	return count, nil
}

// ============================================================================
// SYSTEM INSTRUCTIONS
// ============================================================================

// GetSystemInstructions retorna System Instructions para a persona atual
func (pm *PersonaManager) GetSystemInstructions(patientID int64) (string, error) {
	// 1. Obter persona atual
	session, err := pm.GetCurrentPersona(patientID)
	if err != nil {
		return "", err
	}

	personaCode := "companion" // Default
	if session != nil {
		personaCode = session.PersonaCode
	}

	// 2. Obter definição
	definition, err := pm.GetPersonaDefinition(personaCode)
	if err != nil {
		return "", err
	}

	// 3. Retornar template (pode ser customizado dinamicamente)
	return definition.SystemInstructionTemplate, nil
}

// ============================================================================
// DEFINIÇÕES DE PERSONA
// ============================================================================

// GetPersonaDefinition retorna definição de uma persona
func (pm *PersonaManager) GetPersonaDefinition(personaCode string) (*PersonaDefinition, error) {
	ctx := context.Background()

	rows, err := pm.db.QueryByLabel(ctx, "persona_definitions",
		" AND n.persona_code = $code",
		map[string]interface{}{
			"code": personaCode,
		}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("persona '%s' não encontrada", personaCode)
	}

	m := rows[0]
	pd := &PersonaDefinition{
		ID:                           database.GetString(m, "id"),
		PersonaCode:                  database.GetString(m, "persona_code"),
		PersonaName:                  database.GetString(m, "persona_name"),
		Description:                  database.GetString(m, "description"),
		VoiceID:                      database.GetString(m, "voice_id"),
		Tone:                         database.GetString(m, "tone"),
		EmotionalDepth:               database.GetFloat64(m, "emotional_depth"),
		NarrativeFreedom:             database.GetFloat64(m, "narrative_freedom"),
		MaxSessionDurationMinutes:    int(database.GetInt64(m, "max_session_duration_minutes")),
		MaxDailyInteractions:         int(database.GetInt64(m, "max_daily_interactions")),
		MaxIntimacyLevel:             database.GetFloat64(m, "max_intimacy_level"),
		RequireProfessionalOversight: database.GetBool(m, "require_professional_oversight"),
		CanOverridePatientRefusal:    database.GetBool(m, "can_override_patient_refusal"),
		AllowedTools:                 getStringSlice(m, "allowed_tools"),
		ProhibitedTools:              getStringSlice(m, "prohibited_tools"),
		AllowedTopics:                getStringSlice(m, "allowed_topics"),
		ProhibitedTopics:             getStringSlice(m, "prohibited_topics"),
		SystemInstructionTemplate:    database.GetString(m, "system_instruction_template"),
		Priorities:                   getStringSlice(m, "priorities"),
		IsActive:                     database.GetBool(m, "is_active"),
		CreatedAt:                    database.GetTime(m, "created_at"),
	}

	return pd, nil
}

// ListAllPersonas lista todas as personas disponíveis
func (pm *PersonaManager) ListAllPersonas() ([]PersonaDefinition, error) {
	ctx := context.Background()

	rows, err := pm.db.QueryByLabel(ctx, "persona_definitions",
		" AND n.is_active = $active",
		map[string]interface{}{
			"active": true,
		}, 0)
	if err != nil {
		return nil, err
	}

	personas := []PersonaDefinition{}
	for _, m := range rows {
		pd := PersonaDefinition{
			ID:               database.GetString(m, "id"),
			PersonaCode:      database.GetString(m, "persona_code"),
			PersonaName:      database.GetString(m, "persona_name"),
			Description:      database.GetString(m, "description"),
			Tone:             database.GetString(m, "tone"),
			EmotionalDepth:   database.GetFloat64(m, "emotional_depth"),
			NarrativeFreedom: database.GetFloat64(m, "narrative_freedom"),
			IsActive:         database.GetBool(m, "is_active"),
		}
		personas = append(personas, pd)
	}

	return personas, nil
}

// ============================================================================
// REGRAS DE ATIVAÇÃO
// ============================================================================

// EvaluateActivationRules avalia regras e retorna persona que deve ser ativada
func (pm *PersonaManager) EvaluateActivationRules(patientID int64) (string, string, error) {
	ctx := context.Background()

	// Buscar estado do paciente
	patientState, err := pm.getPatientState(patientID)
	if err != nil {
		return "", "", err
	}

	// Buscar regras ativas (NQL doesn't support ORDER BY, sort in Go)
	rows, err := pm.db.QueryByLabel(ctx, "persona_activation_rules",
		" AND n.is_active = $active",
		map[string]interface{}{
			"active": true,
		}, 0)
	if err != nil {
		return "", "", err
	}

	// Sort by priority (ascending)
	sortByPriority(rows)

	for _, m := range rows {
		ruleName := database.GetString(m, "rule_name")
		targetPersona := database.GetString(m, "target_persona_code")
		conditionsRaw := database.GetString(m, "conditions")

		// Avaliar condições
		var conditions map[string]interface{}
		if conditionsRaw != "" {
			json.Unmarshal([]byte(conditionsRaw), &conditions)
		}

		if pm.evaluateConditions(conditions, patientState) {
			log.Printf("✅ [PERSONA] Regra '%s' ativada → persona '%s'", ruleName, targetPersona)
			return targetPersona, ruleName, nil
		}
	}

	// Nenhuma regra ativada = usar default (companion)
	return "companion", "default", nil
}

// evaluateConditions avalia se condições são satisfeitas
func (pm *PersonaManager) evaluateConditions(conditions map[string]interface{}, state map[string]interface{}) bool {
	for key, expectedValue := range conditions {
		actualValue, exists := state[key]
		if !exists {
			return false
		}

		// Comparação simples (pode ser expandida)
		if actualValue != expectedValue {
			return false
		}
	}

	return true
}

// getPatientState obtém estado atual do paciente
func (pm *PersonaManager) getPatientState(patientID int64) (map[string]interface{}, error) {
	ctx := context.Background()
	state := make(map[string]interface{})

	// Buscar PHQ-9 mais recente
	phq9Rows, err := pm.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.assessment_type = $atype",
		map[string]interface{}{
			"pid":   patientID,
			"atype": "PHQ-9",
		}, 0)
	if err == nil && len(phq9Rows) > 0 {
		// Find most recent by completed_at
		latest := findMostRecent(phq9Rows, "completed_at")
		if latest != nil {
			score := database.GetFloat64(latest, "total_score")
			state["phq9_score"] = score
		}
	}

	// Buscar C-SSRS
	cssrsRows, err := pm.db.QueryByLabel(ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.assessment_type = $atype",
		map[string]interface{}{
			"pid":   patientID,
			"atype": "C-SSRS",
		}, 0)
	if err == nil && len(cssrsRows) > 0 {
		latest := findMostRecent(cssrsRows, "completed_at")
		if latest != nil {
			score := database.GetInt64(latest, "total_score")
			state["cssrs_score"] = score
		}
	}

	// Verificar se houve crise recente (últimas 24 horas)
	crisisRows, err := pm.db.QueryByLabel(ctx, "crisis_events",
		" AND n.patient_id = $pid",
		map[string]interface{}{
			"pid": patientID,
		}, 0)
	crisisRecent := false
	if err == nil {
		cutoff := time.Now().Add(-24 * time.Hour)
		for _, row := range crisisRows {
			occurredAt := database.GetTime(row, "occurred_at")
			if occurredAt.After(cutoff) {
				crisisRecent = true
				break
			}
		}
	}
	state["crisis_detected"] = crisisRecent

	return state, nil
}

// ============================================================================
// UTILITÁRIOS
// ============================================================================

func (pm *PersonaManager) personaExists(personaCode string) (bool, error) {
	ctx := context.Background()

	rows, err := pm.db.QueryByLabel(ctx, "persona_definitions",
		" AND n.persona_code = $code AND n.is_active = $active",
		map[string]interface{}{
			"code":   personaCode,
			"active": true,
		}, 1)
	if err != nil {
		return false, err
	}

	return len(rows) > 0, nil
}

// mapToSession converts a NietzscheDB content map to a PersonaSession.
func mapToSession(m map[string]interface{}) *PersonaSession {
	s := &PersonaSession{
		ID:                 fmt.Sprintf("%v", database.GetInt64(m, "id")),
		PatientID:          database.GetInt64(m, "patient_id"),
		PersonaCode:        database.GetString(m, "persona_code"),
		StartedAt:          database.GetTime(m, "started_at"),
		EndedAt:            database.GetTimePtr(m, "ended_at"),
		TriggerReason:      database.GetString(m, "trigger_reason"),
		TriggeredBy:        database.GetString(m, "triggered_by"),
		ToolsUsed:          getStringSlice(m, "tools_used"),
		BoundaryViolations: int(database.GetInt64(m, "boundary_violations")),
		EscalationRequired: database.GetBool(m, "escalation_required"),
		Status:             database.GetString(m, "status"),
	}

	// DurationSeconds (nullable int)
	if v, ok := m["duration_seconds"]; ok && v != nil {
		dur := int(database.GetInt64(m, "duration_seconds"))
		s.DurationSeconds = &dur
	}

	return s
}

// mapFromSession builds a content map from a session (for extracting id).
func mapFromSession(s *PersonaSession) map[string]interface{} {
	return map[string]interface{}{
		"id": parseSessionID(s.ID),
	}
}

// parseSessionID converts the string session ID back to int64.
func parseSessionID(id string) int64 {
	var n int64
	fmt.Sscanf(id, "%d", &n)
	return n
}

// sessionIDToInt64 is an alias for parseSessionID.
func sessionIDToInt64(id string) int64 {
	return parseSessionID(id)
}

// getStringSlice extracts a []string from a NietzscheDB content map.
// Handles both []interface{} (JSON-decoded) and []string (native).
func getStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return []string{}
	}

	switch arr := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	case []string:
		return arr
	case string:
		// Could be a JSON-encoded array or a Postgres-style array
		if strings.HasPrefix(arr, "[") {
			var parsed []string
			if json.Unmarshal([]byte(arr), &parsed) == nil {
				return parsed
			}
		}
		if strings.HasPrefix(arr, "{") {
			return parsePostgresArray(arr)
		}
		if arr == "" {
			return []string{}
		}
		return []string{arr}
	}

	return []string{}
}

// findMostRecent finds the row with the most recent timestamp in the given field.
func findMostRecent(rows []map[string]interface{}, field string) map[string]interface{} {
	if len(rows) == 0 {
		return nil
	}

	best := rows[0]
	bestTime := database.GetTime(best, field)

	for _, row := range rows[1:] {
		t := database.GetTime(row, field)
		if t.After(bestTime) {
			best = row
			bestTime = t
		}
	}

	return best
}

// sortByPriority sorts rows by the "priority" field (ascending, lowest first).
func sortByPriority(rows []map[string]interface{}) {
	// Simple insertion sort (rule sets are small)
	for i := 1; i < len(rows); i++ {
		key := rows[i]
		keyPri := database.GetInt64(key, "priority")
		j := i - 1
		for j >= 0 && database.GetInt64(rows[j], "priority") > keyPri {
			rows[j+1] = rows[j]
			j--
		}
		rows[j+1] = key
	}
}

// parsePostgresArray parses a Postgres-style array string like {a,b,c}.
// Kept for backward compatibility with data that may still use this format.
func parsePostgresArray(pgArray string) []string {
	if len(pgArray) < 2 {
		return []string{}
	}

	// Remove { e }
	cleaned := pgArray[1 : len(pgArray)-1]
	if cleaned == "" {
		return []string{}
	}

	// Split por vírgula
	result := []string{}
	current := ""
	inQuotes := false

	for i := 0; i < len(cleaned); i++ {
		char := cleaned[i]

		if char == '"' {
			inQuotes = !inQuotes
			continue
		}

		if char == ',' && !inQuotes {
			if current != "" {
				result = append(result, current)
				current = ""
			}
			continue
		}

		current += string(char)
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
