package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// DataRightsService handles LGPD data subject rights (Art. 18)
type DataRightsService struct {
	db    *sql.DB
	audit *LGPDAuditService
}

// NewDataRightsService creates a new data rights service
func NewDataRightsService(db *sql.DB, audit *LGPDAuditService) *DataRightsService {
	return &DataRightsService{
		db:    db,
		audit: audit,
	}
}

// DataExportResult represents the result of a data export
type DataExportResult struct {
	SubjectID    int64                  `json:"subject_id"`
	ExportedAt   time.Time              `json:"exported_at"`
	Format       string                 `json:"format"`
	Data         map[string]interface{} `json:"data"`
	DownloadToken string                `json:"download_token,omitempty"`
	ExpiresAt    time.Time              `json:"expires_at,omitempty"`
}

// DeletionResult represents the result of a data deletion
type DeletionResult struct {
	SubjectID      int64            `json:"subject_id"`
	DeletedAt      time.Time        `json:"deleted_at"`
	TablesAffected []string         `json:"tables_affected"`
	RecordsDeleted map[string]int   `json:"records_deleted"`
	RetainedData   map[string]string `json:"retained_data,omitempty"`
	Success        bool             `json:"success"`
	Errors         []string         `json:"errors,omitempty"`
}

// ExportableTable represents a table that can be exported
type ExportableTable struct {
	Name        string
	IDColumn    string
	Category    DataCategory
	Description string
}

// GetExportableTables returns all tables containing personal data
func GetExportableTables() []ExportableTable {
	return []ExportableTable{
		{"idosos", "id", CategoryPersonal, "Personal information"},
		{"memorias_episodicas", "idoso_id", CategoryConversation, "Episodic memories"},
		{"memorias_semanticas", "idoso_id", CategoryConversation, "Semantic memories"},
		{"memorias_procedurais", "idoso_id", CategoryBehavioral, "Procedural memories"},
		{"conversas", "idoso_id", CategoryConversation, "Conversation history"},
		{"clinical_assessments", "patient_id", CategoryClinical, "Clinical assessments"},
		{"alertas", "idoso_id", CategorySensitive, "Emergency alerts"},
		{"agendamentos", "idoso_id", CategoryPersonal, "Appointments"},
		{"lgpd_consents", "subject_id", CategoryPersonal, "Consent records"},
	}
}

// ExportPersonalData exports all personal data for a subject (Art. 18, V - Portability)
func (s *DataRightsService) ExportPersonalData(ctx context.Context, subjectID int64, format string) (*DataExportResult, error) {
	log.Printf("📤 [LGPD] Starting data export for subject %d", subjectID)

	result := &DataExportResult{
		SubjectID:  subjectID,
		ExportedAt: time.Now(),
		Format:     format,
		Data:       make(map[string]interface{}),
	}

	// Export from each table
	for _, table := range GetExportableTables() {
		data, err := s.exportTableData(ctx, table, subjectID)
		if err != nil {
			log.Printf("⚠️ [LGPD] Error exporting %s: %v", table.Name, err)
			continue
		}
		if data != nil {
			result.Data[table.Name] = map[string]interface{}{
				"category":    string(table.Category),
				"description": table.Description,
				"records":     data,
			}
		}
	}

	// Generate download token if database available
	if s.db != nil {
		token := uuid.New().String()
		expiresAt := time.Now().Add(24 * time.Hour)

		_, err := s.db.ExecContext(ctx, `
			INSERT INTO lgpd_data_exports (
				subject_id, format, status,
				download_token, download_expires_at, completed_at
			) VALUES ($1, $2, 'completed', $3, $4, NOW())
		`, subjectID, format, token, expiresAt)

		if err == nil {
			result.DownloadToken = token
			result.ExpiresAt = expiresAt
		}
	}

	// Log the export
	if s.audit != nil {
		s.audit.LogRightExercised(ctx, subjectID, EventRightToPortability,
			fmt.Sprintf("Data export completed in %s format", format))
	}

	log.Printf("✅ [LGPD] Data export completed for subject %d (%d tables)", subjectID, len(result.Data))

	return result, nil
}

