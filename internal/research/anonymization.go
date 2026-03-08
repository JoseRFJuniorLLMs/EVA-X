// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package research

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/database"
)

// ============================================================================
// ANONYMIZATION PIPELINE (LGPD/GDPR COMPLIANT)
// ============================================================================

type Anonymizer struct {
	db *database.DB
}

func NewAnonymizer(db *database.DB) *Anonymizer {
	return &Anonymizer{db: db}
}

// ResearchDatapoint representa um ponto de dados anonimizado
type ResearchDatapoint struct {
	CohortID            string    `json:"cohort_id"`
	AnonymousPatientID  string    `json:"anonymous_patient_id"`
	ObservationDate     time.Time `json:"observation_date"`
	DaysSinceBaseline   int       `json:"days_since_baseline"`

	// Clinical features
	PHQ9Score           *float64  `json:"phq9_score,omitempty"`
	GAD7Score           *float64  `json:"gad7_score,omitempty"`
	CSSRSScore          *int      `json:"cssrs_score,omitempty"`

	// Adherence & Sleep
	MedicationAdherence7d *float64 `json:"medication_adherence_7d,omitempty"`
	SleepHoursAvg7d      *float64 `json:"sleep_hours_avg_7d,omitempty"`
	SleepEfficiency      *float64 `json:"sleep_efficiency,omitempty"`

	// Voice biomarkers
	VoicePitchMeanHz     *float64 `json:"voice_pitch_mean_hz,omitempty"`
	VoicePitchStdHz      *float64 `json:"voice_pitch_std_hz,omitempty"`
	VoiceJitter          *float64 `json:"voice_jitter,omitempty"`
	VoiceShimmer         *float64 `json:"voice_shimmer,omitempty"`
	VoiceHNRDb           *float64 `json:"voice_hnr_db,omitempty"`
	SpeechRateWPM        *float64 `json:"speech_rate_wpm,omitempty"`
	PauseDurationAvgMs   *float64 `json:"pause_duration_avg_ms,omitempty"`

	// Social & Cognitive
	SocialIsolationDays  *int     `json:"social_isolation_days,omitempty"`
	InteractionCount7d   *int     `json:"interaction_count_7d,omitempty"`
	CognitiveLoadScore   *float64 `json:"cognitive_load_score,omitempty"`

	// Outcomes
	CrisisOccurred       bool     `json:"crisis_occurred"`
	CrisisSeverity       *string  `json:"crisis_severity,omitempty"`
	Hospitalization      bool     `json:"hospitalization"`
	TreatmentDropout     bool     `json:"treatment_dropout"`

	// Quality metrics
	DataCompleteness     float64  `json:"data_completeness"`
	DataQualityScore     float64  `json:"data_quality_score"`
}

// CollectAndAnonymizePatientData coleta dados longitudinais e anonimiza
func (a *Anonymizer) CollectAndAnonymizePatientData(cohortID string, patientID int64, followupDays int) error {
	// 1. Gerar ID anonimizado
	anonymousID := AnonymizePatientID(patientID)

	// 2. Determinar baseline date
	baselineDate, err := a.getPatientBaselineDate(patientID)
	if err != nil {
		return fmt.Errorf("erro ao obter baseline date: %w", err)
	}

	// 3. Coletar dados dia a dia durante followup period
	endDate := baselineDate.AddDate(0, 0, followupDays)

	for currentDate := baselineDate; currentDate.Before(endDate); currentDate = currentDate.AddDate(0, 0, 1) {
		datapoint, err := a.collectDatapointForDate(patientID, anonymousID, currentDate, baselineDate)
		if err != nil {
			log.Printf("⚠️ Erro ao coletar datapoint para %s: %v", currentDate.Format("2006-01-02"), err)
			continue
		}

		// 4. Salvar datapoint anonimizado
		err = a.saveDatapoint(cohortID, datapoint)
		if err != nil {
			log.Printf("⚠️ Erro ao salvar datapoint: %v", err)
		}
	}

	return nil
}

