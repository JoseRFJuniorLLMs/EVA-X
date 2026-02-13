package research

import (
	"database/sql"
	"fmt"
	"log"
	"math"
)

// ============================================================================
// LONGITUDINAL ANALYSIS (TIME SERIES & LAG CORRELATIONS)
// ============================================================================

type LongitudinalAnalyzer struct {
	db        *sql.DB
	statMethods *StatisticalMethods
}

func NewLongitudinalAnalyzer(db *sql.DB) *LongitudinalAnalyzer {
	return &LongitudinalAnalyzer{
		db:        db,
		statMethods: NewStatisticalMethods(),
	}
}

// CorrelationResult representa resultado de análise de correlação
type CorrelationResult struct {
	PredictorVariable         string
	OutcomeVariable          string
	LagDays                  int
	CorrelationCoefficient   float64
	PValue                   float64
	ConfidenceIntervalLower  float64
	ConfidenceIntervalUpper  float64
	NObservations            int
	NPatients                int
	AdjustedForCovariates    []string
}

// TimeSeries representa uma série temporal de um paciente
type TimeSeries struct {
	AnonymousPatientID string
	Days               []int
	Values             []float64
}

// ============================================================================
// LAG CORRELATION ANALYSIS
// ============================================================================

// CalculateLagCorrelations calcula correlações com diferentes lags
func (la *LongitudinalAnalyzer) CalculateLagCorrelations(
	cohortID string,
	predictorVariable string,
	outcomeVariable string,
	maxLag int,
) ([]CorrelationResult, error) {

	log.Printf("🔬 [LONGITUDINAL] Calculando lag correlations: %s → %s (lag 0-%d)", predictorVariable, outcomeVariable, maxLag)

	results := []CorrelationResult{}

	// Para cada lag de 0 até maxLag
	for lag := 0; lag <= maxLag; lag++ {
		result, err := la.calculateSingleLagCorrelation(cohortID, predictorVariable, outcomeVariable, lag)
		if err != nil {
			log.Printf("⚠️ Erro no lag %d: %v", lag, err)
			continue
		}

		if result != nil {
			results = append(results, *result)

			if result.PValue < 0.05 {
				log.Printf("   ✅ Lag %d: r=%.3f, p=%.6f (SIGNIFICATIVO)", lag, result.CorrelationCoefficient, result.PValue)
			}
		}
	}

	return results, nil
}

// calculateSingleLagCorrelation calcula correlação para um lag específico
func (la *LongitudinalAnalyzer) calculateSingleLagCorrelation(
	cohortID string,
	predictorVariable string,
	outcomeVariable string,
	lag int,
) (*CorrelationResult, error) {

	// 1. Buscar pares (predictor_t, outcome_t+lag) do banco
	predictorSeries, err := la.getTimeSeriesData(cohortID, predictorVariable)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar predictor: %w", err)
	}

	outcomeSeries, err := la.getTimeSeriesData(cohortID, outcomeVariable)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar outcome: %w", err)
	}

	// 2. Alinhar séries com lag
	predictorValues, outcomeValues := la.alignSeriesWithLag(predictorSeries, outcomeSeries, lag)

	if len(predictorValues) < 10 {
		return nil, fmt.Errorf("insuficiente dados para lag %d (n=%d)", lag, len(predictorValues))
	}

	// 3. Calcular correlação de Pearson
	r := la.statMethods.PearsonCorrelation(predictorValues, outcomeValues)

	// 4. Calcular p-value (teste de significância)
	n := len(predictorValues)
	pValue := la.statMethods.CorrelationPValue(r, n)

	// 5. Intervalo de confiança 95%
	ciLower, ciUpper := la.statMethods.CorrelationConfidenceInterval(r, n, 0.95)

	// 6. Contar pacientes únicos
	nPatients := la.countUniquePatients(predictorSeries)

	result := &CorrelationResult{
		PredictorVariable:        predictorVariable,
		OutcomeVariable:         outcomeVariable,
		LagDays:                 lag,
		CorrelationCoefficient:  r,
		PValue:                  pValue,
		ConfidenceIntervalLower: ciLower,
		ConfidenceIntervalUpper: ciUpper,
		NObservations:           n,
		NPatients:               nPatients,
	}

	return result, nil
}

