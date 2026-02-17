// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package predictive

import (
	"math"
	"math/rand"
)

// HMCSampler implementa Hamiltonian Monte Carlo para amostragem de trajetorias
// Em vez de random walk (ruido gaussiano puro), usa gradientes de energia
// para explorar o espaco de estados de forma mais eficiente.
//
// Referencia: Neal, R.M. (2011) "MCMC using Hamiltonian dynamics"
//
// Conceito:
// - Cada "estado do paciente" tem uma "energia" (potencial)
// - Estados repetitivos/improdutivos = alta energia
// - Estados novos e clinicamente relevantes = baixa energia
// - HMC usa gradientes para mover-se naturalmente para estados de baixa energia
type HMCSampler struct {
	// Leapfrog integration parameters
	StepSize    float64 // Epsilon (tamanho do passo leapfrog)
	NumSteps    int     // L (numero de passos leapfrog por iteracao)
	MassMatrix  []float64 // M (diagonal mass matrix, uma entrada por dimensao)

	// Energy function parameters
	RepetitionPenalty float64 // Penalidade para estados repetitivos
	NoveltyReward     float64 // Recompensa para estados novos e relevantes
	ClinicalWeight    float64 // Peso para fatores clinicos

	// State dimension names (PHQ9, Adherence, Sleep)
	dim int
}

// HMCState estado do sistema para HMC (posicao no espaco de fase)
type HMCState struct {
	PHQ9      float64
	Adherence float64
	Sleep     float64
}

// NewHMCSampler cria novo sampler HMC com parametros default
func NewHMCSampler() *HMCSampler {
	return &HMCSampler{
		StepSize:          0.01,  // Passo pequeno para estabilidade
		NumSteps:          20,    // 20 leapfrog steps por amostra
		MassMatrix:        []float64{1.0, 1.0, 1.0}, // Mass identidade (PHQ9, Adherence, Sleep)
		RepetitionPenalty: 2.0,
		NoveltyReward:     1.5,
		ClinicalWeight:    3.0,
		dim:               3,
	}
}

// toVector converte estado para vetor
func (s *HMCState) toVector() []float64 {
	return []float64{s.PHQ9, s.Adherence, s.Sleep}
}

// fromVector preenche estado a partir de vetor
func (s *HMCState) fromVector(v []float64) {
	s.PHQ9 = v[0]
	s.Adherence = v[1]
	s.Sleep = v[2]
}

// clamp aplica limites fisicos aos valores
func (s *HMCState) clamp() {
	s.PHQ9 = math.Max(0, math.Min(27, s.PHQ9))
	s.Adherence = math.Max(0, math.Min(1, s.Adherence))
	s.Sleep = math.Max(2, math.Min(10, s.Sleep))
}

// PotentialEnergy calcula U(q) - a energia potencial de um estado
// Baseado no modelo clinico: estados "ruins" tem alta energia
func (hmc *HMCSampler) PotentialEnergy(state *HMCState, currentAdherence float64) float64 {
	energy := 0.0

	// 1. Componente clinica: PHQ9 alto = alta energia (ruim)
	// Normalizado: PHQ9/27 para escala 0-1
	energy += hmc.ClinicalWeight * (state.PHQ9 / 27.0) * (state.PHQ9 / 27.0)

	// 2. Adesao baixa = alta energia
	energy += hmc.ClinicalWeight * (1.0 - state.Adherence) * (1.0 - state.Adherence)

	// 3. Sono ruim = alta energia (desvio de 7h ideal)
	sleepDeviation := (state.Sleep - 7.0) / 4.0 // Normalizado
	energy += hmc.ClinicalWeight * sleepDeviation * sleepDeviation

	// 4. Coupling: ma adesao + PHQ9 alto = energia sinergica
	if state.Adherence < 0.5 && state.PHQ9 > 15 {
		energy += hmc.RepetitionPenalty * 2.0
	}

	// 5. Isolamento implicit: se adesao cai, sono piora junto
	if state.Adherence < currentAdherence-0.2 {
		energy += hmc.RepetitionPenalty * 0.5
	}

	return energy
}

// GradientPotentialEnergy calcula dU/dq (gradiente numerico)
// Usa diferenca finita central: dU/dq_i ≈ (U(q+h) - U(q-h)) / 2h
func (hmc *HMCSampler) GradientPotentialEnergy(state *HMCState, currentAdherence float64) []float64 {
	h := 0.001 // Step size para diferenca finita
	grad := make([]float64, hmc.dim)
	q := state.toVector()

	for i := 0; i < hmc.dim; i++ {
		// q + h*e_i
		qPlus := make([]float64, hmc.dim)
		copy(qPlus, q)
		qPlus[i] += h

		// q - h*e_i
		qMinus := make([]float64, hmc.dim)
		copy(qMinus, q)
		qMinus[i] -= h

		sPlus := &HMCState{}
		sPlus.fromVector(qPlus)
		sMinus := &HMCState{}
		sMinus.fromVector(qMinus)

		grad[i] = (hmc.PotentialEnergy(sPlus, currentAdherence) -
			hmc.PotentialEnergy(sMinus, currentAdherence)) / (2 * h)
	}

	return grad
}

// KineticEnergy calcula K(p) = p^T * M^{-1} * p / 2
func (hmc *HMCSampler) KineticEnergy(momentum []float64) float64 {
	ke := 0.0
	for i := 0; i < hmc.dim; i++ {
		ke += momentum[i] * momentum[i] / hmc.MassMatrix[i]
	}
	return ke / 2.0
}