// DeletePersonalData deletes personal data for a subject (Art. 18, VI - Right to Erasure)
func (s *DataRightsService) DeletePersonalData(ctx context.Context, subjectID int64, retainAuditLog bool) (*DeletionResult, error) {
	log.Printf("🗑️ [LGPD] Starting data deletion for subject %d", subjectID)

	result := &DeletionResult{
		SubjectID:      subjectID,
		DeletedAt:      time.Now(),
		TablesAffected: []string{},
		RecordsDeleted: make(map[string]int),
		RetainedData:   make(map[string]string),
		Success:        true,
	}

	// Tables that can be deleted (order matters due to foreign keys)
	deletableTables := []struct {
		name     string
		idColumn string
		canDelete bool
		retainReason string
	}{
		{"conversas", "idoso_id", true, ""},
		{"memorias_episodicas", "idoso_id", true, ""},
		{"memorias_semanticas", "idoso_id", true, ""},
		{"memorias_procedurais", "idoso_id", true, ""},
		{"agendamentos", "idoso_id", true, ""},
		{"alertas", "idoso_id", true, ""},
		// Clinical data - retain for legal/medical reasons
		{"clinical_assessments", "patient_id", false, "Medical records retention requirement (5 years)"},
		// LGPD logs - retain for compliance
		{"lgpd_audit_log", "subject_id", !retainAuditLog, "LGPD compliance audit trail"},
	}

	if s.db == nil {
		// Simulation mode
		for _, table := range deletableTables {
			if table.canDelete {
				result.TablesAffected = append(result.TablesAffected, table.name)
				result.RecordsDeleted[table.name] = 1 // Simulated
			} else {
				result.RetainedData[table.name] = table.retainReason
			}
		}
		log.Printf("✅ [LGPD] Simulated deletion for subject %d", subjectID)
		return result, nil
	}

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete from each table
	for _, table := range deletableTables {
		if !table.canDelete {
			result.RetainedData[table.name] = table.retainReason
			continue
		}

		query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table.name, table.idColumn)
		res, err := tx.ExecContext(ctx, query, subjectID)
		if err != nil {
			log.Printf("⚠️ [LGPD] Error deleting from %s: %v", table.name, err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", table.name, err))
			continue
		}

		rowsAffected, _ := res.RowsAffected()
		if rowsAffected > 0 {
			result.TablesAffected = append(result.TablesAffected, table.name)
			result.RecordsDeleted[table.name] = int(rowsAffected)
		}
	}

	// Anonymize the main idosos record instead of deleting
	// (required for referential integrity and audit trail)
	anonymizeQuery := `
		UPDATE idosos SET
			nome = 'ANONIMIZADO',
			cpf = NULL,
			email = NULL,
			telefone = NULL,
			endereco = NULL,
			data_nascimento = NULL,
			foto_url = NULL,
			anonimizado = true,
			anonimizado_em = NOW()
		WHERE id = $1
	`
	_, err = tx.ExecContext(ctx, anonymizeQuery, subjectID)
	if err != nil {
		log.Printf("⚠️ [LGPD] Error anonymizing subject: %v", err)
		result.Errors = append(result.Errors, fmt.Sprintf("anonymization: %v", err))
	} else {
		result.TablesAffected = append(result.TablesAffected, "idosos (anonymized)")
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit deletion: %w", err)
	}

	if len(result.Errors) > 0 {
		result.Success = false
	}

	// Log the deletion
	if s.audit != nil {
		totalDeleted := 0
		for _, count := range result.RecordsDeleted {
			totalDeleted += count
		}
		s.audit.LogRightExercised(ctx, subjectID, EventRightToDelete,
			fmt.Sprintf("Data deletion completed: %d records deleted from %d tables",
				totalDeleted, len(result.TablesAffected)))
	}

	log.Printf("✅ [LGPD] Data deletion completed for subject %d (%d tables)", subjectID, len(result.TablesAffected))

	return result, nil
}

// RectifyPersonalData corrects personal data (Art. 18, III - Right to Rectification)
func (s *DataRightsService) RectifyPersonalData(ctx context.Context, subjectID int64, field string, oldValue, newValue string) error {
	log.Printf("✏️ [LGPD] Rectifying %s for subject %d", field, subjectID)

	// Validate field is rectifiable
	allowedFields := map[string]bool{
		"nome": true, "email": true, "telefone": true, "endereco": true,
		"data_nascimento": true, "genero": true,
	}

	if !allowedFields[field] {
		return fmt.Errorf("field %s cannot be rectified", field)
	}

	if s.db != nil {
		query := fmt.Sprintf("UPDATE idosos SET %s = $1, atualizado_em = NOW() WHERE id = $2", field)
		_, err := s.db.ExecContext(ctx, query, newValue, subjectID)
		if err != nil {
			return fmt.Errorf("failed to rectify: %w", err)
		}
	}

	// Log the rectification
	if s.audit != nil {
		s.audit.LogRightExercised(ctx, subjectID, EventRightToRectify,
			fmt.Sprintf("Field %s rectified", field))
	}

	log.Printf("✅ [LGPD] Rectification completed for subject %d", subjectID)

	return nil
}

// GetDataAccessReport generates a report of all data access for a subject (Art. 18, VII)
func (s *DataRightsService) GetDataAccessReport(ctx context.Context, subjectID int64, startDate, endDate time.Time) ([]map[string]interface{}, error) {
	if s.db == nil {
		// Return mock data for testing
		return []map[string]interface{}{
			{
				"timestamp":    time.Now(),
				"event_type":   "DATA_ACCESS",
				"actor":        "eva-system",
				"resource":     "conversas",
				"description":  "Conversation data accessed",
			},
		}, nil
	}

	query := `
		SELECT timestamp, event_type, actor_id, actor_type, resource, action, description
		FROM lgpd_audit_log
		WHERE subject_id = $1
		  AND timestamp BETWEEN $2 AND $3
		  AND event_type IN ('DATA_ACCESS', 'DATA_CREATE', 'DATA_UPDATE')
		ORDER BY timestamp DESC
	`

	rows, err := s.db.QueryContext(ctx, query, subjectID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var timestamp time.Time
		var eventType, actorID, actorType, resource, action, description string

		if err := rows.Scan(&timestamp, &eventType, &actorID, &actorType, &resource, &action, &description); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"timestamp":   timestamp,
			"event_type":  eventType,
			"actor":       actorID,
			"actor_type":  actorType,
			"resource":    resource,
			"action":      action,
			"description": description,
		})
	}

	// Log the access report request
	if s.audit != nil {
		s.audit.LogRightExercised(ctx, subjectID, EventRightToAccess,
			fmt.Sprintf("Data access report generated for period %s to %s",
				startDate.Format("2006-01-02"), endDate.Format("2006-01-02")))
	}

	return results, nil
}

// Helper methods

func (s *DataRightsService) exportTableData(ctx context.Context, table ExportableTable, subjectID int64) (interface{}, error) {
	if s.db == nil {
		// Return mock data for testing
		return []map[string]interface{}{
			{"id": 1, "data": "sample"},
		}, nil
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", table.Name, table.IDColumn)
	rows, err := s.db.QueryContext(ctx, query, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if len(results) == 0 {
		return nil, nil
	}

	return results, nil
}

// ExportToJSON exports data in JSON format
func (r *DataExportResult) ExportToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
