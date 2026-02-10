package prediction

import (
	"database/sql"
	"eva-mind/internal/cortex/explainability"
	"fmt"
	"log"
)

// CrisisPredictor prediz risco de crises e gera explicações
type CrisisPredictor struct {
	db       *sql.DB
	explainer *explainability.ClinicalDecisionExplainer
}

// NewCrisisPredictor cria novo preditor
func NewCrisisPredictor(db *sql.DB) *CrisisPredictor {
	return &CrisisPredictor{
		db:       db,
		explainer: explainability.NewClinicalDecisionExplainer(db),
	}
}

// PredictCrisisRisk prediz risco de crise e gera explicação
func (cp *CrisisPredictor) PredictCrisisRisk(patientID int64) (*explainability.Explanation, error) {
	log.Printf("🔮 [PREDICTOR] Iniciando predição de risco de crise para paciente %d", patientID)

	// 1. Coletar features de diferentes fontes
	features, err := cp.collectFeatures(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao coletar features: %w", err)
	}

	if len(features) == 0 {
		return nil, fmt.Errorf("nenhuma feature disponível para paciente %d", patientID)
	}

	// 2. Calcular score de risco
	riskScore, severity, timeframe := cp.calculateRiskScore(features)

	log.Printf("📊 [PREDICTOR] Score de risco calculado: %.2f (%s)", riskScore, severity)

	// 3. Criar predição
	prediction := explainability.ClinicalPrediction{
		PatientID:           patientID,
		DecisionType:        "crisis_prediction",
		PredictionScore:     riskScore,
		PredictionTimeframe: timeframe,
		Severity:            severity,
		Features:            features,
		ModelVersion:        "v1.0.0",
	}

	// 4. Gerar explicação usando o explainer
	explanation, err := cp.explainer.ExplainDecision(prediction)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar explicação: %w", err)
	}

	return explanation, nil
}

// collectFeatures coleta features de todas as fontes disponíveis
func (cp *CrisisPredictor) collectFeatures(patientID int64) (map[string]explainability.Feature, error) {
	features := make(map[string]explainability.Feature)

	// 1. Adesão medicamentosa (últimos 7 dias)
	medicationAdherence, err := cp.getMedicationAdherence(patientID, 7)
	if err == nil && medicationAdherence >= 0 {
		status := cp.getAdherenceStatus(medicationAdherence)
		features["medication_adherence"] = explainability.Feature{
			Name:          "medication_adherence",
			CurrentValue:  medicationAdherence,
			BaselineValue: 0.85, // 85% é considerado bom
			Category:      "primary",
			Status:        status,
			Details: map[string]interface{}{
				"period_days": 7,
				"description": "Porcentagem de doses tomadas conforme prescrito",
			},
		}
	}

	// 2. Score PHQ-9 mais recente
	phq9Score, err := cp.getLatestPHQ9Score(patientID)
	if err == nil && phq9Score >= 0 {
		status := cp.getPHQ9Status(phq9Score)
		features["phq9_score"] = explainability.Feature{
			Name:          "phq9_score",
			CurrentValue:  phq9Score,
			BaselineValue: 5.0, // Score 5 = limite mild
			Category:      "primary",
			Status:        status,
			Details: map[string]interface{}{
				"interpretation": cp.interpretPHQ9(phq9Score),
				"scale_range":    "0-27",
			},
		}
	}

	// 3. Score GAD-7 mais recente
	gad7Score, err := cp.getLatestGAD7Score(patientID)
	if err == nil && gad7Score >= 0 {
		status := cp.getGAD7Status(gad7Score)
		features["gad7_score"] = explainability.Feature{
			Name:          "gad7_score",
			CurrentValue:  gad7Score,
			BaselineValue: 5.0,
			Category:      "secondary",
			Status:        status,
			Details: map[string]interface{}{
				"interpretation": cp.interpretGAD7(gad7Score),
				"scale_range":    "0-21",
			},
		}
	}

	// 4. Qualidade do sono (últimos 7 dias)
	sleepQuality, err := cp.getSleepQuality(patientID, 7)
	if err == nil && sleepQuality > 0 {
		status := cp.getSleepStatus(sleepQuality)
		features["sleep_quality"] = explainability.Feature{
			Name:          "sleep_quality",
			CurrentValue:  sleepQuality,
			BaselineValue: 7.5, // 7-8h é ideal
			Category:      "secondary",
			Status:        status,
			Details: map[string]interface{}{
				"avg_hours_per_night": sleepQuality,
				"period_days":         7,
			},
		}
	}

	// 5. Biomarcadores de voz (pitch mean - últimos 7 dias)
	voicePitchMean, err := cp.getVoicePitchMean(patientID, 7)
	if err == nil && voicePitchMean > 0 {
		// Buscar baseline (últimos 30 dias)
		voicePitchBaseline, _ := cp.getVoicePitchMean(patientID, 30)
		if voicePitchBaseline == 0 {
			voicePitchBaseline = 150.0 // Default masculino adulto
		}

		status := cp.getVoicePitchStatus(voicePitchMean, voicePitchBaseline)
		features["voice_pitch_mean"] = explainability.Feature{
			Name:          "voice_pitch_mean",
			CurrentValue:  voicePitchMean,
			BaselineValue: voicePitchBaseline,
			Category:      "secondary",
			Status:        status,
			Details: map[string]interface{}{
				"unit":          "Hz",
				"description":   "Pitch médio (tom de voz)",
				"change_from_baseline": voicePitchMean - voicePitchBaseline,
			},
		}
	}

	// 6. Isolamento social (dias sem interação humana)
	daysSinceHumanInteraction, err := cp.getDaysSinceHumanInteraction(patientID)
	if err == nil {
		status := cp.getIsolationStatus(daysSinceHumanInteraction)
		features["social_isolation"] = explainability.Feature{
			Name:          "social_isolation",
			CurrentValue:  float64(daysSinceHumanInteraction),
			BaselineValue: 2.0, // Ideal é contato a cada 2 dias
			Category:      "secondary",
			Status:        status,
			Details: map[string]interface{}{
				"days_without_contact": daysSinceHumanInteraction,
			},
		}
	}

	// 7. Carga cognitiva atual
	cognitiveLoad, err := cp.getCognitiveLoadScore(patientID)
	if err == nil {
		status := cp.getCognitiveLoadStatus(cognitiveLoad)
		features["cognitive_load"] = explainability.Feature{
			Name:          "cognitive_load",
			CurrentValue:  cognitiveLoad,
			BaselineValue: 0.5, // 0.5 é considerado normal
			Category:      "tertiary",
			Status:        status,
		}
	}

	return features, nil
}

