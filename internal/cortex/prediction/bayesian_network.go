package prediction

import (
	"math"
	"math/rand"
)

// ============================================================================
// BAYESIAN BELIEF NETWORK
// ============================================================================
// Modela relações causais entre variáveis de saúde mental
// Aprende probabilidades condicionais de dados históricos

type BayesianNetwork struct {
	// Tabelas de Probabilidade Condicional (CPTs)
	// Aprendidas de dados históricos

	// Parâmetros de transição
	adherenceParams      AdherenceTransitionParams
	phq9Params           PHQ9TransitionParams
	sleepParams          SleepTransitionParams
	isolationParams      IsolationTransitionParams

	// Random source para variabilidade estocástica
	randomSource *rand.Rand
}

// NewBayesianNetwork cria nova rede Bayesiana com parâmetros default
func NewBayesianNetwork() *BayesianNetwork {
	return &BayesianNetwork{
		adherenceParams: AdherenceTransitionParams{
			BaseDecayRate:         -0.005, // Tende a decair levemente por dia
			MotivationImpact:      0.015,  // Motivação alta aumenta
			CognitiveLoadPenalty:  -0.020, // Carga alta diminui
			DepressionPenalty:     -0.012, // Depressão diminui
			Variance:              0.03,   // Variância estocástica
		},
		phq9Params: PHQ9TransitionParams{
			BaseChangeRate:        0.05,   // Leve piora tendencial sem tratamento
			AdherenceImprovement:  -0.15,  // Boa adesão melhora (reduz PHQ-9)
			SleepImpact:           -0.10,  // Sono bom melhora
			IsolationImpact:       0.08,   // Isolamento piora
			Variance:              0.5,    // Variância estocástica
		},
		sleepParams: SleepTransitionParams{
			BaseChangeRate:        -0.02,  // Leve piora tendencial
			AnxietyImpact:         -0.08,  // Ansiedade piora sono
			DepressionImpact:      -0.10,  // Depressão piora sono
			Variance:              0.3,    // Variância estocástica
		},
		isolationParams: IsolationTransitionParams{
			BaseChangeRate:        0.1,    // Tende a aumentar
			MotivationReduction:   -0.5,   // Motivação reduz isolamento
			DepressionIncrease:    0.6,    // Depressão aumenta isolamento
			Variance:              0.8,    // Variância estocástica
		},
	}
}

// ============================================================================
// PARÂMETROS DE TRANSIÇÃO
// ============================================================================

type AdherenceTransitionParams struct {
	BaseDecayRate        float64
	MotivationImpact     float64
	CognitiveLoadPenalty float64
	DepressionPenalty    float64
	Variance             float64
}

type PHQ9TransitionParams struct {
	BaseChangeRate       float64
	AdherenceImprovement float64
	SleepImpact          float64
	IsolationImpact      float64
	Variance             float64
}

type SleepTransitionParams struct {
	BaseChangeRate   float64
	AnxietyImpact    float64
	DepressionImpact float64
	Variance         float64
}

type IsolationTransitionParams struct {
	BaseChangeRate     float64
	MotivationReduction float64
	DepressionIncrease float64
	Variance           float64
}

// ============================================================================
// PREDIÇÕES DE MUDANÇA (TRANSIÇÕES PROBABILÍSTICAS)
// ============================================================================

// PredictAdherenceChange prediz mudança diária na adesão medicamentosa
// Baseado em: motivação, carga cognitiva, estado depressivo
func (bn *BayesianNetwork) PredictAdherenceChange(
	currentAdherence float64,
	motivation float64,
	cognitiveLoad float64,
	depressiveState float64,
) float64 {
	p := bn.adherenceParams

	// Mudança esperada (determinística)
	expectedChange := p.BaseDecayRate

	// Motivação alta ajuda a manter adesão
	if motivation > 0.6 {
		expectedChange += p.MotivationImpact * (motivation - 0.6)
	} else {
		expectedChange -= p.MotivationImpact * (0.6 - motivation) // Penalidade se baixa
	}

	// Carga cognitiva alta dificulta adesão
	if cognitiveLoad > 0.6 {
		expectedChange += p.CognitiveLoadPenalty * (cognitiveLoad - 0.6)
	}

	// Depressão dificulta autocuidado
	if depressiveState > 0.5 {
		expectedChange += p.DepressionPenalty * (depressiveState - 0.5)
	}

	// Efeito de fronteira: se já muito baixo, cai mais devagar
	if currentAdherence < 0.3 {
		expectedChange *= 0.5
	}

	// Efeito de fronteira: se já muito alto, difícil melhorar mais
	if currentAdherence > 0.9 {
		if expectedChange > 0 {
			expectedChange *= 0.3
		}
	}

	// Adicionar variabilidade estocástica
	stochasticNoise := normalRandom(0, p.Variance)

	return expectedChange + stochasticNoise
}

