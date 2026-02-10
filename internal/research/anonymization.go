package research

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// ANONYMIZATION PIPELINE (LGPD/GDPR COMPLIANT)
// ============================================================================

type Anonymizer struct {
	db *sql.DB
}

func NewAnonymizer(db *sql.DB) *Anonymizer {
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

// saveDatapoint salva datapoint anonimizado no banco
func (a *Anonymizer) saveDatapoint(cohortID string, dp *ResearchDatapoint) error {
	query := `
		INSERT INTO research_datapoints (
			cohort_id, anonymous_patient_id, observation_date, days_since_baseline,
			phq9_score, gad7_score, cssrs_score,
			medication_adherence_7d, sleep_hours_avg_7d, sleep_efficiency,
			voice_pitch_mean_hz, voice_pitch_std_hz, voice_jitter, voice_shimmer,
			voice_hnr_db, speech_rate_wpm, pause_duration_avg_ms,
			social_isolation_days, interaction_count_7d, cognitive_load_score,
			crisis_occurred, crisis_severity, hospitalization, treatment_dropout,
			data_completeness, data_quality_score,
			is_anonymized, anonymization_date
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28
		)
		ON CONFLICT (cohort_id, anonymous_patient_id, observation_date) DO UPDATE
		SET phq9_score = EXCLUDED.phq9_score,
		    gad7_score = EXCLUDED.gad7_score,
		    data_completeness = EXCLUDED.data_completeness
	`

	_, err := a.db.Exec(
		query,
		cohortID, dp.AnonymousPatientID, dp.ObservationDate, dp.DaysSinceBaseline,
		dp.PHQ9Score, dp.GAD7Score, dp.CSSRSScore,
		dp.MedicationAdherence7d, dp.SleepHoursAvg7d, dp.SleepEfficiency,
		dp.VoicePitchMeanHz, dp.VoicePitchStdHz, dp.VoiceJitter, dp.VoiceShimmer,
		dp.VoiceHNRDb, dp.SpeechRateWPM, dp.PauseDurationAvgMs,
		dp.SocialIsolationDays, dp.InteractionCount7d, dp.CognitiveLoadScore,
		dp.CrisisOccurred, dp.CrisisSeverity, dp.Hospitalization, dp.TreatmentDropout,
		dp.DataCompleteness, dp.DataQualityScore,
		true, time.Now(),
	)

	return err
}

// ============================================================================
// QUERIES DE COLETA DE DADOS
// ============================================================================

func (a *Anonymizer) getPatientBaselineDate(patientID int64) (time.Time, error) {
	// Baseline = primeira interação ou data de cadastro
	query := `
		SELECT MIN(data_criacao)
		FROM idosos
		WHERE id = $1
	`

	var baselineDate time.Time
	err := a.db.QueryRow(query, patientID).Scan(&baselineDate)
	if err != nil {
		// Fallback: 6 meses atrás
		baselineDate = time.Now().AddDate(0, -6, 0)
	}

	return baselineDate, nil
}

func (a *Anonymizer) getLatestPHQ9BeforeDate(patientID int64, date time.Time) (*float64, error) {
	query := `
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1
		  AND assessment_type = 'PHQ-9'
		  AND completed_at <= $2
		ORDER BY completed_at DESC
		LIMIT 1
	`

	var score float64
	err := a.db.QueryRow(query, patientID, date).Scan(&score)
	if err != nil {
		return nil, err
	}

	return &score, nil
}

func (a *Anonymizer) getLatestGAD7BeforeDate(patientID int64, date time.Time) (*float64, error) {
	query := `
		SELECT total_score
		FROM clinical_assessments
		WHERE patient_id = $1
		  AND assessment_type = 'GAD-7'
		  AND completed_at <= $2
		ORDER BY completed_at DESC
		LIMIT 1
	`

	var score float64
	err := a.db.QueryRow(query, patientID, date).Scan(&score)
	if err != nil {
		return nil, err
	}

	return &score, nil
}

func (a *Anonymizer) getMedicationAdherence7d(patientID int64, date time.Time) (*float64, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE taken_at IS NOT NULL)::FLOAT /
			NULLIF(COUNT(*), 0) AS adherence
		FROM medication_logs
		WHERE patient_id = $1
		  AND scheduled_time BETWEEN $2 - INTERVAL '7 days' AND $2
	`

	var adherence float64
	err := a.db.QueryRow(query, patientID, date).Scan(&adherence)
	if err != nil {
		return nil, err
	}

	return &adherence, nil
}

func (a *Anonymizer) getSleepMetrics7d(patientID int64, date time.Time) (*float64, *float64, error) {
	query := `
		SELECT
			AVG(CAST(valor AS FLOAT)) as avg_hours,
			AVG(CASE WHEN metadata->>'efficiency' IS NOT NULL
			    THEN CAST(metadata->>'efficiency' AS FLOAT)
			    ELSE 0.75 END) as avg_efficiency
		FROM sinais_vitais
		WHERE idoso_id = $1
		  AND tipo = 'sono'
		  AND data_medicao BETWEEN $2 - INTERVAL '7 days' AND $2
	`

	var hours, efficiency sql.NullFloat64
	err := a.db.QueryRow(query, patientID, date).Scan(&hours, &efficiency)
	if err != nil || !hours.Valid {
		return nil, nil, err
	}

	h := hours.Float64
	e := efficiency.Float64
	return &h, &e, nil
}

func (a *Anonymizer) getVoiceMetrics7d(patientID int64, date time.Time) (map[string]*float64, error) {
	query := `
		SELECT
			AVG(vpf.pitch_mean) as pitch_mean,
			STDDEV(vpf.pitch_mean) as pitch_std,
			AVG(vpf.jitter) as jitter,
			AVG(vpf.shimmer) as shimmer,
			AVG(vpf.hnr) as hnr,
			AVG(vpf.speech_rate) as speech_rate,
			AVG(vpf.pause_duration_avg_ms) as pause_duration
		FROM voice_prosody_features vpf
		JOIN voice_prosody_analyses vpa ON vpf.analysis_id = vpa.id
		WHERE vpa.patient_id = $1
		  AND vpa.created_at BETWEEN $2 - INTERVAL '7 days' AND $2
	`

	var pitchMean, pitchStd, jitter, shimmer, hnr, speechRate, pauseDuration sql.NullFloat64
	err := a.db.QueryRow(query, patientID, date).Scan(
		&pitchMean, &pitchStd, &jitter, &shimmer, &hnr, &speechRate, &pauseDuration,
	)

	if err != nil {
		return nil, err
	}

	metrics := make(map[string]*float64)
	if pitchMean.Valid {
		v := pitchMean.Float64
		metrics["pitch_mean"] = &v
	}
	if pitchStd.Valid {
		v := pitchStd.Float64
		metrics["pitch_std"] = &v
	}
	if jitter.Valid {
		v := jitter.Float64
		metrics["jitter"] = &v
	}
	if shimmer.Valid {
		v := shimmer.Float64
		metrics["shimmer"] = &v
	}
	if hnr.Valid {
		v := hnr.Float64
		metrics["hnr"] = &v
	}
	if speechRate.Valid {
		v := speechRate.Float64
		metrics["speech_rate"] = &v
	}
	if pauseDuration.Valid {
		v := pauseDuration.Float64
		metrics["pause_duration"] = &v
	}

	return metrics, nil
}

func (a *Anonymizer) getSocialIsolationDays(patientID int64, date time.Time) (*int, error) {
	query := `
		SELECT COALESCE(
			EXTRACT(DAY FROM $2 - MAX(call_time)),
			999
		)::INTEGER as days
		FROM call_logs
		WHERE patient_id = $1
		  AND call_time <= $2
		  AND call_type IN ('family', 'caregiver', 'friend')
	`

	var days int
	err := a.db.QueryRow(query, patientID, date).Scan(&days)
	if err != nil {
		return nil, err
	}

	return &days, nil
}

func (a *Anonymizer) getInteractionCount7d(patientID int64, date time.Time) (*int, error) {
	query := `
		SELECT COUNT(*)::INTEGER
		FROM conversation_sessions
		WHERE patient_id = $1
		  AND started_at BETWEEN $2 - INTERVAL '7 days' AND $2
	`

	var count int
	err := a.db.QueryRow(query, patientID, date).Scan(&count)
	if err != nil {
		return nil, err
	}

	return &count, nil
}

func (a *Anonymizer) getCognitiveLoadScore(patientID int64, date time.Time) (*float64, error) {
	query := `
		SELECT current_load_score
		FROM cognitive_load_state
		WHERE patient_id = $1
	`

	var score float64
	err := a.db.QueryRow(query, patientID).Scan(&score)
	if err != nil {
		return nil, err
	}

	return &score, nil
}

func (a *Anonymizer) checkCrisisOnDate(patientID int64, date time.Time) (bool, *string, error) {
	query := `
		SELECT severity
		FROM crisis_events
		WHERE patient_id = $1
		  AND DATE(occurred_at) = DATE($2)
		LIMIT 1
	`

	var severity string
	err := a.db.QueryRow(query, patientID, date).Scan(&severity)
	if err == sql.ErrNoRows {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	return true, &severity, nil
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
	var kValue int
	query := `SELECT calculate_k_anonymity($1, ARRAY['observation_date']::TEXT[])`
	err := a.db.QueryRow(query, cohortID).Scan(&kValue)
	return kValue, err
}
