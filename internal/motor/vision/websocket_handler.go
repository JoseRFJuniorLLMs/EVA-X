package vision

import (
	"encoding/json"
	"eva-mind/internal/brainstem/database"
	"fmt"
	"log"
	"time"
)

// WebSocketMedicationHandler handles medication scan messages from WebSocket
type WebSocketMedicationHandler struct {
	identifier *MedicationIdentifier
	db         *database.DB
}

// NewWebSocketMedicationHandler creates a new WebSocket handler
func NewWebSocketMedicationHandler(apiKey string, db *database.DB) (*WebSocketMedicationHandler, error) {
	identifier, err := NewMedicationIdentifier(apiKey)
	if err != nil {
		return nil, err
	}

	return &WebSocketMedicationHandler{
		identifier: identifier,
		db:         db,
	}, nil
}

// MedicationScanMessage represents incoming scan message from mobile
type MedicationScanMessage struct {
	Type                 string                   `json:"type"`
	Action               string                   `json:"action"`
	SessionID            string                   `json:"session_id"`
	ImageData            string                   `json:"image_data"` // base64
	ImageFormat          string                   `json:"image_format"`
	CandidateMedications []map[string]interface{} `json:"candidate_medications"`
	Timestamp            string                   `json:"timestamp"`
}

// MedicationScanResponse represents response to mobile
type MedicationScanResponse struct {
	Action     string                 `json:"action"`
	Status     string                 `json:"status"` // "success", "not_found", "error"
	SessionID  string                 `json:"session_id"`
	Medication map[string]interface{} `json:"medication,omitempty"`
	Safety     map[string]interface{} `json:"safety,omitempty"`
	Message    string                 `json:"message,omitempty"`
	VisualFeedback map[string]interface{} `json:"visual_feedback,omitempty"`
}

// HandleMedicationScanMessage processes a medication scan message
func (h *WebSocketMedicationHandler) HandleMedicationScanMessage(
	msg MedicationScanMessage,
	idosoID int64,
) (*MedicationScanResponse, error) {

	log.Printf("🔍 [WEBSOCKET] Recebida mensagem de scan. SessionID: %s, Action: %s", msg.SessionID, msg.Action)

	if msg.Action == "identify" {
		return h.handleIdentify(msg, idosoID)
	} else if msg.Action == "confirm_taken" {
		return h.handleConfirmTaken(msg, idosoID)
	} else if msg.Action == "cancel" {
		return h.handleCancel(msg)
	}

	return nil, fmt.Errorf("unknown action: %s", msg.Action)
}

// handleIdentify processes medication identification request
func (h *WebSocketMedicationHandler) handleIdentify(
	msg MedicationScanMessage,
	idosoID int64,
) (*MedicationScanResponse, error) {

	// 1. Get candidate medications from database
	var candidateMeds []database.Medicamento
	for _, candMap := range msg.CandidateMedications {
		// Re-fetch from DB to ensure data integrity
		if id, ok := candMap["id"].(float64); ok {
			med, err := h.db.GetMedicationByID(int64(id))
			if err == nil {
				candidateMeds = append(candidateMeds, *med)
			}
		}
	}

	// 2. Identify medication from image
	result, err := h.identifier.IdentifyFromImage(msg.ImageData, candidateMeds, h.db)
	if err != nil {
		log.Printf("❌ [WEBSOCKET] Erro na identificação: %v", err)
		return &MedicationScanResponse{
			Action:    "medication_scan_result",
			Status:    "error",
			SessionID: msg.SessionID,
			Message:   "Erro ao processar imagem. Tente novamente.",
		}, nil
	}

	// 3. Check if medication was found
	if result.MatchedMedication == nil {
		log.Printf("⚠️ [WEBSOCKET] Medicamento não encontrado")
		return &MedicationScanResponse{
			Action:    "medication_scan_result",
			Status:    "not_found",
			SessionID: msg.SessionID,
			Message:   fmt.Sprintf("Não reconheço este medicamento. Detectei: %s %s", result.MedicationName, result.Dosage),
		}, nil
	}

	// 4. Log the visual scan attempt
	err = h.logVisualScan(idosoID, result, msg.SessionID)
	if err != nil {
		log.Printf("⚠️ [WEBSOCKET] Erro ao logar scan: %v", err)
	}

	// 5. Build response
	med := result.MatchedMedication
	safeToTake := true
	warnings := []string{}

	if result.SafetyCheck != nil {
		safeToTake = result.SafetyCheck.SafeToTake
		warnings = result.SafetyCheck.Warnings
	}

	response := &MedicationScanResponse{
		Action:    "medication_scan_result",
		Status:    "success",
		SessionID: msg.SessionID,
		Medication: map[string]interface{}{
			"id":         med.ID,
			"name":       med.Nome,
			"dosage":     med.Dosagem,
			"color":      med.CorEmbalagem,
			"confidence": result.Confidence,
			"is_correct": true,
		},
		Safety: map[string]interface{}{
			"safe_to_take":     safeToTake,
			"warnings":         warnings,
			"scheduled_time":   med.Horarios,
			"current_time":     time.Now().Format("15:04"),
		},
		VisualFeedback: map[string]interface{}{
			"bounding_box":    []int{120, 340, 580, 890}, // Placeholder - would need actual detection
			"highlight_color": "green",
		},
	}

	log.Printf("✅ [WEBSOCKET] Identificação bem-sucedida: %s", med.Nome)
	return response, nil
}

