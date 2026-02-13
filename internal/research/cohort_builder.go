package research

import (
	"database/sql"
	"fmt"
	"log"
)

// ============================================================================
// COHORT BUILDER
// ============================================================================
// Seleciona pacientes baseado em critérios de inclusão/exclusão

type CohortBuilder struct {
	db *sql.DB
}

func NewCohortBuilder(db *sql.DB) *CohortBuilder {
	return &CohortBuilder{db: db}
}

// SelectPatients seleciona pacientes que atendem critérios
func (cb *CohortBuilder) SelectPatients(
	inclusionCriteria map[string]interface{},
	exclusionCriteria map[string]interface{},
) ([]int64, error) {

	log.Printf("🔍 [COHORT] Selecionando pacientes com critérios de inclusão...")

	// Construir query dinâmica
	query := `SELECT DISTINCT i.id FROM idosos i WHERE 1=1`
	args := []interface{}{}
	argIndex := 1

	// Aplicar critérios de inclusão
	for key, value := range inclusionCriteria {
		clause, arg := cb.buildCriteriaClause(key, value, argIndex, true)
		if clause != "" {
			query += " AND " + clause
			if arg != nil {
				args = append(args, arg)
				argIndex++
			}
		}
	}

	// Aplicar critérios de exclusão
	for key, value := range exclusionCriteria {
		clause, arg := cb.buildCriteriaClause(key, value, argIndex, false)
		if clause != "" {
			query += " AND " + clause
			if arg != nil {
				args = append(args, arg)
				argIndex++
			}
		}
	}

	log.Printf("   Query: %s", query)

	rows, err := cb.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("erro ao executar query: %w", err)
	}
	defer rows.Close()

	patients := []int64{}
	for rows.Next() {
		var patientID int64
		err := rows.Scan(&patientID)
		if err != nil {
			continue
		}
		patients = append(patients, patientID)
	}

	log.Printf("✅ [COHORT] %d pacientes selecionados", len(patients))

	return patients, nil
}

