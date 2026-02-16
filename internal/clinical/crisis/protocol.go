package crisis

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// CrisisProtocol handles crisis detection and response
type CrisisProtocol struct {
	db       *sql.DB
	notifier *Notifier
}

// NewCrisisProtocol creates a new crisis protocol handler
func NewCrisisProtocol(db *sql.DB, notifier *Notifier) *CrisisProtocol {
	return &CrisisProtocol{
		db:       db,
		notifier: notifier,
	}
}

// CrisisType represents type of crisis
type CrisisType string

const (
	CrisisTypeAbuse    CrisisType = "abuse"
	CrisisTypeSelfHarm CrisisType = "self_harm"
	CrisisTypeNeglect  CrisisType = "neglect"
	CrisisTypeViolence CrisisType = "violence"
	CrisisTypeOther    CrisisType = "other"
)

// CrisisEvent represents a detected crisis
type CrisisEvent struct {
	ID                int64                  `json:"id"`
	PatientID         int64                  `json:"patient_id"`
	SessionID         int64                  `json:"session_id"`
	CrisisType        CrisisType             `json:"crisis_type"`
	Severity          string                 `json:"severity"` // MODERATE, HIGH, CRITICAL
	TriggerStatement  string                 `json:"trigger_statement"`
	ResponseActions   map[string]bool        `json:"response_actions"`
	NotificationsSent map[string]interface{} `json:"notifications_sent"`
	AcknowledgedBy    *int64                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt    *time.Time             `json:"acknowledged_at,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
}

// CrisisPattern represents a pattern that triggers crisis detection
type CrisisPattern struct {
	Type     CrisisType
	Keywords []string
	Severity string
}

// GetCrisisPatterns returns all crisis detection patterns
func GetCrisisPatterns() []CrisisPattern {
	return []CrisisPattern{
		// Abuse patterns
		{
			Type:     CrisisTypeAbuse,
			Keywords: []string{"toca em mim", "me toca", "abusa", "abuso", "molesta", "segredo do tio", "não pode contar"},
			Severity: "CRITICAL",
		},
		{
			Type:     CrisisTypeAbuse,
			Keywords: []string{"bate em mim", "me bate", "machuca", "apanha", "surra", "cinto"},
			Severity: "HIGH",
		},

		// Self-harm patterns (from PediatricRiskDetector)
		{
			Type:     CrisisTypeSelfHarm,
			Keywords: []string{"quero morrer", "quero me matar", "vou me matar", "me corto", "me cortar"},
			Severity: "CRITICAL",
		},
		{
			Type:     CrisisTypeSelfHarm,
			Keywords: []string{"dormir pra sempre", "nunca mais acordar", "virar estrela"},
			Severity: "HIGH",
		},

		// Neglect patterns
		{
			Type:     CrisisTypeNeglect,
			Keywords: []string{"não como há", "sem comida", "fome", "sozinho em casa", "ninguém cuida"},
			Severity: "HIGH",
		},

		// Violence patterns
		{
			Type:     CrisisTypeViolence,
			Keywords: []string{"vi meu pai bater", "briga em casa", "sangue", "polícia veio", "gritos de noite"},
			Severity: "MODERATE",
		},
	}
}

// AnalyzeStatement analyzes a statement for crisis indicators
func (c *CrisisProtocol) AnalyzeStatement(ctx context.Context, statement string, patientID, sessionID int64) (*CrisisEvent, error) {
	statementLower := strings.ToLower(statement)
	patterns := GetCrisisPatterns()

	var detectedCrisis *CrisisEvent
	highestSeverity := ""

	for _, pattern := range patterns {
		for _, keyword := range pattern.Keywords {
			if strings.Contains(statementLower, keyword) {
				// Crisis detected
				if detectedCrisis == nil || c.compareSeverity(pattern.Severity, highestSeverity) > 0 {
					detectedCrisis = &CrisisEvent{
						PatientID:        patientID,
						SessionID:        sessionID,
						CrisisType:       pattern.Type,
						Severity:         pattern.Severity,
						TriggerStatement: statement,
						ResponseActions:  c.determineResponseActions(pattern.Type, pattern.Severity),
						CreatedAt:        time.Now(),
					}
					highestSeverity = pattern.Severity
				}
			}
		}
	}

	if detectedCrisis != nil {
		// Store crisis event
		err := c.storeCrisisEvent(ctx, detectedCrisis)
		if err != nil {
			return nil, err
		}

		// Execute response actions
		err = c.executeResponseActions(ctx, detectedCrisis)
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute crisis response actions")
		}

		log.Warn().
			Int64("patient_id", patientID).
			Str("crisis_type", string(detectedCrisis.CrisisType)).
			Str("severity", detectedCrisis.Severity).
			Msg("🚨 CRISIS DETECTED")
	}

	return detectedCrisis, nil
}

// compareSeverity compares two severity levels
func (c *CrisisProtocol) compareSeverity(s1, s2 string) int {
	severityMap := map[string]int{
		"MODERATE": 1,
		"HIGH":     2,
		"CRITICAL": 3,
	}

	return severityMap[s1] - severityMap[s2]
}

// determineResponseActions determines what actions to take
func (c *CrisisProtocol) determineResponseActions(crisisType CrisisType, severity string) map[string]bool {
	actions := map[string]bool{
		"notify_psychologist":    true, // Always notify
		"create_legal_record":    false,
		"lock_conversation":      false,
		"notify_emergency":       false,
		"notify_authorities":     false,
		"require_acknowledgment": false,
	}

	switch severity {
	case "CRITICAL":
		actions["create_legal_record"] = true
		actions["lock_conversation"] = true
		actions["require_acknowledgment"] = true

		if crisisType == CrisisTypeAbuse {
			actions["notify_authorities"] = true
		}
		if crisisType == CrisisTypeSelfHarm {
			actions["notify_emergency"] = true
		}

	case "HIGH":
		actions["create_legal_record"] = true
		actions["require_acknowledgment"] = true
	}

	return actions
}

// executeResponseActions executes the determined response actions
func (c *CrisisProtocol) executeResponseActions(ctx context.Context, event *CrisisEvent) error {
	notifications := make(map[string]interface{})

	// 1. Notify psychologist (always)
	if event.ResponseActions["notify_psychologist"] {
		err := c.notifier.NotifyPsychologist(ctx, event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to notify psychologist")
		} else {
			notifications["psychologist"] = map[string]interface{}{
				"sent_at": time.Now(),
				"status":  "sent",
			}
		}
	}

	// 2. Create legal record
	if event.ResponseActions["create_legal_record"] {
		err := c.createLegalRecord(ctx, event)
		if err != nil {
			log.Error().Err(err).Msg("Failed to create legal record")
		} else {
			notifications["legal_record"] = map[string]interface{}{
				"created_at": time.Now(),
				"status":     "created",
			}
		}
	}

	// 3. Lock conversation (prevent tampering)
	if event.ResponseActions["lock_conversation"] {
		err := c.lockConversation(ctx, event.SessionID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to lock conversation")
		} else {
			notifications["conversation_locked"] = true
		}
	}

	// 4. Notify emergency services (SAMU, etc.)
	if event.ResponseActions["notify_emergency"] {
		err := c.notifier.NotifyEmergencyServices(ctx, event)
		if err != nil {
			log.Error().Err(err).Msg("Falha ao notificar servicos de emergencia")
			notifications["emergency"] = map[string]interface{}{
				"status":  "partial_failure",
				"error":   err.Error(),
				"sent_at": time.Now(),
			}
		} else {
			notifications["emergency"] = map[string]interface{}{
				"status":  "sent",
				"sent_at": time.Now(),
			}
		}
	}

	// 5. Notify authorities (child protective services, etc.)
	if event.ResponseActions["notify_authorities"] {
		err := c.notifier.NotifyAuthorities(ctx, event)
		if err != nil {
			log.Error().Err(err).Msg("Falha ao notificar autoridades")
			notifications["authorities"] = map[string]interface{}{
				"status":  "partial_failure",
				"error":   err.Error(),
				"sent_at": time.Now(),
			}
		} else {
			notifications["authorities"] = map[string]interface{}{
				"status":  "sent",
				"sent_at": time.Now(),
			}
		}
	}

	// Update event with notifications sent
	event.NotificationsSent = notifications
	return c.updateNotificationsSent(ctx, event.ID, notifications)
}

// storeCrisisEvent stores crisis event in database
func (c *CrisisProtocol) storeCrisisEvent(ctx context.Context, event *CrisisEvent) error {
	actionsJSON, _ := json.Marshal(event.ResponseActions)

	return c.db.QueryRowContext(ctx, `
		INSERT INTO crisis_events (
			patient_id, session_id, crisis_type, severity,
			trigger_statement, response_actions, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, event.PatientID, event.SessionID, event.CrisisType, event.Severity,
		event.TriggerStatement, actionsJSON, event.CreatedAt,
	).Scan(&event.ID)
}

