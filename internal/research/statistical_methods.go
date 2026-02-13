package research

import (
	"math"
)

// ============================================================================
// STATISTICAL METHODS
// ============================================================================
// Implementação de métodos estatísticos básicos

type StatisticalMethods struct{}

func NewStatisticalMethods() *StatisticalMethods {
	return &StatisticalMethods{}
}

// ============================================================================
// CORRELAÇÃO
// ============================================================================

// PearsonCorrelation calcula correlação de Pearson entre duas variáveis
func (sm *StatisticalMethods) PearsonCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	_ = float64(len(x)) // n calculado mas não usado diretamente

	// Calcular médias
	meanX := sm.Mean(x)
	meanY := sm.Mean(y)

	// Calcular numerador e denominadores
	numerator := 0.0
	sumSqX := 0.0
	sumSqY := 0.0

	for i := 0; i < len(x); i++ {
		devX := x[i] - meanX
		devY := y[i] - meanY

		numerator += devX * devY
		sumSqX += devX * devX
		sumSqY += devY * devY
	}

	denominator := math.Sqrt(sumSqX * sumSqY)

	if denominator == 0 {
		return 0
	}

	r := numerator / denominator
	return r
}

// CorrelationPValue calcula p-value para correlação
// H0: r = 0 (sem correlação)
func (sm *StatisticalMethods) CorrelationPValue(r float64, n int) float64 {
	if n < 3 {
		return 1.0
	}

	// t-statistic
	t := r * math.Sqrt(float64(n-2)) / math.Sqrt(1-r*r)

	// Degrees of freedom
	df := n - 2

	// P-value (two-tailed) usando aproximação
	pValue := sm.TDistributionPValue(math.Abs(t), df) * 2

	return pValue
}

// CorrelationConfidenceInterval calcula intervalo de confiança para r
func (sm *StatisticalMethods) CorrelationConfidenceInterval(r float64, n int, confidence float64) (float64, float64) {
	if n < 4 {
		return r, r
	}

	// Fisher's Z transformation
	zr := 0.5 * math.Log((1+r)/(1-r))

	// Standard error
	se := 1.0 / math.Sqrt(float64(n-3))

	// Z-score para o nível de confiança
	z := sm.ZScoreForConfidence(confidence)

	// Intervalo em Z
	zLower := zr - z*se
	zUpper := zr + z*se

	// Transformar de volta para r
	rLower := (math.Exp(2*zLower) - 1) / (math.Exp(2*zLower) + 1)
	rUpper := (math.Exp(2*zUpper) - 1) / (math.Exp(2*zUpper) + 1)

	return rLower, rUpper
}

// ============================================================================
// REGRESSÃO LINEAR
// ============================================================================

// SimpleLinearRegression: y = mx + b
// Retorna: slope (m), intercept (b), r-squared
func (sm *StatisticalMethods) SimpleLinearRegression(x, y []float64) (float64, float64, float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 0, 0
	}

	_ = float64(len(x)) // n calculado mas não usado diretamente

	// Calcular médias
	meanX := sm.Mean(x)
	meanY := sm.Mean(y)

	// Calcular slope (m)
	numerator := 0.0
	denominator := 0.0

	for i := 0; i < len(x); i++ {
		devX := x[i] - meanX
		numerator += devX * (y[i] - meanY)
		denominator += devX * devX
	}

	slope := numerator / denominator
	intercept := meanY - slope*meanX

	// Calcular R²
	ssTotal := 0.0
	ssResidual := 0.0

	for i := 0; i < len(x); i++ {
		predicted := slope*x[i] + intercept
		ssTotal += (y[i] - meanY) * (y[i] - meanY)
		ssResidual += (y[i] - predicted) * (y[i] - predicted)
	}

	rSquared := 1 - (ssResidual / ssTotal)

	return slope, intercept, rSquared
}

// ============================================================================
// TESTES ESTATÍSTICOS
// ============================================================================

