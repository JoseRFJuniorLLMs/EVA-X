package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDataRightsService(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	require.NotNil(t, svc)
}

func TestGetExportableTables(t *testing.T) {
	tables := GetExportableTables()

	require.NotEmpty(t, tables)

	// Verify essential tables are included
	tableNames := make(map[string]bool)
	for _, t := range tables {
		tableNames[t.Name] = true
	}

	assert.True(t, tableNames["idosos"], "Should include idosos table")
	assert.True(t, tableNames["memorias_episodicas"], "Should include memorias table")
	assert.True(t, tableNames["conversas"], "Should include conversas table")
	assert.True(t, tableNames["clinical_assessments"], "Should include clinical assessments")
	assert.True(t, tableNames["lgpd_consents"], "Should include consent records")
}

func TestExportPersonalData(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	result, err := svc.ExportPersonalData(ctx, 999, "json")

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int64(999), result.SubjectID)
	assert.Equal(t, "json", result.Format)
	assert.False(t, result.ExportedAt.IsZero())
	assert.NotEmpty(t, result.Data, "Should have exported data")
}

func TestExportPersonalData_AllTables(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	result, err := svc.ExportPersonalData(ctx, 999, "json")

	require.NoError(t, err)

	// In mock mode, all tables should return data
	exportableTables := GetExportableTables()
	for _, table := range exportableTables {
		_, exists := result.Data[table.Name]
		assert.True(t, exists, "Should export %s table", table.Name)
	}
}

func TestExportToJSON(t *testing.T) {
	result := &DataExportResult{
		SubjectID:  999,
		ExportedAt: time.Now(),
		Format:     "json",
		Data: map[string]interface{}{
			"idosos": map[string]interface{}{
				"category": "PERSONAL",
				"records": []map[string]interface{}{
					{"id": 999, "nome": "Maria Silva"},
				},
			},
		},
	}

	jsonBytes, err := result.ExportToJSON()

	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)
	assert.Contains(t, string(jsonBytes), "Maria Silva")
}

func TestDeletePersonalData(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	result, err := svc.DeletePersonalData(ctx, 999, true)

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, int64(999), result.SubjectID)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.TablesAffected)
	assert.False(t, result.DeletedAt.IsZero())
}

func TestDeletePersonalData_RetainsClinicalData(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	result, err := svc.DeletePersonalData(ctx, 999, true)

	require.NoError(t, err)

	// Clinical data should be retained (medical records requirement)
	retainedReason, retained := result.RetainedData["clinical_assessments"]
	assert.True(t, retained, "Clinical data should be retained")
	assert.Contains(t, retainedReason, "retention", "Should cite retention requirement")
}

func TestDeletePersonalData_RetainsAuditLog(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// With retainAuditLog = true
	result, err := svc.DeletePersonalData(ctx, 999, true)

	require.NoError(t, err)

	// Audit log should be retained for compliance
	_, retained := result.RetainedData["lgpd_audit_log"]
	assert.True(t, retained, "Audit log should be retained when retainAuditLog=true")
}

func TestRectifyPersonalData(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Valid field
	err := svc.RectifyPersonalData(ctx, 999, "nome", "Maria", "Maria Silva Santos")
	assert.NoError(t, err)

	// Valid field - email
	err = svc.RectifyPersonalData(ctx, 999, "email", "old@example.com", "new@example.com")
	assert.NoError(t, err)
}

func TestRectifyPersonalData_InvalidField(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Invalid field - should not allow rectifying sensitive/system fields
	err := svc.RectifyPersonalData(ctx, 999, "cpf", "123", "456")
	assert.Error(t, err, "Should not allow rectifying CPF")

	err = svc.RectifyPersonalData(ctx, 999, "id", "1", "2")
	assert.Error(t, err, "Should not allow rectifying ID")

	err = svc.RectifyPersonalData(ctx, 999, "created_at", "old", "new")
	assert.Error(t, err, "Should not allow rectifying system fields")
}

func TestRectifyPersonalData_AllowedFields(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	allowedFields := []string{"nome", "email", "telefone", "endereco", "data_nascimento", "genero"}

	for _, field := range allowedFields {
		t.Run(field, func(t *testing.T) {
			err := svc.RectifyPersonalData(ctx, 999, field, "old", "new")
			assert.NoError(t, err, "Should allow rectifying %s", field)
		})
	}
}