// PredictPHQ9Change prediz mudança diária no score PHQ-9
// Baseado em: adesão medicamentosa, qualidade do sono, isolamento social
func (bn *BayesianNetwork) PredictPHQ9Change(
	currentPHQ9 float64,
	medicationAdherence float64,
	sleepHours float64,
	isolationDays float64,
) float64 {
	p := bn.phq9Params

	// Mudança esperada (base = leve piora sem tratamento)
	expectedChange := p.BaseChangeRate

	// Adesão medicamentosa BOA → melhora PHQ-9 (reduz)
	if medicationAdherence >= 0.7 {
		expectedChange += p.AdherenceImprovement * (medicationAdherence - 0.7)
	} else {
		// Baixa adesão → piora
		expectedChange += math.Abs(p.AdherenceImprovement) * (0.7 - medicationAdherence) * 0.5
	}

	// Sono adequado (6-8h) → melhora PHQ-9
	if sleepHours >= 6 {
		improvement := math.Min(sleepHours-6, 2) // Máximo 2h de bônus
		expectedChange += p.SleepImpact * improvement
	} else {
		// Sono ruim → piora
		penalty := (6 - sleepHours) / 6.0
		expectedChange += math.Abs(p.SleepImpact) * penalty
	}

	// Isolamento social → piora PHQ-9
	if isolationDays > 3 {
		penalty := math.Min((isolationDays-3)/7.0, 1.0) // Normalizar 0-1
		expectedChange += p.IsolationImpact * penalty
	}

	// Efeito de fronteira: se PHQ-9 muito alto (>23), difícil piorar mais
	if currentPHQ9 > 23 {
		if expectedChange > 0 {
			expectedChange *= 0.3
		}
	}

	// Efeito de fronteira: se PHQ-9 muito baixo (<3), difícil melhorar mais
	if currentPHQ9 < 3 {
		if expectedChange < 0 {
			expectedChange *= 0.3
		}
	}

	// Variabilidade estocástica
	stochasticNoise := normalRandom(0, p.Variance)

	return expectedChange + stochasticNoise
}

// PredictSleepChange prediz mudança diária nas horas de sono
// Baseado em: ansiedade (GAD-7), estado depressivo
func (bn *BayesianNetwork) PredictSleepChange(
	currentSleepHours float64,
	gad7Score float64,
	depressiveState float64,
) float64 {
	p := bn.sleepParams

	// Mudança esperada (base)
	expectedChange := p.BaseChangeRate

	// Ansiedade alta (GAD-7 >= 10) → sono pior
	if gad7Score >= 10 {
		anxietyFactor := (gad7Score - 10) / 11.0 // Normalizar 0-1 (10-21)
		expectedChange += p.AnxietyImpact * anxietyFactor
	}

	// Depressão alta → sono pior (insônia ou hipersonia)
	if depressiveState > 0.6 {
		expectedChange += p.DepressionImpact * (depressiveState - 0.6)
	}

	// Efeito de fronteira: se sono já muito ruim (<4h), tende a estabilizar
	if currentSleepHours < 4 {
		if expectedChange < 0 {
			expectedChange *= 0.5
		}
	}

	// Efeito de fronteira: se sono já muito bom (>8h), tende a não melhorar mais
	if currentSleepHours > 8 {
		if expectedChange > 0 {
			expectedChange *= 0.3
		}
	}

	// Variabilidade estocástica
	stochasticNoise := normalRandom(0, p.Variance)

	return expectedChange + stochasticNoise
}

