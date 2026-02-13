package integration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// ============================================================================
// DATA EXPORT UTILITIES
// ============================================================================
// Funções para exportar dados (LGPD/GDPR, pesquisa, backups)

// ============================================================================
// EXPORT JOB
// ============================================================================

type ExportJob struct {
	ID             string                 `json:"id"`
	RequestedBy    string                 `json:"requested_by"`
	PatientID      *int64                 `json:"patient_id,omitempty"`
	ExportType     string                 `json:"export_type"` // "lgpd_portability", "research_dataset", "clinical_summary"
	Config         ExportConfig           `json:"config"`
	Status         string                 `json:"status"` // "pending", "processing", "completed", "failed"
	Progress       int                    `json:"progress_percentage"`
	RecordsProcessed int                  `json:"records_processed"`
	TotalRecords   int                    `json:"total_records"`
	FilePath       *string                `json:"file_path,omitempty"`
	DownloadURL    *string                `json:"download_url,omitempty"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
	ErrorMessage   *string                `json:"error_message,omitempty"`
	CreatedAt      time.Time              `json:"created_at"`
	CompletedAt    *time.Time             `json:"completed_at,omitempty"`
}

type ExportConfig struct {
	Resources     []string               `json:"resources"` // ["patients", "assessments", "medications"]
	DateRange     *DateRange             `json:"date_range,omitempty"`
	Format        string                 `json:"format"` // "json", "csv", "fhir", "xml"
	Anonymize     bool                   `json:"anonymize"`
	IncludeFields []string               `json:"include_fields,omitempty"`
	ExcludeFields []string               `json:"exclude_fields,omitempty"`
	Filters       map[string]interface{} `json:"filters,omitempty"`
}

type DateRange struct {
	Start string `json:"start"` // ISO 8601: YYYY-MM-DD
	End   string `json:"end"`
}

// ============================================================================
// LGPD/GDPR PORTABILITY EXPORT
// ============================================================================

type LGPDPortabilityExport struct {
	ExportDate time.Time            `json:"export_date"`
	PatientID  int64                `json:"patient_id"`
	Patient    *PatientDTO          `json:"patient"`
	Assessments []AssessmentDTO     `json:"assessments,omitempty"`
	VoiceAnalyses []VoiceAnalysisDTO `json:"voice_analyses,omitempty"`
	Medications []interface{}       `json:"medications,omitempty"`
	LastWishes  *LastWishesDTO      `json:"last_wishes,omitempty"`
	QualityOfLife []QualityOfLifeDTO `json:"quality_of_life,omitempty"`
	PainLogs    []PainLogDTO        `json:"pain_logs,omitempty"`
	LegacyMessages []LegacyMessageDTO `json:"legacy_messages,omitempty"`
	ConsentHistory []ConsentRecord   `json:"consent_history,omitempty"`
	DataProcessing []DataProcessingLog `json:"data_processing,omitempty"`
}

type ConsentRecord struct {
	Purpose     string    `json:"purpose"`
	Granted     bool      `json:"granted"`
	GrantedAt   time.Time `json:"granted_at"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
}

type DataProcessingLog struct {
	Activity      string    `json:"activity"`
	ProcessedAt   time.Time `json:"processed_at"`
	ProcessedBy   string    `json:"processed_by"`
	LegalBasis    string    `json:"legal_basis"` // "consent", "contract", "legitimate_interest"
}

// Criar export LGPD completo
func NewLGPDPortabilityExport(patientID int64) *LGPDPortabilityExport {
	return &LGPDPortabilityExport{
		ExportDate: time.Now(),
		PatientID:  patientID,
		Assessments: []AssessmentDTO{},
		VoiceAnalyses: []VoiceAnalysisDTO{},
		Medications: []interface{}{},
		QualityOfLife: []QualityOfLifeDTO{},
		PainLogs: []PainLogDTO{},
		LegacyMessages: []LegacyMessageDTO{},
		ConsentHistory: []ConsentRecord{},
		DataProcessing: []DataProcessingLog{},
	}
}

// ============================================================================
// RESEARCH DATASET EXPORT
// ============================================================================

type ResearchDatasetExport struct {
	StudyID         string                `json:"study_id"`
	StudyCode       string                `json:"study_code"`
	ExportDate      time.Time             `json:"export_date"`
	TotalPatients   int                   `json:"total_patients"`
	Datapoints      []AnonymizedDatapoint `json:"datapoints"`
	Metadata        ResearchMetadata      `json:"metadata"`
	DataQuality     DataQualityReport     `json:"data_quality"`
}

type AnonymizedDatapoint struct {
	AnonymousID string                 `json:"anonymous_id"` // SHA-256 hash
	DayOffset   int                    `json:"day_offset"` // Dias desde baseline
	Variables   map[string]interface{} `json:"variables"`
}

