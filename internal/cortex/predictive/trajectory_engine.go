package predictive

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

// TrajectoryEngine simula trajetorias de saude mental usando HMC + Monte Carlo
type TrajectoryEngine struct {
	db           *sql.DB
	modelVersion string
	hmcSampler   *HMCSampler // Hamiltonian Monte Carlo sampler
	useHMC       bool        // Flag para alternar entre HMC e random walk classico
}

// PatientState representa estado atual do paciente
type PatientState struct {
	PatientID            int64
	PHQ9Score            float64 // 0-27
	GAD7Score            float64 // 0-21
	MedicationAdherence  float64 // 0-1
	SleepHours           float64 // horas por noite
	SocialIsolationDays  int     // dias sem contato humano
	VoiceEnergyScore     float64 // 0-1
	LastCrisisDate       *time.Time
	DaysSinceLastCrisis  int
}

// TrajectorySimulation resultado de uma simulação
type TrajectorySimulation struct {
	ID                           string
	PatientID                    int64
	SimulationDate               time.Time
	DaysAhead                    int
	NSimulations                 int
	CrisisProbability7d          float64
	CrisisProbability30d         float64
	HospitalizationProbability30d float64
	TreatmentDropoutProbability90d float64
	FallRiskProbability7d        float64
	ProjectedPHQ9                float64
	ProjectedMedicationAdherence float64
	ProjectedSleepHours          float64
	ProjectedIsolationDays       int
	CriticalFactors              []string
	SampleTrajectories           []DailyState
	ModelVersion                 string
}

// DailyState estado em um dia específico da simulação
type DailyState struct {
	Day       int     `json:"day"`
	PHQ9      float64 `json:"phq9"`
	Adherence float64 `json:"adherence"`
	Sleep     float64 `json:"sleep"`
	Crisis    bool    `json:"crisis"`
}

// InterventionScenario cenário what-if
type InterventionScenario struct {
	ID                      string
	SimulationID            string
	ScenarioType            string // baseline, with_intervention
	ScenarioName            string
	Interventions           []Intervention
	CrisisProbability7d     float64
	CrisisProbability30d    float64
	RiskReduction7d         float64
	RiskReduction30d        float64
	EffectivenessScore      float64
	EstimatedCostMonthly    float64
	Feasibility             string
}

// Intervention representa uma intervenção
type Intervention struct {
	Type           string  `json:"type"`
	Frequency      string  `json:"frequency,omitempty"`
	ImpactAdherence float64 `json:"impact_adherence,omitempty"`
	ImpactPHQ9     float64 `json:"impact_phq9,omitempty"`
	ImpactSleep    float64 `json:"impact_sleep,omitempty"`
	ImpactIsolation int    `json:"impact_isolation,omitempty"`
}

// RecommendedIntervention recomendação de intervenção
type RecommendedIntervention struct {
	ID                   string
	InterventionType     string
	Priority             string // critical, high, medium, low
	UrgencyTimeframe     string
	Title                string
	Description          string
	Rationale            string
	ExpectedRiskReduction float64
	ExpectedPHQ9Improvement float64
	ConfidenceLevel      float64
	ActionSteps          []string
	ResponsibleParties   []string
	EstimatedCost        float64
	Status               string
}

// NewTrajectoryEngine cria novo engine de trajetoria com HMC habilitado
func NewTrajectoryEngine(db *sql.DB) *TrajectoryEngine {
	return &TrajectoryEngine{
		db:           db,
		modelVersion: "v2.0.0-hmc",
		hmcSampler:   NewHMCSampler(),
		useHMC:       true,
	}
}

// SetUseHMC alterna entre HMC (true) e random walk classico (false)
func (te *TrajectoryEngine) SetUseHMC(use bool) {
	te.useHMC = use
	if use {
		te.modelVersion = "v2.0.0-hmc"
	} else {
		te.modelVersion = "v1.0.0-random-walk"
	}
}