// calculateRiskScore calcula score de risco baseado nas features
func (cp *CrisisPredictor) calculateRiskScore(features map[string]explainability.Feature) (float64, string, string) {
	// Algoritmo simplificado de scoring
	riskScore := 0.0
	riskFactors := 0

	// Pesos por feature
	weights := map[string]float64{
		"medication_adherence": 0.35, // 35% de peso
		"phq9_score":           0.25, // 25%
		"gad7_score":           0.15, // 15%
		"sleep_quality":        0.10, // 10%
		"voice_pitch_mean":     0.10, // 10%
		"social_isolation":     0.05, // 5%
	}

	for name, feature := range features {
		weight := weights[name]
		if weight == 0 {
			weight = 0.05 // Peso padrão para outras features
		}

		// Calcular contribuição da feature para o risco
		contribution := 0.0

		switch feature.Status {
		case "critical":
			contribution = 1.0
			riskFactors++
		case "concerning":
			contribution = 0.75
			riskFactors++
		case "warning":
			contribution = 0.5
		case "normal":
			contribution = 0.0
		}

		riskScore += contribution * weight
	}

	// Normalizar (0-1)
	if riskScore > 1.0 {
		riskScore = 1.0
	}

	// Determinar severidade
	severity := "low"
	timeframe := "7-14 days"

	if riskScore >= 0.75 {
		severity = "critical"
		timeframe = "24-48h"
	} else if riskScore >= 0.60 {
		severity = "high"
		timeframe = "3-5 days"
	} else if riskScore >= 0.40 {
		severity = "medium"
		timeframe = "5-7 days"
	}

	// Se múltiplos fatores críticos, escalar severidade
	if riskFactors >= 3 {
		severity = "critical"
		timeframe = "24h"
	} else if riskFactors >= 2 && severity != "critical" {
		severity = "high"
	}

	return riskScore, severity, timeframe
}

// ========================================
// QUERIES DE FEATURES
// ========================================