// SampleMomentum amostra momento da distribuicao N(0, M)
func (hmc *HMCSampler) SampleMomentum() []float64 {
	p := make([]float64, hmc.dim)
	for i := 0; i < hmc.dim; i++ {
		p[i] = rand.NormFloat64() * math.Sqrt(hmc.MassMatrix[i])
	}
	return p
}

// LeapfrogIntegrate executa integracao leapfrog (Stormer-Verlet)
// Preserva volume no espaco de fase (simpletico)
func (hmc *HMCSampler) LeapfrogIntegrate(state *HMCState, momentum []float64, currentAdherence float64) (*HMCState, []float64) {
	q := state.toVector()
	p := make([]float64, hmc.dim)
	copy(p, momentum)

	// Half step for momentum
	grad := hmc.GradientPotentialEnergy(state, currentAdherence)
	for i := 0; i < hmc.dim; i++ {
		p[i] -= hmc.StepSize * grad[i] / 2
	}

	// L full steps for position + momentum
	for step := 0; step < hmc.NumSteps; step++ {
		// Full step for position
		for i := 0; i < hmc.dim; i++ {
			q[i] += hmc.StepSize * p[i] / hmc.MassMatrix[i]
		}

		// Full step for momentum (except last)
		newState := &HMCState{}
		newState.fromVector(q)
		grad = hmc.GradientPotentialEnergy(newState, currentAdherence)

		if step < hmc.NumSteps-1 {
			for i := 0; i < hmc.dim; i++ {
				p[i] -= hmc.StepSize * grad[i]
			}
		}
	}

	// Half step for momentum at the end
	for i := 0; i < hmc.dim; i++ {
		p[i] -= hmc.StepSize * grad[i] / 2
	}

	// Negate momentum (for reversibility)
	for i := 0; i < hmc.dim; i++ {
		p[i] = -p[i]
	}

	result := &HMCState{}
	result.fromVector(q)
	result.clamp()

	return result, p
}

// Sample gera uma nova amostra usando HMC
// Retorna: novo estado, se foi aceito (Metropolis-Hastings)
func (hmc *HMCSampler) Sample(current *HMCState, currentAdherence float64) (*HMCState, bool) {
	// 1. Amostrar momento
	momentum := hmc.SampleMomentum()

	// 2. Calcular Hamiltoniano atual: H = U(q) + K(p)
	currentU := hmc.PotentialEnergy(current, currentAdherence)
	currentK := hmc.KineticEnergy(momentum)
	currentH := currentU + currentK

	// 3. Integrar leapfrog
	proposed, proposedMomentum := hmc.LeapfrogIntegrate(current, momentum, currentAdherence)

	// 4. Calcular Hamiltoniano proposto
	proposedU := hmc.PotentialEnergy(proposed, currentAdherence)
	proposedK := hmc.KineticEnergy(proposedMomentum)
	proposedH := proposedU + proposedK

	// 5. Criterio de Metropolis-Hastings: aceitar com prob min(1, exp(-dH))
	deltaH := proposedH - currentH
	acceptProb := math.Min(1.0, math.Exp(-deltaH))

	if rand.Float64() < acceptProb {
		return proposed, true // Aceito
	}

	return current, false // Rejeitado, manter estado atual
}

// RunHMCTrajectory executa uma trajetoria completa usando HMC
// Substitui runMonteCarloSimulations para uma unica simulacao
func (hmc *HMCSampler) RunHMCTrajectory(initial *HMCState, daysAhead int) []DailyState {
	trajectory := make([]DailyState, daysAhead)
	current := &HMCState{
		PHQ9:      initial.PHQ9,
		Adherence: initial.Adherence,
		Sleep:     initial.Sleep,
	}

	baselineAdherence := initial.Adherence

	for day := 0; day < daysAhead; day++ {
		// HMC sample: propoe novo estado com gradiente
		proposed, accepted := hmc.Sample(current, baselineAdherence)

		if accepted {
			current = proposed
		}

		// Adicionar pequeno ruido diario (perturbacao estocastica)
		// HMC da a direcao, ruido da a variabilidade natural dia-a-dia
		current.PHQ9 += rand.NormFloat64() * 0.2
		current.Adherence += rand.NormFloat64() * 0.01
		current.Sleep += rand.NormFloat64() * 0.15
		current.clamp()

		// Crisis check usando modelo logistico (mantido do original)
		crisisProb := calculateCrisisProb(current.PHQ9, current.Adherence, current.Sleep)
		crisis := rand.Float64() < crisisProb

		trajectory[day] = DailyState{
			Day:       day + 1,
			PHQ9:      math.Round(current.PHQ9*100) / 100,
			Adherence: math.Round(current.Adherence*1000) / 1000,
			Sleep:     math.Round(current.Sleep*10) / 10,
			Crisis:    crisis,
		}
	}

	return trajectory
}

// calculateCrisisProb modelo logistico para probabilidade de crise diaria
func calculateCrisisProb(phq9, adherence, sleep float64) float64 {
	logit := -5.0
	logit += (phq9 - 10) * 0.15
	logit += (0.7 - adherence) * 2.0
	logit += (6 - sleep) * 0.3
	return 1.0 / (1.0 + math.Exp(-logit))
}