// SimulateTrajectory executa simulação Monte Carlo para um paciente
func (te *TrajectoryEngine) SimulateTrajectory(patientID int64, daysAhead int, nSimulations int) (*TrajectorySimulation, error) {
	log.Printf("🔮 [TRAJECTORY] Iniciando simulação para paciente %d (%d dias, %d simulações)", patientID, daysAhead, nSimulations)
	startTime := time.Now()

	// 1. Buscar estado atual do paciente
	currentState, err := te.getCurrentState(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estado: %w", err)
	}

	// 2. Executar simulações Monte Carlo
	results := te.runMonteCarloSimulations(currentState, daysAhead, nSimulations)

	// 3. Agregar resultados
	simulation := te.aggregateResults(patientID, daysAhead, nSimulations, results)
	simulation.ModelVersion = te.modelVersion

	// 4. Identificar fatores críticos
	simulation.CriticalFactors = te.identifyCriticalFactors(currentState)

	// 5. Salvar no banco
	err = te.saveSimulation(simulation, currentState)
	if err != nil {
		return nil, fmt.Errorf("erro ao salvar simulação: %w", err)
	}

	computationTime := time.Since(startTime).Milliseconds()
	log.Printf("✅ [TRAJECTORY] Simulação concluída em %dms: Crise 7d=%.1f%%, 30d=%.1f%%",
		computationTime, simulation.CrisisProbability7d*100, simulation.CrisisProbability30d*100)

	return simulation, nil
}

// SimulateWithIntervention simula cenário com intervenção
func (te *TrajectoryEngine) SimulateWithIntervention(simulationID string, interventions []Intervention) (*InterventionScenario, error) {
	log.Printf("💉 [TRAJECTORY] Simulando cenário com %d intervenções", len(interventions))

	// 1. Buscar simulação baseline
	baseline, err := te.getSimulation(simulationID)
	if err != nil {
		return nil, err
	}

	// 2. Buscar estado atual
	currentState, err := te.getCurrentState(baseline.PatientID)
	if err != nil {
		return nil, err
	}

	// 3. Aplicar intervenções ao estado
	modifiedState := te.applyInterventions(currentState, interventions)

	// 4. Executar nova simulação
	results := te.runMonteCarloSimulations(modifiedState, 30, 500)
	newSimulation := te.aggregateResults(baseline.PatientID, 30, 500, results)

	// 5. Calcular impacto
	scenario := &InterventionScenario{
		SimulationID:         simulationID,
		ScenarioType:         "with_intervention",
		ScenarioName:         te.generateScenarioName(interventions),
		Interventions:        interventions,
		CrisisProbability7d:  newSimulation.CrisisProbability7d,
		CrisisProbability30d: newSimulation.CrisisProbability30d,
		RiskReduction7d:      baseline.CrisisProbability7d - newSimulation.CrisisProbability7d,
		RiskReduction30d:     baseline.CrisisProbability30d - newSimulation.CrisisProbability30d,
		EstimatedCostMonthly: te.calculateCost(interventions),
		Feasibility:          te.assessFeasibility(interventions),
	}

	// 6. Calcular score de efetividade
	if baseline.CrisisProbability30d > 0 {
		scenario.EffectivenessScore = scenario.RiskReduction30d / baseline.CrisisProbability30d
	}

	// 7. Salvar cenário
	err = te.saveScenario(scenario)
	if err != nil {
		return nil, err
	}

	return scenario, nil
}