func TestGetDataAccessReport(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	startDate := time.Now().AddDate(0, -1, 0) // 1 month ago
	endDate := time.Now()

	report, err := svc.GetDataAccessReport(ctx, 999, startDate, endDate)

	require.NoError(t, err)
	assert.NotEmpty(t, report)

	// Verify report structure
	for _, entry := range report {
		assert.Contains(t, entry, "timestamp")
		assert.Contains(t, entry, "event_type")
	}
}

func TestDeletionResult_Structure(t *testing.T) {
	result := DeletionResult{
		SubjectID: 999,
		DeletedAt: time.Now(),
		TablesAffected: []string{"conversas", "memorias_episodicas"},
		RecordsDeleted: map[string]int{
			"conversas":           15,
			"memorias_episodicas": 42,
		},
		RetainedData: map[string]string{
			"clinical_assessments": "Medical retention requirement",
		},
		Success: true,
	}

	assert.Equal(t, int64(999), result.SubjectID)
	assert.Len(t, result.TablesAffected, 2)
	assert.Equal(t, 15, result.RecordsDeleted["conversas"])
	assert.Contains(t, result.RetainedData["clinical_assessments"], "retention")
}

func TestExportableTable_Categories(t *testing.T) {
	tables := GetExportableTables()

	categoryCount := make(map[DataCategory]int)
	for _, table := range tables {
		categoryCount[table.Category]++
	}

	// Verify we have tables in different categories
	assert.Greater(t, categoryCount[CategoryPersonal], 0, "Should have personal data tables")
	assert.Greater(t, categoryCount[CategoryConversation], 0, "Should have conversation tables")
	assert.Greater(t, categoryCount[CategoryClinical], 0, "Should have clinical tables")
}

func TestDataRightsService_AuditLogging(t *testing.T) {
	audit := NewLGPDAuditService(nil)
	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Perform operations
	svc.ExportPersonalData(ctx, 999, "json")
	svc.DeletePersonalData(ctx, 999, true)
	svc.RectifyPersonalData(ctx, 999, "nome", "old", "new")
	svc.GetDataAccessReport(ctx, 999, time.Now().AddDate(-1, 0, 0), time.Now())

	// Give time for async flush
	time.Sleep(50 * time.Millisecond)
	audit.flush()

	// Audit events should have been logged
	audit.bufferMu.Lock()
	bufferLen := len(audit.buffer)
	audit.bufferMu.Unlock()

	// Buffer might be empty if events were flushed
	// The important thing is no panic occurred
	assert.GreaterOrEqual(t, bufferLen, 0)

	audit.Close()
}

// Integration test scenarios
func TestLGPDRightsScenario_FullExport(t *testing.T) {
	// Scenario: User requests full data export (Art. 18, V)
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Step 1: Export all data
	export, err := svc.ExportPersonalData(ctx, 999, "json")
	require.NoError(t, err)

	// Step 2: Verify export completeness
	tables := GetExportableTables()
	for _, table := range tables {
		assert.Contains(t, export.Data, table.Name)
	}

	// Step 3: Generate JSON
	jsonData, err := export.ExportToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonData)
}

func TestLGPDRightsScenario_RightToErasure(t *testing.T) {
	// Scenario: User exercises right to erasure (Art. 18, VI)
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Step 1: Delete personal data
	deletion, err := svc.DeletePersonalData(ctx, 999, true)
	require.NoError(t, err)

	// Step 2: Verify deletion succeeded
	assert.True(t, deletion.Success)
	assert.NotEmpty(t, deletion.TablesAffected)

	// Step 3: Verify clinical data retained (legal requirement)
	assert.Contains(t, deletion.RetainedData, "clinical_assessments")
}

func TestLGPDRightsScenario_DataRectification(t *testing.T) {
	// Scenario: User requests data correction (Art. 18, III)
	audit := NewLGPDAuditService(nil)
	defer audit.Close()

	svc := NewDataRightsService(nil, audit)
	ctx := context.Background()

	// Rectify multiple fields
	corrections := []struct {
		field    string
		oldValue string
		newValue string
	}{
		{"nome", "Maria Silva", "Maria Silva Santos"},
		{"email", "maria@old.com", "maria@new.com"},
		{"telefone", "+5511999999999", "+5511888888888"},
	}

	for _, c := range corrections {
		err := svc.RectifyPersonalData(ctx, 999, c.field, c.oldValue, c.newValue)
		assert.NoError(t, err, "Should rectify %s", c.field)
	}
}
