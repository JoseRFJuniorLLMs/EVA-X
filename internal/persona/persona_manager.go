package persona

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// PERSONA MANAGER (SPRINT 5)
// ============================================================================
// Gerencia personas configuráveis (Companion, Clinical, Emergency, Educator)

type PersonaManager struct {
	db *sql.DB
}

func NewPersonaManager(db *sql.DB) *PersonaManager {
	return &PersonaManager{db: db}
}

// ============================================================================
// PERSONA DEFINITION
// ============================================================================

type PersonaDefinition struct {
	ID                       string                 `json:"id"`
	PersonaCode              string                 `json:"persona_code"`
	PersonaName              string                 `json:"persona_name"`
	Description              string                 `json:"description"`
	VoiceID                  string                 `json:"voice_id"`
	Tone                     string                 `json:"tone"`
	EmotionalDepth           float64                `json:"emotional_depth"`
	NarrativeFreedom         float64                `json:"narrative_freedom"`
	MaxSessionDurationMinutes int                    `json:"max_session_duration_minutes"`
	MaxDailyInteractions     int                    `json:"max_daily_interactions"`
	MaxIntimacyLevel         float64                `json:"max_intimacy_level"`
	RequireProfessionalOversight bool                `json:"require_professional_oversight"`
	CanOverridePatientRefusal bool                   `json:"can_override_patient_refusal"`
	AllowedTools             []string               `json:"allowed_tools"`
	ProhibitedTools          []string               `json:"prohibited_tools"`
	AllowedTopics            []string               `json:"allowed_topics"`
	ProhibitedTopics         []string               `json:"prohibited_topics"`
	SystemInstructionTemplate string                 `json:"system_instruction_template"`
	Priorities               []string               `json:"priorities"`
	IsActive                 bool                   `json:"is_active"`
	CreatedAt                time.Time              `json:"created_at"`
}

// PersonaSession representa uma sessão ativa de persona
type PersonaSession struct {
	ID                   string    `json:"id"`
	PatientID            int64     `json:"patient_id"`
	PersonaCode          string    `json:"persona_code"`
	StartedAt            time.Time `json:"started_at"`
	EndedAt              *time.Time `json:"ended_at,omitempty"`
	DurationSeconds      *int      `json:"duration_seconds,omitempty"`
	TriggerReason        string    `json:"trigger_reason"`
	TriggeredBy          string    `json:"triggered_by"`
	ToolsUsed            []string  `json:"tools_used"`
	BoundaryViolations   int       `json:"boundary_violations"`
	EscalationRequired   bool      `json:"escalation_required"`
	Status               string    `json:"status"`
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
	query := `
		INSERT INTO persona_sessions (
			patient_id, persona_code, trigger_reason, triggered_by, status
		) VALUES ($1, $2, $3, $4, 'active')
		RETURNING id, started_at
	`

	session := &PersonaSession{
		PatientID:     patientID,
		PersonaCode:   personaCode,
		TriggerReason: triggerReason,
		TriggeredBy:   triggeredBy,
		Status:        "active",
	}

	err = pm.db.QueryRow(query, patientID, personaCode, triggerReason, triggeredBy).
		Scan(&session.ID, &session.StartedAt)

	if err != nil {
		return nil, fmt.Errorf("erro ao criar sessão: %w", err)
	}

	log.Printf("✅ [PERSONA] Persona '%s' ativada (session_id: %s)", personaCode, session.ID)

	return session, nil
}

// GetCurrentPersona retorna persona ativa de um paciente
func (pm *PersonaManager) GetCurrentPersona(patientID int64) (*PersonaSession, error) {
	query := `
		SELECT id, patient_id, persona_code, started_at, ended_at, duration_seconds,
		       trigger_reason, triggered_by, tools_used, boundary_violations,
		       escalation_required, status
		FROM persona_sessions
		WHERE patient_id = $1 AND status = 'active'
		ORDER BY started_at DESC
		LIMIT 1
	`

	session := &PersonaSession{}
	var toolsUsedStr sql.NullString

	err := pm.db.QueryRow(query, patientID).Scan(
		&session.ID, &session.PatientID, &session.PersonaCode,
		&session.StartedAt, &session.EndedAt, &session.DurationSeconds,
		&session.TriggerReason, &session.TriggeredBy,
		&toolsUsedStr, &session.BoundaryViolations,
		&session.EscalationRequired, &session.Status,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Nenhuma persona ativa
	}

	if err != nil {
		return nil, err
	}

	// Parse tools_used array
	if toolsUsedStr.Valid {
		session.ToolsUsed = parsePostgresArray(toolsUsedStr.String)
	}

	return session, nil
}

