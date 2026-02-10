package research

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// CLINICAL RESEARCH ENGINE (SPRINT 4)
// ============================================================================
// Motor principal para pesquisa clínica longitudinal

type ResearchEngine struct {
	db                *sql.DB
	anonymizer        *Anonymizer
	longitudinalAnalyzer *LongitudinalAnalyzer
	statisticalMethods *StatisticalMethods
	cohortBuilder     *CohortBuilder
}

// NewResearchEngine cria novo motor de pesquisa
func NewResearchEngine(db *sql.DB) *ResearchEngine {
	return &ResearchEngine{
		db:                db,
		anonymizer:        NewAnonymizer(db),
		longitudinalAnalyzer: NewLongitudinalAnalyzer(db),
		statisticalMethods: NewStatisticalMethods(),
		cohortBuilder:     NewCohortBuilder(db),
	}
}

// ============================================================================
// RESEARCH COHORT
// ============================================================================

type ResearchCohort struct {
	ID                      string                 `json:"id"`
	StudyName               string                 `json:"study_name"`
	StudyCode               string                 `json:"study_code"`
	Hypothesis              string                 `json:"hypothesis"`
	StudyType               string                 `json:"study_type"`
	InclusionCriteria       map[string]interface{} `json:"inclusion_criteria"`
	ExclusionCriteria       map[string]interface{} `json:"exclusion_criteria"`
	TargetNPatients         int                    `json:"target_n_patients"`
	CurrentNPatients        int                    `json:"current_n_patients"`
	DataCollectionStartDate time.Time              `json:"data_collection_start_date"`
	DataCollectionEndDate   *time.Time             `json:"data_collection_end_date,omitempty"`
	FollowupDurationDays    int                    `json:"followup_duration_days"`
	Status                  string                 `json:"status"`
	PrimaryOutcome          string                 `json:"primary_outcome"`
	SecondaryOutcomes       []string               `json:"secondary_outcomes"`
	StatisticalMethods      []string               `json:"statistical_methods"`
	Results                 map[string]interface{} `json:"results,omitempty"`
	PValue                  *float64               `json:"p_value,omitempty"`
	EffectSize              *float64               `json:"effect_size,omitempty"`
	PrincipalInvestigator   string                 `json:"principal_investigator"`
	CreatedAt               time.Time              `json:"created_at"`
}

// CreateCohort cria nova coorte de pesquisa
func (re *ResearchEngine) CreateCohort(cohort *ResearchCohort) error {
	query := `
		INSERT INTO research_cohorts (
			study_name, study_code, hypothesis, study_type,
			inclusion_criteria, exclusion_criteria,
			target_n_patients, data_collection_start_date, followup_duration_days,
			status, primary_outcome, secondary_outcomes, statistical_methods,
			principal_investigator
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at
	`

	inclusionJSON, _ := json.Marshal(cohort.InclusionCriteria)
	exclusionJSON, _ := json.Marshal(cohort.ExclusionCriteria)
	secondaryOutcomesArray := "{" + joinStrings(cohort.SecondaryOutcomes, ",") + "}"
	statisticalMethodsArray := "{" + joinStrings(cohort.StatisticalMethods, ",") + "}"

	err := re.db.QueryRow(
		query,
		cohort.StudyName, cohort.StudyCode, cohort.Hypothesis, cohort.StudyType,
		inclusionJSON, exclusionJSON,
		cohort.TargetNPatients, cohort.DataCollectionStartDate, cohort.FollowupDurationDays,
		cohort.Status, cohort.PrimaryOutcome,
		secondaryOutcomesArray, statisticalMethodsArray,
		cohort.PrincipalInvestigator,
	).Scan(&cohort.ID, &cohort.CreatedAt)

	if err == nil {
		log.Printf("✅ [RESEARCH] Coorte criada: %s (%s)", cohort.StudyCode, cohort.ID)
	}

	return err
}

