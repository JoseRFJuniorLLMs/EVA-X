// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package predictive

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"eva/internal/brainstem/database"
)

// TrajectoryEngine simula trajetorias de saude mental usando HMC + Monte Carlo
type TrajectoryEngine struct {
	db           *database.DB
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

// TrajectorySimulation resultado de uma simulacao
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

// DailyState estado em um dia especifico da simulacao
type DailyState struct {
	Day       int     `json:"day"`
	PHQ9      float64 `json:"phq9"`
	Adherence float64 `json:"adherence"`
	Sleep     float64 `json:"sleep"`
	Crisis    bool    `json:"crisis"`
}

// InterventionScenario cenario what-if
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

// Intervention representa uma intervencao
type Intervention struct {
	Type           string  `json:"type"`
	Frequency      string  `json:"frequency,omitempty"`
	ImpactAdherence float64 `json:"impact_adherence,omitempty"`
	ImpactPHQ9     float64 `json:"impact_phq9,omitempty"`
	ImpactSleep    float64 `json:"impact_sleep,omitempty"`
	ImpactIsolation int    `json:"impact_isolation,omitempty"`
}

// RecommendedIntervention recomendacao de intervencao
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
func NewTrajectoryEngine(db *database.DB) *TrajectoryEngine {
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

// SimulateTrajectory executa simulacao Monte Carlo para um paciente
func (te *TrajectoryEngine) SimulateTrajectory(patientID int64, daysAhead int, nSimulations int) (*TrajectorySimulation, error) {
	log.Printf("[TRAJECTORY] Iniciando simulacao para paciente %d (%d dias, %d simulacoes)", patientID, daysAhead, nSimulations)
	startTime := time.Now()

	// 1. Buscar estado atual do paciente
	currentState, err := te.getCurrentState(patientID)
	if err != nil {
		return nil, fmt.Errorf("erro ao buscar estado: %w", err)
	}

	// 2. Executar simulacoes Monte Carlo
	results := te.runMonteCarloSimulations(currentState, daysAhead, nSimulations)

	// 3. Agregar resultados
	simulation := te.aggregateResults(patientID, daysAhead, nSimulations, results)
	simulation.ModelVersion = te.modelVersion

	// 4. Identificar fatores criticos
	simulation.CriticalFactors = te.identifyCriticalFactors(currentState)

	// 5. Salvar no banco
	err = te.saveSimulation(simulation, currentState)
	if err != nil {
		return nil, fmt.Errorf("erro ao salvar simulacao: %w", err)
	}

	computationTime := time.Since(startTime).Milliseconds()
	log.Printf("[TRAJECTORY] Simulacao concluida em %dms: Crise 7d=%.1f%%, 30d=%.1f%%",
		computationTime, simulation.CrisisProbability7d*100, simulation.CrisisProbability30d*100)

	return simulation, nil
}

// SimulateWithIntervention simula cenario com intervencao
func (te *TrajectoryEngine) SimulateWithIntervention(simulationID string, interventions []Intervention) (*InterventionScenario, error) {
	log.Printf("[TRAJECTORY] Simulando cenario com %d intervencoes", len(interventions))

	// 1. Buscar simulacao baseline
	baseline, err := te.getSimulation(simulationID)
	if err != nil {
		return nil, err
	}

	// 2. Buscar estado atual
	currentState, err := te.getCurrentState(baseline.PatientID)
	if err != nil {
		return nil, err
	}

	// 3. Aplicar intervencoes ao estado
	modifiedState := te.applyInterventions(currentState, interventions)

	// 4. Executar nova simulacao
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

	// 7. Salvar cenario
	err = te.saveScenario(scenario)
	if err != nil {
		return nil, err
	}

	return scenario, nil
}

// GenerateRecommendations gera recomendacoes baseadas na simulacao
func (te *TrajectoryEngine) GenerateRecommendations(simulationID string) ([]RecommendedIntervention, error) {
	simulation, err := te.getSimulation(simulationID)
	if err != nil {
		return nil, err
	}

	var recommendations []RecommendedIntervention

	// Regras de recomendacao baseadas no risco

	// 1. Risco critico (>60% em 30 dias)
	if simulation.CrisisProbability30d > 0.6 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "psychiatric_consultation",
			Priority:              "critical",
			UrgencyTimeframe:      "24-48h",
			Title:                 "Consulta psiquiatrica urgente",
			Description:           "Agendar consulta psiquiatrica de emergencia devido ao alto risco de crise.",
			Rationale:             fmt.Sprintf("Probabilidade de crise em 30 dias: %.0f%%", simulation.CrisisProbability30d*100),
			ExpectedRiskReduction: 0.25,
			ConfidenceLevel:       0.85,
			ActionSteps: []string{
				"Contatar psiquiatra responsavel",
				"Agendar consulta em ate 48h",
				"Preparar relatorio EVA para consulta",
			},
			ResponsibleParties: []string{"familiar", "psiquiatra"},
			EstimatedCost:      350.00,
		})
	}

	// 2. Adesao medicamentosa baixa
	if simulation.ProjectedMedicationAdherence < 0.6 {
		recommendations = append(recommendations, RecommendedIntervention{
			InterventionType:      "medication_adherence_boost",
			Priority:              "high",
			UrgencyTimeframe:      "3-5 dias",
			Title:                 "Intensificar lembretes de medicacao",
			Description:           "Implementar protocolo intensivo de lembretes de medicacao com acompanhamento.",
			Rationale:             fmt.Sprintf("Adesao projetada: %.0f%% (abaixo do minimo seguro de 70%%)", simulation.ProjectedMedicationAdherence*100),
			ExpectedRiskReduction: 0.15,
			ExpectedPHQ9Improvement: 3.0,
			ConfidenceLevel:       0.75,
			ActionSteps: []string{
				"Ativar lembretes 2x/dia no app",
				"Configurar confirmacao por voz",
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
			Description:           "Implementar rotina de sono com tecnicas de relaxamento guiadas pela EVA.",
			Rationale:             fmt.Sprintf("Sono projetado: %.1f horas (minimo saudavel: 6h)", simulation.ProjectedSleepHours),
			ExpectedRiskReduction: 0.10,
			ConfidenceLevel:       0.70,
			ActionSteps: []string{
				"Ativar historias para dormir as 21h",
				"Evitar interacoes intensas apos 20h",
				"Monitorar padrao de sono por 7 dias",
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
			Description:           "Coordenar chamadas de video com familiares e visitas presenciais.",
			Rationale:             fmt.Sprintf("Projecao de %d dias sem contato humano significativo", simulation.ProjectedIsolationDays),
			ExpectedRiskReduction: 0.12,
			ConfidenceLevel:       0.68,
			ActionSteps: []string{
				"Alertar familiar primario",
				"Agendar 2 videochamadas esta semana",
				"Sugerir visita presencial se possivel",
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
			Title:                 "Intensificar acompanhamento terapeutico",
			Description:           "Aumentar frequencia de interacoes terapeuticas e considerar psicoterapia.",
			Rationale:             fmt.Sprintf("PHQ-9 projetado: %.0f (depressao moderadamente severa)", simulation.ProjectedPHQ9),
			ExpectedRiskReduction: 0.18,
			ExpectedPHQ9Improvement: 4.5,
			ConfidenceLevel:       0.72,
			ActionSteps: []string{
				"Ativar conversas terapeuticas diarias",
				"Aplicar PHQ-9 semanal",
				"Considerar encaminhamento para psicoterapia",
			},
			ResponsibleParties: []string{"EVA", "psicologo"},
			EstimatedCost:      200.00,
		})
	}

	// Salvar recomendacoes
	for i := range recommendations {
		err := te.saveRecommendation(simulationID, &recommendations[i])
		if err != nil {
			log.Printf("[TRAJECTORY] Erro ao salvar recomendacao: %v", err)
		}
	}

	log.Printf("[TRAJECTORY] Geradas %d recomendacoes para simulacao %s", len(recommendations), simulationID)

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
	// Modelo logistico simplificado
	// Em producao: usar rede Bayesiana treinada

	logit := -5.0 // Base (baixa probabilidade)

	// PHQ9 aumenta risco
	logit += (phq9 - 10) * 0.15

	// Ma adesao aumenta risco
	logit += (0.7 - adherence) * 2.0

	// Sono ruim aumenta risco
	logit += (6 - sleep) * 0.3

	// Converter logit para probabilidade
	prob := 1.0 / (1.0 + math.Exp(-logit))

	return prob
}

// aggregateResults agrega resultados das simulacoes
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

	// Calcular medias
	avgPHQ9 := average(finalPHQ9s)
	avgAdherence := average(finalAdherences)
	avgSleep := average(finalSleeps)

	// Pegar amostra de 10 trajetorias
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
	ctx := context.Background()
	state := &PatientState{PatientID: patientID}

	// Buscar ultimo PHQ-9
	rows, err := te.db.QueryByLabel(ctx, "clinical_scale_results",
		" AND n.idoso_id = $idoso_id AND n.scale_type = $scale_type",
		map[string]interface{}{"idoso_id": patientID, "scale_type": "phq9"}, 1)
	if err == nil && len(rows) > 0 {
		state.PHQ9Score = database.GetFloat64(rows[0], "score")
	}

	// Buscar adesao medicamentosa (placeholder)
	state.MedicationAdherence = 0.65 // TODO: Integrar com sistema de medicacao

	// Buscar sono medio (placeholder)
	state.SleepHours = 5.5 // TODO: Integrar com monitoramento de sono

	// Buscar isolamento (placeholder)
	state.SocialIsolationDays = 3 // TODO: Calcular baseado em interacoes

	return state, nil
}

// identifyCriticalFactors identifica fatores criticos
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
	modified := *state // Copia

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
	return fmt.Sprintf("Com %d intervencoes", len(interventions))
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

// Funcoes de banco de dados (NietzscheDB)

func (te *TrajectoryEngine) saveSimulation(sim *TrajectorySimulation, state *PatientState) error {
	ctx := context.Background()
	trajectoriesJSON, _ := json.Marshal(sim.SampleTrajectories)
	initialStateJSON, _ := json.Marshal(state)
	criticalFactorsJSON, _ := json.Marshal(sim.CriticalFactors)

	content := map[string]interface{}{
		"patient_id":                     sim.PatientID,
		"simulation_date":                time.Now().Format(time.RFC3339),
		"days_ahead":                     sim.DaysAhead,
		"n_simulations":                  sim.NSimulations,
		"crisis_probability_7d":          sim.CrisisProbability7d,
		"crisis_probability_30d":         sim.CrisisProbability30d,
		"hospitalization_probability_30d": sim.HospitalizationProbability30d,
		"projected_phq9_score":           sim.ProjectedPHQ9,
		"projected_medication_adherence": sim.ProjectedMedicationAdherence,
		"projected_sleep_hours":          sim.ProjectedSleepHours,
		"critical_factors":               string(criticalFactorsJSON),
		"sample_trajectories":            string(trajectoriesJSON),
		"initial_state":                  string(initialStateJSON),
		"model_version":                  sim.ModelVersion,
	}

	id, err := te.db.Insert(ctx, "trajectory_simulations", content)
	if err != nil {
		return err
	}

	sim.ID = fmt.Sprintf("%d", id)
	return nil
}

func (te *TrajectoryEngine) getSimulation(id string) (*TrajectorySimulation, error) {
	ctx := context.Background()
	rows, err := te.db.QueryByLabel(ctx, "trajectory_simulations",
		" AND n.id = $sim_id",
		map[string]interface{}{"sim_id": id}, 1)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("simulation not found: %s", id)
	}

	m := rows[0]
	sim := &TrajectorySimulation{
		ID:                            database.GetString(m, "id"),
		PatientID:                     database.GetInt64(m, "patient_id"),
		SimulationDate:                database.GetTime(m, "simulation_date"),
		DaysAhead:                     int(database.GetInt64(m, "days_ahead")),
		NSimulations:                  int(database.GetInt64(m, "n_simulations")),
		CrisisProbability7d:           database.GetFloat64(m, "crisis_probability_7d"),
		CrisisProbability30d:          database.GetFloat64(m, "crisis_probability_30d"),
		HospitalizationProbability30d: database.GetFloat64(m, "hospitalization_probability_30d"),
		ProjectedPHQ9:                 database.GetFloat64(m, "projected_phq9_score"),
		ProjectedMedicationAdherence:  database.GetFloat64(m, "projected_medication_adherence"),
		ProjectedSleepHours:           database.GetFloat64(m, "projected_sleep_hours"),
		ModelVersion:                  database.GetString(m, "model_version"),
	}

	return sim, nil
}