// deactivateCurrentPersona desativa persona ativa atual
func (pm *PersonaManager) deactivateCurrentPersona(patientID int64) error {
	query := `
		UPDATE persona_sessions
		SET status = 'completed',
		    ended_at = NOW()
		WHERE patient_id = $1
		  AND status = 'active'
	`

	_, err := pm.db.Exec(query, patientID)
	return err
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

	// 2. Verificar permissão via SQL function
	var allowed bool
	query := `SELECT is_tool_allowed($1, $2)`

	err = pm.db.QueryRow(query, session.PersonaCode, toolName).Scan(&allowed)
	if err != nil {
		return false, fmt.Sprintf("erro ao verificar permissão: %v", err)
	}

	if !allowed {
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

	query := `
		UPDATE persona_sessions
		SET tools_used = array_append(COALESCE(tools_used, ARRAY[]::TEXT[]), $1)
		WHERE id = $2
	`

	_, err = pm.db.Exec(query, toolName, session.ID)
	return err
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

	query := `
		UPDATE persona_sessions
		SET boundary_violations = boundary_violations + 1,
		    violated_rules = array_append(COALESCE(violated_rules, ARRAY[]::TEXT[]), $1)
		WHERE id = $2
	`

	_, err = pm.db.Exec(query, violationRule, session.ID)

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
	query := `
		SELECT COUNT(*)
		FROM persona_sessions
		WHERE patient_id = $1
		  AND started_at > NOW() - INTERVAL '24 hours'
	`

	var count int
	err := pm.db.QueryRow(query, patientID).Scan(&count)
	return count, err
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
	query := `
		SELECT
			id, persona_code, persona_name, description, voice_id, tone,
			emotional_depth, narrative_freedom,
			max_session_duration_minutes, max_daily_interactions,
			max_intimacy_level, require_professional_oversight, can_override_patient_refusal,
			allowed_tools, prohibited_tools, allowed_topics, prohibited_topics,
			system_instruction_template, priorities, is_active, created_at
		FROM persona_definitions
		WHERE persona_code = $1
	`

	pd := &PersonaDefinition{}
	var allowedToolsStr, prohibitedToolsStr, allowedTopicsStr, prohibitedTopicsStr, prioritiesStr string

	err := pm.db.QueryRow(query, personaCode).Scan(
		&pd.ID, &pd.PersonaCode, &pd.PersonaName, &pd.Description, &pd.VoiceID, &pd.Tone,
		&pd.EmotionalDepth, &pd.NarrativeFreedom,
		&pd.MaxSessionDurationMinutes, &pd.MaxDailyInteractions,
		&pd.MaxIntimacyLevel, &pd.RequireProfessionalOversight, &pd.CanOverridePatientRefusal,
		&allowedToolsStr, &prohibitedToolsStr, &allowedTopicsStr, &prohibitedTopicsStr,
		&pd.SystemInstructionTemplate, &prioritiesStr, &pd.IsActive, &pd.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	// Parse arrays
	pd.AllowedTools = parsePostgresArray(allowedToolsStr)
	pd.ProhibitedTools = parsePostgresArray(prohibitedToolsStr)
	pd.AllowedTopics = parsePostgresArray(allowedTopicsStr)
	pd.ProhibitedTopics = parsePostgresArray(prohibitedTopicsStr)
	pd.Priorities = parsePostgresArray(prioritiesStr)

	return pd, nil
}

// ListAllPersonas lista todas as personas disponíveis
func (pm *PersonaManager) ListAllPersonas() ([]PersonaDefinition, error) {
	query := `
		SELECT
			id, persona_code, persona_name, description, tone,
			emotional_depth, narrative_freedom, is_active
		FROM persona_definitions
		WHERE is_active = TRUE
		ORDER BY persona_code
	`

	rows, err := pm.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	personas := []PersonaDefinition{}
	for rows.Next() {
		var pd PersonaDefinition
		err := rows.Scan(
			&pd.ID, &pd.PersonaCode, &pd.PersonaName, &pd.Description, &pd.Tone,
			&pd.EmotionalDepth, &pd.NarrativeFreedom, &pd.IsActive,
		)
		if err != nil {
			continue
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
	// Buscar estado do paciente
	patientState, err := pm.getPatientState(patientID)
	if err != nil {
		return "", "", err
	}

	// Buscar regras ativas ordenadas por prioridade
	query := `
		SELECT id, rule_name, target_persona_code, conditions
		FROM persona_activation_rules
		WHERE is_active = TRUE
		ORDER BY priority ASC
	`

	rows, err := pm.db.Query(query)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()

	for rows.Next() {
		var ruleID, ruleName, targetPersona string
		var conditionsJSON []byte

		err := rows.Scan(&ruleID, &ruleName, &targetPersona, &conditionsJSON)
		if err != nil {
			continue
		}

		// Avaliar condições
		var conditions map[string]interface{}
		json.Unmarshal(conditionsJSON, &conditions)

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
	state := make(map[string]interface{})

	// Buscar PHQ-9 mais recente
	var phq9Score sql.NullFloat64
	pm.db.QueryRow(`
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1 AND assessment_type = 'PHQ-9'
		ORDER BY completed_at DESC
		LIMIT 1
	`, patientID).Scan(&phq9Score)

	if phq9Score.Valid {
		state["phq9_score"] = phq9Score.Float64
	}

	// Buscar C-SSRS
	var cssrsScore sql.NullInt64
	pm.db.QueryRow(`
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1 AND assessment_type = 'C-SSRS'
		ORDER BY completed_at DESC
		LIMIT 1
	`, patientID).Scan(&cssrsScore)

	if cssrsScore.Valid {
		state["cssrs_score"] = cssrsScore.Int64
	}

	// Verificar se houve crise recente
	var crisisRecent bool
	pm.db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM crisis_events
			WHERE patient_id = $1
			  AND occurred_at > NOW() - INTERVAL '24 hours'
		)
	`, patientID).Scan(&crisisRecent)

	state["crisis_detected"] = crisisRecent

	return state, nil
}

// ============================================================================
// UTILITÁRIOS
// ============================================================================

func (pm *PersonaManager) personaExists(personaCode string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM persona_definitions WHERE persona_code = $1 AND is_active = TRUE)`
	err := pm.db.QueryRow(query, personaCode).Scan(&exists)
	return exists, err
}

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