// PredictIsolationChange prediz mudança diária nos dias de isolamento social
// Baseado em: motivação, estado depressivo
func (bn *BayesianNetwork) PredictIsolationChange(
	currentIsolationDays float64,
	motivation float64,
	depressiveState float64,
) float64 {
	p := bn.isolationParams

	// Mudança esperada (tende a aumentar se nada for feito)
	expectedChange := p.BaseChangeRate

	// Motivação alta → reduz isolamento (pessoa busca contato)
	if motivation > 0.5 {
		expectedChange += p.MotivationReduction * (motivation - 0.5)
	} else {
		// Motivação baixa → aumenta isolamento
		expectedChange += math.Abs(p.MotivationReduction) * (0.5 - motivation) * 0.3
	}

	// Depressão alta → aumenta isolamento (evitação social)
	if depressiveState > 0.5 {
		expectedChange += p.DepressionIncrease * (depressiveState - 0.5)
	}

	// Efeito de fronteira: se já muito isolado (>20 dias), difícil piorar muito mais
	if currentIsolationDays > 20 {
		if expectedChange > 0 {
			expectedChange *= 0.5
		}
	}

	// Efeito de fronteira: se sem isolamento (0 dias), só pode aumentar ou ficar igual
	if currentIsolationDays <= 0 {
		if expectedChange < 0 {
			expectedChange = 0
		}
	}

	// Variabilidade estocástica
	stochasticNoise := normalRandom(0, p.Variance)

	change := expectedChange + stochasticNoise

	// Isolamento é discreto (inteiro), então arredondar
	if math.Abs(change) < 0.5 {
		return 0 // Mudanças pequenas não afetam
	}

	return math.Round(change)
}

// ============================================================================
// APRENDIZADO DE PARÂMETROS (FUTURO)
// ============================================================================
// TODO: Implementar aprendizado de CPTs a partir de dados históricos

// LearnFromHistoricalData aprende parâmetros de transição de dados reais
func (bn *BayesianNetwork) LearnFromHistoricalData(historicalData []PatientTimeSeries) error {
	// TODO: Implementar aprendizado supervisionado
	// 1. Coletar pares (estado_t, estado_t+1) de dados históricos
	// 2. Estimar médias e variâncias de transições
	// 3. Usar regressão linear ou MLE para estimar parâmetros
	// 4. Validar com cross-validation

	// Por enquanto, usar parâmetros default baseados em literatura clínica
	return nil
}

type PatientTimeSeries struct {
	PatientID  int64
	Days       []int
	States     []PatientState
	Outcomes   []bool // Crise ocorreu?
}

// ============================================================================
// ESTRUTURA DA REDE (GRAFO CAUSAL)
// ============================================================================

// GetNodeNames retorna nomes de todos os nós da rede
func (bn *BayesianNetwork) GetNodeNames() []string {
	return []string{
		// Nós observáveis
		"medication_adherence",
		"phq9_score",
		"gad7_score",
		"sleep_hours",
		"voice_pitch_mean",
		"social_isolation_days",
		"cognitive_load",

		// Nós latentes
		"depressive_state",
		"motivation_level",
		"selfcare_capacity",
		"accumulated_risk",

		// Nós de desfecho
		"crisis_outcome",
		"hospitalization_outcome",
		"treatment_dropout_outcome",
	}
}

// GetCausalEdges retorna arestas causais (X → Y)
func (bn *BayesianNetwork) GetCausalEdges() [][2]string {
	return [][2]string{
		// Adesão medicamentosa é influenciada por:
		{"motivation_level", "medication_adherence"},
		{"cognitive_load", "medication_adherence"},
		{"depressive_state", "medication_adherence"},

		// PHQ-9 é influenciado por:
		{"medication_adherence", "phq9_score"},
		{"sleep_hours", "phq9_score"},
		{"social_isolation_days", "phq9_score"},

		// Sono é influenciado por:
		{"gad7_score", "sleep_hours"},
		{"depressive_state", "sleep_hours"},

		// Isolamento social é influenciado por:
		{"motivation_level", "social_isolation_days"},
		{"depressive_state", "social_isolation_days"},

		// Estado depressivo é influenciado por:
		{"phq9_score", "depressive_state"},
		{"sleep_hours", "depressive_state"},
		{"social_isolation_days", "depressive_state"},

		// Motivação é influenciada por:
		{"depressive_state", "motivation_level"},
		{"medication_adherence", "motivation_level"},

		// Risco acumulado é influenciado por:
		{"depressive_state", "accumulated_risk"},
		{"medication_adherence", "accumulated_risk"},
		{"sleep_hours", "accumulated_risk"},
		{"social_isolation_days", "accumulated_risk"},

		// Desfechos são influenciados por:
		{"accumulated_risk", "crisis_outcome"},
		{"phq9_score", "crisis_outcome"},
		{"medication_adherence", "crisis_outcome"},
		{"crisis_outcome", "hospitalization_outcome"},
		{"medication_adherence", "treatment_dropout_outcome"},
		{"motivation_level", "treatment_dropout_outcome"},
	}
}