// TTest independent samples t-test
func (sm *StatisticalMethods) TTest(group1, group2 []float64) (tStat float64, pValue float64) {
	n1 := len(group1)
	n2 := len(group2)

	if n1 < 2 || n2 < 2 {
		return 0, 1.0
	}

	mean1 := sm.Mean(group1)
	mean2 := sm.Mean(group2)

	var1 := sm.Variance(group1)
	var2 := sm.Variance(group2)

	// Pooled variance
	pooledVar := ((float64(n1-1)*var1 + float64(n2-1)*var2) / float64(n1+n2-2))

	// Standard error
	se := math.Sqrt(pooledVar * (1.0/float64(n1) + 1.0/float64(n2)))

	// T-statistic
	tStat = (mean1 - mean2) / se

	// Degrees of freedom
	df := n1 + n2 - 2

	// P-value (two-tailed)
	pValue = sm.TDistributionPValue(math.Abs(tStat), df) * 2

	return tStat, pValue
}

// ============================================================================
// ESTATÍSTICAS DESCRITIVAS
// ============================================================================

// Mean calcula média
func (sm *StatisticalMethods) Mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

// Variance calcula variância
func (sm *StatisticalMethods) Variance(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	mean := sm.Mean(values)
	sumSq := 0.0

	for _, v := range values {
		dev := v - mean
		sumSq += dev * dev
	}

	return sumSq / float64(len(values)-1)
}

// StandardDeviation calcula desvio padrão
func (sm *StatisticalMethods) StandardDeviation(values []float64) float64 {
	return math.Sqrt(sm.Variance(values))
}

// Median calcula mediana
func (sm *StatisticalMethods) Median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Copiar e ordenar
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Simple bubble sort (ok para pequenos datasets)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2.0
	}

	return sorted[n/2]
}

// Percentile calcula percentil (0-100)
func (sm *StatisticalMethods) Percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Ordenar
	sorted := make([]float64, len(values))
	copy(sorted, values)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calcular índice
	index := (p / 100.0) * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))

	if lower == upper {
		return sorted[lower]
	}

	// Interpolação linear
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

// ============================================================================
// DISTRIBUIÇÕES
// ============================================================================

// TDistributionPValue aproximação de p-value para distribuição t
func (sm *StatisticalMethods) TDistributionPValue(t float64, df int) float64 {
	// Aproximação usando normal para df > 30
	if df > 30 {
		return sm.NormalDistributionPValue(t)
	}

	// Aproximação simples para df < 30
	// (Para produção, usar biblioteca estatística completa)
	dfFloat := float64(df)
	x := dfFloat / (dfFloat + t*t)
	p := 0.5 * math.Pow(x, dfFloat/2.0)

	return p
}

// NormalDistributionPValue p-value para distribuição normal padrão
func (sm *StatisticalMethods) NormalDistributionPValue(z float64) float64 {
	// Aproximação de erro complementar
	// P(Z > z) para z > 0

	z = math.Abs(z)

	// Aproximação de Abramowitz & Stegun (1964)
	t := 1.0 / (1.0 + 0.2316419*z)
	d := 0.3989423 * math.Exp(-z*z/2.0)
	p := d * t * (0.3193815 + t*(-0.3565638+t*(1.781478+t*(-1.821256+t*1.330274))))

	return p
}

// ZScoreForConfidence retorna z-score para um nível de confiança
func (sm *StatisticalMethods) ZScoreForConfidence(confidence float64) float64 {
	// Mapeamento comum
	zScores := map[float64]float64{
		0.90: 1.645,
		0.95: 1.960,
		0.99: 2.576,
	}

	if z, ok := zScores[confidence]; ok {
		return z
	}

	// Default: 95%
	return 1.960
}

// ============================================================================
// EFFECT SIZE
// ============================================================================

// CohensD calcula Cohen's d (effect size para t-test)
func (sm *StatisticalMethods) CohensD(group1, group2 []float64) float64 {
	mean1 := sm.Mean(group1)
	mean2 := sm.Mean(group2)

	n1 := len(group1)
	n2 := len(group2)

	var1 := sm.Variance(group1)
	var2 := sm.Variance(group2)

	// Pooled standard deviation
	pooledSD := math.Sqrt(((float64(n1-1)*var1 + float64(n2-1)*var2) / float64(n1+n2-2)))

	d := (mean1 - mean2) / pooledSD

	return d
}

// InterpretCohensD interpreta o tamanho do efeito
func (sm *StatisticalMethods) InterpretCohensD(d float64) string {
	absD := math.Abs(d)

	if absD < 0.2 {
		return "negligible"
	} else if absD < 0.5 {
		return "small"
	} else if absD < 0.8 {
		return "medium"
	}

	return "large"
}
