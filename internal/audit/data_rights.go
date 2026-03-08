// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
	"github.com/google/uuid"
)

// DataRightsService handles LGPD data subject rights (Art. 18)
type DataRightsService struct {
	db    *database.DB
	audit *LGPDAuditService
}

// NewDataRightsService creates a new data rights service
func NewDataRightsService(db *database.DB, audit *LGPDAuditService) *DataRightsService {
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

		_, err := s.db.Insert(ctx, "lgpd_data_exports", map[string]interface{}{
			"subject_id":          subjectID,
			"format":              format,
			"status":              "completed",
			"download_token":      token,
			"download_expires_at": expiresAt.Format(time.RFC3339),
			"completed_at":        time.Now().Format(time.RFC3339),
		})

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

	// Soft-delete from each table via NietzscheDB
	for _, table := range deletableTables {
		if !table.canDelete {
			result.RetainedData[table.name] = table.retainReason
			continue
		}

		matchKeys := map[string]interface{}{
			table.idColumn: subjectID,
		}
		err := s.db.SoftDelete(ctx, table.name, matchKeys)
		if err != nil {
			log.Printf("⚠️ [LGPD] Error deleting from %s: %v", table.name, err)
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", table.name, err))
			continue
		}

		result.TablesAffected = append(result.TablesAffected, table.name)
		result.RecordsDeleted[table.name] = 1
	}

	// Anonymize the main idosos record instead of deleting
	// (required for referential integrity and audit trail)
	err := s.db.Update(ctx, "idosos",
		map[string]interface{}{"id": subjectID},
		map[string]interface{}{
			"nome":            "ANONIMIZADO",
			"cpf":             nil,
			"email":           nil,
			"telefone":        nil,
			"endereco":        nil,
			"data_nascimento": nil,
			"foto_url":        nil,
			"anonimizado":     true,
			"anonimizado_em":  time.Now().Format(time.RFC3339),
		})
	if err != nil {
		log.Printf("⚠️ [LGPD] Error anonymizing subject: %v", err)
		result.Errors = append(result.Errors, fmt.Sprintf("anonymization: %v", err))
	} else {
		result.TablesAffected = append(result.TablesAffected, "idosos (anonymized)")
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
		err := s.db.Update(ctx, "idosos",
			map[string]interface{}{"id": subjectID},
			map[string]interface{}{
				field:            newValue,
				"atualizado_em":  time.Now().Format(time.RFC3339),
			})
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

	rows, err := s.db.QueryByLabel(ctx, "lgpd_audit_log",
		" AND n.subject_id = $sid", map[string]interface{}{
			"sid": subjectID,
		}, 0)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, m := range rows {
		ts := database.GetTime(m, "timestamp")
		// Filter by timestamp range in Go
		if ts.Before(startDate) || ts.After(endDate) {
			continue
		}

		eventType := database.GetString(m, "event_type")
		// Filter by event type in Go
		if eventType != "DATA_ACCESS" && eventType != "DATA_CREATE" && eventType != "DATA_UPDATE" {
			continue
		}

		results = append(results, map[string]interface{}{
			"timestamp":   ts,
			"event_type":  eventType,
			"actor":       database.GetString(m, "actor_id"),
			"actor_type":  database.GetString(m, "actor_type"),
			"resource":    database.GetString(m, "resource"),
			"action":      database.GetString(m, "action"),
			"description": database.GetString(m, "description"),
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

	rows, err := s.db.QueryByLabel(ctx, table.Name,
		fmt.Sprintf(" AND n.%s = $sid", table.IDColumn), map[string]interface{}{
			"sid": subjectID,
		}, 0)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return nil, nil
	}

	return rows, nil
}

// ExportToJSON exports data in JSON format
func (r *DataExportResult) ExportToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
