// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package research

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// COHORT BUILDER
// ============================================================================
// Seleciona pacientes baseado em critérios de inclusão/exclusão

type CohortBuilder struct {
	db *database.DB
}

func NewCohortBuilder(db *database.DB) *CohortBuilder {
	return &CohortBuilder{db: db}
}

// SelectPatients seleciona pacientes que atendem critérios
func (cb *CohortBuilder) SelectPatients(
	inclusionCriteria map[string]interface{},
	exclusionCriteria map[string]interface{},
) ([]int64, error) {

	log.Printf("🔍 [COHORT] Selecionando pacientes com critérios de inclusão...")
	ctx := context.Background()

	// 1. Buscar todos os pacientes ativos (idosos)
	rows, err := cb.db.QueryByLabel(ctx, "Idoso", " AND n.ativo = $ativo", map[string]interface{}{"ativo": true}, 0)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar pacientes: %w", err)
	}

	patients := []int64{}

	for _, row := range rows {
		patientID := database.GetInt64(row, "pg_id")
		if patientID == 0 {
			patientID = database.GetInt64(row, "id")
		}

		// Verificar critérios de inclusão
		meetsInclusion := true
		for key, value := range inclusionCriteria {
			if !cb.checkCriterionFromRow(ctx, patientID, row, key, value, true) {
				meetsInclusion = false
				break
			}
		}
		if !meetsInclusion {
			continue
		}

		// Verificar critérios de exclusão
		meetsExclusion := true
		for key, value := range exclusionCriteria {
			if !cb.checkCriterionFromRow(ctx, patientID, row, key, value, false) {
				meetsExclusion = false
				break
			}
		}
		if !meetsExclusion {
			continue
		}

		patients = append(patients, patientID)
	}

	log.Printf("✅ [COHORT] %d pacientes selecionados", len(patients))

	return patients, nil
}