// collectDatapointForDate coleta dados de um paciente em uma data específica
func (a *Anonymizer) collectDatapointForDate(patientID int64, anonymousID string, date time.Time, baselineDate time.Time) (*ResearchDatapoint, error) {
	dp := &ResearchDatapoint{
		AnonymousPatientID: anonymousID,
		ObservationDate:    date,
		DaysSinceBaseline:  int(date.Sub(baselineDate).Hours() / 24),
	}

	// Coletar PHQ-9 mais recente até esta data
	phq9, _ := a.getLatestPHQ9BeforeDate(patientID, date)
	dp.PHQ9Score = phq9

	// Coletar GAD-7
	gad7, _ := a.getLatestGAD7BeforeDate(patientID, date)
	dp.GAD7Score = gad7

	// Adesão medicamentosa (últimos 7 dias)
	adherence, _ := a.getMedicationAdherence7d(patientID, date)
	dp.MedicationAdherence7d = adherence

	// Sono (últimos 7 dias)
	sleepHours, sleepEff, _ := a.getSleepMetrics7d(patientID, date)
	dp.SleepHoursAvg7d = sleepHours
	dp.SleepEfficiency = sleepEff

	// Biomarcadores de voz (últimos 7 dias)
	voiceMetrics, _ := a.getVoiceMetrics7d(patientID, date)
	if voiceMetrics != nil {
		dp.VoicePitchMeanHz = voiceMetrics["pitch_mean"]
		dp.VoicePitchStdHz = voiceMetrics["pitch_std"]
		dp.VoiceJitter = voiceMetrics["jitter"]
		dp.VoiceShimmer = voiceMetrics["shimmer"]
		dp.VoiceHNRDb = voiceMetrics["hnr"]
		dp.SpeechRateWPM = voiceMetrics["speech_rate"]
		dp.PauseDurationAvgMs = voiceMetrics["pause_duration"]
	}

	// Isolamento social
	isolationDays, _ := a.getSocialIsolationDays(patientID, date)
	dp.SocialIsolationDays = isolationDays

	// Interações (últimos 7 dias)
	interactionCount, _ := a.getInteractionCount7d(patientID, date)
	dp.InteractionCount7d = interactionCount

	// Carga cognitiva
	cogLoad, _ := a.getCognitiveLoadScore(patientID, date)
	dp.CognitiveLoadScore = cogLoad

	// Verificar se houve crise nesta data
	crisisOccurred, crisisSeverity, _ := a.checkCrisisOnDate(patientID, date)
	dp.CrisisOccurred = crisisOccurred
	dp.CrisisSeverity = crisisSeverity

	// Hospitalização
	hospitalized, _ := a.checkHospitalizationOnDate(patientID, date)
	dp.Hospitalization = hospitalized

	// Calcular completeness e quality
	dp.DataCompleteness = a.calculateDataCompleteness(dp)
	dp.DataQualityScore = a.calculateDataQuality(dp)

	return dp, nil
}

