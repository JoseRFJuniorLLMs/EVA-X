package integration

import (
	"encoding/json"
	"time"
)

// ============================================================================
// JSON SERIALIZERS
// ============================================================================
// Helpers para serializar dados EVA-Mind para JSON limpo (para API Python)

// ============================================================================
// PATIENT DTOs
// ============================================================================

type PatientDTO struct {
	ID                 int64     `json:"id"`
	Name               string    `json:"name"`
	DateOfBirth        string    `json:"date_of_birth"` // ISO 8601: YYYY-MM-DD
	Age                int       `json:"age"`
	Gender             string    `json:"gender"`
	Email              *string   `json:"email,omitempty"`
	Phone              *string   `json:"phone,omitempty"`
	Address            *string   `json:"address,omitempty"`
	EmergencyContact   *string   `json:"emergency_contact,omitempty"`
	HealthConditions   []string  `json:"health_conditions,omitempty"`
	CurrentMedications []string  `json:"current_medications,omitempty"`
	Allergies          []string  `json:"allergies,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type PatientListDTO struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Age         int    `json:"age"`
	Gender      string `json:"gender"`
	RiskLevel   string `json:"risk_level,omitempty"` // 'low', 'moderate', 'high', 'critical'
	LastContact string `json:"last_contact,omitempty"`
}

// ============================================================================
// ASSESSMENT DTOs
// ============================================================================

type AssessmentDTO struct {
	ID             string                 `json:"id"`
	PatientID      int64                  `json:"patient_id"`
	AssessmentType string                 `json:"assessment_type"` // 'PHQ-9', 'GAD-7', 'C-SSRS', 'WHOQOL-BREF'
	Status         string                 `json:"status"`          // 'pending', 'in_progress', 'completed', 'cancelled'
	TotalScore     *int                   `json:"total_score,omitempty"`
	Severity       *string                `json:"severity,omitempty"` // 'minimal', 'mild', 'moderate', 'severe'
	Responses      map[string]interface{} `json:"responses,omitempty"`
	StartedAt      *time.Time             `json:"started_at,omitempty"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
	AdministeredBy string                 `json:"administered_by"` // 'eva', 'clinician', 'self'
	Notes          *string                `json:"notes,omitempty"`
	Flags          []string               `json:"flags,omitempty"` // ['suicidal_ideation', 'severe_depression']
	CreatedAt      time.Time              `json:"created_at"`
}

type AssessmentSummaryDTO struct {
	AssessmentType string     `json:"assessment_type"`
	Count          int        `json:"count"`
	LatestScore    *int       `json:"latest_score,omitempty"`
	LatestDate     *time.Time `json:"latest_date,omitempty"`
	Trend          *string    `json:"trend,omitempty"` // 'improving', 'stable', 'worsening'
}

// ============================================================================
// VOICE ANALYSIS DTOs
// ============================================================================

type VoiceAnalysisDTO struct {
	ID               string                 `json:"id"`
	PatientID        int64                  `json:"patient_id"`
	RecordingDate    time.Time              `json:"recording_date"`
	AudioDuration    float64                `json:"audio_duration_seconds"`
	ProsodyFeatures  map[string]float64     `json:"prosody_features"`
	EmotionDetected  *string                `json:"emotion_detected,omitempty"`
	ConfidenceScore  float64                `json:"confidence_score"`
	Flags            []string               `json:"flags,omitempty"`
	ClinicalInsights map[string]interface{} `json:"clinical_insights,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
}

// ============================================================================
// TRAJECTORY DTOs
// ============================================================================

type TrajectoryDTO struct {
	ID                    string                 `json:"id"`
	PatientID             int64                  `json:"patient_id"`
	BaselineDate          time.Time              `json:"baseline_date"`
	ForecastHorizonMonths int                    `json:"forecast_horizon_months"`
	BaselineState         map[string]interface{} `json:"baseline_state"`
	Predictions           []PredictionPointDTO   `json:"predictions"`
	RiskFactors           []RiskFactorDTO        `json:"risk_factors"`
	ProtectiveFactors     []string               `json:"protective_factors,omitempty"`
	OverallRiskLevel      string                 `json:"overall_risk_level"` // 'low', 'moderate', 'high'
	Recommendations       []string               `json:"recommendations,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
}

type PredictionPointDTO struct {
	MonthOffset int                    `json:"month_offset"`
	Date        string                 `json:"date"` // YYYY-MM
	PredictedPHQ9       *int           `json:"predicted_phq9,omitempty"`
	PredictedGAD7       *int           `json:"predicted_gad7,omitempty"`
	SuicideRisk         *float64       `json:"suicide_risk,omitempty"`
	HospitalizationRisk *float64       `json:"hospitalization_risk,omitempty"`
	Confidence          float64        `json:"confidence"`
	Scenarios           map[string]interface{} `json:"scenarios,omitempty"`
}

type RiskFactorDTO struct {
	Factor      string  `json:"factor"`
	Impact      string  `json:"impact"`      // 'low', 'moderate', 'high'
	Modifiable  bool    `json:"modifiable"`
	CurrentValue *string `json:"current_value,omitempty"`
}

// ============================================================================
// RESEARCH DTOs
// ============================================================================