// checkCriterionFromRow verifica um critério contra os dados do paciente e dados relacionados
func (cb *CohortBuilder) checkCriterionFromRow(ctx context.Context, patientID int64, row map[string]interface{}, key string, value interface{}, include bool) bool {
	// Age criteria
	if key == "min_age" {
		if minAge, ok := value.(float64); ok {
			birthDate := database.GetTime(row, "data_nascimento")
			if birthDate.IsZero() {
				return false
			}
			age := float64(time.Since(birthDate).Hours() / 24 / 365)
			return age >= minAge
		}
	}

	if key == "max_age" {
		if maxAge, ok := value.(float64); ok {
			birthDate := database.GetTime(row, "data_nascimento")
			if birthDate.IsZero() {
				return false
			}
			age := float64(time.Since(birthDate).Hours() / 24 / 365)
			return age <= maxAge
		}
	}

	// Voice data availability
	if key == "has_voice_data" && value == true {
		voiceRows, err := cb.db.QueryByLabel(ctx, "VoiceProsodyAnalysis",
			" AND n.patient_id = $patient_id",
			map[string]interface{}{"patient_id": patientID}, 1)
		return err == nil && len(voiceRows) > 0
	}

	// Sleep data availability
	if key == "sleep_data_available" && value == true {
		sleepRows, err := cb.db.QueryByLabel(ctx, "SinalVital",
			" AND n.idoso_id = $idoso_id AND n.tipo = $tipo",
			map[string]interface{}{"idoso_id": patientID, "tipo": "sono"}, 1)
		return err == nil && len(sleepRows) > 0
	}

	// Medication logs availability
	if key == "medication_logs_available" && value == true {
		medRows, err := cb.db.QueryByLabel(ctx, "MedicationLog",
			" AND n.patient_id = $patient_id",
			map[string]interface{}{"patient_id": patientID}, 1)
		return err == nil && len(medRows) > 0
	}

	// Interaction logs availability
	if key == "interaction_logs_available" && value == true {
		sessRows, err := cb.db.QueryByLabel(ctx, "ConversationSession",
			" AND n.patient_id = $patient_id",
			map[string]interface{}{"patient_id": patientID}, 1)
		return err == nil && len(sessRows) > 0
	}

	// Minimum PHQ-9 assessments
	if key == "min_phq9_assessments" {
		if minAssessments, ok := value.(float64); ok {
			assessRows, err := cb.db.QueryByLabel(ctx, "ClinicalAssessment",
				" AND n.patient_id = $patient_id AND n.assessment_type = $type AND n.status = $status",
				map[string]interface{}{"patient_id": patientID, "type": "PHQ-9", "status": "completed"}, 0)
			return err == nil && float64(len(assessRows)) >= minAssessments
		}
	}

	// Minimum assessments (general)
	if key == "min_assessments" {
		if minAssessments, ok := value.(float64); ok {
			assessRows, err := cb.db.QueryByLabel(ctx, "ClinicalAssessment",
				" AND n.patient_id = $patient_id AND n.status = $status",
				map[string]interface{}{"patient_id": patientID, "status": "completed"}, 0)
			return err == nil && float64(len(assessRows)) >= minAssessments
		}
	}

	// On antidepressants
	if key == "on_antidepressants" && value == true {
		medRows, err := cb.db.QueryByLabel(ctx, "Medicamento",
			" AND n.idoso_id = $idoso_id",
			map[string]interface{}{"idoso_id": patientID}, 0)
		if err != nil {
			return false
		}
		antidepressants := []string{"fluoxetina", "sertralina", "escitalopram", "paroxetina", "venlafaxina", "duloxetina", "bupropiona", "mirtazapina"}
		for _, med := range medRows {
			medicamento := strings.ToLower(database.GetString(med, "medicamento"))
			for _, ad := range antidepressants {
				if strings.Contains(medicamento, ad) {
					return true
				}
			}
		}
		return false
	}

	// Baseline PHQ-9 range 5-15
	if key == "baseline_phq9_5_to_15" && value == true {
		assessRows, err := cb.db.QueryByLabel(ctx, "ClinicalAssessment",
			" AND n.patient_id = $patient_id AND n.assessment_type = $type",
			map[string]interface{}{"patient_id": patientID, "type": "PHQ-9"}, 0)
		if err != nil || len(assessRows) == 0 {
			return false
		}
		// Find the most recent assessment
		var latestScore float64
		var latestTime time.Time
		for _, a := range assessRows {
			t := database.GetTime(a, "completed_at")
			if t.After(latestTime) {
				latestTime = t
				latestScore = database.GetFloat64(a, "total_score")
			}
		}
		return latestScore >= 5 && latestScore <= 15
	}

	// Baseline PHQ-9 >= 10
	if key == "baseline_phq9_10_plus" && value == true {
		assessRows, err := cb.db.QueryByLabel(ctx, "ClinicalAssessment",
			" AND n.patient_id = $patient_id AND n.assessment_type = $type",
			map[string]interface{}{"patient_id": patientID, "type": "PHQ-9"}, 0)
		if err != nil || len(assessRows) == 0 {
			return false
		}
		var latestScore float64
		var latestTime time.Time
		for _, a := range assessRows {
			t := database.GetTime(a, "completed_at")
			if t.After(latestTime) {
				latestTime = t
				latestScore = database.GetFloat64(a, "total_score")
			}
		}
		return latestScore >= 10
	}

	// Exclusion: hospitalized
	if key == "hospitalized" && value == true && !include {
		thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
		laudoRows, err := cb.db.QueryByLabel(ctx, "LaudoMedico",
			" AND n.idoso_id = $idoso_id AND n.tipo_laudo = $tipo",
			map[string]interface{}{"idoso_id": patientID, "tipo": "hospitalizacao"}, 0)
		if err != nil || len(laudoRows) == 0 {
			return true // No hospitalization records = passes exclusion
		}
		for _, l := range laudoRows {
			dataLaudo := database.GetTime(l, "data_laudo")
			if dataLaudo.After(thirtyDaysAgo) {
				return false // Recent hospitalization = fails exclusion
			}
		}
		return true
	}

	// Exclusion: severe cognitive impairment
	if key == "severe_cognitive_impairment" && value == true && !include {
		cogImpairment := database.GetString(row, "cognitive_impairment_severe")
		return cogImpairment == "" || cogImpairment == "false"
	}

	// Exclusion: severe hearing impairment
	if key == "severe_hearing_impairment" && value == true && !include {
		hearingImpairment := database.GetString(row, "hearing_impairment_severe")
		return hearingImpairment == "" || hearingImpairment == "false"
	}

	// Exclusion: lives in facility
	if key == "lives_in_facility" && value == true && !include {
		livingSituation := database.GetString(row, "living_situation")
		return livingSituation != "assisted_living"
	}

	// Exclusion: diagnosed sleep apnea
	if key == "diagnosed_sleep_apnea" && value == true && !include {
		condRows, err := cb.db.QueryByLabel(ctx, "MentalHealthCondition",
			" AND n.patient_id = $patient_id AND n.condition_type = $type",
			map[string]interface{}{"patient_id": patientID, "type": "sleep_apnea"}, 1)
		return err == nil && len(condRows) == 0
	}

	// Default: ignorar critério desconhecido
	log.Printf("⚠️ [COHORT] Critério desconhecido: %s", key)
	return true
}