// GenerateRecommendations gera recomendações baseadas na simulação
func (te *TrajectoryEngine) GenerateRecommendations(simulationID string) ([]RecommendedIntervention, error) {
	simulation, err := te.getSimulation(simulationID)
	if err != nil {
		return nil, err
	}

	var recommendations []RecommendedIntervention

	// Regras de recomendação baseadas no risco

	// 1. Risco crítico (>60% em 30 dias)
	if simulation.CrisisProbability30d > 0.6 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "psychiatric_consultation",
			Priority:              "critical",
			UrgencyTimeframe:      "24-48h",
			Title:                 "Consulta psiquiátrica urgente",
			Description:           "Agendar consulta psiquiátrica de emergência devido ao alto risco de crise.",
			Rationale:             fmt.Sprintf("Probabilidade de crise em 30 dias: %.0f%%", simulation.CrisisProbability30d*100),
			ExpectedRiskReduction: 0.25,
			ConfidenceLevel:       0.85,
			ActionSteps: []string{
				"Contatar psiquiatra responsável",
				"Agendar consulta em até 48h",
				"Preparar relatório EVA para consulta",
			},
			ResponsibleParties: []string{"familiar", "psiquiatra"},
			EstimatedCost:      350.00,
		})
	}

	// 2. Adesão medicamentosa baixa
	if simulation.ProjectedMedicationAdherence < 0.6 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "medication_adherence_boost",
			Priority:              "high",
			UrgencyTimeframe:      "3-5 dias",
			Title:                 "Intensificar lembretes de medicação",
			Description:           "Implementar protocolo intensivo de lembretes de medicação com acompanhamento.",
			Rationale:             fmt.Sprintf("Adesão projetada: %.0f%% (abaixo do mínimo seguro de 70%%)", simulation.ProjectedMedicationAdherence*100),
			ExpectedRiskReduction: 0.15,
			ExpectedPHQ9Improvement: 3.0,
			ConfidenceLevel:       0.75,
			ActionSteps: []string{
				"Ativar lembretes 2x/dia no app",
				"Configurar confirmação por voz",
				"Alertar cuidador sobre doses",
			},
			ResponsibleParties: []string{"EVA", "cuidador"},
			EstimatedCost:      0,
		})
	}

	// 3. Problemas de sono
	if simulation.ProjectedSleepHours < 5 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "sleep_hygiene_protocol",
			Priority:              "medium",
			UrgencyTimeframe:      "1 semana",
			Title:                 "Protocolo de higiene do sono",
			Description:           "Implementar rotina de sono com técnicas de relaxamento guiadas pela EVA.",
			Rationale:             fmt.Sprintf("Sono projetado: %.1f horas (mínimo saudável: 6h)", simulation.ProjectedSleepHours),
			ExpectedRiskReduction: 0.10,
			ConfidenceLevel:       0.70,
			ActionSteps: []string{
				"Ativar histórias para dormir às 21h",
				"Evitar interações intensas após 20h",
				"Monitorar padrão de sono por 7 dias",
			},
			ResponsibleParties: []string{"EVA", "paciente"},
			EstimatedCost:      0,
		})
	}

	// 4. Isolamento social
	if simulation.ProjectedIsolationDays > 5 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "family_engagement",
			Priority:              "high",
			UrgencyTimeframe:      "48h",
			Title:                 "Aumentar contato familiar",
			Description:           "Coordenar chamadas de vídeo com familiares e visitas presenciais.",
			Rationale:             fmt.Sprintf("Projeção de %d dias sem contato humano significativo", simulation.ProjectedIsolationDays),
			ExpectedRiskReduction: 0.12,
			ConfidenceLevel:       0.68,
			ActionSteps: []string{
				"Alertar familiar primário",
				"Agendar 2 videochamadas esta semana",
				"Sugerir visita presencial se possível",
			},
			ResponsibleParties: []string{"familiar", "EVA"},
			EstimatedCost:      0,
		})
	}

	// 5. PHQ-9 em piora
	if simulation.ProjectedPHQ9 > 15 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "therapy_intensification",
			Priority:              te.getPriorityByPHQ9(simulation.ProjectedPHQ9),
			UrgencyTimeframe:      "1 semana",
			Title:                 "Intensificar acompanhamento terapêutico",
			Description:           "Aumentar frequência de interações terapêuticas e considerar psicoterapia.",
			Rationale:             fmt.Sprintf("PHQ-9 projetado: %.0f (depressão moderadamente severa)", simulation.ProjectedPHQ9),
			ExpectedRiskReduction: 0.18,
			ExpectedPHQ9Improvement: 4.5,
			ConfidenceLevel:       0.72,
			ActionSteps: []string{
				"Ativar conversas terapêuticas diárias",
				"Aplicar PHQ-9 semanal",
				"Considerar encaminhamento para psicoterapia",
			},
			ResponsibleParties: []string{"EVA", "psicólogo"},
			EstimatedCost:      200.00,
		})
	}

	// Salvar recomendações
	for i := range recommendations {
		err := te.saveRecommendation(simulationID, &recommendations[i])
		if err != nil {
			log.Printf("⚠️ [TRAJECTORY] Erro ao salvar recomendação: %v", err)
		}
	}

	log.Printf("📋 [TRAJECTORY] Geradas %d recomendações para simulação %s", len(recommendations), simulationID)

	return recommendations, nil
}

// runMonteCarloSimulations executa N simulacoes usando HMC ou random walk
func (te *TrajectoryEngine) runMonteCarloSimulations(state *PatientState, daysAhead int, n int) [][]DailyState {
	results := make([][]DailyState, n)

	for sim := 0; sim < n; sim++ {
		if te.useHMC && te.hmcSampler != nil {
			// HMC: usa gradientes de energia para explorar estados
			initial := &HMCState{
				PHQ9:      state.PHQ9Score,
				Adherence: state.MedicationAdherence,
				Sleep:     state.SleepHours,
			}
			results[sim] = te.hmcSampler.RunHMCTrajectory(initial, daysAhead)
		} else {
			// Fallback: random walk classico (v1.0)
			results[sim] = te.runRandomWalkTrajectory(state, daysAhead)
		}
	}

	return results
}

