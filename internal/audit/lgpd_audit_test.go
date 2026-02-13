package audit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditEventTypes(t *testing.T) {
	// Verify all event types are defined
	assert.Equal(t, AuditEventType("DATA_ACCESS"), EventDataAccess)
	assert.Equal(t, AuditEventType("DATA_CREATE"), EventDataCreate)
	assert.Equal(t, AuditEventType("DATA_UPDATE"), EventDataUpdate)
	assert.Equal(t, AuditEventType("DATA_DELETE"), EventDataDelete)
	assert.Equal(t, AuditEventType("DATA_EXPORT"), EventDataExport)

	assert.Equal(t, AuditEventType("CONSENT_GIVEN"), EventConsentGiven)
	assert.Equal(t, AuditEventType("CONSENT_REVOKED"), EventConsentRevoked)

	assert.Equal(t, AuditEventType("CLINICAL_ASSESSMENT"), EventClinicalAssessment)
	assert.Equal(t, AuditEventType("ALERT_SENT"), EventAlertSent)

	assert.Equal(t, AuditEventType("RIGHT_TO_ACCESS"), EventRightToAccess)
	assert.Equal(t, AuditEventType("RIGHT_TO_DELETE"), EventRightToDelete)
	assert.Equal(t, AuditEventType("RIGHT_TO_PORTABILITY"), EventRightToPortability)
}

func TestDataCategories(t *testing.T) {
	// Verify LGPD data categories
	assert.Equal(t, DataCategory("PERSONAL"), CategoryPersonal)
	assert.Equal(t, DataCategory("SENSITIVE"), CategorySensitive)
	assert.Equal(t, DataCategory("BIOMETRIC"), CategoryBiometric)
	assert.Equal(t, DataCategory("CLINICAL"), CategoryClinical)
}

func TestLegalBasis(t *testing.T) {
	// Verify Art. 7 LGPD legal bases
	assert.Equal(t, LegalBasis("CONSENT"), BasisConsent)
	assert.Equal(t, LegalBasis("LEGAL_OBLIGATION"), BasisLegalObligation)
	assert.Equal(t, LegalBasis("HEALTH_PROTECTION"), BasisHealthProtection)
	assert.Equal(t, LegalBasis("LEGITIMATE_INTEREST"), BasisLegitimateInterest)
}

func TestNewLGPDAuditService(t *testing.T) {
	// Create service without database (for testing)
	svc := NewLGPDAuditService(nil)
	require.NotNil(t, svc)

	// Should have buffer initialized
	assert.NotNil(t, svc.buffer)
	assert.Equal(t, 50, svc.flushSize)

	// Cleanup
	svc.Close()
}

func TestLogEvent(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	event := AuditEvent{
		EventType:    EventDataAccess,
		DataCategory: CategoryPersonal,
		LegalBasis:   BasisConsent,
		ActorID:      "user-123",
		ActorType:    "user",
		SubjectID:    999,
		Resource:     "idosos",
		Action:       "READ",
		Description:  "Test access",
		Success:      true,
	}

	err := svc.LogEvent(ctx, event)
	assert.NoError(t, err)

	// Event should be in buffer
	svc.bufferMu.Lock()
	assert.GreaterOrEqual(t, len(svc.buffer), 1)
	svc.bufferMu.Unlock()
}

func TestLogEvent_SetsDefaults(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	event := AuditEvent{
		EventType:    EventDataAccess,
		DataCategory: CategoryPersonal,
		LegalBasis:   BasisConsent,
		ActorID:      "user-123",
		ActorType:    "user",
		SubjectID:    999,
		Resource:     "idosos",
		Action:       "READ",
		Success:      true,
		// Timestamp and ID not set - should be auto-generated
	}

	err := svc.LogEvent(ctx, event)
	assert.NoError(t, err)

	// Check that defaults were set
	svc.bufferMu.Lock()
	if len(svc.buffer) > 0 {
		lastEvent := svc.buffer[len(svc.buffer)-1]
		assert.NotEmpty(t, lastEvent.ID)
		assert.False(t, lastEvent.Timestamp.IsZero())
		assert.Greater(t, lastEvent.RetentionDays, 0)
	}
	svc.bufferMu.Unlock()
}