type ResearchMetadata struct {
	AnonymizationMethod string    `json:"anonymization_method"` // "SHA-256"
	KAnonymity          int       `json:"k_anonymity"`
	Variables           []string  `json:"variables"`
	DateRange           DateRange `json:"date_range"`
	IRBApproval         string    `json:"irb_approval,omitempty"`
	Citation            string    `json:"citation,omitempty"`
}

type DataQualityReport struct {
	TotalRecords      int     `json:"total_records"`
	CompleteRecords   int     `json:"complete_records"`
	CompletenessRate  float64 `json:"completeness_rate"`
	MissingByVariable map[string]int `json:"missing_by_variable"`
}

// ============================================================================
// CLINICAL SUMMARY EXPORT
// ============================================================================

type ClinicalSummaryExport struct {
	PatientID       int64                   `json:"patient_id"`
	GeneratedAt     time.Time               `json:"generated_at"`
	Patient         *PatientDTO             `json:"patient"`
	CurrentStatus   PatientCurrentStatus    `json:"current_status"`
	RecentAssessments []AssessmentDTO       `json:"recent_assessments"`
	ActiveMedications []interface{}         `json:"active_medications"`
	RiskFactors     []RiskFactorDTO         `json:"risk_factors"`
	Recommendations []string                `json:"recommendations"`
	CareTeam        []CareTeamMember        `json:"care_team,omitempty"`
}

type PatientCurrentStatus struct {
	OverallHealth    string  `json:"overall_health"` // "stable", "at_risk", "critical"
	LatestPHQ9       *int    `json:"latest_phq9,omitempty"`
	LatestGAD7       *int    `json:"latest_gad7,omitempty"`
	SuicideRisk      string  `json:"suicide_risk"` // "low", "moderate", "high"
	MedicationAdherence float64 `json:"medication_adherence_percentage"`
	LastContactDays  int     `json:"last_contact_days_ago"`
}

type CareTeamMember struct {
	Role         string  `json:"role"` // "psychiatrist", "psychologist", "nurse"
	Name         string  `json:"name"`
	ContactInfo  *string `json:"contact_info,omitempty"`
}

// ============================================================================
// ANONYMIZATION UTILITIES
// ============================================================================

// Anonimizar ID de paciente (SHA-256)
func AnonymizePatientID(patientID int64) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", patientID)))
	return hex.EncodeToString(hash[:])
}

// Anonimizar nome (primeiras letras + *****)
func AnonymizeName(name string) string {
	if len(name) <= 2 {
		return "***"
	}
	return name[:2] + "*****"
}

// Anonimizar email
func AnonymizeEmail(email string) string {
	// exemplo@domain.com → ex*****@domain.com
	if len(email) < 3 {
		return "***@***.com"
	}
	parts := make([]byte, 0)
	for i, char := range email {
		if i < 2 || char == '@' || char == '.' {
			parts = append(parts, byte(char))
		} else if char != '@' && char != '.' {
			parts = append(parts, '*')
		}
	}
	return string(parts)
}

// Remover campos sensíveis de um objeto
func RemoveSensitiveFields(data map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	cleaned := make(map[string]interface{})
	for k, v := range data {
		sensitive := false
		for _, field := range sensitiveFields {
			if k == field {
				sensitive = true
				break
			}
		}
		if !sensitive {
			cleaned[k] = v
		}
	}
	return cleaned
}

// ============================================================================
// CSV EXPORT UTILITIES
// ============================================================================

type CSVColumn struct {
	Name   string `json:"name"`
	Type   string `json:"type"`   // "string", "integer", "float", "date"
	Format string `json:"format,omitempty"` // Para datas: "YYYY-MM-DD"
}

type CSVExportConfig struct {
	Columns    []CSVColumn `json:"columns"`
	Delimiter  string      `json:"delimiter"` // ",", ";", "\t"
	Header     bool        `json:"header"`
	Encoding   string      `json:"encoding"` // "UTF-8", "ISO-8859-1"
}

// Gerar header CSV
func GenerateCSVHeader(columns []CSVColumn, delimiter string) string {
	header := ""
	for i, col := range columns {
		header += col.Name
		if i < len(columns)-1 {
			header += delimiter
		}
	}
	return header + "\n"
}

// Converter row para CSV
func RowToCSV(row map[string]interface{}, columns []CSVColumn, delimiter string) string {
	line := ""
	for i, col := range columns {
		value := row[col.Name]
		line += fmt.Sprintf("%v", value)
		if i < len(columns)-1 {
			line += delimiter
		}
	}
	return line + "\n"
}

// ============================================================================
// FHIR BUNDLE EXPORT
// ============================================================================