// handleConfirmTaken processes medication taken confirmation
func (h *WebSocketMedicationHandler) handleConfirmTaken(
	msg MedicationScanMessage,
	idosoID int64,
) (*MedicationScanResponse, error) {

	log.Printf("✅ [WEBSOCKET] Paciente confirmou que tomou medicamento")

	// Extract medication ID from candidate medications
	var medicationID int64
	for _, candMap := range msg.CandidateMedications {
		if id, ok := candMap["id"].(float64); ok {
			medicationID = int64(id)
			break
		}
	}

	if medicationID == 0 {
		return &MedicationScanResponse{
			Action:    "medication_confirm_result",
			Status:    "error",
			SessionID: msg.SessionID,
			Message:   "ID do medicamento não encontrado",
		}, nil
	}

	// Log medication as taken
	err := h.db.LogMedicationTaken(medicationID, time.Now(), "")
	if err != nil {
		log.Printf("❌ [WEBSOCKET] Erro ao registrar medicação: %v", err)
		return &MedicationScanResponse{
			Action:    "medication_confirm_result",
			Status:    "error",
			SessionID: msg.SessionID,
			Message:   "Erro ao registrar medicação",
		}, nil
	}

	return &MedicationScanResponse{
		Action:    "medication_confirm_result",
		Status:    "success",
		SessionID: msg.SessionID,
		Message:   "Medicação registrada com sucesso",
	}, nil
}

// handleCancel processes scan cancellation
func (h *WebSocketMedicationHandler) handleCancel(msg MedicationScanMessage) (*MedicationScanResponse, error) {
	log.Printf("❌ [WEBSOCKET] Scan cancelado pelo usuário")

	return &MedicationScanResponse{
		Action:    "medication_scan_cancelled",
		Status:    "cancelled",
		SessionID: msg.SessionID,
		Message:   "Scan cancelado",
	}, nil
}

// logVisualScan logs the visual scan attempt to database
func (h *WebSocketMedicationHandler) logVisualScan(
	idosoID int64,
	result *IdentificationResult,
	sessionID string,
) error {

	// Create log entry in medication_visual_logs table
	query := `
		INSERT INTO medication_visual_logs (
			patient_id, session_id, scan_status, confidence_score,
			gemini_model_used, created_at
		) VALUES ($1, $2, 'success', $3, 'gemini-2.0-flash-exp', NOW())
	`

	_, err := h.db.Conn.Exec(query, idosoID, sessionID, result.Confidence)
	if err != nil {
		return err
	}

	// Create identification entry if matched
	if result.MatchedMedication != nil {
		queryIdent := `
			INSERT INTO medication_identifications (
				visual_log_id, medication_name, dosage, pharmaceutical_form,
				pill_color, manufacturer, confidence, created_at
			) SELECT
				id, $2, $3, $4, $5, $6, $7, NOW()
			FROM medication_visual_logs
			WHERE session_id = $1
			ORDER BY created_at DESC
			LIMIT 1
		`

		_, err = h.db.Conn.Exec(
			queryIdent,
			sessionID,
			result.MedicationName,
			result.Dosage,
			result.PharmaceuticalForm,
			result.Color,
			result.Manufacturer,
			result.Confidence,
		)

		if err != nil {
			log.Printf("⚠️ Erro ao criar identification entry: %v", err)
		}
	}

	return nil
}

// ParseMessage parses a WebSocket message into MedicationScanMessage
func ParseMessage(messageJSON []byte) (*MedicationScanMessage, error) {
	var msg MedicationScanMessage
	err := json.Unmarshal(messageJSON, &msg)
	if err != nil {
		return nil, err
	}
	return &msg, nil
}

// Close closes the handler and its dependencies
func (h *WebSocketMedicationHandler) Close() error {
	return h.identifier.Close()
}
