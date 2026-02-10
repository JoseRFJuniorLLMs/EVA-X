package prediction

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

// ============================================================================
// PREDICTIVE LIFE TRAJECTORY ENGINE (SPRINT 3)
// ============================================================================
// Implementa Bayesian Belief Network + Monte Carlo para simular trajetórias
// futuras de saúde mental e estimar probabilidades de crises

// TrajectorySimulator simula trajetórias futuras de pacientes
type TrajectorySimulator struct {
	db            *sql.DB
	predictor     *CrisisPredictor
	bayesianNet   *BayesianNetwork
	modelVersion  string
	nSimulations  int // Número de simulações Monte Carlo (default: 1000)
	randomSource  *rand.Rand
}

// NewTrajectorySimulator cria novo simulador de trajetórias
func NewTrajectorySimulator(db *sql.DB) *TrajectorySimulator {
	return &TrajectorySimulator{
		db:           db,
		predictor:    NewCrisisPredictor(db),
		bayesianNet:  NewBayesianNetwork(),
		modelVersion: "v1.0.0",
		nSimulations: 1000,
		randomSource: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// PatientState representa o estado atual de um paciente
type PatientState struct {
	// Features observáveis
	MedicationAdherence float64 `json:"medication_adherence"` // 0-1
	PHQ9Score           float64 `json:"phq9_score"`           // 0-27
	GAD7Score           float64 `json:"gad7_score"`           // 0-21
	SleepHours          float64 `json:"sleep_hours"`          // horas/noite
	VoicePitchMean      float64 `json:"voice_pitch_mean"`     // Hz
	SocialIsolationDays int     `json:"isolation_days"`       // dias sem contato
	CognitiveLoad       float64 `json:"cognitive_load"`       // 0-1

	// Variáveis latentes (inferidas)
	DepressiveState  float64 `json:"depressive_state"`  // 0-1
	MotivationLevel  float64 `json:"motivation_level"`  // 0-1
	SelfCareCapacity float64 `json:"selfcare_capacity"` // 0-1
	AccumulatedRisk  float64 `json:"accumulated_risk"`  // 0-1

	// Timestamp
	Day int `json:"day"` // Dia da simulação (0 = hoje)
}

// TrajectoryOutcome representa desfechos de uma trajetória
type TrajectoryOutcome struct {
	CrisisIn7Days      bool    `json:"crisis_in_7d"`
	CrisisIn30Days     bool    `json:"crisis_in_30d"`
	DayOfCrisis        int     `json:"day_of_crisis,omitempty"` // -1 se não houve
	Hospitalization    bool    `json:"hospitalization"`
	TreatmentDropout   bool    `json:"treatment_dropout"`
	FallRisk           bool    `json:"fall_risk"`
	FinalPHQ9          float64 `json:"final_phq9"`
	FinalAdherence     float64 `json:"final_adherence"`
	FinalSleepHours    float64 `json:"final_sleep_hours"`
	FinalIsolationDays int     `json:"final_isolation_days"`
}

// Trajectory representa uma trajetória individual simulada
type Trajectory struct {
	States  []PatientState    `json:"states"`
	Outcome TrajectoryOutcome `json:"outcome"`
}

// SimulationResults agrega resultados de N simulações
type SimulationResults struct {
	SimulationID               string               `json:"simulation_id"`
	PatientID                  int64                `json:"patient_id"`
	DaysAhead                  int                  `json:"days_ahead"`
	NSimulations               int                  `json:"n_simulations"`
	ComputationTimeMs          int64                `json:"computation_time_ms"`
	InitialState               PatientState         `json:"initial_state"`
	CrisisProbability7d        float64              `json:"crisis_probability_7d"`
	CrisisProbability30d       float64              `json:"crisis_probability_30d"`
	HospitalizationProb30d     float64              `json:"hospitalization_probability_30d"`
	TreatmentDropoutProb90d    float64              `json:"treatment_dropout_probability_90d"`
	FallRiskProb7d             float64              `json:"fall_risk_probability_7d"`
	ProjectedPHQ9              float64              `json:"projected_phq9_score"`
	ProjectedAdherence         float64              `json:"projected_medication_adherence"`
	ProjectedSleepHours        float64              `json:"projected_sleep_hours"`
	ProjectedIsolationDays     int                  `json:"projected_social_isolation_days"`
	CriticalFactors            []string             `json:"critical_factors"`
	SampleTrajectories         []map[string]float64 `json:"sample_trajectories"` // 10 trajetórias para viz
	BayesianNetworkConfig      map[string]interface{} `json:"bayesian_network_config"`
}

// InterventionScenario representa um cenário "what-if" com intervenções
type InterventionScenario struct {
	ScenarioName              string                 `json:"scenario_name"`
	ScenarioType              string                 `json:"scenario_type"` // baseline, with_intervention
	Description               string                 `json:"description"`
	Interventions             []Intervention         `json:"interventions"`
	CrisisProbability7d       float64                `json:"crisis_probability_7d"`
	CrisisProbability30d      float64                `json:"crisis_probability_30d"`
	HospitalizationProb30d    float64                `json:"hospitalization_probability_30d"`
	ProjectedPHQ9             float64                `json:"projected_phq9_score"`
	ProjectedAdherence        float64                `json:"projected_medication_adherence"`
	ProjectedSleepHours       float64                `json:"projected_sleep_hours"`
	RiskReduction7d           float64                `json:"risk_reduction_7d"`  // vs baseline
	RiskReduction30d          float64                `json:"risk_reduction_30d"` // vs baseline
	EffectivenessScore        float64                `json:"effectiveness_score"` // 0-1
	EstimatedCostMonthly      float64                `json:"estimated_cost_monthly"`
	Feasibility               string                 `json:"feasibility"` // high, medium, low
	RequiredResources         []string               `json:"required_resources"`
}

// Intervention representa uma intervenção específica
type Intervention struct {
	Type              string  `json:"type"`
	Description       string  `json:"description"`
	ImpactAdherence   float64 `json:"impact_adherence,omitempty"`   // +/-
	ImpactPHQ9        float64 `json:"impact_phq9,omitempty"`        // +/-
	ImpactSleep       float64 `json:"impact_sleep,omitempty"`       // +/-
	ImpactIsolation   int     `json:"impact_isolation,omitempty"`   // +/-
	ImpactMotivation  float64 `json:"impact_motivation,omitempty"`  // +/-
	Frequency         string  `json:"frequency,omitempty"`
}

// RecommendedIntervention representa uma intervenção recomendada
type RecommendedIntervention struct {
	InterventionType       string   `json:"intervention_type"`
	Priority               string   `json:"priority"` // critical, high, medium, low
	UrgencyTimeframe       string   `json:"urgency_timeframe"`
	Title                  string   `json:"title"`
	Description            string   `json:"description"`
	Rationale              string   `json:"rationale"`
	ExpectedRiskReduction  float64  `json:"expected_risk_reduction"`
	ExpectedPHQ9Improvement float64  `json:"expected_phq9_improvement"`
	ConfidenceLevel        float64  `json:"confidence_level"`
	ActionSteps            []string `json:"action_steps"`
	ResponsibleParties     []string `json:"responsible_parties"`
	EstimatedCost          float64  `json:"estimated_cost"`
}

// ============================================================================
// SIMULAÇÃO DE TRAJETÓRIA
// ============================================================================

// SimulateTrajectory simula trajetória futura de um paciente
func (ts *TrajectorySimulator) SimulateTrajectory(patientID int64, daysAhead int) (*SimulationResults, error) {
	startTime := time.Now()

	log.Printf("🔮 [TRAJECTORY] Simulando trajetória para paciente %d (%d dias, %d simulações)", patientID, daysAhead, ts.nSimulations)

	// 1. Obter estado inicial
	initialState, err := ts.getCurrentState(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao obter estado inicial: %w", err)
	}

	// 2. Executar N simulações Monte Carlo
	trajectories := make([]Trajectory, ts.nSimulations)
	for i := 0; i < ts.nSimulations; i++ {
		trajectories[i] = ts.simulateSingleTrajectory(initialState, daysAhead)
	}

	// 3. Agregar resultados
	results := ts.aggregateResults(patientID, initialState, trajectories, daysAhead)
	results.ComputationTimeMs = time.Since(startTime).Milliseconds()

	log.Printf("✅ [TRAJECTORY] Simulação concluída em %dms", results.ComputationTimeMs)
	log.Printf("   📊 Risco 7d: %.1f%% | Risco 30d: %.1f%%", results.CrisisProbability7d*100, results.CrisisProbability30d*100)

	// 4. Salvar no banco de dados
	err = ts.saveSimulationResults(results)
	if err != nil {
		log.Printf("⚠️ [TRAJECTORY] Erro ao salvar resultados: %v", err)
	}

	return results, nil
}

// getCurrentState obtém estado atual do paciente
func (ts *TrajectorySimulator) getCurrentState(patientID int64) (PatientState, error) {
	// Coletar features usando o CrisisPredictor
	features, err := ts.predictor.collectFeatures(patientID)
	if err != nil {
		return PatientState{}, err
	}

	state := PatientState{
		Day: 0,
	}

	// Mapear features para state
	if f, ok := features["medication_adherence"]; ok {
		state.MedicationAdherence = f.CurrentValue
	}
	if f, ok := features["phq9_score"]; ok {
		state.PHQ9Score = f.CurrentValue
	}
	if f, ok := features["gad7_score"]; ok {
		state.GAD7Score = f.CurrentValue
	}
	if f, ok := features["sleep_quality"]; ok {
		state.SleepHours = f.CurrentValue
	}
	if f, ok := features["voice_pitch_mean"]; ok {
		state.VoicePitchMean = f.CurrentValue
	}
	if f, ok := features["social_isolation"]; ok {
		state.SocialIsolationDays = int(f.CurrentValue)
	}
	if f, ok := features["cognitive_load"]; ok {
		state.CognitiveLoad = f.CurrentValue
	}

	// Inferir variáveis latentes
	state.DepressiveState = ts.inferDepressiveState(state)
	state.MotivationLevel = ts.inferMotivationLevel(state)
	state.SelfCareCapacity = ts.inferSelfCareCapacity(state)
	state.AccumulatedRisk = ts.calculateAccumulatedRisk(state)

	return state, nil
}

// simulateSingleTrajectory simula uma única trajetória
func (ts *TrajectorySimulator) simulateSingleTrajectory(initialState PatientState, daysAhead int) Trajectory {
	trajectory := Trajectory{
		States: make([]PatientState, daysAhead+1),
	}

	// Estado inicial
	trajectory.States[0] = initialState
	state := initialState

	// Simular dia a dia
	for day := 1; day <= daysAhead; day++ {
		state = ts.applyTransitions(state)
		state.Day = day
		trajectory.States[day] = state

		// Verificar se houve crise
		if !trajectory.Outcome.CrisisIn30Days && ts.checkCrisisOccurred(state) {
			trajectory.Outcome.CrisisIn30Days = true
			trajectory.Outcome.DayOfCrisis = day

			if day <= 7 {
				trajectory.Outcome.CrisisIn7Days = true
			}
		}

		// Verificar hospitalização (se crise severa)
		if trajectory.Outcome.CrisisIn30Days && state.PHQ9Score >= 20 && state.AccumulatedRisk > 0.8 {
			trajectory.Outcome.Hospitalization = true
		}

		// Verificar abandono de tratamento
		if state.MedicationAdherence < 0.3 && state.MotivationLevel < 0.2 {
			trajectory.Outcome.TreatmentDropout = true
		}

		// Verificar risco de queda (se depressão + isolamento + idade)
		if state.PHQ9Score > 15 && state.SocialIsolationDays > 5 {
			trajectory.Outcome.FallRisk = true
		}
	}

	// Estado final
	finalState := trajectory.States[daysAhead]
	trajectory.Outcome.FinalPHQ9 = finalState.PHQ9Score
	trajectory.Outcome.FinalAdherence = finalState.MedicationAdherence
	trajectory.Outcome.FinalSleepHours = finalState.SleepHours
	trajectory.Outcome.FinalIsolationDays = finalState.SocialIsolationDays

	if trajectory.Outcome.DayOfCrisis == 0 {
		trajectory.Outcome.DayOfCrisis = -1 // Não houve crise
	}

	return trajectory
}

// applyTransitions aplica transições probabilísticas do Bayesian Network
func (ts *TrajectorySimulator) applyTransitions(currentState PatientState) PatientState {
	nextState := currentState

	// 1. Adesão medicamentosa (influenciada por motivação, carga cognitiva, depressão)
	adherenceChange := ts.bayesianNet.PredictAdherenceChange(
		currentState.MedicationAdherence,
		currentState.MotivationLevel,
		currentState.CognitiveLoad,
		currentState.DepressiveState,
	)
	nextState.MedicationAdherence = clamp(currentState.MedicationAdherence+adherenceChange, 0, 1)

	// 2. PHQ-9 (influenciado por adesão, sono, isolamento)
	phq9Change := ts.bayesianNet.PredictPHQ9Change(
		currentState.PHQ9Score,
		currentState.MedicationAdherence,
		currentState.SleepHours,
		float64(currentState.SocialIsolationDays),
	)
	nextState.PHQ9Score = clamp(currentState.PHQ9Score+phq9Change, 0, 27)

	// 3. Qualidade do sono (influenciado por ansiedade, depressão)
	sleepChange := ts.bayesianNet.PredictSleepChange(
		currentState.SleepHours,
		currentState.GAD7Score,
		currentState.DepressiveState,
	)
	nextState.SleepHours = clamp(currentState.SleepHours+sleepChange, 0, 12)

	// 4. Isolamento social (influenciado por motivação, energia)
	isolationChange := ts.bayesianNet.PredictIsolationChange(
		float64(currentState.SocialIsolationDays),
		currentState.MotivationLevel,
		currentState.DepressiveState,
	)
	nextState.SocialIsolationDays = int(clamp(float64(currentState.SocialIsolationDays)+isolationChange, 0, 30))

	// 5. GAD-7 (varia lentamente)
	gad7Change := ts.randomSource.NormFloat64() * 0.5 // Pequena variação aleatória
	nextState.GAD7Score = clamp(currentState.GAD7Score+gad7Change, 0, 21)

	// 6. Voice pitch (correlacionado com depressão)
	if currentState.DepressiveState > 0.6 {
		pitchChange := ts.randomSource.NormFloat64() * 2.0 // Mais variabilidade se deprimido
		nextState.VoicePitchMean = currentState.VoicePitchMean + pitchChange
	}

	// 7. Recalcular variáveis latentes
	nextState.DepressiveState = ts.inferDepressiveState(nextState)
	nextState.MotivationLevel = ts.inferMotivationLevel(nextState)
	nextState.SelfCareCapacity = ts.inferSelfCareCapacity(nextState)
	nextState.AccumulatedRisk = ts.calculateAccumulatedRisk(nextState)

	return nextState
}

// checkCrisisOccurred verifica se houve crise no estado atual
func (ts *TrajectorySimulator) checkCrisisOccurred(state PatientState) bool {
	// Crise = risco acumulado muito alto
	if state.AccumulatedRisk > 0.85 {
		return true
	}

	// Ou PHQ-9 crítico + outros fatores
	if state.PHQ9Score >= 20 && (state.MedicationAdherence < 0.5 || state.SleepHours < 4) {
		return true
	}

	// Ou ideação suicida (inferido de PHQ-9 alto + isolamento)
	if state.PHQ9Score >= 18 && state.SocialIsolationDays >= 7 {
		return true
	}

	return false
}

// aggregateResults agrega resultados de todas as simulações
func (ts *TrajectorySimulator) aggregateResults(patientID int64, initialState PatientState, trajectories []Trajectory, daysAhead int) *SimulationResults {
	results := &SimulationResults{
		PatientID:    patientID,
		DaysAhead:    daysAhead,
		NSimulations: ts.nSimulations,
		InitialState: initialState,
	}

	// Contar desfechos
	crises7d := 0
	crises30d := 0
	hospitalizations := 0
	dropouts := 0
	falls := 0

	sumFinalPHQ9 := 0.0
	sumFinalAdherence := 0.0
	sumFinalSleep := 0.0
	sumFinalIsolation := 0

	for _, traj := range trajectories {
		if traj.Outcome.CrisisIn7Days {
			crises7d++
		}
		if traj.Outcome.CrisisIn30Days {
			crises30d++
		}
		if traj.Outcome.Hospitalization {
			hospitalizations++
		}
		if traj.Outcome.TreatmentDropout {
			dropouts++
		}
		if traj.Outcome.FallRisk {
			falls++
		}

		sumFinalPHQ9 += traj.Outcome.FinalPHQ9
		sumFinalAdherence += traj.Outcome.FinalAdherence
		sumFinalSleep += traj.Outcome.FinalSleepHours
		sumFinalIsolation += traj.Outcome.FinalIsolationDays
	}

	n := float64(ts.nSimulations)
	results.CrisisProbability7d = float64(crises7d) / n
	results.CrisisProbability30d = float64(crises30d) / n
	results.HospitalizationProb30d = float64(hospitalizations) / n
	results.TreatmentDropoutProb90d = float64(dropouts) / n
	results.FallRiskProb7d = float64(falls) / n

	results.ProjectedPHQ9 = sumFinalPHQ9 / n
	results.ProjectedAdherence = sumFinalAdherence / n
	results.ProjectedSleepHours = sumFinalSleep / n
	results.ProjectedIsolationDays = int(float64(sumFinalIsolation) / n)

	// Identificar fatores críticos
	results.CriticalFactors = ts.identifyCriticalFactors(initialState, results)

	// Amostrar 10 trajetórias para visualização
	results.SampleTrajectories = ts.sampleTrajectoriesForVisualization(trajectories, 10)

	// Config da rede Bayesiana
	results.BayesianNetworkConfig = map[string]interface{}{
		"model_version": ts.modelVersion,
		"nodes":         ts.bayesianNet.GetNodeNames(),
	}

	return results
}

// identifyCriticalFactors identifica fatores de risco críticos
func (ts *TrajectorySimulator) identifyCriticalFactors(state PatientState, results *SimulationResults) []string {
	factors := []string{}

	if state.MedicationAdherence < 0.5 {
		factors = append(factors, "low_medication_adherence")
	}
	if state.PHQ9Score >= 15 {
		factors = append(factors, "moderate_to_severe_depression")
	}
	if state.SleepHours < 5 {
		factors = append(factors, "poor_sleep_quality")
	}
	if state.SocialIsolationDays >= 5 {
		factors = append(factors, "social_isolation")
	}
	if state.GAD7Score >= 10 {
		factors = append(factors, "moderate_to_severe_anxiety")
	}
	if state.CognitiveLoad > 0.7 {
		factors = append(factors, "high_cognitive_load")
	}
	if state.MotivationLevel < 0.3 {
		factors = append(factors, "low_motivation")
	}

	// Adicionar tendência (se risco está aumentando)
	if results.ProjectedPHQ9 > state.PHQ9Score+3 {
		factors = append(factors, "worsening_depression_trend")
	}
	if results.ProjectedAdherence < state.MedicationAdherence-0.15 {
		factors = append(factors, "declining_adherence_trend")
	}

	return factors
}

// sampleTrajectoriesForVisualization amostra trajetórias para dashboard
func (ts *TrajectorySimulator) sampleTrajectoriesForVisualization(trajectories []Trajectory, nSamples int) []map[string]float64 {
	samples := []map[string]float64{}

	if len(trajectories) == 0 {
		return samples
	}

	// Pegar média de todos os dias
	daysAhead := len(trajectories[0].States) - 1

	for day := 0; day <= daysAhead; day++ {
		sumPHQ9 := 0.0
		sumAdherence := 0.0
		sumSleep := 0.0
		sumIsolation := 0.0

		for _, traj := range trajectories {
			if day < len(traj.States) {
				sumPHQ9 += traj.States[day].PHQ9Score
				sumAdherence += traj.States[day].MedicationAdherence
				sumSleep += traj.States[day].SleepHours
				sumIsolation += float64(traj.States[day].SocialIsolationDays)
			}
		}

		n := float64(len(trajectories))
		samples = append(samples, map[string]float64{
			"day":        float64(day),
			"phq9":       sumPHQ9 / n,
			"adherence":  sumAdherence / n,
			"sleep":      sumSleep / n,
			"isolation":  sumIsolation / n,
		})
	}

	return samples
}

// ============================================================================
// VARIÁVEIS LATENTES (INFERÊNCIA)
// ============================================================================

func (ts *TrajectorySimulator) inferDepressiveState(state PatientState) float64 {
	// Normalizar PHQ-9 (0-27) para 0-1
	depFromPHQ9 := state.PHQ9Score / 27.0

	// Considerar outros indicadores
	sleepFactor := 0.0
	if state.SleepHours < 6 {
		sleepFactor = (6 - state.SleepHours) / 6.0 // 0-1
	}

	isolationFactor := 0.0
	if state.SocialIsolationDays > 3 {
		isolationFactor = math.Min(float64(state.SocialIsolationDays-3)/7.0, 1.0)
	}

	// Média ponderada
	depState := (depFromPHQ9*0.6 + sleepFactor*0.2 + isolationFactor*0.2)
	return clamp(depState, 0, 1)
}

func (ts *TrajectorySimulator) inferMotivationLevel(state PatientState) float64 {
	// Motivação inversamente relacionada à depressão
	baseMot := 1.0 - state.DepressiveState

	// Adesão medicamentosa reflete motivação
	if state.MedicationAdherence > 0 {
		baseMot = (baseMot + state.MedicationAdherence) / 2.0
	}

	// Sono afeta energia/motivação
	if state.SleepHours < 6 {
		baseMot *= 0.7
	}

	return clamp(baseMot, 0, 1)
}

func (ts *TrajectorySimulator) inferSelfCareCapacity(state PatientState) float64 {
	// Capacidade de autocuidado = motivação + não deprimido + sono adequado
	selfCare := state.MotivationLevel * 0.4

	if state.DepressiveState < 0.5 {
		selfCare += 0.3
	}

	if state.SleepHours >= 6 {
		selfCare += 0.3
	}

	return clamp(selfCare, 0, 1)
}

func (ts *TrajectorySimulator) calculateAccumulatedRisk(state PatientState) float64 {
	// Risco acumulado = combinação de múltiplos fatores
	risk := 0.0

	// PHQ-9 peso alto
	risk += (state.PHQ9Score / 27.0) * 0.35

	// Adesão baixa = risco alto
	risk += (1.0 - state.MedicationAdherence) * 0.30

	// Sono ruim
	if state.SleepHours < 6 {
		risk += ((6.0 - state.SleepHours) / 6.0) * 0.15
	}

	// Isolamento
	if state.SocialIsolationDays > 3 {
		risk += math.Min(float64(state.SocialIsolationDays-3)/10.0, 0.15)
	}

	// GAD-7
	risk += (state.GAD7Score / 21.0) * 0.10

	return clamp(risk, 0, 1)
}

// ============================================================================
// CENÁRIOS DE INTERVENÇÃO
// ============================================================================

// SimulateScenarios simula diferentes cenários de intervenção
func (ts *TrajectorySimulator) SimulateScenarios(patientID int64, daysAhead int) ([]InterventionScenario, error) {
	scenarios := []InterventionScenario{}

	// 1. Cenário baseline (sem intervenção)
	baseline, err := ts.SimulateTrajectory(patientID, daysAhead)
	if err != nil {
		return nil, err
	}

	baselineScenario := InterventionScenario{
		ScenarioName:           "Trajetória Atual (Sem Intervenção)",
		ScenarioType:           "baseline",
		Description:            "Continuando o padrão atual sem mudanças",
		Interventions:          []Intervention{},
		CrisisProbability7d:    baseline.CrisisProbability7d,
		CrisisProbability30d:   baseline.CrisisProbability30d,
		HospitalizationProb30d: baseline.HospitalizationProb30d,
		ProjectedPHQ9:          baseline.ProjectedPHQ9,
		ProjectedAdherence:     baseline.ProjectedAdherence,
		ProjectedSleepHours:    baseline.ProjectedSleepHours,
		EffectivenessScore:     0.0, // Baseline = 0
	}
	scenarios = append(scenarios, baselineScenario)

	// 2. Cenário: Melhorar adesão medicamentosa
	if baseline.InitialState.MedicationAdherence < 0.7 {
		adherenceScenario := ts.simulateInterventionScenario(
			baseline.InitialState,
			daysAhead,
			[]Intervention{
				{
					Type:            "medication_reminders",
					Description:     "Lembretes 2x/dia + alarmes",
					ImpactAdherence: +0.20,
					Frequency:       "2x/day",
				},
			},
			"Aumento de Adesão Medicamentosa",
			"Lembretes frequentes, alarmes e acompanhamento",
			150.0, // R$ 150/mês
		)
		adherenceScenario.RiskReduction7d = baseline.CrisisProbability7d - adherenceScenario.CrisisProbability7d
		adherenceScenario.RiskReduction30d = baseline.CrisisProbability30d - adherenceScenario.CrisisProbability30d
		adherenceScenario.EffectivenessScore = adherenceScenario.RiskReduction30d / math.Max(baseline.CrisisProbability30d, 0.01)
		scenarios = append(scenarios, adherenceScenario)
	}

	// 3. Cenário: Protocolo de sono
	if baseline.InitialState.SleepHours < 6 {
		sleepScenario := ts.simulateInterventionScenario(
			baseline.InitialState,
			daysAhead,
			[]Intervention{
				{
					Type:        "sleep_hygiene_protocol",
					Description: "CBT-I + restrição de cafeína + rotina",
					ImpactSleep: +2.0,
					ImpactPHQ9:  -2.0,
					Frequency:   "daily",
				},
			},
			"Protocolo de Higiene do Sono",
			"Terapia cognitivo-comportamental para insônia",
			300.0,
		)
		sleepScenario.RiskReduction7d = baseline.CrisisProbability7d - sleepScenario.CrisisProbability7d
		sleepScenario.RiskReduction30d = baseline.CrisisProbability30d - sleepScenario.CrisisProbability30d
		sleepScenario.EffectivenessScore = sleepScenario.RiskReduction30d / math.Max(baseline.CrisisProbability30d, 0.01)
		scenarios = append(scenarios, sleepScenario)
	}

	// 4. Cenário: Engajamento social
	if baseline.InitialState.SocialIsolationDays >= 5 {
		socialScenario := ts.simulateInterventionScenario(
			baseline.InitialState,
			daysAhead,
			[]Intervention{
				{
					Type:            "family_engagement",
					Description:     "Ligações familiares 2x/semana",
					ImpactIsolation: -4,
					ImpactMotivation: +0.15,
					Frequency:       "2x/week",
				},
			},
			"Engajamento Social e Familiar",
			"Contato regular com família e amigos",
			0.0, // Grátis
		)
		socialScenario.RiskReduction7d = baseline.CrisisProbability7d - socialScenario.CrisisProbability7d
		socialScenario.RiskReduction30d = baseline.CrisisProbability30d - socialScenario.CrisisProbability30d
		socialScenario.EffectivenessScore = socialScenario.RiskReduction30d / math.Max(baseline.CrisisProbability30d, 0.01)
		scenarios = append(scenarios, socialScenario)
	}

	// 5. Cenário: Consulta psiquiátrica (ajuste medicação)
	if baseline.InitialState.PHQ9Score >= 15 {
		psychiatricScenario := ts.simulateInterventionScenario(
			baseline.InitialState,
			daysAhead,
			[]Intervention{
				{
					Type:        "psychiatric_consultation",
					Description: "Consulta + ajuste de dose",
					ImpactPHQ9:  -4.0,
					ImpactMotivation: +0.10,
					Frequency:   "weekly",
				},
			},
			"Consulta Psiquiátrica e Ajuste Medicamentoso",
			"Reavaliação clínica e otimização do tratamento",
			800.0, // R$ 800/consulta
		)
		psychiatricScenario.RiskReduction7d = baseline.CrisisProbability7d - psychiatricScenario.CrisisProbability7d
		psychiatricScenario.RiskReduction30d = baseline.CrisisProbability30d - psychiatricScenario.CrisisProbability30d
		psychiatricScenario.EffectivenessScore = psychiatricScenario.RiskReduction30d / math.Max(baseline.CrisisProbability30d, 0.01)
		scenarios = append(scenarios, psychiatricScenario)
	}

	// 6. Cenário: Intervenção combinada (múltiplas intervenções)
	if len(scenarios) > 2 { // Se há pelo menos 2 intervenções disponíveis
		allInterventions := []Intervention{}
		for _, scenario := range scenarios[1:] { // Pular baseline
			allInterventions = append(allInterventions, scenario.Interventions...)
		}

		combinedScenario := ts.simulateInterventionScenario(
			baseline.InitialState,
			daysAhead,
			allInterventions,
			"Intervenção Combinada (Máximo Impacto)",
			"Todas as intervenções aplicadas simultaneamente",
			0.0, // Calcular soma depois
		)

		// Calcular custo total
		totalCost := 0.0
		for _, scenario := range scenarios[1:] {
			totalCost += scenario.EstimatedCostMonthly
		}
		combinedScenario.EstimatedCostMonthly = totalCost

		combinedScenario.RiskReduction7d = baseline.CrisisProbability7d - combinedScenario.CrisisProbability7d
		combinedScenario.RiskReduction30d = baseline.CrisisProbability30d - combinedScenario.CrisisProbability30d
		combinedScenario.EffectivenessScore = combinedScenario.RiskReduction30d / math.Max(baseline.CrisisProbability30d, 0.01)
		scenarios = append(scenarios, combinedScenario)
	}

	// Salvar cenários no banco
	for _, scenario := range scenarios {
		err := ts.saveInterventionScenario(baseline.SimulationID, patientID, scenario)
		if err != nil {
			log.Printf("⚠️ [TRAJECTORY] Erro ao salvar cenário: %v", err)
		}
	}

	return scenarios, nil
}

// simulateInterventionScenario simula com intervenções aplicadas
func (ts *TrajectorySimulator) simulateInterventionScenario(
	initialState PatientState,
	daysAhead int,
	interventions []Intervention,
	name string,
	description string,
	cost float64,
) InterventionScenario {

	// Aplicar impactos das intervenções ao estado inicial
	modifiedState := initialState

	for _, intervention := range interventions {
		modifiedState.MedicationAdherence = clamp(modifiedState.MedicationAdherence+intervention.ImpactAdherence, 0, 1)
		modifiedState.PHQ9Score = clamp(modifiedState.PHQ9Score+intervention.ImpactPHQ9, 0, 27)
		modifiedState.SleepHours = clamp(modifiedState.SleepHours+intervention.ImpactSleep, 0, 12)
		modifiedState.SocialIsolationDays = int(clamp(float64(modifiedState.SocialIsolationDays)+float64(intervention.ImpactIsolation), 0, 30))
		modifiedState.MotivationLevel = clamp(modifiedState.MotivationLevel+intervention.ImpactMotivation, 0, 1)
	}

	// Recalcular latentes com novo estado
	modifiedState.DepressiveState = ts.inferDepressiveState(modifiedState)
	modifiedState.SelfCareCapacity = ts.inferSelfCareCapacity(modifiedState)
	modifiedState.AccumulatedRisk = ts.calculateAccumulatedRisk(modifiedState)

	// Simular trajetórias com estado modificado
	trajectories := make([]Trajectory, ts.nSimulations)
	for i := 0; i < ts.nSimulations; i++ {
		trajectories[i] = ts.simulateSingleTrajectory(modifiedState, daysAhead)
	}

	// Agregar
	crises7d := 0
	crises30d := 0
	hospitalizations := 0
	sumFinalPHQ9 := 0.0
	sumFinalAdherence := 0.0
	sumFinalSleep := 0.0

	for _, traj := range trajectories {
		if traj.Outcome.CrisisIn7Days {
			crises7d++
		}
		if traj.Outcome.CrisisIn30Days {
			crises30d++
		}
		if traj.Outcome.Hospitalization {
			hospitalizations++
		}
		sumFinalPHQ9 += traj.Outcome.FinalPHQ9
		sumFinalAdherence += traj.Outcome.FinalAdherence
		sumFinalSleep += traj.Outcome.FinalSleepHours
	}

	n := float64(ts.nSimulations)

	return InterventionScenario{
		ScenarioName:           name,
		ScenarioType:           "with_intervention",
		Description:            description,
		Interventions:          interventions,
		CrisisProbability7d:    float64(crises7d) / n,
		CrisisProbability30d:   float64(crises30d) / n,
		HospitalizationProb30d: float64(hospitalizations) / n,
		ProjectedPHQ9:          sumFinalPHQ9 / n,
		ProjectedAdherence:     sumFinalAdherence / n,
		ProjectedSleepHours:    sumFinalSleep / n,
		EstimatedCostMonthly:   cost,
		Feasibility:            "high",
		RequiredResources:      []string{"EVA system", "family cooperation"},
	}
}

// GenerateRecommendations gera recomendações baseadas nos cenários
func (ts *TrajectorySimulator) GenerateRecommendations(patientID int64, baseline *SimulationResults, scenarios []InterventionScenario) []RecommendedIntervention {
	recommendations := []RecommendedIntervention{}

	// Se risco baixo, não precisa intervenções urgentes
	if baseline.CrisisProbability30d < 0.2 {
		return recommendations
	}

	// Analisar cada cenário (exceto baseline)
	for _, scenario := range scenarios {
		if scenario.ScenarioType == "baseline" {
			continue
		}

		// Se redução de risco significativa, recomendar
		if scenario.RiskReduction30d > 0.10 { // Redução de pelo menos 10%
			priority := "low"
			timeframe := "7-14 days"

			if baseline.CrisisProbability30d >= 0.6 {
				priority = "critical"
				timeframe = "24-48h"
			} else if baseline.CrisisProbability30d >= 0.4 {
				priority = "high"
				timeframe = "3-5 days"
			} else if baseline.CrisisProbability30d >= 0.2 {
				priority = "medium"
				timeframe = "5-7 days"
			}

			recommendation := RecommendedIntervention{
				InterventionType:        scenario.ScenarioName,
				Priority:                priority,
				UrgencyTimeframe:        timeframe,
				Title:                   scenario.ScenarioName,
				Description:             scenario.Description,
				Rationale:               fmt.Sprintf("Esta intervenção pode reduzir o risco de crise em %.1f%% (de %.1f%% para %.1f%%)", scenario.RiskReduction30d*100, baseline.CrisisProbability30d*100, scenario.CrisisProbability30d*100),
				ExpectedRiskReduction:   scenario.RiskReduction30d,
				ExpectedPHQ9Improvement: baseline.ProjectedPHQ9 - scenario.ProjectedPHQ9,
				ConfidenceLevel:         0.75, // Baseado em dados históricos
				ActionSteps:             ts.generateActionSteps(scenario),
				ResponsibleParties:      []string{"família", "cuidador", "equipe médica"},
				EstimatedCost:           scenario.EstimatedCostMonthly,
			}

			recommendations = append(recommendations, recommendation)
		}
	}

	// Salvar recomendações no banco
	for _, rec := range recommendations {
		err := ts.saveRecommendedIntervention(baseline.SimulationID, patientID, rec)
		if err != nil {
			log.Printf("⚠️ [TRAJECTORY] Erro ao salvar recomendação: %v", err)
		}
	}

	return recommendations
}

func (ts *TrajectorySimulator) generateActionSteps(scenario InterventionScenario) []string {
	steps := []string{}

	for _, intervention := range scenario.Interventions {
		switch intervention.Type {
		case "medication_reminders":
			steps = append(steps, "Configurar alarmes no celular nos horários das medicações")
			steps = append(steps, "Ativar lembretes automáticos via EVA")
			steps = append(steps, "Família acompanhar adesão diariamente")

		case "sleep_hygiene_protocol":
			steps = append(steps, "Estabelecer horário fixo para dormir e acordar")
			steps = append(steps, "Evitar cafeína após 16h")
			steps = append(steps, "Criar ritual relaxante antes de dormir")
			steps = append(steps, "Limitar telas 1h antes de dormir")

		case "family_engagement":
			steps = append(steps, "Agendar ligações fixas com família (ex: terça e sexta às 19h)")
			steps = append(steps, "Visitas presenciais semanais se possível")
			steps = append(steps, "Incluir paciente em atividades familiares virtuais")

		case "psychiatric_consultation":
			steps = append(steps, "Agendar consulta com psiquiatra nas próximas 48-72h")
			steps = append(steps, "Levar histórico recente de sintomas e adesão")
			steps = append(steps, "Discutir possível ajuste de medicação")
		}
	}

	return steps
}

// ============================================================================
// UTILITÁRIOS
// ============================================================================

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ============================================================================
// PERSISTÊNCIA (DATABASE)
// ============================================================================

func (ts *TrajectorySimulator) saveSimulationResults(results *SimulationResults) error {
	query := `
		INSERT INTO trajectory_simulations (
			patient_id, days_ahead, n_simulations,
			crisis_probability_7d, crisis_probability_30d,
			hospitalization_probability_30d, treatment_dropout_probability_90d,
			fall_risk_probability_7d,
			projected_phq9_score, projected_medication_adherence,
			projected_sleep_hours, projected_social_isolation_days,
			critical_factors, sample_trajectories, initial_state,
			model_version, bayesian_network_config, computation_time_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING id
	`

	initialStateJSON, _ := json.Marshal(results.InitialState)
	sampleTrajJSON, _ := json.Marshal(results.SampleTrajectories)
	bayesianConfigJSON, _ := json.Marshal(results.BayesianNetworkConfig)
	criticalFactorsArray := "{" + joinStrings(results.CriticalFactors, ",") + "}"

	var id string
	err := ts.db.QueryRow(
		query,
		results.PatientID, results.DaysAhead, results.NSimulations,
		results.CrisisProbability7d, results.CrisisProbability30d,
		results.HospitalizationProb30d, results.TreatmentDropoutProb90d,
		results.FallRiskProb7d,
		results.ProjectedPHQ9, results.ProjectedAdherence,
		results.ProjectedSleepHours, results.ProjectedIsolationDays,
		criticalFactorsArray, sampleTrajJSON, initialStateJSON,
		ts.modelVersion, bayesianConfigJSON, results.ComputationTimeMs,
	).Scan(&id)

	if err == nil {
		results.SimulationID = id
	}

	return err
}

func (ts *TrajectorySimulator) saveInterventionScenario(simulationID string, patientID int64, scenario InterventionScenario) error {
	query := `
		INSERT INTO intervention_scenarios (
			simulation_id, patient_id, scenario_type, scenario_name, scenario_description,
			interventions, crisis_probability_7d, crisis_probability_30d,
			hospitalization_probability_30d, projected_phq9_score,
			projected_medication_adherence, projected_sleep_hours,
			risk_reduction_7d, risk_reduction_30d, effectiveness_score,
			estimated_cost_monthly, feasibility, required_resources
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
	`

	interventionsJSON, _ := json.Marshal(scenario.Interventions)
	resourcesArray := "{" + joinStrings(scenario.RequiredResources, ",") + "}"

	_, err := ts.db.Exec(
		query,
		simulationID, patientID, scenario.ScenarioType, scenario.ScenarioName, scenario.Description,
		interventionsJSON, scenario.CrisisProbability7d, scenario.CrisisProbability30d,
		scenario.HospitalizationProb30d, scenario.ProjectedPHQ9,
		scenario.ProjectedAdherence, scenario.ProjectedSleepHours,
		scenario.RiskReduction7d, scenario.RiskReduction30d, scenario.EffectivenessScore,
		scenario.EstimatedCostMonthly, scenario.Feasibility, resourcesArray,
	)

	return err
}

func (ts *TrajectorySimulator) saveRecommendedIntervention(simulationID string, patientID int64, rec RecommendedIntervention) error {
	query := `
		INSERT INTO recommended_interventions (
			simulation_id, patient_id, intervention_type, priority, urgency_timeframe,
			title, description, rationale,
			expected_risk_reduction, expected_phq9_improvement, confidence_level,
			action_steps, responsible_parties, estimated_cost, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`

	actionStepsArray := "{" + joinStrings(rec.ActionSteps, ",") + "}"
	responsibleArray := "{" + joinStrings(rec.ResponsibleParties, ",") + "}"

	_, err := ts.db.Exec(
		query,
		simulationID, patientID, rec.InterventionType, rec.Priority, rec.UrgencyTimeframe,
		rec.Title, rec.Description, rec.Rationale,
		rec.ExpectedRiskReduction, rec.ExpectedPHQ9Improvement, rec.ConfidenceLevel,
		actionStepsArray, responsibleArray, rec.EstimatedCost, "pending",
	)

	return err
}

func joinStrings(arr []string, sep string) string {
	if len(arr) == 0 {
		return ""
	}
	result := ""
	for i, s := range arr {
		if i > 0 {
			result += sep
		}
		result += `"` + s + `"`
	}
	return result
}