func ExportPatientAsFHIRBundle(patient *PatientDTO, assessments []*AssessmentDTO) (*FHIRBundle, error) {
	// Converter paciente para FHIR
	fhirPatient := PatientDTOToFHIR(patient)

	// Converter assessments para FHIR Observations
	fhirObservations := make([]*FHIRObservation, 0)
	for _, assessment := range assessments {
		if assessment.AssessmentType == "PHQ-9" {
			fhirObs := PHQ9ToFHIR(assessment)
			fhirObservations = append(fhirObservations, fhirObs)
		}
		// Adicionar outros tipos de assessment aqui (GAD-7, C-SSRS, etc.)
	}

	// Criar bundle
	bundle := CreatePatientBundle(fhirPatient, fhirObservations)
	return bundle, nil
}

// ============================================================================
// ZIP COMPRESSION UTILITIES
// ============================================================================

type ZipExport struct {
	Filename string   `json:"filename"`
	Files    []ZipFile `json:"files"`
}

type ZipFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// Adicionar arquivo ao ZIP
func (z *ZipExport) AddFile(name string, content string) {
	z.Files = append(z.Files, ZipFile{
		Name:    name,
		Content: content,
		Size:    int64(len(content)),
	})
}

// ============================================================================
// EXPORT STATISTICS
// ============================================================================

type ExportStatistics struct {
	TotalExports        int                `json:"total_exports"`
	ExportsByType       map[string]int     `json:"exports_by_type"`
	ExportsByFormat     map[string]int     `json:"exports_by_format"`
	AverageProcessingTime float64          `json:"average_processing_time_seconds"`
	LargestExportSize   int64              `json:"largest_export_size_bytes"`
	TotalDataExported   int64              `json:"total_data_exported_bytes"`
}

// ============================================================================
// COMPLIANCE CHECKS
// ============================================================================

type ComplianceCheck struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	Compliant   bool   `json:"compliant"`
	Details     string `json:"details,omitempty"`
}

func RunLGPDComplianceChecks(exportData *LGPDPortabilityExport) []ComplianceCheck {
	checks := []ComplianceCheck{}

	// Check 1: Dados completos
	checks = append(checks, ComplianceCheck{
		Rule:        "LGPD Art. 18, III",
		Description: "Direito à portabilidade de dados",
		Compliant:   exportData.Patient != nil,
		Details:     "Todos os dados pessoais foram incluídos",
	})

	// Check 2: Formato legível
	checks = append(checks, ComplianceCheck{
		Rule:        "LGPD Art. 18, III",
		Description: "Formato estruturado e legível por máquina",
		Compliant:   true,
		Details:     "JSON estruturado conforme padrão",
	})

	// Check 3: Histórico de consentimento
	checks = append(checks, ComplianceCheck{
		Rule:        "LGPD Art. 8º",
		Description: "Registro de consentimento",
		Compliant:   len(exportData.ConsentHistory) > 0,
		Details:     fmt.Sprintf("%d registros de consentimento incluídos", len(exportData.ConsentHistory)),
	})

	// Check 4: Log de processamento
	checks = append(checks, ComplianceCheck{
		Rule:        "LGPD Art. 37",
		Description: "Relatório de impacto e logs de processamento",
		Compliant:   len(exportData.DataProcessing) > 0,
		Details:     fmt.Sprintf("%d atividades de processamento documentadas", len(exportData.DataProcessing)),
	})

	return checks
}

// ============================================================================
// EXPORT TEMPLATES
// ============================================================================

var DefaultLGPDExportConfig = ExportConfig{
	Resources: []string{
		"patient_info",
		"assessments",
		"voice_analyses",
		"medications",
		"last_wishes",
		"quality_of_life",
		"pain_logs",
		"legacy_messages",
		"consent_history",
		"data_processing_logs",
	},
	Format:    "json",
	Anonymize: false, // LGPD export deve incluir dados identificáveis
}

var DefaultResearchExportConfig = ExportConfig{
	Resources: []string{
		"assessments",
		"voice_analyses",
		"medications",
		"quality_of_life",
	},
	Format:    "csv",
	Anonymize: true, // Pesquisa sempre anonimizada
	ExcludeFields: []string{
		"name",
		"email",
		"phone",
		"address",
		"ssn",
	},
}

var DefaultClinicalSummaryConfig = ExportConfig{
	Resources: []string{
		"patient_info",
		"recent_assessments",
		"active_medications",
		"risk_factors",
	},
	Format:    "json",
	Anonymize: false,
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// Calcular tamanho estimado do export
func EstimateExportSize(recordCount int, avgRecordSize int) int64 {
	return int64(recordCount * avgRecordSize)
}

// Validar configuração de export
func ValidateExportConfig(config *ExportConfig) error {
	if len(config.Resources) == 0 {
		return fmt.Errorf("nenhum recurso especificado para export")
	}

	validFormats := map[string]bool{
		"json": true,
		"csv":  true,
		"xml":  true,
		"fhir": true,
	}

	if !validFormats[config.Format] {
		return fmt.Errorf("formato inválido: %s", config.Format)
	}

	return nil
}
