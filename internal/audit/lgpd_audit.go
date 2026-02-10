package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// AuditEventType represents the type of audit event
type AuditEventType string

const (
	// Data access events
	EventDataAccess   AuditEventType = "DATA_ACCESS"
	EventDataCreate   AuditEventType = "DATA_CREATE"
	EventDataUpdate   AuditEventType = "DATA_UPDATE"
	EventDataDelete   AuditEventType = "DATA_DELETE"
	EventDataExport   AuditEventType = "DATA_EXPORT"

	// Consent events
	EventConsentGiven    AuditEventType = "CONSENT_GIVEN"
	EventConsentRevoked  AuditEventType = "CONSENT_REVOKED"
	EventConsentUpdated  AuditEventType = "CONSENT_UPDATED"

	// Clinical events
	EventClinicalAssessment AuditEventType = "CLINICAL_ASSESSMENT"
	EventAlertSent          AuditEventType = "ALERT_SENT"
	EventMemoryAccess       AuditEventType = "MEMORY_ACCESS"

	// System events
	EventLogin         AuditEventType = "LOGIN"
	EventLogout        AuditEventType = "LOGOUT"
	EventAuthFailure   AuditEventType = "AUTH_FAILURE"

	// LGPD specific
	EventRightToAccess     AuditEventType = "RIGHT_TO_ACCESS"
	EventRightToRectify    AuditEventType = "RIGHT_TO_RECTIFY"
	EventRightToDelete     AuditEventType = "RIGHT_TO_DELETE"
	EventRightToPortability AuditEventType = "RIGHT_TO_PORTABILITY"
)

// DataCategory represents LGPD data categories
type DataCategory string

const (
	CategoryPersonal    DataCategory = "PERSONAL"       // Nome, CPF, etc
	CategorySensitive   DataCategory = "SENSITIVE"      // Dados de saúde
	CategoryBiometric   DataCategory = "BIOMETRIC"      // Voz, imagem
	CategoryBehavioral  DataCategory = "BEHAVIORAL"     // Padrões de uso
	CategoryClinical    DataCategory = "CLINICAL"       // Avaliações clínicas
	CategoryConversation DataCategory = "CONVERSATION"  // Conversas
)

// LegalBasis represents the legal basis for processing (Art. 7 LGPD)
type LegalBasis string

const (
	BasisConsent            LegalBasis = "CONSENT"             // Art. 7, I
	BasisLegalObligation    LegalBasis = "LEGAL_OBLIGATION"    // Art. 7, II
	BasisPublicPolicy       LegalBasis = "PUBLIC_POLICY"       // Art. 7, III
	BasisResearch           LegalBasis = "RESEARCH"            // Art. 7, IV
	BasisContractExecution  LegalBasis = "CONTRACT_EXECUTION"  // Art. 7, V
	BasisLegitimateInterest LegalBasis = "LEGITIMATE_INTEREST" // Art. 7, IX
	BasisHealthProtection   LegalBasis = "HEALTH_PROTECTION"   // Art. 7, VIII
)

// AuditEvent represents a single audit log entry
type AuditEvent struct {
	ID            string          `json:"id"`
	Timestamp     time.Time       `json:"timestamp"`
	EventType     AuditEventType  `json:"event_type"`
	DataCategory  DataCategory    `json:"data_category"`
	LegalBasis    LegalBasis      `json:"legal_basis"`

	// Actor information
	ActorID       string          `json:"actor_id"`       // Who performed the action
	ActorType     string          `json:"actor_type"`     // user, system, caregiver
	ActorIP       string          `json:"actor_ip,omitempty"`

	// Data subject information
	SubjectID     int64           `json:"subject_id"`     // idoso_id (data subject)
	SubjectCPF    string          `json:"subject_cpf,omitempty"` // Hashed for privacy

	// Event details
	Resource      string          `json:"resource"`       // Table/collection affected
	Action        string          `json:"action"`         // Specific action taken
	Description   string          `json:"description"`
	FieldsAccessed []string       `json:"fields_accessed,omitempty"`

	// Metadata
	Metadata      map[string]interface{} `json:"metadata,omitempty"`

	// Retention
	RetentionDays int             `json:"retention_days"` // How long to keep

	// Result
	Success       bool            `json:"success"`
	ErrorMessage  string          `json:"error_message,omitempty"`
}

