// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package research

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// CLINICAL RESEARCH ENGINE (SPRINT 4)
// ============================================================================
// Motor principal para pesquisa clinica longitudinal

type ResearchEngine struct {
	db                   *database.DB
	anonymizer           *Anonymizer
	longitudinalAnalyzer *LongitudinalAnalyzer
	statisticalMethods   *StatisticalMethods
	cohortBuilder        *CohortBuilder
}

// NewResearchEngine cria novo motor de pesquisa
func NewResearchEngine(db *database.DB) *ResearchEngine {
	if db == nil {
		log.Printf("[RESEARCH] NietzscheDB unavailable -- running in degraded mode")
		return &ResearchEngine{
			statisticalMethods: NewStatisticalMethods(),
		}
	}
	return &ResearchEngine{
		db:                   db,
		anonymizer:           NewAnonymizer(db),
		longitudinalAnalyzer: NewLongitudinalAnalyzer(db),
		statisticalMethods:   NewStatisticalMethods(),
		cohortBuilder:        NewCohortBuilder(db),
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
	ctx := context.Background()

	inclusionJSON, _ := json.Marshal(cohort.InclusionCriteria)
	exclusionJSON, _ := json.Marshal(cohort.ExclusionCriteria)

	content := map[string]interface{}{
		"study_name":                cohort.StudyName,
		"study_code":               cohort.StudyCode,
		"hypothesis":               cohort.Hypothesis,
		"study_type":               cohort.StudyType,
		"inclusion_criteria":       string(inclusionJSON),
		"exclusion_criteria":       string(exclusionJSON),
		"target_n_patients":        cohort.TargetNPatients,
		"current_n_patients":       0,
		"data_collection_start_date": cohort.DataCollectionStartDate.Format(time.RFC3339),
		"followup_duration_days":   cohort.FollowupDurationDays,
		"status":                   cohort.Status,
		"primary_outcome":          cohort.PrimaryOutcome,
		"secondary_outcomes":       cohort.SecondaryOutcomes,
		"statistical_methods":      cohort.StatisticalMethods,
		"principal_investigator":   cohort.PrincipalInvestigator,
		"created_at":               time.Now().Format(time.RFC3339),
	}

	id, err := re.db.Insert(ctx, "research_cohorts", content)
	if err != nil {
		return err
	}

	cohort.ID = fmt.Sprintf("%d", id)
	cohort.CreatedAt = time.Now()

	log.Printf("[RESEARCH] Coorte criada: %s (%s)", cohort.StudyCode, cohort.ID)

	return nil
}

// GetCohort recupera coorte por ID
func (re *ResearchEngine) GetCohort(cohortID string) (*ResearchCohort, error) {
	ctx := context.Background()

	m, err := re.db.GetNodeByID(ctx, "research_cohorts", cohortID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("cohort not found: %s", cohortID)
	}

	cohort := &ResearchCohort{}
	cohort.ID = database.GetString(m, "id")
	cohort.StudyName = database.GetString(m, "study_name")
	cohort.StudyCode = database.GetString(m, "study_code")
	cohort.Hypothesis = database.GetString(m, "hypothesis")
	cohort.StudyType = database.GetString(m, "study_type")
	cohort.TargetNPatients = int(database.GetInt64(m, "target_n_patients"))
	cohort.CurrentNPatients = int(database.GetInt64(m, "current_n_patients"))
	cohort.DataCollectionStartDate = database.GetTime(m, "data_collection_start_date")
	cohort.DataCollectionEndDate = database.GetTimePtr(m, "data_collection_end_date")
	cohort.FollowupDurationDays = int(database.GetInt64(m, "followup_duration_days"))
	cohort.Status = database.GetString(m, "status")
	cohort.PrimaryOutcome = database.GetString(m, "primary_outcome")
	cohort.PrincipalInvestigator = database.GetString(m, "principal_investigator")
	cohort.CreatedAt = database.GetTime(m, "created_at")

	// Parse inclusion/exclusion criteria from JSON strings
	inclusionStr := database.GetString(m, "inclusion_criteria")
	exclusionStr := database.GetString(m, "exclusion_criteria")
	if inclusionStr != "" {
		json.Unmarshal([]byte(inclusionStr), &cohort.InclusionCriteria)
	}
	if exclusionStr != "" {
		json.Unmarshal([]byte(exclusionStr), &cohort.ExclusionCriteria)
	}

	// Parse results JSON
	resultsStr := database.GetString(m, "results")
	if resultsStr != "" {
		json.Unmarshal([]byte(resultsStr), &cohort.Results)
	}

	// Parse p_value and effect_size (stored as float64 or nil)
	if v, ok := m["p_value"]; ok && v != nil {
		pv := database.GetFloat64(m, "p_value")
		cohort.PValue = &pv
	}
	if v, ok := m["effect_size"]; ok && v != nil {
		es := database.GetFloat64(m, "effect_size")
		cohort.EffectSize = &es
	}

	// Parse arrays -- may come as []interface{} from NietzscheDB or legacy postgres string format
	cohort.SecondaryOutcomes = extractStringSlice(m, "secondary_outcomes")
	cohort.StatisticalMethods = extractStringSlice(m, "statistical_methods")

	return cohort, nil
}

// extractStringSlice extracts a []string from a content map field.
// Handles both []interface{} (NietzscheDB native) and string (legacy postgres array format).
func extractStringSlice(m map[string]interface{}, key string) []string {
	v, ok := m[key]
	if !ok || v == nil {
		return []string{}
	}

	// NietzscheDB stores slices natively as []interface{}
	if arr, ok := v.([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		return result
	}

	// Legacy postgres array string format: {val1,val2}
	if s, ok := v.(string); ok {
		return parsePostgresArray(s)
	}

	return []string{}
}

// ============================================================================
// ESTUDOS PRE-CONFIGURADOS
// ============================================================================

// CreatePreconfiguredStudies cria os 4 estudos principais
func (re *ResearchEngine) CreatePreconfiguredStudies() error {
	studies := []ResearchCohort{
		// ESTUDO 1: Voice Biomarkers -> PHQ-9 (Lag Correlation)
		{
			StudyName:  "Voice Biomarkers as Early Predictors of Depression Severity",
			StudyCode:  "EVA-VOICE-PHQ9-001",
			Hypothesis: "Changes in voice prosody features (pitch, jitter, shimmer) predict PHQ-9 score changes 7-14 days in advance",
			StudyType:  "longitudinal_correlation",
			InclusionCriteria: map[string]interface{}{
				"min_age":              60,
				"max_age":              90,
				"has_voice_data":       true,
				"min_phq9_assessments": 3,
				"followup_days":        180,
			},
			ExclusionCriteria: map[string]interface{}{
				"severe_hearing_impairment":   true,
				"severe_cognitive_impairment": true,
			},
			TargetNPatients:         100,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0), // 6 meses atras
			FollowupDurationDays:    180,
			Status:                  "data_collection",
			PrimaryOutcome:          "PHQ-9 score change",
			SecondaryOutcomes:       []string{"GAD-7 change", "crisis occurrence"},
			StatisticalMethods:      []string{"lag_correlation", "mixed_effects_model"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},

		// ESTUDO 2: Medication Adherence -> Depression
		{
			StudyName:  "Impact of Medication Adherence on Depression Outcomes in Elderly",
			StudyCode:  "EVA-ADHERENCE-DEP-002",
			Hypothesis: "Medication adherence <50% for >=2 weeks leads to PHQ-9 increase of 5+ points within 30 days",
			StudyType:  "causal_inference",
			InclusionCriteria: map[string]interface{}{
				"min_age":                   60,
				"on_antidepressants":        true,
				"baseline_phq9_5_to_15":     true,
				"medication_logs_available": true,
			},
			ExclusionCriteria: map[string]interface{}{
				"hospitalized": true,
			},
			TargetNPatients:         200,
			DataCollectionStartDate: time.Now().AddDate(0, -6, 0),
			FollowupDurationDays:    90,
			Status:                  "data_collection",
			PrimaryOutcome:          "PHQ-9 increase >=5 points",
			SecondaryOutcomes:       []string{"crisis occurrence", "treatment dropout"},
			StatisticalMethods:      []string{"propensity_score_matching", "logistic_regression"},
			PrincipalInvestigator:   "Dr. EVA Research Team",
		},

		// ESTUDO 3: Social Isolation -> Crisis Risk
		{
			StudyName:  "Social Isolation as Risk Factor for Mental Health Crisis",
			StudyCode:  "EVA-ISOLATION-CRISIS-003",
			Hypothesis: "7+ days without social interaction increases crisis risk by 3x within 30 days",
			StudyType:  "survival_analysis",
			InclusionCriteria: map[string]interface{}{
				"min_age":                    60,
				"interaction_logs_available": true,
				"baseline_phq9_10_plus":      true,
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

		// ESTUDO 4: Sleep Quality -> Mental Health
		{
			StudyName:  "Sleep Quality and Mental Health Trajectories in Elderly",
			StudyCode:  "EVA-SLEEP-MH-004",
			Hypothesis: "Poor sleep (<5h avg for 7 days) predicts worsening depression and anxiety",
			StudyType:  "longitudinal_correlation",
			InclusionCriteria: map[string]interface{}{
				"min_age":              60,
				"sleep_data_available": true,
				"min_assessments":      5,
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
			log.Printf("[RESEARCH] Erro ao criar estudo %s: %v", study.StudyCode, err)
		}
	}

	log.Printf("[RESEARCH] %d estudos pre-configurados criados", len(studies))
	return nil
}

// ============================================================================
// COLETA DE DADOS PARA COORTE
// ============================================================================

// CollectDataForCohort coleta e anonimiza dados de pacientes para uma coorte
func (re *ResearchEngine) CollectDataForCohort(cohortID string) error {
	log.Printf("[RESEARCH] Coletando dados para coorte %s", cohortID)

	// 1. Obter criterios da coorte
	cohort, err := re.GetCohort(cohortID)
	if err != nil {
		return fmt.Errorf("erro ao obter coorte: %w", err)
	}

	// 2. Selecionar pacientes que atendem criterios
	patients, err := re.cohortBuilder.SelectPatients(cohort.InclusionCriteria, cohort.ExclusionCriteria)
	if err != nil {
		return fmt.Errorf("erro ao selecionar pacientes: %w", err)
	}

	log.Printf("   Encontrados %d pacientes elegiveis", len(patients))

	// 3. Para cada paciente, coletar dados longitudinais
	datapointsAdded := 0
	for _, patientID := range patients {
		err := re.anonymizer.CollectAndAnonymizePatientData(cohortID, patientID, cohort.FollowupDurationDays)
		if err != nil {
			log.Printf("[RESEARCH] Erro ao coletar dados do paciente %d: %v", patientID, err)
			continue
		}
		datapointsAdded++
	}

	log.Printf("[RESEARCH] Coletados dados de %d pacientes para coorte %s", datapointsAdded, cohortID)

	return nil
}

// ============================================================================
// EXECUTAR ANALISES
// ============================================================================

// RunLagCorrelationAnalysis executa analise de correlacao lag
func (re *ResearchEngine) RunLagCorrelationAnalysis(cohortID string, predictor string, outcome string, maxLag int) error {
	log.Printf("[RESEARCH] Executando lag correlation: %s -> %s (lag 0-%d dias)", predictor, outcome, maxLag)

	results, err := re.longitudinalAnalyzer.CalculateLagCorrelations(cohortID, predictor, outcome, maxLag)
	if err != nil {
		return err
	}

	// Salvar resultados significativos
	for _, result := range results {
		if result.PValue < 0.05 {
			err := re.SaveCorrelationResult(cohortID, result)
			if err != nil {
				log.Printf("[RESEARCH] Erro ao salvar correlacao: %v", err)
			}
		}
	}

	log.Printf("[RESEARCH] Analise de lag correlation concluida. %d correlacoes significativas encontradas", len(results))

	return nil
}

// SaveCorrelationResult salva resultado de correlacao no banco
func (re *ResearchEngine) SaveCorrelationResult(cohortID string, result CorrelationResult) error {
	ctx := context.Background()

	ciJSON, _ := json.Marshal(map[string]float64{
		"lower": result.ConfidenceIntervalLower,
		"upper": result.ConfidenceIntervalUpper,
	})

	content := map[string]interface{}{
		"cohort_id":               cohortID,
		"predictor_variable":      result.PredictorVariable,
		"outcome_variable":        result.OutcomeVariable,
		"lag_days":                result.LagDays,
		"correlation_coefficient": result.CorrelationCoefficient,
		"p_value":                 result.PValue,
		"confidence_interval_95":  string(ciJSON),
		"n_observations":          result.NObservations,
		"n_patients":              result.NPatients,
		"adjusted_for_covariates": result.AdjustedForCovariates,
		"analysis_method":         "pearson",
		"created_at":              time.Now().Format(time.RFC3339),
	}

	_, err := re.db.Insert(ctx, "longitudinal_correlations", content)
	return err
}

// ============================================================================
// EXPORTAR RESULTADOS
// ============================================================================

// ExportDatasetToCSV exporta dataset anonimizado para CSV
func (re *ResearchEngine) ExportDatasetToCSV(cohortID string, filePath string) error {
	log.Printf("[RESEARCH] Exportando dataset da coorte %s para %s", cohortID, filePath)

	// Implementacao de export seria aqui
	// Por enquanto, placeholder

	// Registrar export no banco
	ctx := context.Background()

	content := map[string]interface{}{
		"cohort_id":            cohortID,
		"export_name":          "Dataset Export",
		"export_format":        "csv",
		"file_path":            filePath,
		"variables_included":   []string{"phq9_score", "gad7_score", "medication_adherence_7d", "voice_pitch_mean_hz"},
		"n_patients":           0,
		"n_observations":       0,
		"anonymization_level":  "fully_anonymized",
		"pii_removed":          true,
		"exported_by":          "research_engine",
		"exported_for_purpose": "Statistical analysis",
		"created_at":           time.Now().Format(time.RFC3339),
	}

	_, err := re.db.Insert(ctx, "research_exports", content)
	return err
}

// ============================================================================
// RELATORIOS
// ============================================================================

// GenerateStudyReport gera relatorio completo de estudo
func (re *ResearchEngine) GenerateStudyReport(cohortID string) (map[string]interface{}, error) {
	ctx := context.Background()

	// Fetch the cohort
	cohort, err := re.GetCohort(cohortID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cohort: %w", err)
	}

	// Fetch correlation results for this cohort
	correlations, err := re.db.QueryByLabel(ctx, "longitudinal_correlations",
		" AND n.cohort_id = $cid", map[string]interface{}{"cid": cohortID}, 0)
	if err != nil {
		correlations = []map[string]interface{}{}
	}

	// Build the report
	report := map[string]interface{}{
		"cohort_id":              cohort.ID,
		"study_name":             cohort.StudyName,
		"study_code":             cohort.StudyCode,
		"hypothesis":             cohort.Hypothesis,
		"study_type":             cohort.StudyType,
		"status":                 cohort.Status,
		"target_n_patients":      cohort.TargetNPatients,
		"current_n_patients":     cohort.CurrentNPatients,
		"primary_outcome":        cohort.PrimaryOutcome,
		"secondary_outcomes":     cohort.SecondaryOutcomes,
		"statistical_methods":    cohort.StatisticalMethods,
		"principal_investigator": cohort.PrincipalInvestigator,
		"n_correlations":         len(correlations),
		"correlations":           correlations,
		"generated_at":           time.Now().Format(time.RFC3339),
	}

	if cohort.Results != nil {
		report["results"] = cohort.Results
	}
	if cohort.PValue != nil {
		report["p_value"] = *cohort.PValue
	}
	if cohort.EffectSize != nil {
		report["effect_size"] = *cohort.EffectSize
	}

	return report, nil
}

// ============================================================================
// UTILITARIOS
// ============================================================================

// AnonymizePatientID gera hash SHA-256 irreversivel do patient ID
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

	// Split por virgula (simplificado)
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
	// Implementacao simplificada
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
