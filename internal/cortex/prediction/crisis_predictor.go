// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package prediction

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
	"eva/internal/cortex/explainability"
)

// CrisisPredictor prediz risco de crises e gera explicacoes
type CrisisPredictor struct {
	db        *database.DB
	explainer *explainability.ClinicalDecisionExplainer
	ctx       context.Context
}

// NewCrisisPredictor cria novo preditor
func NewCrisisPredictor(db *database.DB) *CrisisPredictor {
	return &CrisisPredictor{
		db:        db,
		explainer: explainability.NewClinicalDecisionExplainer(db),
		ctx:       context.Background(),
	}
}

// PredictCrisisRisk prediz risco de crise e gera explicacao
func (cp *CrisisPredictor) PredictCrisisRisk(patientID int64) (*explainability.Explanation, error) {
	log.Printf("[PREDICTOR] Iniciando predicao de risco de crise para paciente %d", patientID)

	// 1. Coletar features de diferentes fontes
	features, err := cp.collectFeatures(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao coletar features: %w", err)
	}

	if len(features) == 0 {
		return nil, fmt.Errorf("nenhuma feature disponivel para paciente %d", patientID)
	}

	// 2. Calcular score de risco
	riskScore, severity, timeframe := cp.calculateRiskScore(features)

	log.Printf("[PREDICTOR] Score de risco calculado: %.2f (%s)", riskScore, severity)

	// 3. Criar predicao
	prediction := explainability.ClinicalPrediction{
		PatientID:           patientID,
		DecisionType:        "crisis_prediction",
		PredictionScore:     riskScore,
		PredictionTimeframe: timeframe,
		Severity:            severity,
		Features:            features,
		ModelVersion:        "v1.0.0",
	}

	// 4. Gerar explicacao usando o explainer
	explanation, err := cp.explainer.ExplainDecision(prediction)
	if err != nil {
		return nil, fmt.Errorf("erro ao gerar explicacao: %w", err)
	}

	return explanation, nil
}