// saveDatapoint salva datapoint anonimizado no NietzscheDB
func (a *Anonymizer) saveDatapoint(cohortID string, dp *ResearchDatapoint) error {
	ctx := context.Background()

	content := map[string]interface{}{
		"node_label":             "research_datapoint",
		"cohort_id":              cohortID,
		"anonymous_patient_id":   dp.AnonymousPatientID,
		"observation_date":       dp.ObservationDate,
		"days_since_baseline":    dp.DaysSinceBaseline,
		"phq9_score":             dp.PHQ9Score,
		"gad7_score":             dp.GAD7Score,
		"cssrs_score":            dp.CSSRSScore,
		"medication_adherence_7d": dp.MedicationAdherence7d,
		"sleep_hours_avg_7d":     dp.SleepHoursAvg7d,
		"sleep_efficiency":       dp.SleepEfficiency,
		"voice_pitch_mean_hz":    dp.VoicePitchMeanHz,
		"voice_pitch_std_hz":     dp.VoicePitchStdHz,
		"voice_jitter":           dp.VoiceJitter,
		"voice_shimmer":          dp.VoiceShimmer,
		"voice_hnr_db":           dp.VoiceHNRDb,
		"speech_rate_wpm":        dp.SpeechRateWPM,
		"pause_duration_avg_ms":  dp.PauseDurationAvgMs,
		"social_isolation_days":  dp.SocialIsolationDays,
		"interaction_count_7d":   dp.InteractionCount7d,
		"cognitive_load_score":   dp.CognitiveLoadScore,
		"crisis_occurred":        dp.CrisisOccurred,
		"crisis_severity":        dp.CrisisSeverity,
		"hospitalization":        dp.Hospitalization,
		"treatment_dropout":      dp.TreatmentDropout,
		"data_completeness":      dp.DataCompleteness,
		"data_quality_score":     dp.DataQualityScore,
		"is_anonymized":          true,
		"anonymization_date":     time.Now(),
	}

	// Check if this datapoint already exists (upsert by cohort_id + anonymous_patient_id + observation_date)
	existing, _ := a.db.QueryByLabel(ctx, "research_datapoint",
		" AND n.cohort_id = $cohort_id AND n.anonymous_patient_id = $anon_id AND n.observation_date = $obs_date",
		map[string]interface{}{
			"cohort_id": cohortID,
			"anon_id":   dp.AnonymousPatientID,
			"obs_date":  dp.ObservationDate,
		}, 1)

	if len(existing) > 0 {
		// Update existing
		return a.db.Update(ctx, "research_datapoint",
			map[string]interface{}{
				"cohort_id":            cohortID,
				"anonymous_patient_id": dp.AnonymousPatientID,
				"observation_date":     dp.ObservationDate,
			},
			map[string]interface{}{
				"phq9_score":        dp.PHQ9Score,
				"gad7_score":        dp.GAD7Score,
				"data_completeness": dp.DataCompleteness,
			})
	}

	// Insert new
	_, err := a.db.Insert(ctx, "research_datapoint", content)
	return err
}

// ============================================================================
// QUERIES DE COLETA DE DADOS (NietzscheDB)
// ============================================================================

func (a *Anonymizer) getPatientBaselineDate(patientID int64) (time.Time, error) {
	ctx := context.Background()

	// Baseline = data de cadastro do paciente
	row, err := a.db.GetNodeByID(ctx, "Idoso", patientID)
	if err != nil || row == nil {
		// Fallback: 6 meses atrás
		return time.Now().AddDate(0, -6, 0), nil
	}

	baselineDate := database.GetTime(row, "data_criacao")
	if baselineDate.IsZero() {
		baselineDate = database.GetTime(row, "created_at")
	}
	if baselineDate.IsZero() {
		baselineDate = time.Now().AddDate(0, -6, 0)
	}

	return baselineDate, nil
}

func (a *Anonymizer) getLatestPHQ9BeforeDate(patientID int64, date time.Time) (*float64, error) {
	ctx := context.Background()

	rows, err := a.db.QueryByLabel(ctx, "ClinicalAssessment",
		" AND n.patient_id = $patient_id AND n.assessment_type = $type",
		map[string]interface{}{"patient_id": patientID, "type": "PHQ-9"}, 0)
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	// Find the latest assessment before or on the given date
	var latestScore *float64
	var latestTime time.Time
	for _, row := range rows {
		completedAt := database.GetTime(row, "completed_at")
		if completedAt.IsZero() || completedAt.After(date) {
			continue
		}
		if latestScore == nil || completedAt.After(latestTime) {
			latestTime = completedAt
			score := database.GetFloat64(row, "total_score")
			latestScore = &score
		}
	}

	return latestScore, nil
}