// LGPDAuditService handles LGPD compliance audit logging
type LGPDAuditService struct {
	db        *sql.DB
	buffer    []AuditEvent
	bufferMu  sync.Mutex
	flushSize int
	flushChan chan struct{}
	stopChan  chan struct{}
}

// NewLGPDAuditService creates a new audit service
func NewLGPDAuditService(db *sql.DB) *LGPDAuditService {
	svc := &LGPDAuditService{
		db:        db,
		buffer:    make([]AuditEvent, 0, 100),
		flushSize: 50,
		flushChan: make(chan struct{}, 1),
		stopChan:  make(chan struct{}),
	}

	// Start background flush worker
	go svc.flushWorker()

	log.Println("✅ LGPDAuditService initialized")
	return svc
}

// LogEvent logs an audit event
func (s *LGPDAuditService) LogEvent(ctx context.Context, event AuditEvent) error {
	// Set defaults
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	if event.ID == "" {
		event.ID = fmt.Sprintf("aud-%d", time.Now().UnixNano())
	}
	if event.RetentionDays == 0 {
		event.RetentionDays = s.getDefaultRetention(event.DataCategory)
	}

	// Validate legal basis
	if err := s.validateLegalBasis(event); err != nil {
		log.Printf("⚠️ [LGPD] Invalid event: %v", err)
		return err
	}

	// Add to buffer
	s.bufferMu.Lock()
	s.buffer = append(s.buffer, event)
	shouldFlush := len(s.buffer) >= s.flushSize
	s.bufferMu.Unlock()

	if shouldFlush {
		select {
		case s.flushChan <- struct{}{}:
		default:
		}
	}

	return nil
}

// LogDataAccess logs access to personal data
func (s *LGPDAuditService) LogDataAccess(ctx context.Context, subjectID int64, actorID, resource string, fields []string, legalBasis LegalBasis) error {
	return s.LogEvent(ctx, AuditEvent{
		EventType:      EventDataAccess,
		DataCategory:   s.inferDataCategory(resource),
		LegalBasis:     legalBasis,
		ActorID:        actorID,
		ActorType:      "system",
		SubjectID:      subjectID,
		Resource:       resource,
		Action:         "READ",
		Description:    fmt.Sprintf("Data access: %s for subject %d", resource, subjectID),
		FieldsAccessed: fields,
		Success:        true,
	})
}

// LogClinicalAssessment logs a clinical assessment event
func (s *LGPDAuditService) LogClinicalAssessment(ctx context.Context, subjectID int64, assessmentType, result string) error {
	return s.LogEvent(ctx, AuditEvent{
		EventType:     EventClinicalAssessment,
		DataCategory:  CategoryClinical,
		LegalBasis:    BasisHealthProtection, // Art. 7, VIII - proteção da vida
		ActorID:       "eva-system",
		ActorType:     "system",
		SubjectID:     subjectID,
		Resource:      "clinical_assessments",
		Action:        "ASSESS",
		Description:   fmt.Sprintf("Clinical assessment: %s for subject %d", assessmentType, subjectID),
		Metadata: map[string]interface{}{
			"assessment_type": assessmentType,
			"result":          result,
		},
		Success: true,
	})
}

// LogAlertSent logs when an alert is sent
func (s *LGPDAuditService) LogAlertSent(ctx context.Context, subjectID int64, alertType, channel, recipient string) error {
	return s.LogEvent(ctx, AuditEvent{
		EventType:    EventAlertSent,
		DataCategory: CategorySensitive,
		LegalBasis:   BasisHealthProtection, // Emergency health protection
		ActorID:      "eva-system",
		ActorType:    "system",
		SubjectID:    subjectID,
		Resource:     "alerts",
		Action:       "SEND",
		Description:  fmt.Sprintf("Alert sent: %s via %s for subject %d", alertType, channel, subjectID),
		Metadata: map[string]interface{}{
			"alert_type": alertType,
			"channel":    channel,
			"recipient":  recipient,
		},
		Success: true,
	})
}

