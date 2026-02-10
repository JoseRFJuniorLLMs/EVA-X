package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// ============================================================================
// WEBHOOK PAYLOAD BUILDERS
// ============================================================================
// Helpers para criar payloads de eventos que serão enviados via webhooks

// ============================================================================
// BASE WEBHOOK EVENT
// ============================================================================

type WebhookEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "patient.created", "assessment.completed", etc.
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"` // "EVA-Mind"
	Data      map[string]interface{} `json:"data"`
	Signature string                 `json:"signature,omitempty"` // HMAC-SHA256
}

// ============================================================================
// PATIENT EVENTS
// ============================================================================

func PatientCreatedEvent(patient *PatientDTO) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "patient.created",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id": patient.ID,
			"name":       patient.Name,
			"age":        patient.Age,
			"created_at": patient.CreatedAt,
		},
	}
}

func PatientUpdatedEvent(patientID int64, changes map[string]interface{}) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "patient.updated",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id": patientID,
			"changes":    changes,
		},
	}
}

// ============================================================================
// ASSESSMENT EVENTS
// ============================================================================

func AssessmentCompletedEvent(assessment *AssessmentDTO) *WebhookEvent {
	data := map[string]interface{}{
		"assessment_id":   assessment.ID,
		"patient_id":      assessment.PatientID,
		"assessment_type": assessment.AssessmentType,
		"total_score":     assessment.TotalScore,
		"severity":        assessment.Severity,
		"completed_at":    assessment.CompletedAt,
	}

	// Adicionar flags se houver
	if len(assessment.Flags) > 0 {
		data["flags"] = assessment.Flags
	}

	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "assessment.completed",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data:      data,
	}
}

func SuicideRiskDetectedEvent(patientID int64, assessmentID string, cssrsScore int) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "crisis.suicide_risk_detected",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id":    patientID,
			"assessment_id": assessmentID,
			"cssrs_score":   cssrsScore,
			"risk_level":    mapCSSRSToRiskLevel(cssrsScore),
			"action_required": "immediate_intervention",
		},
	}
}

func mapCSSRSToRiskLevel(score int) string {
	if score >= 4 {
		return "imminent"
	} else if score >= 2 {
		return "moderate"
	}
	return "low"
}

// ============================================================================
// CRISIS EVENTS
// ============================================================================

func CrisisDetectedEvent(patientID int64, crisisType string, severity string, details map[string]interface{}) *WebhookEvent {
	data := map[string]interface{}{
		"patient_id":   patientID,
		"crisis_type":  crisisType, // "suicidal_ideation", "severe_anxiety", "panic_attack"
		"severity":     severity,    // "low", "moderate", "high", "critical"
		"detected_at":  time.Now(),
	}

	// Merge details
	for k, v := range details {
		data[k] = v
	}

	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "crisis.detected",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data:      data,
	}
}

// ============================================================================
// PERSONA EVENTS
// ============================================================================

func PersonaTransitionEvent(patientID int64, fromPersona, toPersona, reason string) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "persona.transition",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id":   patientID,
			"from_persona": fromPersona,
			"to_persona":   toPersona,
			"reason":       reason,
		},
	}
}

// ============================================================================
// EXIT PROTOCOL EVENTS
// ============================================================================

func PainAlertEvent(patientID int64, painLog *PainLogDTO) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "exit.pain_alert",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id":     patientID,
			"pain_intensity": painLog.PainIntensity,
			"pain_location":  painLog.PainLocation,
			"timestamp":      painLog.Timestamp,
			"alert_level":    "high",
		},
	}
}

func QualityOfLifeChangedEvent(patientID int64, oldScore, newScore float64, trend string) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "exit.quality_of_life_changed",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id":     patientID,
			"old_qol_score":  oldScore,
			"new_qol_score":  newScore,
			"change":         newScore - oldScore,
			"trend":          trend, // "improving", "stable", "declining"
		},
	}
}

// ============================================================================
// RESEARCH EVENTS
// ============================================================================

func ResearchFindingEvent(studyID, studyCode string, finding *FindingDTO) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "research.finding_discovered",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"study_id":    studyID,
			"study_code":  studyCode,
			"predictor":   finding.Predictor,
			"outcome":     finding.Outcome,
			"correlation": finding.Correlation,
			"p_value":     finding.PValue,
			"effect_size": finding.EffectSize,
		},
	}
}

// ============================================================================
// MEDICATION EVENTS
// ============================================================================

func MedicationAdherenceAlertEvent(patientID int64, medicationName string, missedDoses int) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "medication.adherence_alert",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id":      patientID,
			"medication_name": medicationName,
			"missed_doses":    missedDoses,
			"alert_level":     "medium",
		},
	}
}

// ============================================================================
// TRAJECTORY EVENTS
// ============================================================================

func TrajectoryRiskIncreasedEvent(patientID int64, riskType string, oldRisk, newRisk float64) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      "trajectory.risk_increased",
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data: map[string]interface{}{
			"patient_id": patientID,
			"risk_type":  riskType, // "suicide", "hospitalization", "crisis"
			"old_risk":   oldRisk,
			"new_risk":   newRisk,
			"increase":   newRisk - oldRisk,
		},
	}
}

// ============================================================================
// WEBHOOK SIGNATURE (HMAC-SHA256)
// ============================================================================

// Gerar assinatura HMAC-SHA256 para webhook
func SignWebhookPayload(payload string, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// Verificar assinatura de webhook
func VerifyWebhookSignature(payload string, signature string, secret string) bool {
	expectedSignature := SignWebhookPayload(payload, secret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// Adicionar assinatura ao evento
func (e *WebhookEvent) AddSignature(secret string) error {
	payload, err := ToJSONCompact(e)
	if err != nil {
		return err
	}
	e.Signature = SignWebhookPayload(payload, secret)
	return nil
}

// ============================================================================
// WEBHOOK DELIVERY HELPERS
// ============================================================================

type WebhookDeliveryResult struct {
	Success        bool      `json:"success"`
	HTTPStatusCode int       `json:"http_status_code"`
	ResponseTime   int64     `json:"response_time_ms"`
	ErrorMessage   string    `json:"error_message,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func generateEventID() string {
	// Gerar ID único para evento (você pode usar UUID library)
	return time.Now().Format("20060102150405")
}

// Serializar evento para JSON (para enviar via HTTP)
func (e *WebhookEvent) ToJSON() (string, error) {
	return ToJSONCompact(e)
}

// ============================================================================
// WEBHOOK BATCH (MÚLTIPLOS EVENTOS)
// ============================================================================

type WebhookBatch struct {
	Events []WebhookEvent `json:"events"`
	Total  int            `json:"total"`
}

func NewWebhookBatch(events []WebhookEvent) *WebhookBatch {
	return &WebhookBatch{
		Events: events,
		Total:  len(events),
	}
}

// ============================================================================
// CUSTOM WEBHOOK EVENT
// ============================================================================

func CustomEvent(eventType string, data map[string]interface{}) *WebhookEvent {
	return &WebhookEvent{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    "EVA-Mind",
		Data:      data,
	}
}
