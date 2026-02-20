// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package prediction

import (
	"log"
	"math"
	"math/rand"
	"time"
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
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
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
	stochasticNoise := bn.normalRandom(0, p.Variance)

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
	stochasticNoise := bn.normalRandom(0, p.Variance)

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
	stochasticNoise := bn.normalRandom(0, p.Variance)

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
	stochasticNoise := bn.normalRandom(0, p.Variance)

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
// usando exponential moving average com learning rate de 0.1
func (bn *BayesianNetwork) LearnFromHistoricalData(historicalData []PatientTimeSeries) error {
	if len(historicalData) == 0 {
		return nil
	}

	learningRate := 0.1
	transitionCount := 0

	for _, patient := range historicalData {
		if len(patient.States) < 2 {
			continue
		}

		// Iterar por pares consecutivos de estados
		for t := 0; t < len(patient.States)-1; t++ {
			current := patient.States[t]
			next := patient.States[t+1]
			transitionCount++

			// --- Aprender parâmetros de adesão medicamentosa ---
			adherenceChange := next.MedicationAdherence - current.MedicationAdherence
			// Atualizar BaseDecayRate via EMA
			bn.adherenceParams.BaseDecayRate = (1-learningRate)*bn.adherenceParams.BaseDecayRate +
				learningRate*adherenceChange

			// Atualizar impacto da motivação
			if current.MotivationLevel > 0.6 {
				observedMotivationImpact := adherenceChange / math.Max(current.MotivationLevel-0.6, 0.01)
				bn.adherenceParams.MotivationImpact = (1-learningRate)*bn.adherenceParams.MotivationImpact +
					learningRate*observedMotivationImpact
			}

			// --- Aprender parâmetros PHQ-9 ---
			phq9Change := next.PHQ9Score - current.PHQ9Score
			bn.phq9Params.BaseChangeRate = (1-learningRate)*bn.phq9Params.BaseChangeRate +
				learningRate*phq9Change

			// Atualizar impacto da adesão no PHQ-9
			if current.MedicationAdherence >= 0.7 {
				observedAdherenceImpact := phq9Change / math.Max(current.MedicationAdherence-0.7, 0.01)
				bn.phq9Params.AdherenceImprovement = (1-learningRate)*bn.phq9Params.AdherenceImprovement +
					learningRate*observedAdherenceImpact
			}

			// --- Aprender parâmetros de sono ---
			sleepChange := next.SleepHours - current.SleepHours
			bn.sleepParams.BaseChangeRate = (1-learningRate)*bn.sleepParams.BaseChangeRate +
				learningRate*sleepChange

			// --- Aprender parâmetros de isolamento ---
			isolationChange := float64(next.SocialIsolationDays - current.SocialIsolationDays)
			bn.isolationParams.BaseChangeRate = (1-learningRate)*bn.isolationParams.BaseChangeRate +
				learningRate*isolationChange
		}
	}

	// Clampar parâmetros a faixas razoáveis
	bn.adherenceParams.BaseDecayRate = clampParam(bn.adherenceParams.BaseDecayRate, -0.05, 0.05)
	bn.adherenceParams.MotivationImpact = clampParam(bn.adherenceParams.MotivationImpact, 0.001, 0.10)
	bn.adherenceParams.CognitiveLoadPenalty = clampParam(bn.adherenceParams.CognitiveLoadPenalty, -0.10, 0.0)
	bn.adherenceParams.DepressionPenalty = clampParam(bn.adherenceParams.DepressionPenalty, -0.10, 0.0)

	bn.phq9Params.BaseChangeRate = clampParam(bn.phq9Params.BaseChangeRate, -0.50, 0.50)
	bn.phq9Params.AdherenceImprovement = clampParam(bn.phq9Params.AdherenceImprovement, -1.0, 0.0)
	bn.phq9Params.SleepImpact = clampParam(bn.phq9Params.SleepImpact, -0.50, 0.0)
	bn.phq9Params.IsolationImpact = clampParam(bn.phq9Params.IsolationImpact, 0.0, 0.50)

	bn.sleepParams.BaseChangeRate = clampParam(bn.sleepParams.BaseChangeRate, -0.20, 0.20)
	bn.sleepParams.AnxietyImpact = clampParam(bn.sleepParams.AnxietyImpact, -0.30, 0.0)
	bn.sleepParams.DepressionImpact = clampParam(bn.sleepParams.DepressionImpact, -0.30, 0.0)

	bn.isolationParams.BaseChangeRate = clampParam(bn.isolationParams.BaseChangeRate, -1.0, 1.0)
	bn.isolationParams.MotivationReduction = clampParam(bn.isolationParams.MotivationReduction, -2.0, 0.0)
	bn.isolationParams.DepressionIncrease = clampParam(bn.isolationParams.DepressionIncrease, 0.0, 2.0)

	log.Printf("[BayesianNetwork] Parâmetros aprendidos de %d transições em %d pacientes",
		transitionCount, len(historicalData))

	return nil
}

// clampParam restringe um parâmetro a uma faixa razoável
func clampParam(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
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

// normalRandom gera número aleatório de distribuição normal usando o source local
func (bn *BayesianNetwork) normalRandom(mean, stddev float64) float64 {
	// Box-Muller transform usando randomSource local (thread-safe por instância)
	u1 := bn.randomSource.Float64()
	u2 := bn.randomSource.Float64()

	z0 := math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)

	return mean + z0*stddev
}

// ============================================================================
// VALIDAÇÃO DE MODELO
// ============================================================================

// ValidateModel valida acurácia do modelo com dados de teste.
// Calcula accuracy (dentro de 10% de tolerância) e uma estimativa de AUC.
func (bn *BayesianNetwork) ValidateModel(testData []PatientTimeSeries) (accuracy float64, auc float64) {
	if len(testData) == 0 {
		return 0.0, 0.0
	}

	totalPredictions := 0
	correctPredictions := 0

	// Para cálculo de AUC simplificado: contar true positives e false positives
	truePositives := 0
	falsePositives := 0
	trueNegatives := 0
	falseNegatives := 0

	for _, patient := range testData {
		if len(patient.States) < 2 {
			continue
		}

		for t := 0; t < len(patient.States)-1; t++ {
			current := patient.States[t]
			actualNext := patient.States[t+1]
			totalPredictions++

			// Predizer mudanças usando parâmetros atuais
			predictedAdherenceChange := bn.PredictAdherenceChange(
				current.MedicationAdherence,
				current.MotivationLevel,
				current.CognitiveLoad,
				current.DepressiveState,
			)
			predictedPHQ9Change := bn.PredictPHQ9Change(
				current.PHQ9Score,
				current.MedicationAdherence,
				current.SleepHours,
				float64(current.SocialIsolationDays),
			)

			// Valores preditos
			predictedAdherence := current.MedicationAdherence + predictedAdherenceChange
			predictedPHQ9 := current.PHQ9Score + predictedPHQ9Change

			// Accuracy: verificar se a predição está dentro de 10% do valor real
			adherenceTolerance := math.Max(actualNext.MedicationAdherence*0.10, 0.05)
			phq9Tolerance := math.Max(actualNext.PHQ9Score*0.10, 1.0)

			adherenceCorrect := math.Abs(predictedAdherence-actualNext.MedicationAdherence) <= adherenceTolerance
			phq9Correct := math.Abs(predictedPHQ9-actualNext.PHQ9Score) <= phq9Tolerance

			if adherenceCorrect && phq9Correct {
				correctPredictions++
			}

			// AUC: usar risco de crise como classificador binário
			// Predizer crise se risco acumulado > 0.5
			predictedCrisis := bn.InferProbabilityCrisis(current) > 0.5

			// Resultado real: verificar se há outcome de crise
			actualCrisis := false
			if t < len(patient.Outcomes) {
				actualCrisis = patient.Outcomes[t]
			}

			if predictedCrisis && actualCrisis {
				truePositives++
			} else if predictedCrisis && !actualCrisis {
				falsePositives++
			} else if !predictedCrisis && !actualCrisis {
				trueNegatives++
			} else if !predictedCrisis && actualCrisis {
				falseNegatives++
			}
		}
	}

	// Calcular accuracy
	if totalPredictions > 0 {
		accuracy = float64(correctPredictions) / float64(totalPredictions)
	}

	// Calcular AUC aproximado usando TPR e FPR
	// AUC ~ (TPR + TNR) / 2 (estimativa simplificada para ponto único do ROC)
	tpr := 0.0
	tnr := 0.0
	if truePositives+falseNegatives > 0 {
		tpr = float64(truePositives) / float64(truePositives+falseNegatives)
	}
	if trueNegatives+falsePositives > 0 {
		tnr = float64(trueNegatives) / float64(trueNegatives+falsePositives)
	}
	auc = (tpr + tnr) / 2.0

	log.Printf("[BayesianNetwork] Validação: accuracy=%.2f, AUC=%.2f (%d predições, TP=%d FP=%d TN=%d FN=%d)",
		accuracy, auc, totalPredictions, truePositives, falsePositives, trueNegatives, falseNegatives)

	return accuracy, auc
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