func (te *TrajectoryEngine) saveScenario(scenario *InterventionScenario) error {
	ctx := context.Background()
	interventionsJSON, _ := json.Marshal(scenario.Interventions)

	content := map[string]interface{}{
		"simulation_id":          scenario.SimulationID,
		"scenario_type":          scenario.ScenarioType,
		"scenario_name":          scenario.ScenarioName,
		"interventions":          string(interventionsJSON),
		"crisis_probability_7d":  scenario.CrisisProbability7d,
		"crisis_probability_30d": scenario.CrisisProbability30d,
		"risk_reduction_7d":      scenario.RiskReduction7d,
		"risk_reduction_30d":     scenario.RiskReduction30d,
		"effectiveness_score":    scenario.EffectivenessScore,
		"estimated_cost_monthly": scenario.EstimatedCostMonthly,
		"feasibility":            scenario.Feasibility,
	}

	id, err := te.db.Insert(ctx, "intervention_scenarios", content)
	if err != nil {
		return err
	}
	scenario.ID = fmt.Sprintf("%d", id)
	return nil
}

func (te *TrajectoryEngine) saveRecommendation(simulationID string, rec *RecommendedIntervention) error {
	ctx := context.Background()
	actionStepsJSON, _ := json.Marshal(rec.ActionSteps)
	responsibleJSON, _ := json.Marshal(rec.ResponsibleParties)

	content := map[string]interface{}{
		"simulation_id":            simulationID,
		"intervention_type":        rec.InterventionType,
		"priority":                 rec.Priority,
		"urgency_timeframe":        rec.UrgencyTimeframe,
		"title":                    rec.Title,
		"description":              rec.Description,
		"rationale":                rec.Rationale,
		"expected_risk_reduction":  rec.ExpectedRiskReduction,
		"expected_phq9_improvement": rec.ExpectedPHQ9Improvement,
		"confidence_level":         rec.ConfidenceLevel,
		"action_steps":             string(actionStepsJSON),
		"responsible_parties":      string(responsibleJSON),
		"estimated_cost":           rec.EstimatedCost,
		"status":                   "pending",
	}

	id, err := te.db.Insert(ctx, "recommended_interventions", content)
	if err != nil {
		return err
	}
	rec.ID = fmt.Sprintf("%d", id)
	return nil
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