func (a *Anonymizer) getLatestGAD7BeforeDate(patientID int64, date time.Time) (*float64, error) {
	ctx := context.Background()

	rows, err := a.db.QueryByLabel(ctx, "ClinicalAssessment",
		" AND n.patient_id = $patient_id AND n.assessment_type = $type",
		map[string]interface{}{"patient_id": patientID, "type": "GAD-7"}, 0)
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	var latestScore *float64
	var latestTime time.Time
	for _, row := range rows {
		completedAt := database.GetTime(row, "completed_at")
		if completedAt.IsZero() || completedAt.After(date) {
			continue
		}
		if latestScore == nil || completedAt.After(latestTime) {
			latestTime = completedAt
			score := database.GetFloat64(row, "total_score")
			latestScore = &score
		}
	}

	return latestScore, nil
}

func (a *Anonymizer) getMedicationAdherence7d(patientID int64, date time.Time) (*float64, error) {
	ctx := context.Background()
	sevenDaysAgo := date.AddDate(0, 0, -7)

	rows, err := a.db.QueryByLabel(ctx, "MedicationLog",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 0)
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	// Filter to the 7-day window and calculate adherence
	totalScheduled := 0
	totalTaken := 0
	for _, row := range rows {
		scheduledTime := database.GetTime(row, "scheduled_time")
		if scheduledTime.Before(sevenDaysAgo) || scheduledTime.After(date) {
			continue
		}
		totalScheduled++
		takenAt := database.GetTime(row, "taken_at")
		if !takenAt.IsZero() {
			totalTaken++
		}
	}

	if totalScheduled == 0 {
		return nil, nil
	}

	adherence := float64(totalTaken) / float64(totalScheduled)
	return &adherence, nil
}

func (a *Anonymizer) getSleepMetrics7d(patientID int64, date time.Time) (*float64, *float64, error) {
	ctx := context.Background()
	sevenDaysAgo := date.AddDate(0, 0, -7)

	rows, err := a.db.QueryByLabel(ctx, "SinalVital",
		" AND n.idoso_id = $idoso_id AND n.tipo = $tipo",
		map[string]interface{}{"idoso_id": patientID, "tipo": "sono"}, 0)
	if err != nil || len(rows) == 0 {
		return nil, nil, err
	}

	var totalHours, totalEfficiency float64
	count := 0
	for _, row := range rows {
		dataMedicao := database.GetTime(row, "data_medicao")
		if dataMedicao.Before(sevenDaysAgo) || dataMedicao.After(date) {
			continue
		}
		count++
		totalHours += database.GetFloat64(row, "valor")
		eff := database.GetFloat64(row, "efficiency")
		if eff == 0 {
			eff = 0.75 // default
		}
		totalEfficiency += eff
	}

	if count == 0 {
		return nil, nil, nil
	}

	avgHours := totalHours / float64(count)
	avgEfficiency := totalEfficiency / float64(count)
	return &avgHours, &avgEfficiency, nil
}

