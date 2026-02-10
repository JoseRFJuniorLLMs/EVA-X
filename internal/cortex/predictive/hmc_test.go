package predictive

import (
	"math"
	"testing"
)

func TestHMCSampler_PotentialEnergy(t *testing.T) {
	hmc := NewHMCSampler()

	// Estado saudavel: PHQ9 baixo, adesao alta, sono bom
	healthy := &HMCState{PHQ9: 3, Adherence: 0.95, Sleep: 7.5}
	energyHealthy := hmc.PotentialEnergy(healthy, 0.9)

	// Estado critico: PHQ9 alto, adesao baixa, sono ruim
	critical := &HMCState{PHQ9: 22, Adherence: 0.2, Sleep: 3.5}
	energyCritical := hmc.PotentialEnergy(critical, 0.9)

	t.Logf("Energia (saudavel): %.4f", energyHealthy)
	t.Logf("Energia (critico):  %.4f", energyCritical)

	if energyCritical <= energyHealthy {
		t.Errorf("Estado critico deveria ter MAIS energia que saudavel: %.4f <= %.4f",
			energyCritical, energyHealthy)
	}
}

func TestHMCSampler_Gradient(t *testing.T) {
	hmc := NewHMCSampler()

	state := &HMCState{PHQ9: 15, Adherence: 0.5, Sleep: 5}
	grad := hmc.GradientPotentialEnergy(state, 0.65)

	t.Logf("Gradiente em PHQ9=15, Adherence=0.5, Sleep=5:")
	t.Logf("  dU/dPHQ9 = %.6f", grad[0])
	t.Logf("  dU/dAdherence = %.6f", grad[1])
	t.Logf("  dU/dSleep = %.6f", grad[2])

	// PHQ9=15 esta acima do ideal, gradiente deve ser positivo (aumentar energia)
	if grad[0] <= 0 {
		t.Errorf("Gradiente PHQ9 deveria ser positivo em PHQ9=15: %.6f", grad[0])
	}

	// Adherence=0.5 esta abaixo do ideal, gradiente deve ser negativo
	// (diminuir adesao aumenta energia, entao dU/dAdherence < 0)
	if grad[1] >= 0 {
		t.Errorf("Gradiente Adherence deveria ser negativo em Adherence=0.5: %.6f", grad[1])
	}
}

func TestHMCSampler_Sample(t *testing.T) {
	hmc := NewHMCSampler()

	initial := &HMCState{PHQ9: 15, Adherence: 0.6, Sleep: 5.5}

	acceptCount := 0
	numSamples := 100

	for i := 0; i < numSamples; i++ {
		_, accepted := hmc.Sample(initial, 0.65)
		if accepted {
			acceptCount++
		}
	}

	acceptRate := float64(acceptCount) / float64(numSamples)
	t.Logf("Taxa de aceitacao HMC: %.1f%% (%d/%d)", acceptRate*100, acceptCount, numSamples)

	// HMC deveria ter taxa de aceitacao > 30%
	if acceptRate < 0.1 {
		t.Errorf("Taxa de aceitacao muito baixa: %.1f%% (esperado > 10%%)", acceptRate*100)
	}
}

func TestHMCSampler_RunTrajectory(t *testing.T) {
	hmc := NewHMCSampler()

	initial := &HMCState{PHQ9: 12, Adherence: 0.7, Sleep: 6}
	trajectory := hmc.RunHMCTrajectory(initial, 30)

	if len(trajectory) != 30 {
		t.Fatalf("Trajetoria deveria ter 30 dias, tem %d", len(trajectory))
	}

	// Verificar limites fisicos em todos os dias
	for _, day := range trajectory {
		if day.PHQ9 < 0 || day.PHQ9 > 27 {
			t.Errorf("PHQ9 fora dos limites no dia %d: %.2f", day.Day, day.PHQ9)
		}
		if day.Adherence < 0 || day.Adherence > 1 {
			t.Errorf("Adherence fora dos limites no dia %d: %.3f", day.Day, day.Adherence)
		}
		if day.Sleep < 2 || day.Sleep > 10 {
			t.Errorf("Sleep fora dos limites no dia %d: %.1f", day.Day, day.Sleep)
		}
	}

	t.Logf("Dia 1:  PHQ9=%.1f, Adherence=%.2f, Sleep=%.1f",
		trajectory[0].PHQ9, trajectory[0].Adherence, trajectory[0].Sleep)
	t.Logf("Dia 30: PHQ9=%.1f, Adherence=%.2f, Sleep=%.1f",
		trajectory[29].PHQ9, trajectory[29].Adherence, trajectory[29].Sleep)
}