// buildCriteriaClause kept for reference but no longer used with NietzscheDB
func (cb *CohortBuilder) buildCriteriaClause(key string, value interface{}, argIndex int, include bool) (string, interface{}) {
	// Legacy SQL criteria builder - no longer used
	log.Printf("⚠️ [COHORT] buildCriteriaClause is deprecated (NietzscheDB migration)")
	return "", nil
}

// CountEligiblePatients conta pacientes elegíveis sem selecioná-los
func (cb *CohortBuilder) CountEligiblePatients(
	inclusionCriteria map[string]interface{},
	exclusionCriteria map[string]interface{},
) (int, error) {

	patients, err := cb.SelectPatients(inclusionCriteria, exclusionCriteria)
	if err != nil {
		return 0, err
	}

	return len(patients), nil
}

// ValidatePatientForCohort valida se um paciente específico atende critérios
func (cb *CohortBuilder) ValidatePatientForCohort(
	patientID int64,
	inclusionCriteria map[string]interface{},
	exclusionCriteria map[string]interface{},
) (bool, []string) {

	reasons := []string{}
	ctx := context.Background()

	// Buscar dados do paciente
	row, err := cb.db.GetNodeByID(ctx, "Idoso", patientID)
	if err != nil || row == nil {
		return false, []string{"Paciente não encontrado"}
	}

	// Verificar cada critério de inclusão
	for key, value := range inclusionCriteria {
		if !cb.checkCriterionFromRow(ctx, patientID, row, key, value, true) {
			reasons = append(reasons, "Não atende: "+key)
		}
	}

	// Verificar cada critério de exclusão
	for key, value := range exclusionCriteria {
		if !cb.checkCriterionFromRow(ctx, patientID, row, key, value, false) {
			reasons = append(reasons, "Excluído: "+key)
		}
	}

	return len(reasons) == 0, reasons
}

// checkSingleCriterion verifica um critério individual (legacy, delegates to checkCriterionFromRow)
func (cb *CohortBuilder) checkSingleCriterion(patientID int64, key string, value interface{}, include bool) (bool, string) {
	ctx := context.Background()
	row, err := cb.db.GetNodeByID(ctx, "Idoso", patientID)
	if err != nil || row == nil {
		return false, "Paciente não encontrado"
	}
	result := cb.checkCriterionFromRow(ctx, patientID, row, key, value, include)
	if !result {
		return false, key
	}
	return true, ""
}