// collectFeatures coleta features de todas as fontes disponiveis
func (cp *CrisisPredictor) collectFeatures(patientID int64) (map[string]explainability.Feature, error) {
	features := make(map[string]explainability.Feature)

	// 1. Adesao medicamentosa (ultimos 7 dias)
	medicationAdherence, err := cp.getMedicationAdherence(patientID, 7)
	if err == nil && medicationAdherence >= 0 {
		status := cp.getAdherenceStatus(medicationAdherence)
		features["medication_adherence"] = explainability.Feature{
			Name:          "medication_adherence",
			CurrentValue:  medicationAdherence,
			BaselineValue: 0.85, // 85% e considerado bom
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

	// 4. Qualidade do sono (ultimos 7 dias)
	sleepQuality, err := cp.getSleepQuality(patientID, 7)
	if err == nil && sleepQuality > 0 {
		status := cp.getSleepStatus(sleepQuality)
		features["sleep_quality"] = explainability.Feature{
			Name:          "sleep_quality",
			CurrentValue:  sleepQuality,
			BaselineValue: 7.5, // 7-8h e ideal
			Category:      "secondary",
			Status:        status,
			Details: map[string]interface{}{
				"avg_hours_per_night": sleepQuality,
				"period_days":         7,
			},
		}
	}

	// 5. Biomarcadores de voz (pitch mean - ultimos 7 dias)
	voicePitchMean, err := cp.getVoicePitchMean(patientID, 7)
	if err == nil && voicePitchMean > 0 {
		// Buscar baseline (ultimos 30 dias)
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
				"unit":                  "Hz",
				"description":           "Pitch medio (tom de voz)",
				"change_from_baseline":  voicePitchMean - voicePitchBaseline,
			},
		}
	}

	// 6. Isolamento social (dias sem interacao humana)
	daysSinceHumanInteraction, err := cp.getDaysSinceHumanInteraction(patientID)
	if err == nil {
		status := cp.getIsolationStatus(daysSinceHumanInteraction)
		features["social_isolation"] = explainability.Feature{
			Name:          "social_isolation",
			CurrentValue:  float64(daysSinceHumanInteraction),
			BaselineValue: 2.0, // Ideal e contato a cada 2 dias
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
			BaselineValue: 0.5, // 0.5 e considerado normal
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
			weight = 0.05 // Peso padrao para outras features
		}

		// Calcular contribuicao da feature para o risco
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

	// Se multiplos fatores criticos, escalar severidade
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
	// Query all medication logs for this patient in the period
	rows, err := cp.db.QueryByLabel(cp.ctx, "medication_logs",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil || len(rows) == 0 {
		return -1, err
	}

	// Calculate adherence: count taken / total
	total := 0
	taken := 0
	for _, m := range rows {
		total++
		takenAt := database.GetString(m, "taken_at")
		if takenAt != "" {
			taken++
		}
	}

	if total == 0 {
		return -1, fmt.Errorf("no medication logs")
	}

	return float64(taken) / float64(total), nil
}

func (cp *CrisisPredictor) getLatestPHQ9Score(patientID int64) (float64, error) {
	rows, err := cp.db.QueryByLabel(cp.ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.assessment_type = $atype AND n.status = $status",
		map[string]interface{}{
			"pid":    patientID,
			"atype":  "PHQ-9",
			"status": "completed",
		},
		1,
	)
	if err != nil || len(rows) == 0 {
		return -1, err
	}

	score := database.GetFloat64(rows[0], "total_score")
	return score, nil
}

func (cp *CrisisPredictor) getLatestGAD7Score(patientID int64) (float64, error) {
	rows, err := cp.db.QueryByLabel(cp.ctx, "clinical_assessments",
		" AND n.patient_id = $pid AND n.assessment_type = $atype AND n.status = $status",
		map[string]interface{}{
			"pid":    patientID,
			"atype":  "GAD-7",
			"status": "completed",
		},
		1,
	)
	if err != nil || len(rows) == 0 {
		return -1, err
	}

	score := database.GetFloat64(rows[0], "total_score")
	return score, nil
}

func (cp *CrisisPredictor) getSleepQuality(patientID int64, days int) (float64, error) {
	// Buscar sinais vitais tipo sono
	rows, err := cp.db.QueryByLabel(cp.ctx, "sinais_vitais",
		" AND n.idoso_id = $pid AND n.tipo = $tipo",
		map[string]interface{}{
			"pid":  patientID,
			"tipo": "sono",
		},
		0,
	)
	if err != nil || len(rows) == 0 {
		return 0, err
	}

	// Calculate average
	total := 0.0
	count := 0
	for _, m := range rows {
		val := database.GetFloat64(m, "valor")
		if val > 0 {
			total += val
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no sleep data")
	}

	return total / float64(count), nil
}

func (cp *CrisisPredictor) getVoicePitchMean(patientID int64, days int) (float64, error) {
	// Query voice prosody features
	rows, err := cp.db.QueryByLabel(cp.ctx, "voice_prosody_features",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil || len(rows) == 0 {
		return 0, err
	}

	// Calculate average pitch_mean
	total := 0.0
	count := 0
	for _, m := range rows {
		pitchMean := database.GetFloat64(m, "pitch_mean")
		if pitchMean > 0 {
			total += pitchMean
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no voice data")
	}

	return total / float64(count), nil
}

func (cp *CrisisPredictor) getDaysSinceHumanInteraction(patientID int64) (int, error) {
	// Query call logs for family/caregiver/friend calls
	rows, err := cp.db.QueryByLabel(cp.ctx, "call_logs",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		0,
	)
	if err != nil || len(rows) == 0 {
		return 999, nil // Assumir muito tempo sem contato
	}

	// Find most recent call of type family/caregiver/friend
	var latestCallTime time.Time
	for _, m := range rows {
		callType := database.GetString(m, "call_type")
		if callType == "family" || callType == "caregiver" || callType == "friend" {
			callTime := database.GetTime(m, "call_time")
			if latestCallTime.IsZero() || callTime.After(latestCallTime) {
				latestCallTime = callTime
			}
		}
	}

	if latestCallTime.IsZero() {
		return 999, nil
	}

	daysSince := int(time.Since(latestCallTime).Hours() / 24)
	return daysSince, nil
}

func (cp *CrisisPredictor) getCognitiveLoadScore(patientID int64) (float64, error) {
	rows, err := cp.db.QueryByLabel(cp.ctx, "cognitive_load_state",
		" AND n.patient_id = $pid",
		map[string]interface{}{"pid": patientID},
		1,
	)
	if err != nil || len(rows) == 0 {
		return 0, err
	}

	return database.GetFloat64(rows[0], "current_load_score"), nil
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
		return "Depressao severa"
	} else if score >= 15 {
		return "Depressao moderadamente severa"
	} else if score >= 10 {
		return "Depressao moderada"
	} else if score >= 5 {
		return "Depressao leve"
	}
	return "Minimo ou nenhum"
}

func (cp *CrisisPredictor) interpretGAD7(score float64) string {
	if score >= 15 {
		return "Ansiedade severa"
	} else if score >= 10 {
		return "Ansiedade moderada"
	} else if score >= 5 {
		return "Ansiedade leve"
	}
	return "Minimo ou nenhum"
}