// runRandomWalkTrajectory implementacao original com random walk (fallback)
func (te *TrajectoryEngine) runRandomWalkTrajectory(state *PatientState, daysAhead int) []DailyState {
	trajectory := make([]DailyState, daysAhead)
	currentPHQ9 := state.PHQ9Score
	currentAdherence := state.MedicationAdherence
	currentSleep := state.SleepHours

	for day := 0; day < daysAhead; day++ {
		phq9Delta := rand.NormFloat64() * 0.5
		if currentAdherence < 0.5 {
			phq9Delta += 0.2
		}
		if currentSleep < 5 {
			phq9Delta += 0.15
		}
		currentPHQ9 = math.Max(0, math.Min(27, currentPHQ9+phq9Delta))

		adherenceDelta := rand.NormFloat64() * 0.02
		if currentPHQ9 > 15 {
			adherenceDelta -= 0.01
		}
		currentAdherence = math.Max(0, math.Min(1, currentAdherence+adherenceDelta))

		sleepDelta := rand.NormFloat64() * 0.3
		if currentPHQ9 > 15 {
			sleepDelta -= 0.2
		}
		currentSleep = math.Max(2, math.Min(10, currentSleep+sleepDelta))

		crisisProbToday := te.calculateDailyCrisisProbability(currentPHQ9, currentAdherence, currentSleep)
		crisis := rand.Float64() < crisisProbToday

		trajectory[day] = DailyState{
			Day:       day + 1,
			PHQ9:      currentPHQ9,
			Adherence: currentAdherence,
			Sleep:     currentSleep,
			Crisis:    crisis,
		}
	}

	return trajectory
}

// calculateDailyCrisisProbability calcula probabilidade de crise em um dia
func (te *TrajectoryEngine) calculateDailyCrisisProbability(phq9, adherence, sleep float64) float64 {
	// Modelo logístico simplificado
	// Em produção: usar rede Bayesiana treinada

	logit := -5.0 // Base (baixa probabilidade)

	// PHQ9 aumenta risco
	logit += (phq9 - 10) * 0.15

	// Má adesão aumenta risco
	logit += (0.7 - adherence) * 2.0

	// Sono ruim aumenta risco
	logit += (6 - sleep) * 0.3

	// Converter logit para probabilidade
	prob := 1.0 / (1.0 + math.Exp(-logit))

	return prob
}

// aggregateResults agrega resultados das simulações
func (te *TrajectoryEngine) aggregateResults(patientID int64, daysAhead int, n int, results [][]DailyState) *TrajectorySimulation {
	crisisCount7d := 0
	crisisCount30d := 0
	var finalPHQ9s, finalAdherences, finalSleeps []float64

	for _, trajectory := range results {
		hadCrisis7d := false
		hadCrisis30d := false

		for day, state := range trajectory {
			if state.Crisis {
				if day < 7 {
					hadCrisis7d = true
				}
				if day < 30 {
					hadCrisis30d = true
				}
			}
		}

		if hadCrisis7d {
			crisisCount7d++
		}
		if hadCrisis30d {
			crisisCount30d++
		}

		// Coletar estados finais
		if len(trajectory) > 0 {
			final := trajectory[len(trajectory)-1]
			finalPHQ9s = append(finalPHQ9s, final.PHQ9)
			finalAdherences = append(finalAdherences, final.Adherence)
			finalSleeps = append(finalSleeps, final.Sleep)
		}
	}

	// Calcular médias
	avgPHQ9 := average(finalPHQ9s)
	avgAdherence := average(finalAdherences)
	avgSleep := average(finalSleeps)

	// Pegar amostra de 10 trajetórias
	sampleSize := 10
	if len(results) < sampleSize {
		sampleSize = len(results)
	}
	sampleTrajectories := make([]DailyState, 0)
	if len(results) > 0 && len(results[0]) > 0 {
		sampleTrajectories = results[0][:min(30, len(results[0]))]
	}

	return &TrajectorySimulation{
		PatientID:                     patientID,
		SimulationDate:                time.Now(),
		DaysAhead:                     daysAhead,
		NSimulations:                  n,
		CrisisProbability7d:           float64(crisisCount7d) / float64(n),
		CrisisProbability30d:          float64(crisisCount30d) / float64(n),
		HospitalizationProbability30d: float64(crisisCount30d) / float64(n) * 0.3, // 30% das crises hospitalizam
		ProjectedPHQ9:                 avgPHQ9,
		ProjectedMedicationAdherence:  avgAdherence,
		ProjectedSleepHours:           avgSleep,
		SampleTrajectories:            sampleTrajectories,
	}
}

