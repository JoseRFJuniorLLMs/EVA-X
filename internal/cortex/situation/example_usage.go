// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package situation

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Este arquivo contém exemplos de uso do Situational Modulator
// NÃO É CÓDIGO DE PRODUÇÃO - apenas referência para integração

// ExampleUsage_BasicModulation demonstra uso básico
func ExampleUsage_BasicModulation() {
	// Setup
	modulator := NewModulator(nil, nil) // nil cache = sem cache (para exemplo)

	// Cenário 1: Usuário em luto
	fmt.Println("=== Cenário 1: Luto ===")
	sit, _ := modulator.Infer(
		context.Background(),
		"user123",
		"Minha mãe faleceu ontem, estou muito triste",
		[]Event{},
	)

	fmt.Printf("Stressors: %v\n", sit.Stressors)
	fmt.Printf("Emotion Score: %.2f\n", sit.EmotionScore)
	fmt.Printf("Intensity: %.2f\n", sit.Intensity)

	baseWeights := map[string]float64{
		"ANSIEDADE":        0.5,
		"BUSCA_SEGURANÇA":  0.4,
		"EXTROVERSÃO":      0.6,
	}

	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)
	fmt.Printf("ANSIEDADE: %.2f → %.2f\n", baseWeights["ANSIEDADE"], modulatedWeights["ANSIEDADE"])
	fmt.Printf("EXTROVERSÃO: %.2f → %.2f\n", baseWeights["EXTROVERSÃO"], modulatedWeights["EXTROVERSÃO"])

	// Output esperado:
	// Stressors: [luto]
	// Emotion Score: -0.30
	// Intensity: 0.45
	// ANSIEDADE: 0.50 → 0.90 (+80%)
	// EXTROVERSÃO: 0.60 → 0.30 (-50%)
}

// ExampleUsage_IntegrationFDPN demonstra integração com FDPN
func ExampleUsage_IntegrationFDPN() {
	fmt.Println("=== Integração FDPN ===")

	// Pseudo-código (integração real requer FDPNEngine instanciado)
	/*
		fdpn := memory.NewFDPNEngine(nietzscheGraph, nietzscheCache, nietzscheVector)
		modulator := situation.NewModulator(nietzscheCache, nil)

		// Antes de cada query:
		sit, _ := modulator.Infer(ctx, userID, query, recentEvents)

		baseWeights := getPersonalityWeights(userID)
		modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

		// Usar weights modulados no priming
		activatedNodes, _ := fdpn.StreamingPrimeWithSituation(ctx, userID, query, recentEvents, modulator)

		// Alerta se crise
		if sit.Intensity > 0.8 && contains(sit.Stressors, "crise") {
			alertService.SendCritical(userID, "Possível crise detectada", sit)
		}
	*/

	log.Println("Ver fdpn_situational.go para implementação real")
}

// ExampleUsage_MultipleStressors demonstra múltiplos stressors
func ExampleUsage_MultipleStressors() {
	modulator := NewModulator(nil, nil)

	fmt.Println("=== Cenário 2: Múltiplos Stressors ===")

	// Usuário em luto + internado no hospital
	sit, _ := modulator.Infer(
		context.Background(),
		"user456",
		"Minha esposa faleceu e agora estou internado no hospital com problemas no coração",
		[]Event{},
	)

	fmt.Printf("Stressors: %v\n", sit.Stressors)
	fmt.Printf("Intensity: %.2f (alta devido a múltiplos stressors)\n", sit.Intensity)

	baseWeights := map[string]float64{
		"ANSIEDADE":        0.5,
		"BUSCA_SEGURANÇA":  0.4,
		"ALERTA":           0.5,
	}

	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

	// Ambas as regras aplicadas (luto + hospital)
	fmt.Printf("ANSIEDADE: %.2f → %.2f (luto: 1.8x)\n",
		baseWeights["ANSIEDADE"], modulatedWeights["ANSIEDADE"])
	fmt.Printf("BUSCA_SEGURANÇA: %.2f → %.2f (luto: 2.0x, hospital: 1.5x)\n",
		baseWeights["BUSCA_SEGURANÇA"], modulatedWeights["BUSCA_SEGURANÇA"])
	fmt.Printf("ALERTA: %.2f → %.2f (hospital: 2.0x)\n",
		baseWeights["ALERTA"], modulatedWeights["ALERTA"])

	// Output esperado:
	// Stressors: [luto hospital doença]
	// Intensity: 0.90 (alta)
	// ANSIEDADE: 0.50 → 0.90
	// BUSCA_SEGURANÇA: 0.40 → 1.20 (múltiplas regras aplicadas)
	// ALERTA: 0.50 → 1.00
}