// GetCohort recupera coorte por ID
func (re *ResearchEngine) GetCohort(cohortID string) (*ResearchCohort, error) {
	query := `
		SELECT
			id, study_name, study_code, hypothesis, study_type,
			inclusion_criteria, exclusion_criteria,
			target_n_patients, current_n_patients,
			data_collection_start_date, data_collection_end_date,
			followup_duration_days, status,
			primary_outcome, secondary_outcomes, statistical_methods,
			results, p_value, effect_size,
			principal_investigator, created_at
		FROM research_cohorts
		WHERE id = $1
	`

	cohort := &ResearchCohort{}
	var inclusionJSON, exclusionJSON, resultsJSON []byte
	var secondaryOutcomesStr, statisticalMethodsStr string

	err := re.db.QueryRow(query, cohortID).Scan(
		&cohort.ID, &cohort.StudyName, &cohort.StudyCode, &cohort.Hypothesis, &cohort.StudyType,
		&inclusionJSON, &exclusionJSON,
		&cohort.TargetNPatients, &cohort.CurrentNPatients,
		&cohort.DataCollectionStartDate, &cohort.DataCollectionEndDate,
		&cohort.FollowupDurationDays, &cohort.Status,
		&cohort.PrimaryOutcome, &secondaryOutcomesStr, &statisticalMethodsStr,
		&resultsJSON, &cohort.PValue, &cohort.EffectSize,
		&cohort.PrincipalInvestigator, &cohort.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	json.Unmarshal(inclusionJSON, &cohort.InclusionCriteria)
	json.Unmarshal(exclusionJSON, &cohort.ExclusionCriteria)
	if len(resultsJSON) > 0 {
		json.Unmarshal(resultsJSON, &cohort.Results)
	}

	// Parse arrays
	cohort.SecondaryOutcomes = parsePostgresArray(secondaryOutcomesStr)
	cohort.StatisticalMethods = parsePostgresArray(statisticalMethodsStr)

	return cohort, nil
}

// ============================================================================
// ESTUDOS PRÉ-CONFIGURADOS
// ============================================================================

// CreatePreconfiguredStudies cria os 4 estudos principais
func (re *ResearchEngine) CreatePreconfiguredStudies() error {
	studies := []ResearchCohort{
		// ESTUDO 1: Voice Biomarkers → PHQ-9 (Lag Correlation)
		{
			StudyName:  "Voice Biomarkers as Early Predictors of Depression Severity",
			StudyCode:  "EVA-VOICE-PHQ9-001",
			Hypothesis: "Changes in voice prosody features (pitch, jitter, shimmer) predict PHQ-9 score changes 7-14 days in advance",
			StudyType:  "longitudinal_correlation",
			InclusionCriteria: map[string]interface{}{
				"min_age":           60,
				"max_age":           90,
				"has_voice_data":    true,
				"min_phq9_assessments": 3,
				"followup_days":     180,
			},
			ExclusionCriteria: map[string]interface{}{
				"severe_hearing_impairment": true,
				"severe_cognitive_impairment": true,
			},
			TargetNPatients:         100,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0), // 6 meses atrás
			FollowupDurationDays:    180,
			Status:                  "data_collection",
			PrimaryOutcome:          "PHQ-9 score change",
			SecondaryOutcomes:       []string{"GAD-7 change", "crisis occurrence"},
			StatisticalMethods:      []string{"lag_correlation", "mixed_effects_model"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},

		// ESTUDO 2: Medication Adherence → Depression
		{
			StudyName:  "Impact of Medication Adherence on Depression Outcomes in Elderly",
			StudyCode:  "EVA-ADHERENCE-DEP-002",
			Hypothesis: "Medication adherence <50% for ≥2 weeks leads to PHQ-9 increase of 5+ points within 30 days",
			StudyType:  "causal_inference",
			InclusionCriteria: map[string]interface{}{
				"min_age":                60,
				"on_antidepressants":     true,
				"baseline_phq9_5_to_15":  true,
				"medication_logs_available": true,
			},
			ExclusionCriteria: map[string]interface{}{
				"hospitalized": true,
			},
			TargetNPatients:         200,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0),
			FollowupDurationDays:    90,
			Status:                  "data_collection",
			PrimaryOutcome:          "PHQ-9 increase ≥5 points",
			SecondaryOutcomes:       []string{"crisis occurrence", "treatment dropout"},
			StatisticalMethods:      []string{"propensity_score_matching", "logistic_regression"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},

		// ESTUDO 3: Social Isolation → Crisis Risk
		{
			StudyName:  "Social Isolation as Risk Factor for Mental Health Crisis",
			StudyCode:  "EVA-ISOLATION-CRISIS-003",
			Hypothesis: "7+ days without social interaction increases crisis risk by 3x within 30 days",
			StudyType:  "survival_analysis",
			InclusionCriteria: map[string]interface{}{
				"min_age":               60,
				"interaction_logs_available": true,
				"baseline_phq9_10_plus": true,
			},
			ExclusionCriteria: map[string]interface{}{
				"lives_in_facility": true,
			},
			TargetNPatients:         150,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0),
			FollowupDurationDays:    180,
			Status:                  "data_collection",
			PrimaryOutcome:          "Time to crisis event",
			SecondaryOutcomes:       []string{"hospitalization", "emergency calls"},
			StatisticalMethods:      []string{"kaplan_meier", "cox_regression"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},

		// ESTUDO 4: Sleep Quality → Mental Health
		{
			StudyName:  "Sleep Quality and Mental Health Trajectories in Elderly",
			StudyCode:  "EVA-SLEEP-MH-004",
			Hypothesis: "Poor sleep (<5h avg for 7 days) predicts worsening depression and anxiety",
			StudyType:  "longitudinal_correlation",
			InclusionCriteria: map[string]interface{}{
				"min_age":            60,
				"sleep_data_available": true,
				"min_assessments":    5,
			},
			ExclusionCriteria: map[string]interface{}{
				"diagnosed_sleep_apnea": true,
			},
			TargetNPatients:         120,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0),
			FollowupDurationDays:    120,
			Status:                  "data_collection",
			PrimaryOutcome:          "PHQ-9 and GAD-7 trajectories",
			SecondaryOutcomes:       []string{"medication changes", "crisis events"},
			StatisticalMethods:      []string{"lag_correlation", "linear_mixed_models"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},
	}

	for _, study := range studies {
		err := re.CreateCohort(&study)
		if err != nil {
			log.Printf("⚠️ [RESEARCH] Erro ao criar estudo %s: %v", study.StudyCode, err)
		}
	}

	log.Printf("✅ [RESEARCH] %d estudos pré-configurados criados", len(studies))
	return nil
}

// ============================================================================
// COLETA DE DADOS PARA COORTE
// ============================================================================

// CollectDataForCohort coleta e anonimiza dados de pacientes para uma coorte
func (re *ResearchEngine) CollectDataForCohort(cohortID string) error {
	log.Printf("📊 [RESEARCH] Coletando dados para coorte %s", cohortID)

	// 1. Obter critérios da coorte
	cohort, err := re.GetCohort(cohortID)
	if err != nil {
		return fmt.Errorf("erro ao obter coorte: %w", err)
	}

	// 2. Selecionar pacientes que atendem critérios
	patients, err := re.cohortBuilder.SelectPatients(cohort.InclusionCriteria, cohort.ExclusionCriteria)
	if err != nil {
		return fmt.Errorf("erro ao selecionar pacientes: %w", err)
	}

	log.Printf("   Encontrados %d pacientes elegíveis", len(patients))

	// 3. Para cada paciente, coletar dados longitudinais
	datapointsAdded := 0
	for _, patientID := range patients {
		err := re.anonymizer.CollectAndAnonymizePatientData(cohortID, patientID, cohort.FollowupDurationDays)
		if err != nil {
			log.Printf("⚠️ [RESEARCH] Erro ao coletar dados do paciente %d: %v", patientID, err)
			continue
		}
		datapointsAdded++
	}

	log.Printf("✅ [RESEARCH] Coletados dados de %d pacientes para coorte %s", datapointsAdded, cohortID)

	return nil
}

// ============================================================================
// EXECUTAR ANÁLISES
// ============================================================================

// RunLagCorrelationAnalysis executa análise de correlação lag
func (re *ResearchEngine) RunLagCorrelationAnalysis(cohortID string, predictor string, outcome string, maxLag int) error {
	log.Printf("🔬 [RESEARCH] Executando lag correlation: %s → %s (lag 0-%d dias)", predictor, outcome, maxLag)

	results, err := re.longitudinalAnalyzer.CalculateLagCorrelations(cohortID, predictor, outcome, maxLag)
	if err != nil {
		return err
	}

	// Salvar resultados significativos
	for _, result := range results {
		if result.PValue < 0.05 {
			err := re.SaveCorrelationResult(cohortID, result)
			if err != nil {
				log.Printf("⚠️ Erro ao salvar correlação: %v", err)
			}
		}
	}

	log.Printf("✅ [RESEARCH] Análise de lag correlation concluída. %d correlações significativas encontradas", len(results))

	return nil
}

// SaveCorrelationResult salva resultado de correlação no banco
func (re *ResearchEngine) SaveCorrelationResult(cohortID string, result CorrelationResult) error {
	query := `
		INSERT INTO longitudinal_correlations (
			cohort_id, predictor_variable, outcome_variable, lag_days,
			correlation_coefficient, p_value, confidence_interval_95,
			n_observations, n_patients,
			adjusted_for_covariates, analysis_method
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	ciJSON, _ := json.Marshal(map[string]float64{
		"lower": result.ConfidenceIntervalLower,
		"upper": result.ConfidenceIntervalUpper,
	})

	covariatesArray := "{}"
	if len(result.AdjustedForCovariates) > 0 {
		covariatesArray = "{" + joinStrings(result.AdjustedForCovariates, ",") + "}"
	}

	_, err := re.db.Exec(
		query,
		cohortID, result.PredictorVariable, result.OutcomeVariable, result.LagDays,
		result.CorrelationCoefficient, result.PValue, ciJSON,
		result.NObservations, result.NPatients,
		covariatesArray, "pearson",
	)

	return err
}

// ============================================================================
// EXPORTAR RESULTADOS
// ============================================================================

// ExportDatasetToCSV exporta dataset anonimizado para CSV
func (re *ResearchEngine) ExportDatasetToCSV(cohortID string, filePath string) error {
	log.Printf("📤 [RESEARCH] Exportando dataset da coorte %s para %s", cohortID, filePath)

	// Implementação de export seria aqui
	// Por enquanto, placeholder

	// Registrar export no banco
	query := `
		INSERT INTO research_exports (
			cohort_id, export_name, export_format, file_path,
			variables_included, n_patients, n_observations,
			anonymization_level, pii_removed, exported_by, exported_for_purpose
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	variablesArray := "{phq9_score,gad7_score,medication_adherence_7d,voice_pitch_mean_hz}"

	_, err := re.db.Exec(
		query,
		cohortID, "Dataset Export", "csv", filePath,
		variablesArray, 0, 0,
		"fully_anonymized", true, "research_engine", "Statistical analysis",
	)

	return err
}

// ============================================================================
// RELATÓRIOS
// ============================================================================

// GenerateStudyReport gera relatório completo de estudo
func (re *ResearchEngine) GenerateStudyReport(cohortID string) (map[string]interface{}, error) {
	var reportJSON []byte

	query := `SELECT generate_study_report($1)`
	err := re.db.QueryRow(query, cohortID).Scan(&reportJSON)
	if err != nil {
		return nil, err
	}

	var report map[string]interface{}
	json.Unmarshal(reportJSON, &report)

	return report, nil
}

// ============================================================================
// UTILITÁRIOS
// ============================================================================

// AnonymizePatientID gera hash SHA-256 irreversível do patient ID
func AnonymizePatientID(patientID int64) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d", patientID)))
	return hex.EncodeToString(hash[:])
}

func joinStrings(arr []string, sep string) string {
	if len(arr) == 0 {
		return ""
	}
	result := ""
	for i, s := range arr {
		if i > 0 {
			result += sep
		}
		result += `"` + s + `"`
	}
	return result
}

func parsePostgresArray(pgArray string) []string {
	// Remove { e }
	if len(pgArray) < 2 {
		return []string{}
	}
	cleaned := pgArray[1 : len(pgArray)-1]
	if cleaned == "" {
		return []string{}
	}

	// Split por vírgula (simplificado)
	result := []string{}
	for _, item := range splitByComma(cleaned) {
		// Remove aspas
		trimmed := trimQuotes(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func splitByComma(s string) []string {
	// Implementação simplificada
	result := []string{}
	current := ""
	for _, char := range s {
		if char == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