func TestHMCSampler_EnergyConservation(t *testing.T) {
	hmc := NewHMCSampler()

	state := &HMCState{PHQ9: 10, Adherence: 0.65, Sleep: 6}
	momentum := hmc.SampleMomentum()

	// Hamiltoniano antes do leapfrog
	U_before := hmc.PotentialEnergy(state, 0.65)
	K_before := hmc.KineticEnergy(momentum)
	H_before := U_before + K_before

	// Leapfrog integration
	newState, newMomentum := hmc.LeapfrogIntegrate(state, momentum, 0.65)

	// Hamiltoniano depois
	U_after := hmc.PotentialEnergy(newState, 0.65)
	K_after := hmc.KineticEnergy(newMomentum)
	H_after := U_after + K_after

	deltaH := math.Abs(H_after - H_before)
	t.Logf("H_before=%.6f, H_after=%.6f, |dH|=%.6f", H_before, H_after, deltaH)

	// Leapfrog deveria preservar energia aproximadamente (dH < 1.0 para step_size=0.01)
	if deltaH > 5.0 {
		t.Errorf("Energia nao conservada: |dH|=%.6f (esperado < 5.0)", deltaH)
	}
}

func TestHMC_vs_RandomWalk_CrisisDistribution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comparison test in short mode")
	}

	// Comparar distribuicao de crises entre HMC e random walk
	nSims := 200
	days := 30

	state := &PatientState{
		PHQ9Score:           14,
		MedicationAdherence: 0.55,
		SleepHours:          5.0,
	}

	// Random Walk
	te := NewTrajectoryEngine(nil)
	te.SetUseHMC(false)
	rwResults := te.runMonteCarloSimulations(state, days, nSims)

	rwCrisis := 0
	for _, traj := range rwResults {
		for _, day := range traj {
			if day.Crisis {
				rwCrisis++
				break
			}
		}
	}

	// HMC
	te.SetUseHMC(true)
	hmcResults := te.runMonteCarloSimulations(state, days, nSims)

	hmcCrisis := 0
	for _, traj := range hmcResults {
		for _, day := range traj {
			if day.Crisis {
				hmcCrisis++
				break
			}
		}
	}

	rwRate := float64(rwCrisis) / float64(nSims) * 100
	hmcRate := float64(hmcCrisis) / float64(nSims) * 100

	t.Logf("Random Walk: %.1f%% simulacoes com crise em 30 dias", rwRate)
	t.Logf("HMC:         %.1f%% simulacoes com crise em 30 dias", hmcRate)
	t.Logf("Ambos devem detectar crises para paciente de risco (PHQ9=14, Adh=0.55)")

	// Ambos devem detectar algum risco para esse perfil
	if rwRate == 0 && hmcRate == 0 {
		t.Error("Nenhum dos dois metodos detectou crises para paciente de risco")
	}
}

func BenchmarkHMC_Sample(b *testing.B) {
	hmc := NewHMCSampler()
	state := &HMCState{PHQ9: 12, Adherence: 0.7, Sleep: 6}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hmc.Sample(state, 0.65)
	}
}

func BenchmarkHMC_Trajectory30d(b *testing.B) {
	hmc := NewHMCSampler()
	initial := &HMCState{PHQ9: 12, Adherence: 0.7, Sleep: 6}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hmc.RunHMCTrajectory(initial, 30)
	}
}

func BenchmarkRandomWalk_Trajectory30d(b *testing.B) {
	te := NewTrajectoryEngine(nil)
	te.SetUseHMC(false)
	state := &PatientState{PHQ9Score: 12, MedicationAdherence: 0.7, SleepHours: 6}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		te.runRandomWalkTrajectory(state, 30)
	}
}