// ExampleUsage_PartyIntrovert demonstra modulação contextual
func ExampleUsage_PartyIntrovert() {
	modulator := NewModulator(nil, nil)

	fmt.Println("=== Cenário 3: Introvertido em Festa ===")

	sit, _ := modulator.Infer(
		context.Background(),
		"user789",
		"Estou na festa de aniversário da família",
		[]Event{},
	)

	// Pessoa introvertida (EXTROVERSÃO baixa)
	baseWeights := map[string]float64{
		"EXTROVERSÃO": 0.3,
	}

	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

	// EXTROVERSÃO aumenta porque comportamento é INCOMUM (introvertido em festa)
	fmt.Printf("EXTROVERSÃO: %.2f → %.2f (comportamento incomum = mais informativo)\n",
		baseWeights["EXTROVERSÃO"], modulatedWeights["EXTROVERSÃO"])

	// Output esperado:
	// EXTROVERSÃO: 0.30 → 0.42 (aumenta por ser incomum)
}

// ExampleUsage_MidnightAlone demonstra regras contextuais
func ExampleUsage_MidnightAlone() {
	modulator := NewModulator(nil, nil)

	fmt.Println("=== Cenário 4: Madrugada Sozinho ===")

	// Simular madrugada (2 AM)
	// Na prática, isso é detectado automaticamente via time.Now()
	sit := Situation{
		Stressors:     []string{},
		SocialContext: "sozinho",
		TimeOfDay:     "madrugada",
		EmotionScore:  -0.2,
		Intensity:     0.4,
	}

	baseWeights := map[string]float64{
		"SOLIDÃO":   0.3,
		"ANSIEDADE": 0.5,
	}

	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

	fmt.Printf("SOLIDÃO: %.2f → %.2f (+50%%)\n",
		baseWeights["SOLIDÃO"], modulatedWeights["SOLIDÃO"])
	fmt.Printf("ANSIEDADE: %.2f → %.2f (+30%%)\n",
		baseWeights["ANSIEDADE"], modulatedWeights["ANSIEDADE"])

	// Output esperado:
	// SOLIDÃO: 0.30 → 0.45
	// ANSIEDADE: 0.50 → 0.65
}

// ExampleUsage_Performance demonstra performance
func ExampleUsage_Performance() {
	modulator := NewModulator(nil, nil)

	fmt.Println("=== Benchmark de Performance ===")

	iterations := 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		modulator.Infer(
			context.Background(),
			"user123",
			"Texto qualquer para inferência",
			[]Event{},
		)
	}

	duration := time.Since(start)
	avgLatency := duration / time.Duration(iterations)

	fmt.Printf("Iterations: %d\n", iterations)
	fmt.Printf("Total time: %v\n", duration)
	fmt.Printf("Average latency: %v\n", avgLatency)
	fmt.Printf("Target: <10ms per inference\n")

	// Output esperado (sem cache miss):
	// Average latency: ~1-5ms
	// Com cache hit: <1ms
}

// ExampleUsage_CacheEffect demonstra efeito do cache
func ExampleUsage_CacheEffect() {
	// Com cache NietzscheDB (na prática)
	fmt.Println("=== Cache Effect ===")

	// Primeira chamada: cache miss (~5ms)
	// Segunda chamada (mesmo userID): cache hit (<1ms)

	log.Println("Ver testes de integração para benchmark real com NietzscheDB cache")
}

// ExampleUsage_AlertCritical demonstra alertas críticos
func ExampleUsage_AlertCritical() {
	modulator := NewModulator(nil, nil)

	fmt.Println("=== Cenário 5: Crise (Alerta Crítico) ===")

	sit, _ := modulator.Infer(
		context.Background(),
		"user999",
		"Estou em crise, não aguento mais, preciso de ajuda urgente",
		[]Event{},
	)

	fmt.Printf("Stressors: %v\n", sit.Stressors)
	fmt.Printf("Intensity: %.2f\n", sit.Intensity)

	// Verifica se deve disparar alerta
	if sit.Intensity > 0.8 && contains(sit.Stressors, "crise") {
		fmt.Println("🚨 ALERTA CRÍTICO: Crise detectada - notificar cuidador imediatamente")
	}

	baseWeights := map[string]float64{
		"ALERTA":    0.5,
		"ANSIEDADE": 0.5,
		"DESESPERO": 0.2,
	}

	modulatedWeights := modulator.ModulateWeights(baseWeights, sit)

	fmt.Printf("ALERTA: %.2f → %.2f (+150%%)\n",
		baseWeights["ALERTA"], modulatedWeights["ALERTA"])
	fmt.Printf("DESESPERO: %.2f → %.2f (+200%%)\n",
		baseWeights["DESESPERO"], modulatedWeights["DESESPERO"])

	// Output esperado:
	// Stressors: [crise]
	// Intensity: 0.90
	// 🚨 ALERTA CRÍTICO
	// ALERTA: 0.50 → 1.25
	// DESESPERO: 0.20 → 0.60
}