// getCurrentState busca estado atual do paciente
func (te *TrajectoryEngine) getCurrentState(patientID int64) (*PatientState, error) {
	state := &PatientState{PatientID: patientID}

	// Buscar último PHQ-9
	query := `
		SELECT score FROM clinical_scale_results
		WHERE idoso_id = $1 AND scale_type = 'phq9'
		ORDER BY completed_at DESC LIMIT 1
	`
	te.db.QueryRow(query, patientID).Scan(&state.PHQ9Score)

	// Buscar adesão medicamentosa (placeholder)
	state.MedicationAdherence = 0.65 // TODO: Integrar com sistema de medicação

	// Buscar sono médio (placeholder)
	state.SleepHours = 5.5 // TODO: Integrar com monitoramento de sono

	// Buscar isolamento (placeholder)
	state.SocialIsolationDays = 3 // TODO: Calcular baseado em interações

	return state, nil
}

// identifyCriticalFactors identifica fatores críticos
func (te *TrajectoryEngine) identifyCriticalFactors(state *PatientState) []string {
	var factors []string

	if state.MedicationAdherence < 0.6 {
		factors = append(factors, "low_medication_adherence")
	}
	if state.SleepHours < 5 {
		factors = append(factors, "poor_sleep")
	}
	if state.PHQ9Score > 15 {
		factors = append(factors, "high_depression")
	}
	if state.SocialIsolationDays > 5 {
		factors = append(factors, "social_isolation")
	}

	return factors
}

// Helpers

func (te *TrajectoryEngine) applyInterventions(state *PatientState, interventions []Intervention) *PatientState {
	modified := *state // Cópia

	for _, intervention := range interventions {
		modified.MedicationAdherence += intervention.ImpactAdherence
		modified.PHQ9Score += intervention.ImpactPHQ9
		modified.SleepHours += intervention.ImpactSleep
		modified.SocialIsolationDays += intervention.ImpactIsolation
	}

	// Normalizar valores
	modified.MedicationAdherence = math.Max(0, math.Min(1, modified.MedicationAdherence))
	modified.PHQ9Score = math.Max(0, math.Min(27, modified.PHQ9Score))
	modified.SleepHours = math.Max(2, math.Min(10, modified.SleepHours))
	modified.SocialIsolationDays = max(0, modified.SocialIsolationDays)

	return &modified
}

func (te *TrajectoryEngine) generateScenarioName(interventions []Intervention) string {
	if len(interventions) == 0 {
		return "Baseline"
	}
	if len(interventions) == 1 {
		return fmt.Sprintf("Com %s", interventions[0].Type)
	}
	return fmt.Sprintf("Com %d intervenções", len(interventions))
}

func (te *TrajectoryEngine) calculateCost(interventions []Intervention) float64 {
	costs := map[string]float64{
		"psychiatric_consultation":   350,
		"medication_adherence_boost": 0,
		"sleep_hygiene_protocol":     0,
		"family_engagement":          0,
		"therapy_intensification":    200,
	}

	total := 0.0
	for _, i := range interventions {
		if cost, ok := costs[i.Type]; ok {
			total += cost
		}
	}
	return total
}

func (te *TrajectoryEngine) assessFeasibility(interventions []Intervention) string {
	for _, i := range interventions {
		if i.Type == "psychiatric_consultation" {
			return "medium" // Depende de disponibilidade
		}
	}
	return "high"
}

func (te *TrajectoryEngine) getPriorityByPHQ9(phq9 float64) string {
	if phq9 >= 20 {
		return "critical"
	}
	if phq9 >= 15 {
		return "high"
	}
	return "medium"
}

// Funções de banco de dados