func (a *Anonymizer) getVoiceMetrics7d(patientID int64, date time.Time) (map[string]*float64, error) {
	ctx := context.Background()
	sevenDaysAgo := date.AddDate(0, 0, -7)

	// Get voice analyses for this patient
	analyses, err := a.db.QueryByLabel(ctx, "VoiceProsodyAnalysis",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 0)
	if err != nil || len(analyses) == 0 {
		return nil, err
	}

	// Collect analysis IDs within the time window
	var analysisIDs []int64
	for _, a := range analyses {
		createdAt := database.GetTime(a, "created_at")
		if createdAt.Before(sevenDaysAgo) || createdAt.After(date) {
			continue
		}
		id := database.GetInt64(a, "pg_id")
		if id == 0 {
			id = database.GetInt64(a, "id")
		}
		analysisIDs = append(analysisIDs, id)
	}

	if len(analysisIDs) == 0 {
		return nil, nil
	}

	// Get voice features for these analyses
	var allFeatures []map[string]interface{}
	for _, aid := range analysisIDs {
		features, err := a.db.QueryByLabel(ctx, "VoiceProsodyFeature",
			" AND n.analysis_id = $analysis_id",
			map[string]interface{}{"analysis_id": aid}, 0)
		if err == nil {
			allFeatures = append(allFeatures, features...)
		}
	}

	if len(allFeatures) == 0 {
		return nil, nil
	}

	// Calculate averages
	var sumPitch, sumJitter, sumShimmer, sumHNR, sumSpeechRate, sumPause float64
	var sumPitchStd float64
	n := float64(len(allFeatures))

	for _, f := range allFeatures {
		sumPitch += database.GetFloat64(f, "pitch_mean")
		sumPitchStd += database.GetFloat64(f, "pitch_std")
		sumJitter += database.GetFloat64(f, "jitter")
		sumShimmer += database.GetFloat64(f, "shimmer")
		sumHNR += database.GetFloat64(f, "hnr")
		sumSpeechRate += database.GetFloat64(f, "speech_rate")
		sumPause += database.GetFloat64(f, "pause_duration_avg_ms")
	}

	metrics := make(map[string]*float64)
	if sumPitch > 0 {
		v := sumPitch / n
		metrics["pitch_mean"] = &v
	}
	if sumPitchStd > 0 {
		v := sumPitchStd / n
		metrics["pitch_std"] = &v
	}
	if sumJitter > 0 {
		v := sumJitter / n
		metrics["jitter"] = &v
	}
	if sumShimmer > 0 {
		v := sumShimmer / n
		metrics["shimmer"] = &v
	}
	if sumHNR > 0 {
		v := sumHNR / n
		metrics["hnr"] = &v
	}
	if sumSpeechRate > 0 {
		v := sumSpeechRate / n
		metrics["speech_rate"] = &v
	}
	if sumPause > 0 {
		v := sumPause / n
		metrics["pause_duration"] = &v
	}

	return metrics, nil
}

func (a *Anonymizer) getSocialIsolationDays(patientID int64, date time.Time) (*int, error) {
	ctx := context.Background()

	rows, err := a.db.QueryByLabel(ctx, "CallLog",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 0)
	if err != nil {
		return nil, err
	}

	// Find the most recent social call before or on the date
	var latestCallTime time.Time
	socialTypes := map[string]bool{"family": true, "caregiver": true, "friend": true}

	for _, row := range rows {
		callType := database.GetString(row, "call_type")
		if !socialTypes[callType] {
			continue
		}
		callTime := database.GetTime(row, "call_time")
		if callTime.After(date) {
			continue
		}
		if callTime.After(latestCallTime) {
			latestCallTime = callTime
		}
	}

	if latestCallTime.IsZero() {
		days := 999
		return &days, nil
	}

	days := int(date.Sub(latestCallTime).Hours() / 24)
	return &days, nil
}

func (a *Anonymizer) getInteractionCount7d(patientID int64, date time.Time) (*int, error) {
	ctx := context.Background()
	sevenDaysAgo := date.AddDate(0, 0, -7)

	rows, err := a.db.QueryByLabel(ctx, "ConversationSession",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 0)
	if err != nil {
		return nil, err
	}

	count := 0
	for _, row := range rows {
		startedAt := database.GetTime(row, "started_at")
		if startedAt.Before(sevenDaysAgo) || startedAt.After(date) {
			continue
		}
		count++
	}

	return &count, nil
}

func (a *Anonymizer) getCognitiveLoadScore(patientID int64, date time.Time) (*float64, error) {
	ctx := context.Background()

	rows, err := a.db.QueryByLabel(ctx, "CognitiveLoadState",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 1)
	if err != nil || len(rows) == 0 {
		return nil, err
	}

	score := database.GetFloat64(rows[0], "current_load_score")
	return &score, nil
}