// getTimeSeriesData busca dados de série temporal do banco
func (la *LongitudinalAnalyzer) getTimeSeriesData(cohortID string, variable string) (map[string]TimeSeries, error) {
	// Mapear nome da variável para coluna do banco
	columnName := la.mapVariableToColumn(variable)

	query := fmt.Sprintf(`
		SELECT
			anonymous_patient_id,
			days_since_baseline,
			%s
		FROM research_datapoints
		WHERE cohort_id = $1
		  AND %s IS NOT NULL
		ORDER BY anonymous_patient_id, days_since_baseline
	`, columnName, columnName)

	rows, err := la.db.Query(query, cohortID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seriesMap := make(map[string]TimeSeries)

	for rows.Next() {
		var patientID string
		var day int
		var value float64

		err := rows.Scan(&patientID, &day, &value)
		if err != nil {
			continue
		}

		if ts, exists := seriesMap[patientID]; exists {
			ts.Days = append(ts.Days, day)
			ts.Values = append(ts.Values, value)
			seriesMap[patientID] = ts
		} else {
			seriesMap[patientID] = TimeSeries{
				AnonymousPatientID: patientID,
				Days:               []int{day},
				Values:             []float64{value},
			}
		}
	}

	return seriesMap, nil
}

// alignSeriesWithLag alinha duas séries temporais com lag especificado
func (la *LongitudinalAnalyzer) alignSeriesWithLag(
	predictorSeries map[string]TimeSeries,
	outcomeSeries map[string]TimeSeries,
	lag int,
) ([]float64, []float64) {

	predictorValues := []float64{}
	outcomeValues := []float64{}

	// Para cada paciente
	for patientID, predTS := range predictorSeries {
		outcTS, exists := outcomeSeries[patientID]
		if !exists {
			continue
		}

		// Alinhar: predictor em dia t, outcome em dia t+lag
		for i, predDay := range predTS.Days {
			targetDay := predDay + lag

			// Buscar outcome no dia target
			for j, outcDay := range outcTS.Days {
				if outcDay == targetDay {
					predictorValues = append(predictorValues, predTS.Values[i])
					outcomeValues = append(outcomeValues, outcTS.Values[j])
					break
				}
			}
		}
	}

	return predictorValues, outcomeValues
}

// countUniquePatients conta pacientes únicos em séries
func (la *LongitudinalAnalyzer) countUniquePatients(series map[string]TimeSeries) int {
	return len(series)
}

// mapVariableToColumn mapeia nome da variável para coluna do banco
func (la *LongitudinalAnalyzer) mapVariableToColumn(variable string) string {
	mapping := map[string]string{
		"voice_pitch_mean":      "voice_pitch_mean_hz",
		"voice_jitter":          "voice_jitter",
		"voice_shimmer":         "voice_shimmer",
		"voice_hnr":             "voice_hnr_db",
		"speech_rate":           "speech_rate_wpm",
		"phq9":                  "phq9_score",
		"gad7":                  "gad7_score",
		"medication_adherence":  "medication_adherence_7d",
		"sleep_hours":           "sleep_hours_avg_7d",
		"sleep_efficiency":      "sleep_efficiency",
		"social_isolation":      "social_isolation_days",
		"cognitive_load":        "cognitive_load_score",
		"interaction_count":     "interaction_count_7d",
	}

	if column, ok := mapping[variable]; ok {
		return column
	}

	// Default: assume que é o nome da coluna
	return variable
}

// ============================================================================
// TREND ANALYSIS
// ============================================================================

// CalculateTrend calcula tendência de uma variável ao longo do tempo
func (la *LongitudinalAnalyzer) CalculateTrend(cohortID string, variable string) (*TrendAnalysis, error) {
	series, err := la.getTimeSeriesData(cohortID, variable)
	if err != nil {
		return nil, err
	}

	// Agregar todas as séries
	allDays := []float64{}
	allValues := []float64{}

	for _, ts := range series {
		for i, day := range ts.Days {
			allDays = append(allDays, float64(day))
			allValues = append(allValues, ts.Values[i])
		}
	}

	// Regressão linear simples
	slope, intercept, rSquared := la.statMethods.SimpleLinearRegression(allDays, allValues)

	trend := &TrendAnalysis{
		Variable:   variable,
		Slope:      slope,
		Intercept:  intercept,
		RSquared:   rSquared,
		NPoints:    len(allValues),
	}

	if slope > 0 {
		trend.Direction = "increasing"
	} else if slope < 0 {
		trend.Direction = "decreasing"
	} else {
		trend.Direction = "stable"
	}

	return trend, nil
}

type TrendAnalysis struct {
	Variable   string
	Slope      float64
	Intercept  float64
	RSquared   float64
	Direction  string // "increasing", "decreasing", "stable"
	NPoints    int
}

// ============================================================================
// CHANGE POINT DETECTION
// ============================================================================

// DetectChangePoints detecta mudanças abruptas em séries temporais
func (la *LongitudinalAnalyzer) DetectChangePoints(cohortID string, variable string, threshold float64) ([]ChangePoint, error) {
	series, err := la.getTimeSeriesData(cohortID, variable)
	if err != nil {
		return nil, err
	}

	changePoints := []ChangePoint{}

	// Para cada paciente, detectar mudanças
	for patientID, ts := range series {
		if len(ts.Values) < 5 {
			continue
		}

		// Calcular diferenças consecutivas
		for i := 1; i < len(ts.Values); i++ {
			diff := ts.Values[i] - ts.Values[i-1]
			percentChange := (diff / ts.Values[i-1]) * 100

			if math.Abs(percentChange) > threshold {
				cp := ChangePoint{
					PatientID:     patientID,
					Day:           ts.Days[i],
					ValueBefore:   ts.Values[i-1],
					ValueAfter:    ts.Values[i],
					PercentChange: percentChange,
				}
				changePoints = append(changePoints, cp)
			}
		}
	}

	log.Printf("🔍 [LONGITUDINAL] Detectados %d change points para %s (threshold=%.1f%%)", len(changePoints), variable, threshold)

	return changePoints, nil
}

type ChangePoint struct {
	PatientID     string
	Day           int
	ValueBefore   float64
	ValueAfter    float64
	PercentChange float64
}