func TestLogDataAccess(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	err := svc.LogDataAccess(ctx, 999, "system", "idosos", []string{"nome", "cpf"}, BasisConsent)
	assert.NoError(t, err)
}

func TestLogClinicalAssessment(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	// Log C-SSRS assessment
	err := svc.LogClinicalAssessment(ctx, 999, "C-SSRS", "high")
	assert.NoError(t, err)

	// Log PHQ-9 assessment
	err = svc.LogClinicalAssessment(ctx, 999, "PHQ-9", "moderate")
	assert.NoError(t, err)

	// Log GAD-7 assessment
	err = svc.LogClinicalAssessment(ctx, 999, "GAD-7", "severe")
	assert.NoError(t, err)
}

func TestLogAlertSent(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	err := svc.LogAlertSent(ctx, 999, "critica", "push", "caregiver@example.com")
	assert.NoError(t, err)

	err = svc.LogAlertSent(ctx, 999, "alta", "sms", "+5511999999999")
	assert.NoError(t, err)
}

func TestLogConsentChange(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	// Log consent granted
	err := svc.LogConsentChange(ctx, 999, "health_data", true, "Clinical assessments")
	assert.NoError(t, err)

	// Log consent revoked
	err = svc.LogConsentChange(ctx, 999, "research", false, "Academic research")
	assert.NoError(t, err)
}

func TestLogRightExercised(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	ctx := context.Background()

	// Right to access
	err := svc.LogRightExercised(ctx, 999, EventRightToAccess, "Data subject requested access to all personal data")
	assert.NoError(t, err)

	// Right to delete
	err = svc.LogRightExercised(ctx, 999, EventRightToDelete, "Data subject requested deletion of conversation history")
	assert.NoError(t, err)

	// Right to portability
	err = svc.LogRightExercised(ctx, 999, EventRightToPortability, "Data subject requested data export in JSON format")
	assert.NoError(t, err)
}

func TestValidateLegalBasis_SensitiveData(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	// Sensitive data with invalid legal basis should fail validation
	event := AuditEvent{
		EventType:    EventDataAccess,
		DataCategory: CategorySensitive,
		LegalBasis:   BasisLegitimateInterest, // Invalid for sensitive data
		ActorID:      "user-123",
		ActorType:    "user",
		SubjectID:    999,
		Resource:     "clinical_assessments",
		Action:       "READ",
	}

	err := svc.validateLegalBasis(event)
	assert.Error(t, err, "Should reject legitimate interest for sensitive data")

	// Sensitive data with valid legal basis (consent) should pass
	event.LegalBasis = BasisConsent
	err = svc.validateLegalBasis(event)
	assert.NoError(t, err)

	// Sensitive data with health protection should pass
	event.LegalBasis = BasisHealthProtection
	err = svc.validateLegalBasis(event)
	assert.NoError(t, err)
}

func TestValidateLegalBasis_ClinicalData(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	// Clinical data requires special legal bases
	event := AuditEvent{
		EventType:    EventClinicalAssessment,
		DataCategory: CategoryClinical,
		LegalBasis:   BasisLegitimateInterest, // Invalid for clinical
		ActorID:      "system",
		ActorType:    "system",
		SubjectID:    999,
		Resource:     "clinical_assessments",
		Action:       "ASSESS",
	}

	err := svc.validateLegalBasis(event)
	assert.Error(t, err, "Should reject legitimate interest for clinical data")

	// Health protection is valid for clinical data
	event.LegalBasis = BasisHealthProtection
	err = svc.validateLegalBasis(event)
	assert.NoError(t, err)
}