type ResearchStudyDTO struct {
	ID                 string                 `json:"id"`
	StudyCode          string                 `json:"study_code"`
	StudyName          string                 `json:"study_name"`
	Hypothesis         string                 `json:"hypothesis"`
	StudyType          string                 `json:"study_type"`
	Status             string                 `json:"status"`
	CurrentNPatients   int                    `json:"current_n_patients"`
	TargetNPatients    int                    `json:"target_n_patients"`
	ProgressPercentage float64                `json:"progress_percentage"`
	InclusionCriteria  map[string]interface{} `json:"inclusion_criteria"`
	ExclusionCriteria  map[string]interface{} `json:"exclusion_criteria,omitempty"`
	PrimaryOutcome     *string                `json:"primary_outcome,omitempty"`
	SignificantFindings []FindingDTO          `json:"significant_findings,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
}

type FindingDTO struct {
	Predictor   string  `json:"predictor"`
	Outcome     string  `json:"outcome"`
	LagDays     int     `json:"lag_days"`
	Correlation float64 `json:"correlation"`
	PValue      float64 `json:"p_value"`
	EffectSize  string  `json:"effect_size"` // 'small', 'medium', 'large'
}

// ============================================================================
// PERSONA DTOs
// ============================================================================

type PersonaDTO struct {
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Tone             string   `json:"tone"`
	EmotionalDepth   float64  `json:"emotional_depth"`
	NarrativeFreedom float64  `json:"narrative_freedom"`
	AllowedTools     []string `json:"allowed_tools"`
	ProhibitedTools  []string `json:"prohibited_tools"`
	IsActive         bool     `json:"is_active"`
}

type PersonaSessionDTO struct {
	ID           string    `json:"id"`
	PatientID    int64     `json:"patient_id"`
	PersonaCode  string    `json:"persona_code"`
	PersonaName  string    `json:"persona_name"`
	StartedAt    time.Time `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
	DurationSec  *int      `json:"duration_seconds,omitempty"`
	TriggerReason string   `json:"trigger_reason"`
	IsActive     bool      `json:"is_active"`
}

// ============================================================================
// EXIT PROTOCOL DTOs
// ============================================================================

type LastWishesDTO struct {
	ID                       string    `json:"id"`
	PatientID                int64     `json:"patient_id"`
	ResuscitationPreference  *string   `json:"resuscitation_preference,omitempty"`
	PreferredDeathLocation   *string   `json:"preferred_death_location,omitempty"`
	PainManagementPreference *string   `json:"pain_management_preference,omitempty"`
	OrganDonationPreference  *string   `json:"organ_donation_preference,omitempty"`
	BurialCremation          *string   `json:"burial_cremation,omitempty"`
	PersonalStatement        *string   `json:"personal_statement,omitempty"`
	CompletionPercentage     int       `json:"completion_percentage"`
	Completed                bool      `json:"completed"`
	LastReviewedAt           *time.Time `json:"last_reviewed_at,omitempty"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type QualityOfLifeDTO struct {
	ID                         string    `json:"id"`
	PatientID                  int64     `json:"patient_id"`
	AssessmentDate             time.Time `json:"assessment_date"`
	OverallQoLScore            float64   `json:"overall_qol_score"`
	PhysicalDomainScore        float64   `json:"physical_domain_score"`
	PsychologicalDomainScore   float64   `json:"psychological_domain_score"`
	SocialDomainScore          float64   `json:"social_domain_score"`
	EnvironmentalDomainScore   float64   `json:"environmental_domain_score"`
	Interpretation             string    `json:"interpretation"` // 'excellent', 'good', 'moderate', 'low', 'very_low'
}

type PainLogDTO struct {
	ID                   string    `json:"id"`
	PatientID            int64     `json:"patient_id"`
	Timestamp            time.Time `json:"timestamp"`
	PainIntensity        int       `json:"pain_intensity"` // 0-10
	PainLocation         []string  `json:"pain_location,omitempty"`
	PainQuality          []string  `json:"pain_quality,omitempty"`
	OverallWellbeing     *int      `json:"overall_wellbeing,omitempty"`
	InterventionEffectiveness *int `json:"intervention_effectiveness,omitempty"`
	AlertTriggered       bool      `json:"alert_triggered"`
}

type LegacyMessageDTO struct {
	ID                   string  `json:"id"`
	PatientID            int64   `json:"patient_id"`
	RecipientName        string  `json:"recipient_name"`
	RecipientRelationship string `json:"recipient_relationship"`
	MessageType          string  `json:"message_type"`
	DeliveryTrigger      string  `json:"delivery_trigger"`
	IsComplete           bool    `json:"is_complete"`
	HasBeenDelivered     bool    `json:"has_been_delivered"`
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// ToJSON converte qualquer struct para JSON string
func ToJSON(v interface{}) (string, error) {
	bytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToJSONCompact converte para JSON compacto (sem indentação)
func ToJSONCompact(v interface{}) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// ToJSONBytes converte para []byte JSON
func ToJSONBytes(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// FromJSON converte JSON string para struct
func FromJSON(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// ============================================================================
// PAGINATION HELPERS
// ============================================================================

type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalCount int64       `json:"total_count"`
	TotalPages int         `json:"total_pages"`
	HasNext    bool        `json:"has_next"`
	HasPrev    bool        `json:"has_prev"`
}

func NewPaginatedResponse(data interface{}, page, pageSize int, totalCount int64) *PaginatedResponse {
	totalPages := int((totalCount + int64(pageSize) - 1) / int64(pageSize))

	return &PaginatedResponse{
		Data:       data,
		Page:       page,
		PageSize:   pageSize,
		TotalCount: totalCount,
		TotalPages: totalPages,
		HasNext:    page < totalPages,
		HasPrev:    page > 1,
	}
}

// ============================================================================
// ERROR RESPONSE
// ============================================================================

type ErrorResponse struct {
	Error   string                 `json:"error"`
	Message string                 `json:"message"`
	Code    string                 `json:"code,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func NewErrorResponse(err string, message string) *ErrorResponse {
	return &ErrorResponse{
		Error:   err,
		Message: message,
	}
}

// ============================================================================
// SUCCESS RESPONSE
// ============================================================================

type SuccessResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func NewSuccessResponse(message string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
}