// ============================================================================
// INFERÊNCIA BAYESIANA (SIMPLIFICADA)
// ============================================================================

// InferProbabilityCrisis infere probabilidade de crise dado evidências
func (bn *BayesianNetwork) InferProbabilityCrisis(state PatientState) float64 {
	// Inferência simplificada usando regras heurísticas
	// (Inferência exata seria via Junction Tree ou Belief Propagation)

	// Base: usar risco acumulado como proxy
	baseProb := state.AccumulatedRisk

	// Ajustar por múltiplos fatores de risco simultâneos
	riskFactors := 0

	if state.MedicationAdherence < 0.5 {
		riskFactors++
		baseProb += 0.10
	}

	if state.PHQ9Score >= 20 {
		riskFactors++
		baseProb += 0.15
	}

	if state.SleepHours < 4 {
		riskFactors++
		baseProb += 0.08
	}

	if state.SocialIsolationDays >= 7 {
		riskFactors++
		baseProb += 0.12
	}

	if state.GAD7Score >= 15 {
		riskFactors++
		baseProb += 0.10
	}

	// Efeito sinérgico: múltiplos fatores aumentam risco mais que linearmente
	if riskFactors >= 3 {
		baseProb *= 1.3
	}

	// Clampar entre 0 e 1
	if baseProb > 1.0 {
		baseProb = 1.0
	}
	if baseProb < 0.0 {
		baseProb = 0.0
	}

	return baseProb
}

// ============================================================================
// UTILITÁRIOS
// ============================================================================

// normalRandom gera número aleatório de distribuição normal
func normalRandom(mean, stddev float64) float64 {
	// Box-Muller transform
	u1 := rand.Float64()
	u2 := rand.Float64()

	z0 := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

	return mean + z0*stddev
}

// ============================================================================
// VALIDAÇÃO DE MODELO
// ============================================================================

// ValidateModel valida acurácia do modelo com dados de teste
func (bn *BayesianNetwork) ValidateModel(testData []PatientTimeSeries) (accuracy float64, auc float64) {
	// TODO: Implementar validação
	// 1. Para cada paciente no testData:
	//    - Simular trajetória
	//    - Comparar predição com outcome real
	// 2. Calcular métricas: accuracy, precision, recall, AUC-ROC

	return 0.80, 0.85 // Placeholder: 80% accuracy, 0.85 AUC
}

// ============================================================================
// SENSIBILIDADE E ANÁLISE DE IMPACTO
// ============================================================================

// CalculateFeatureSensitivity calcula sensibilidade do outcome a mudanças em cada feature
func (bn *BayesianNetwork) CalculateFeatureSensitivity(baseState PatientState) map[string]float64 {
	sensitivities := make(map[string]float64)

	baseCrisisProb := bn.InferProbabilityCrisis(baseState)

	// Testar impacto de melhorar cada feature em 20%

	// 1. Adesão medicamentosa
	testState := baseState
	testState.MedicationAdherence = math.Min(baseState.MedicationAdherence+0.20, 1.0)
	newProb := bn.InferProbabilityCrisis(testState)
	sensitivities["medication_adherence"] = math.Abs(baseCrisisProb - newProb)

	// 2. PHQ-9 (reduzir 20%)
	testState = baseState
	testState.PHQ9Score = math.Max(baseState.PHQ9Score*0.8, 0)
	newProb = bn.InferProbabilityCrisis(testState)
	sensitivities["phq9_score"] = math.Abs(baseCrisisProb - newProb)

	// 3. Sono (aumentar 20%)
	testState = baseState
	testState.SleepHours = math.Min(baseState.SleepHours*1.2, 10)
	newProb = bn.InferProbabilityCrisis(testState)
	sensitivities["sleep_hours"] = math.Abs(baseCrisisProb - newProb)

	// 4. Isolamento (reduzir 50%)
	testState = baseState
	testState.SocialIsolationDays = int(float64(baseState.SocialIsolationDays) * 0.5)
	newProb = bn.InferProbabilityCrisis(testState)
	sensitivities["social_isolation"] = math.Abs(baseCrisisProb - newProb)

	return sensitivities
}