func (te *TrajectoryEngine) saveSimulation(sim *TrajectorySimulation, state *PatientState) error {
	trajectoriesJSON, _ := json.Marshal(sim.SampleTrajectories)
	initialStateJSON, _ := json.Marshal(state)

	query := `
		INSERT INTO trajectory_simulations (
			patient_id, days_ahead, n_simulations,
			crisis_probability_7d, crisis_probability_30d, hospitalization_probability_30d,
			projected_phq9_score, projected_medication_adherence, projected_sleep_hours,
			critical_factors, sample_trajectories, initial_state, model_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`

	var id string
	err := te.db.QueryRow(
		query,
		sim.PatientID, sim.DaysAhead, sim.NSimulations,
		sim.CrisisProbability7d, sim.CrisisProbability30d, sim.HospitalizationProbability30d,
		sim.ProjectedPHQ9, sim.ProjectedMedicationAdherence, sim.ProjectedSleepHours,
		pqArray(sim.CriticalFactors), trajectoriesJSON, initialStateJSON, sim.ModelVersion,
	).Scan(&id)

	if err != nil {
		return err
	}

	sim.ID = id
	return nil
}

func (te *TrajectoryEngine) getSimulation(id string) (*TrajectorySimulation, error) {
	query := `
		SELECT id, patient_id, simulation_date, days_ahead, n_simulations,
		       crisis_probability_7d, crisis_probability_30d, hospitalization_probability_30d,
		       projected_phq9_score, projected_medication_adherence, projected_sleep_hours,
		       critical_factors, model_version
		FROM trajectory_simulations WHERE id = $1
	`

	sim := &TrajectorySimulation{}
	var criticalFactors []byte

	err := te.db.QueryRow(query, id).Scan(
		&sim.ID, &sim.PatientID, &sim.SimulationDate, &sim.DaysAhead, &sim.NSimulations,
		&sim.CrisisProbability7d, &sim.CrisisProbability30d, &sim.HospitalizationProbability30d,
		&sim.ProjectedPHQ9, &sim.ProjectedMedicationAdherence, &sim.ProjectedSleepHours,
		&criticalFactors, &sim.ModelVersion,
	)

	if err != nil {
		return nil, err
	}

	// Parse critical factors (PostgreSQL array)
	// TODO: Properly parse PostgreSQL array

	return sim, nil
}

func (te *TrajectoryEngine) saveScenario(scenario *InterventionScenario) error {
	interventionsJSON, _ := json.Marshal(scenario.Interventions)

	query := `
		INSERT INTO intervention_scenarios (
			simulation_id, patient_id, scenario_type, scenario_name, interventions,
			crisis_probability_7d, crisis_probability_30d, risk_reduction_7d, risk_reduction_30d,
			effectiveness_score, estimated_cost_monthly, feasibility
		) VALUES ($1, (SELECT patient_id FROM trajectory_simulations WHERE id = $1),
		         $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id
	`

	return te.db.QueryRow(
		query,
		scenario.SimulationID, scenario.ScenarioType, scenario.ScenarioName, interventionsJSON,
		scenario.CrisisProbability7d, scenario.CrisisProbability30d,
		scenario.RiskReduction7d, scenario.RiskReduction30d,
		scenario.EffectivenessScore, scenario.EstimatedCostMonthly, scenario.Feasibility,
	).Scan(&scenario.ID)
}

func (te *TrajectoryEngine) saveRecommendation(simulationID string, rec *RecommendedIntervention) error {
	query := `
		INSERT INTO recommended_interventions (
			simulation_id, patient_id, intervention_type, priority, urgency_timeframe,
			title, description, rationale, expected_risk_reduction, expected_phq9_improvement,
			confidence_level, action_steps, responsible_parties, estimated_cost
		) VALUES ($1, (SELECT patient_id FROM trajectory_simulations WHERE id = $1),
		         $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`

	return te.db.QueryRow(
		query,
		simulationID, rec.InterventionType, rec.Priority, rec.UrgencyTimeframe,
		rec.Title, rec.Description, rec.Rationale,
		rec.ExpectedRiskReduction, rec.ExpectedPHQ9Improvement, rec.ConfidenceLevel,
		pqArray(rec.ActionSteps), pqArray(rec.ResponsibleParties), rec.EstimatedCost,
	).Scan(&rec.ID)
}

// Utility functions

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// pqArray helper for PostgreSQL arrays
func pqArray(arr []string) interface{} {
	// Placeholder - use github.com/lib/pq.Array in production
	return arr
}