func (cp *CrisisPredictor) getMedicationAdherence(patientID int64, days int) (float64, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE taken_at IS NOT NULL)::FLOAT /
			NULLIF(COUNT(*), 0) AS adherence
		FROM medication_logs
		WHERE patient_id = $1
		  AND scheduled_time > NOW() - INTERVAL '1 day' * $2
	`

	var adherence sql.NullFloat64
	err := cp.db.QueryRow(query, patientID, days).Scan(&adherence)
	if err != nil || !adherence.Valid {
		return -1, err
	}

	return adherence.Float64, nil
}

func (cp *CrisisPredictor) getLatestPHQ9Score(patientID int64) (float64, error) {
	query := `
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1
		  AND assessment_type = 'PHQ-9'
		  AND status = 'completed'
		ORDER BY completed_at DESC
		LIMIT 1
	`

	var score sql.NullFloat64
	err := cp.db.QueryRow(query, patientID).Scan(&score)
	if err != nil || !score.Valid {
		return -1, err
	}

	return score.Float64, nil
}

func (cp *CrisisPredictor) getLatestGAD7Score(patientID int64) (float64, error) {
	query := `
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1
		  AND assessment_type = 'GAD-7'
		  AND status = 'completed'
		ORDER BY completed_at DESC
		LIMIT 1
	`

	var score sql.NullFloat64
	err := cp.db.QueryRow(query, patientID).Scan(&score)
	if err != nil || !score.Valid {
		return -1, err
	}

	return score.Float64, nil
}

func (cp *CrisisPredictor) getSleepQuality(patientID int64, days int) (float64, error) {
	// Placeholder: buscar de wearable ou self-report
	query := `
		SELECT AVG(CAST(valor AS FLOAT))
		FROM sinais_vitais
		WHERE idoso_id = $1
		  AND tipo = 'sono'
		  AND data_medicao > NOW() - INTERVAL '1 day' * $2
	`

	var avgSleep sql.NullFloat64
	err := cp.db.QueryRow(query, patientID, days).Scan(&avgSleep)
	if err != nil || !avgSleep.Valid {
		return 0, err
	}

	return avgSleep.Float64, nil
}

func (cp *CrisisPredictor) getVoicePitchMean(patientID int64, days int) (float64, error) {
	query := `
		SELECT AVG(vpf.pitch_mean)
		FROM voice_prosody_features vpf
		JOIN voice_prosody_analyses vpa ON vpf.analysis_id = vpa.id
		WHERE vpa.patient_id = $1
		  AND vpa.created_at > NOW() - INTERVAL '1 day' * $2
	`

	var pitchMean sql.NullFloat64
	err := cp.db.QueryRow(query, patientID, days).Scan(&pitchMean)
	if err != nil || !pitchMean.Valid {
		return 0, err
	}

	return pitchMean.Float64, nil
}

func (cp *CrisisPredictor) getDaysSinceHumanInteraction(patientID int64) (int, error) {
	// Placeholder: buscar de call logs, mensagens família, etc
	query := `
		SELECT COALESCE(
			EXTRACT(DAY FROM NOW() - MAX(call_time)),
			999
		)
		FROM call_logs
		WHERE patient_id = $1
		  AND call_type IN ('family', 'caregiver', 'friend')
	`

	var days sql.NullInt64
	err := cp.db.QueryRow(query, patientID).Scan(&days)
	if err != nil || !days.Valid {
		return 999, nil // Assumir muito tempo sem contato
	}

	return int(days.Int64), nil
}

func (cp *CrisisPredictor) getCognitiveLoadScore(patientID int64) (float64, error) {
	query := `
		SELECT current_load_score
		FROM cognitive_load_state
		WHERE patient_id = $1
	`

	var loadScore sql.NullFloat64
	err := cp.db.QueryRow(query, patientID).Scan(&loadScore)
	if err != nil || !loadScore.Valid {
		return 0, err
	}

	return loadScore.Float64, nil
}

// ========================================
// STATUS HELPERS
// ========================================

func (cp *CrisisPredictor) getAdherenceStatus(adherence float64) string {
	if adherence < 0.5 {
		return "critical"
	} else if adherence < 0.7 {
		return "concerning"
	} else if adherence < 0.85 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getPHQ9Status(score float64) string {
	if score >= 20 {
		return "critical"
	} else if score >= 15 {
		return "concerning"
	} else if score >= 10 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getGAD7Status(score float64) string {
	if score >= 15 {
		return "critical"
	} else if score >= 10 {
		return "concerning"
	} else if score >= 5 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getSleepStatus(hours float64) string {
	if hours < 4 {
		return "critical"
	} else if hours < 5 {
		return "concerning"
	} else if hours < 6 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getVoicePitchStatus(current, baseline float64) string {
	change := current - baseline
	changePercent := (change / baseline) * 100

	if changePercent < -15 || changePercent > 15 {
		return "concerning"
	} else if changePercent < -10 || changePercent > 10 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getIsolationStatus(days int) string {
	if days >= 7 {
		return "critical"
	} else if days >= 5 {
		return "concerning"
	} else if days >= 3 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) getCognitiveLoadStatus(load float64) string {
	if load > 0.85 {
		return "concerning"
	} else if load > 0.7 {
		return "warning"
	}
	return "normal"
}

func (cp *CrisisPredictor) interpretPHQ9(score float64) string {
	if score >= 20 {
		return "Depressão severa"
	} else if score >= 15 {
		return "Depressão moderadamente severa"
	} else if score >= 10 {
		return "Depressão moderada"
	} else if score >= 5 {
		return "Depressão leve"
	}
	return "Mínimo ou nenhum"
}

func (cp *CrisisPredictor) interpretGAD7(score float64) string {
	if score >= 15 {
		return "Ansiedade severa"
	} else if score >= 10 {
		return "Ansiedade moderada"
	} else if score >= 5 {
		return "Ansiedade leve"
	}
	return "Mínimo ou nenhum"
}