// LogConsentChange logs consent changes
func (s *LGPDAuditService) LogConsentChange(ctx context.Context, subjectID int64, consentType string, granted bool, purpose string) error {
	eventType := EventConsentGiven
	if !granted {
		eventType = EventConsentRevoked
	}

	return s.LogEvent(ctx, AuditEvent{
		EventType:    eventType,
		DataCategory: CategoryPersonal,
		LegalBasis:   BasisConsent,
		ActorID:      fmt.Sprintf("subject-%d", subjectID),
		ActorType:    "user",
		SubjectID:    subjectID,
		Resource:     "consents",
		Action:       "CONSENT_CHANGE",
		Description:  fmt.Sprintf("Consent %s: %s for %s", map[bool]string{true: "granted", false: "revoked"}[granted], consentType, purpose),
		Metadata: map[string]interface{}{
			"consent_type": consentType,
			"granted":      granted,
			"purpose":      purpose,
		},
		Success: true,
	})
}

// LogRightExercised logs when a data subject exercises their rights
func (s *LGPDAuditService) LogRightExercised(ctx context.Context, subjectID int64, rightType AuditEventType, details string) error {
	return s.LogEvent(ctx, AuditEvent{
		EventType:    rightType,
		DataCategory: CategoryPersonal,
		LegalBasis:   BasisLegalObligation, // LGPD compliance
		ActorID:      fmt.Sprintf("subject-%d", subjectID),
		ActorType:    "user",
		SubjectID:    subjectID,
		Resource:     "lgpd_requests",
		Action:       string(rightType),
		Description:  details,
		Success:      true,
	})
}

// GetAuditTrail retrieves audit events for a subject
func (s *LGPDAuditService) GetAuditTrail(ctx context.Context, subjectID int64, startDate, endDate time.Time) ([]AuditEvent, error) {
	// Flush any pending events first
	s.flush()

	query := `
		SELECT
			id, timestamp, event_type, data_category, legal_basis,
			actor_id, actor_type, actor_ip,
			subject_id, subject_cpf,
			resource, action, description, fields_accessed,
			metadata, retention_days, success, error_message
		FROM lgpd_audit_log
		WHERE subject_id = $1
		  AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp DESC
	`

	rows, err := s.db.QueryContext(ctx, query, subjectID, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit trail: %w", err)
	}
	defer rows.Close()

	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var fieldsJSON, metadataJSON sql.NullString

		err := rows.Scan(
			&event.ID, &event.Timestamp, &event.EventType, &event.DataCategory, &event.LegalBasis,
			&event.ActorID, &event.ActorType, &event.ActorIP,
			&event.SubjectID, &event.SubjectCPF,
			&event.Resource, &event.Action, &event.Description, &fieldsJSON,
			&metadataJSON, &event.RetentionDays, &event.Success, &event.ErrorMessage,
		)
		if err != nil {
			log.Printf("⚠️ Error scanning audit event: %v", err)
			continue
		}

		if fieldsJSON.Valid {
			json.Unmarshal([]byte(fieldsJSON.String), &event.FieldsAccessed)
		}
		if metadataJSON.Valid {
			json.Unmarshal([]byte(metadataJSON.String), &event.Metadata)
		}

		events = append(events, event)
	}

	// Log the access to audit trail itself (meta-audit)
	s.LogEvent(ctx, AuditEvent{
		EventType:    EventRightToAccess,
		DataCategory: CategoryPersonal,
		LegalBasis:   BasisLegalObligation,
		ActorID:      fmt.Sprintf("subject-%d", subjectID),
		ActorType:    "user",
		SubjectID:    subjectID,
		Resource:     "lgpd_audit_log",
		Action:       "READ_AUDIT_TRAIL",
		Description:  fmt.Sprintf("Audit trail accessed for period %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		Metadata: map[string]interface{}{
			"events_returned": len(events),
		},
		Success: true,
	})

	return events, nil
}

// GetDataInventory returns all data held about a subject (Art. 18, II LGPD)
func (s *LGPDAuditService) GetDataInventory(ctx context.Context, subjectID int64) (map[string]interface{}, error) {
	inventory := make(map[string]interface{})

	// This would query all tables with personal data
	tables := []string{
		"idosos",
		"memorias_episodicas",
		"memorias_semanticas",
		"conversas",
		"clinical_assessments",
		"alertas",
		"agendamentos",
	}

	for _, table := range tables {
		exists, err := s.checkDataExists(ctx, table, subjectID)
		if err != nil {
			log.Printf("⚠️ Error checking %s: %v", table, err)
			continue
		}
		if exists {
			inventory[table] = map[string]interface{}{
				"has_data":       true,
				"data_category":  s.inferDataCategory(table),
				"retention_days": s.getDefaultRetention(s.inferDataCategory(table)),
			}
		}
	}

	// Log the inventory request
	s.LogRightExercised(ctx, subjectID, EventRightToAccess, "Data inventory requested")

	return inventory, nil
}