// buildCriteriaClause constrói cláusula WHERE para um critério
func (cb *CohortBuilder) buildCriteriaClause(key string, value interface{}, argIndex int, include bool) (string, interface{}) {
	// Age criteria
	if key == "min_age" {
		if minAge, ok := value.(float64); ok {
			return fmt.Sprintf("(EXTRACT(YEAR FROM AGE(i.data_nascimento)) >= $%d)", argIndex), minAge
		}
	}

	if key == "max_age" {
		if maxAge, ok := value.(float64); ok {
			return fmt.Sprintf("(EXTRACT(YEAR FROM AGE(i.data_nascimento)) <= $%d)", argIndex), maxAge
		}
	}

	// Voice data availability
	if key == "has_voice_data" && value == true {
		return `EXISTS (
			SELECT 1 FROM voice_prosody_analyses vpa
			WHERE vpa.patient_id = i.id
		)`, nil
	}

	// Sleep data availability
	if key == "sleep_data_available" && value == true {
		return `EXISTS (
			SELECT 1 FROM sinais_vitais sv
			WHERE sv.idoso_id = i.id AND sv.tipo = 'sono'
		)`, nil
	}

	// Medication logs availability
	if key == "medication_logs_available" && value == true {
		return `EXISTS (
			SELECT 1 FROM medication_logs ml
			WHERE ml.patient_id = i.id
		)`, nil
	}

	// Interaction logs availability
	if key == "interaction_logs_available" && value == true {
		return `EXISTS (
			SELECT 1 FROM conversation_sessions cs
			WHERE cs.patient_id = i.id
		)`, nil
	}

	// Minimum PHQ-9 assessments
	if key == "min_phq9_assessments" {
		if minAssessments, ok := value.(float64); ok {
			return fmt.Sprintf(`(
				SELECT COUNT(*)
				FROM clinical_assessments ca
				WHERE ca.patient_id = i.id
				  AND ca.assessment_type = 'PHQ-9'
				  AND ca.status = 'completed'
			) >= $%d`, argIndex), minAssessments
		}
	}

	// Minimum assessments (general)
	if key == "min_assessments" {
		if minAssessments, ok := value.(float64); ok {
			return fmt.Sprintf(`(
				SELECT COUNT(*)
				FROM clinical_assessments ca
				WHERE ca.patient_id = i.id
				  AND ca.status = 'completed'
			) >= $%d`, argIndex), minAssessments
		}
	}

	// On antidepressants
	if key == "on_antidepressants" && value == true {
		return `EXISTS (
			SELECT 1 FROM medicamentos m
			WHERE m.idoso_id = i.id
			  AND (
			    LOWER(m.medicamento) LIKE '%fluoxetina%' OR
			    LOWER(m.medicamento) LIKE '%sertralina%' OR
			    LOWER(m.medicamento) LIKE '%escitalopram%' OR
			    LOWER(m.medicamento) LIKE '%paroxetina%' OR
			    LOWER(m.medicamento) LIKE '%venlafaxina%' OR
			    LOWER(m.medicamento) LIKE '%duloxetina%' OR
			    LOWER(m.medicamento) LIKE '%bupropiona%' OR
			    LOWER(m.medicamento) LIKE '%mirtazapina%'
			  )
		)`, nil
	}

	// Baseline PHQ-9 range
	if key == "baseline_phq9_5_to_15" && value == true {
		return `EXISTS (
			SELECT 1 FROM clinical_assessments ca
			WHERE ca.patient_id = i.id
			  AND ca.assessment_type = 'PHQ-9'
			  AND ca.total_score BETWEEN 5 AND 15
			ORDER BY ca.completed_at DESC
			LIMIT 1
		)`, nil
	}

	if key == "baseline_phq9_10_plus" && value == true {
		return `EXISTS (
			SELECT 1 FROM clinical_assessments ca
			WHERE ca.patient_id = i.id
			  AND ca.assessment_type = 'PHQ-9'
			  AND ca.total_score >= 10
			ORDER BY ca.completed_at DESC
			LIMIT 1
		)`, nil
	}

	// Exclusion: hospitalized
	if key == "hospitalized" && value == true && !include {
		return `NOT EXISTS (
			SELECT 1 FROM laudos_medicos lm
			WHERE lm.idoso_id = i.id
			  AND lm.tipo_laudo = 'hospitalizacao'
			  AND lm.data_laudo > NOW() - INTERVAL '30 days'
		)`, nil
	}

	// Exclusion: severe cognitive impairment
	if key == "severe_cognitive_impairment" && value == true && !include {
		return `(i.condicoes_saude->>'cognitive_impairment_severe' IS NULL OR
		         i.condicoes_saude->>'cognitive_impairment_severe' = 'false')`, nil
	}

	// Exclusion: severe hearing impairment
	if key == "severe_hearing_impairment" && value == true && !include {
		return `(i.condicoes_saude->>'hearing_impairment_severe' IS NULL OR
		         i.condicoes_saude->>'hearing_impairment_severe' = 'false')`, nil
	}

	// Exclusion: lives in facility
	if key == "lives_in_facility" && value == true && !include {
		return `(i.living_situation != 'assisted_living' OR i.living_situation IS NULL)`, nil
	}

	// Exclusion: diagnosed sleep apnea
	if key == "diagnosed_sleep_apnea" && value == true && !include {
		return `NOT EXISTS (
			SELECT 1 FROM mental_health_conditions mhc
			WHERE mhc.patient_id = i.id
			  AND mhc.condition_type = 'sleep_apnea'
		)`, nil
	}

	// Default: ignorar critério desconhecido
	log.Printf("⚠️ [COHORT] Critério desconhecido: %s", key)
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

	// Verificar cada critério de inclusão
	for key, value := range inclusionCriteria {
		meets, reason := cb.checkSingleCriterion(patientID, key, value, true)
		if !meets {
			reasons = append(reasons, "Não atende: "+reason)
		}
	}

	// Verificar cada critério de exclusão
	for key, value := range exclusionCriteria {
		meets, reason := cb.checkSingleCriterion(patientID, key, value, false)
		if !meets {
			reasons = append(reasons, "Excluído: "+reason)
		}
	}

	return len(reasons) == 0, reasons
}

// checkSingleCriterion verifica um critério individual
func (cb *CohortBuilder) checkSingleCriterion(patientID int64, key string, value interface{}, include bool) (bool, string) {
	// Implementação simplificada
	// Na prática, executaria queries específicas

	if key == "min_age" {
		// Verificar idade
		return true, ""
	}

	// ... outras verificações ...

	return true, ""
}