func (a *Anonymizer) checkCrisisOnDate(patientID int64, date time.Time) (bool, *string, error) {
	ctx := context.Background()

	rows, err := a.db.QueryByLabel(ctx, "CrisisEvent",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID}, 0)
	if err != nil || len(rows) == 0 {
		return false, nil, nil
	}

	dateStr := date.Format("2006-01-02")
	for _, row := range rows {
		occurredAt := database.GetTime(row, "occurred_at")
		if occurredAt.Format("2006-01-02") == dateStr {
			severity := database.GetString(row, "severity")
			return true, &severity, nil
		}
	}

	return false, nil, nil
}

func (a *Anonymizer) checkHospitalizationOnDate(patientID int64, date time.Time) (bool, error) {
	// Placeholder - implementar busca em registros de hospitalização
	return false, nil
}

// ============================================================================
// QUALIDADE DE DADOS
// ============================================================================

func (a *Anonymizer) calculateDataCompleteness(dp *ResearchDatapoint) float64 {
	totalFields := 0
	filledFields := 0

	// Contar quantos campos estão preenchidos
	if dp.PHQ9Score != nil {
		filledFields++
	}
	totalFields++

	if dp.GAD7Score != nil {
		filledFields++
	}
	totalFields++

	if dp.MedicationAdherence7d != nil {
		filledFields++
	}
	totalFields++

	if dp.SleepHoursAvg7d != nil {
		filledFields++
	}
	totalFields++

	if dp.VoicePitchMeanHz != nil {
		filledFields++
	}
	totalFields++

	if dp.SocialIsolationDays != nil {
		filledFields++
	}
	totalFields++

	if dp.CognitiveLoadScore != nil {
		filledFields++
	}
	totalFields++

	return float64(filledFields) / float64(totalFields)
}

func (a *Anonymizer) calculateDataQuality(dp *ResearchDatapoint) float64 {
	// Verificar outliers e valores suspeitos
	qualityScore := 1.0

	// PHQ-9 fora de range
	if dp.PHQ9Score != nil && (*dp.PHQ9Score < 0 || *dp.PHQ9Score > 27) {
		qualityScore -= 0.2
	}

	// GAD-7 fora de range
	if dp.GAD7Score != nil && (*dp.GAD7Score < 0 || *dp.GAD7Score > 21) {
		qualityScore -= 0.2
	}

	// Adesão fora de 0-1
	if dp.MedicationAdherence7d != nil && (*dp.MedicationAdherence7d < 0 || *dp.MedicationAdherence7d > 1) {
		qualityScore -= 0.2
	}

	// Sono impossível (>12h ou <0)
	if dp.SleepHoursAvg7d != nil && (*dp.SleepHoursAvg7d < 0 || *dp.SleepHoursAvg7d > 12) {
		qualityScore -= 0.2
	}

	if qualityScore < 0 {
		qualityScore = 0
	}

	return qualityScore
}

// ============================================================================
// K-ANONYMITY CHECK
// ============================================================================

func (a *Anonymizer) CalculateKAnonymity(cohortID string) (int, error) {
	ctx := context.Background()

	// Get all research datapoints for this cohort
	rows, err := a.db.QueryByLabel(ctx, "research_datapoint",
		" AND n.cohort_id = $cohort_id",
		map[string]interface{}{"cohort_id": cohortID}, 0)
	if err != nil {
		return 0, err
	}

	if len(rows) == 0 {
		return 0, nil
	}

	// Group by quasi-identifiers (observation_date) and find minimum group size
	groups := make(map[string]int)
	for _, row := range rows {
		obsDate := database.GetTime(row, "observation_date")
		key := obsDate.Format("2006-01-02")
		groups[key]++
	}

	// K-anonymity = smallest group size
	minK := len(rows) // start with max possible
	for _, count := range groups {
		if count < minK {
			minK = count
		}
	}

	return minK, err
}