// Internal methods

func (s *LGPDAuditService) flushWorker() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopChan:
			s.flush()
			return
		case <-s.flushChan:
			s.flush()
		case <-ticker.C:
			s.flush()
		}
	}
}

func (s *LGPDAuditService) flush() {
	s.bufferMu.Lock()
	if len(s.buffer) == 0 {
		s.bufferMu.Unlock()
		return
	}
	events := s.buffer
	s.buffer = make([]AuditEvent, 0, 100)
	s.bufferMu.Unlock()

	// Batch insert
	for _, event := range events {
		s.insertEvent(event)
	}
}

func (s *LGPDAuditService) insertEvent(event AuditEvent) error {
	if s.db == nil {
		log.Printf("📝 [LGPD Audit] %s: %s (subject: %d)", event.EventType, event.Description, event.SubjectID)
		return nil
	}

	fieldsJSON, _ := json.Marshal(event.FieldsAccessed)
	metadataJSON, _ := json.Marshal(event.Metadata)

	query := `
		INSERT INTO lgpd_audit_log (
			id, timestamp, event_type, data_category, legal_basis,
			actor_id, actor_type, actor_ip,
			subject_id, subject_cpf,
			resource, action, description, fields_accessed,
			metadata, retention_days, success, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	_, err := s.db.Exec(query,
		event.ID, event.Timestamp, event.EventType, event.DataCategory, event.LegalBasis,
		event.ActorID, event.ActorType, event.ActorIP,
		event.SubjectID, event.SubjectCPF,
		event.Resource, event.Action, event.Description, string(fieldsJSON),
		string(metadataJSON), event.RetentionDays, event.Success, event.ErrorMessage,
	)

	if err != nil {
		log.Printf("❌ [LGPD Audit] Failed to insert event: %v", err)
		return err
	}

	return nil
}

func (s *LGPDAuditService) validateLegalBasis(event AuditEvent) error {
	// Sensitive data (health data) requires explicit consent or health protection basis
	if event.DataCategory == CategorySensitive || event.DataCategory == CategoryClinical {
		validBases := []LegalBasis{BasisConsent, BasisHealthProtection, BasisLegalObligation}
		for _, valid := range validBases {
			if event.LegalBasis == valid {
				return nil
			}
		}
		return fmt.Errorf("sensitive data requires explicit consent or health protection legal basis")
	}
	return nil
}

func (s *LGPDAuditService) inferDataCategory(resource string) DataCategory {
	switch resource {
	case "clinical_assessments", "phq9_results", "gad7_results", "cssrs_results":
		return CategoryClinical
	case "memorias_episodicas", "memorias_semanticas", "conversas":
		return CategoryConversation
	case "idosos", "cuidadores":
		return CategoryPersonal
	case "alertas":
		return CategorySensitive
	default:
		return CategoryPersonal
	}
}

func (s *LGPDAuditService) getDefaultRetention(category DataCategory) int {
	switch category {
	case CategoryClinical:
		return 5 * 365 // 5 years for clinical data (regulatory requirement)
	case CategorySensitive:
		return 3 * 365 // 3 years for sensitive data
	case CategoryConversation:
		return 2 * 365 // 2 years for conversations
	default:
		return 365 // 1 year default
	}
}

func (s *LGPDAuditService) checkDataExists(ctx context.Context, table string, subjectID int64) (bool, error) {
	if s.db == nil {
		return false, nil
	}

	// Determine the ID column name based on table
	idColumn := "idoso_id"
	if table == "idosos" {
		idColumn = "id"
	}

	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE %s = $1)", table, idColumn)

	var exists bool
	err := s.db.QueryRowContext(ctx, query, subjectID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Close stops the audit service
func (s *LGPDAuditService) Close() {
	close(s.stopChan)
}