func TestInferDataCategory(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	testCases := []struct {
		resource string
		expected DataCategory
	}{
		{"clinical_assessments", CategoryClinical},
		{"phq9_results", CategoryClinical},
		{"gad7_results", CategoryClinical},
		{"cssrs_results", CategoryClinical},
		{"memorias_episodicas", CategoryConversation},
		{"conversas", CategoryConversation},
		{"idosos", CategoryPersonal},
		{"cuidadores", CategoryPersonal},
		{"alertas", CategorySensitive},
		{"unknown_table", CategoryPersonal}, // Default
	}

	for _, tc := range testCases {
		t.Run(tc.resource, func(t *testing.T) {
			result := svc.inferDataCategory(tc.resource)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetDefaultRetention(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	defer svc.Close()

	testCases := []struct {
		category DataCategory
		expected int
	}{
		{CategoryClinical, 5 * 365},   // 5 years
		{CategorySensitive, 3 * 365},  // 3 years
		{CategoryConversation, 2 * 365}, // 2 years
		{CategoryPersonal, 365},       // 1 year
	}

	for _, tc := range testCases {
		t.Run(string(tc.category), func(t *testing.T) {
			result := svc.getDefaultRetention(tc.category)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAuditEvent_Structure(t *testing.T) {
	event := AuditEvent{
		ID:            "aud-123",
		Timestamp:     time.Now(),
		EventType:     EventClinicalAssessment,
		DataCategory:  CategoryClinical,
		LegalBasis:    BasisHealthProtection,
		ActorID:       "eva-system",
		ActorType:     "system",
		SubjectID:     999,
		SubjectCPF:    "hashed-cpf",
		Resource:      "clinical_assessments",
		Action:        "ASSESS",
		Description:   "C-SSRS assessment completed",
		FieldsAccessed: []string{"score", "risk_level"},
		Metadata: map[string]interface{}{
			"assessment_type": "C-SSRS",
			"risk_level":      "high",
		},
		RetentionDays: 1825, // 5 years
		Success:       true,
	}

	assert.NotEmpty(t, event.ID)
	assert.False(t, event.Timestamp.IsZero())
	assert.Equal(t, EventClinicalAssessment, event.EventType)
	assert.Equal(t, CategoryClinical, event.DataCategory)
	assert.Equal(t, BasisHealthProtection, event.LegalBasis)
	assert.Equal(t, int64(999), event.SubjectID)
	assert.Len(t, event.FieldsAccessed, 2)
	assert.NotNil(t, event.Metadata["assessment_type"])
}

func TestLGPDCompliance_Art18Rights(t *testing.T) {
	// Test that all Art. 18 rights are representable
	rights := []AuditEventType{
		EventRightToAccess,     // Art. 18, II - acesso aos dados
		EventRightToRectify,    // Art. 18, III - correção
		EventRightToDelete,     // Art. 18, VI - eliminação
		EventRightToPortability, // Art. 18, V - portabilidade
	}

	for _, right := range rights {
		assert.NotEmpty(t, right, "All Art. 18 rights should be defined")
	}
}

func TestBufferFlush(t *testing.T) {
	svc := NewLGPDAuditService(nil)
	svc.flushSize = 5 // Low threshold for testing
	defer svc.Close()

	ctx := context.Background()

	// Add events
	for i := 0; i < 10; i++ {
		svc.LogEvent(ctx, AuditEvent{
			EventType:    EventDataAccess,
			DataCategory: CategoryPersonal,
			LegalBasis:   BasisConsent,
			ActorID:      "test",
			ActorType:    "test",
			SubjectID:    int64(i),
			Resource:     "test",
			Action:       "READ",
			Success:      true,
		})
	}

	// Give time for flush
	time.Sleep(100 * time.Millisecond)

	// Manual flush to ensure all events are processed
	svc.flush()

	// Buffer should be empty or smaller after flush
	svc.bufferMu.Lock()
	bufferLen := len(svc.buffer)
	svc.bufferMu.Unlock()

	assert.LessOrEqual(t, bufferLen, 5, "Buffer should be flushed")
}