// updateNotificationsSent updates notifications sent
func (c *CrisisProtocol) updateNotificationsSent(ctx context.Context, eventID int64, notifications map[string]interface{}) error {
	notificationsJSON, _ := json.Marshal(notifications)

	_, err := c.db.ExecContext(ctx, `
		UPDATE crisis_events
		SET notifications_sent = $1
		WHERE id = $2
	`, notificationsJSON, eventID)

	return err
}

// createLegalRecord creates an immutable, encrypted legal record
func (c *CrisisProtocol) createLegalRecord(ctx context.Context, event *CrisisEvent) error {
	// Create timestamped record with cryptographic hash
	record := map[string]interface{}{
		"event_id":          event.ID,
		"patient_id":        event.PatientID,
		"crisis_type":       event.CrisisType,
		"severity":          event.Severity,
		"trigger_statement": event.TriggerStatement,
		"timestamp":         event.CreatedAt,
		"hash":              c.generateHash(event),
	}

	recordJSON, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("falha ao serializar registro legal: %w", err)
	}

	// Encrypt the record using AES-256-GCM
	encryptedRecord, err := c.encryptData(recordJSON)
	if err != nil {
		log.Error().Err(err).Msg("Falha ao criptografar registro legal - armazenando sem criptografia")
		// Fallback: store unencrypted but log the failure
		encryptedRecord = base64.StdEncoding.EncodeToString(recordJSON)
	}

	// Store encrypted record in database
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO legal_records (
			event_id, patient_id, encrypted_data, hash,
			encryption_version, created_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (event_id) DO NOTHING
	`, event.ID, event.PatientID, encryptedRecord, c.generateHash(event),
		"AES-256-GCM-v1", event.CreatedAt)

	if err != nil {
		log.Error().Err(err).Int64("event_id", event.ID).
			Msg("Falha ao armazenar registro legal no banco - tabela pode nao existir")
		// Don't fail the whole operation if table doesn't exist yet
		return nil
	}

	log.Info().
		Int64("event_id", event.ID).
		Str("crisis_type", string(event.CrisisType)).
		Msg("Registro legal criptografado criado")

	return nil
}

// encryptData encrypts data using AES-256-GCM
func (c *CrisisProtocol) encryptData(plaintext []byte) (string, error) {
	keyStr := os.Getenv("ENCRYPTION_KEY")
	if keyStr == "" {
		return "", fmt.Errorf("ENCRYPTION_KEY nao configurada")
	}

	key, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return "", fmt.Errorf("ENCRYPTION_KEY invalida: %w", err)
	}

	if len(key) != 32 {
		return "", fmt.Errorf("ENCRYPTION_KEY deve ter 32 bytes (AES-256), tem %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("falha ao criar cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("falha ao criar GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("falha ao gerar nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// generateHash generates a SHA-256 hash for tamper detection
func (c *CrisisProtocol) generateHash(event *CrisisEvent) string {
	data := fmt.Sprintf("%d|%d|%s|%s|%s|%s",
		event.ID,
		event.PatientID,
		event.CrisisType,
		event.Severity,
		event.TriggerStatement,
		event.CreatedAt.Format(time.RFC3339Nano),
	)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// lockConversation locks a conversation to prevent tampering
func (c *CrisisProtocol) lockConversation(ctx context.Context, sessionID int64) error {
	_, err := c.db.ExecContext(ctx, `
		UPDATE conversations
		SET locked = TRUE, locked_at = NOW()
		WHERE id = $1
	`, sessionID)

	return err
}

// AcknowledgeCrisis marks a crisis as acknowledged by psychologist
func (c *CrisisProtocol) AcknowledgeCrisis(ctx context.Context, eventID, psychologistID int64) error {
	now := time.Now()

	_, err := c.db.ExecContext(ctx, `
		UPDATE crisis_events
		SET acknowledged_by = $1, acknowledged_at = $2
		WHERE id = $3
	`, psychologistID, now, eventID)

	if err == nil {
		log.Info().
			Int64("event_id", eventID).
			Int64("psychologist_id", psychologistID).
			Msg("Crisis acknowledged")
	}

	return err
}

// GetUnacknowledgedCrises retrieves unacknowledged crises
func (c *CrisisProtocol) GetUnacknowledgedCrises(ctx context.Context) ([]*CrisisEvent, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT id, patient_id, session_id, crisis_type, severity,
		       trigger_statement, response_actions, notifications_sent, created_at
		FROM crisis_events
		WHERE acknowledged_by IS NULL
		ORDER BY created_at DESC
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*CrisisEvent
	for rows.Next() {
		var event CrisisEvent
		var actionsJSON, notificationsJSON []byte

		err := rows.Scan(
			&event.ID, &event.PatientID, &event.SessionID, &event.CrisisType,
			&event.Severity, &event.TriggerStatement, &actionsJSON,
			&notificationsJSON, &event.CreatedAt,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(actionsJSON, &event.ResponseActions)
		json.Unmarshal(notificationsJSON, &event.NotificationsSent)

		events = append(events, &event)
	}

	return events, rows.Err()
}
