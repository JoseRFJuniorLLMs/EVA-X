// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package evolution

import (
	"context"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// Orchestrator coordena a agência autônoma da EVA (Zaratustra + Sono)
type Orchestrator struct {
	zaratustra *ZaratustraService
	sleep      *SleepService
	client     *nietzscheInfra.GraphAdapter
}

// NewOrchestrator cria um novo coordenador de evolução
func NewOrchestrator(client *nietzscheInfra.GraphAdapter) *Orchestrator {
	return &Orchestrator{
		zaratustra: NewZaratustraService(client),
		sleep:      NewSleepService(client),
		client:     client,
	}
}

// StartAutonomousCycle inicia um loop de fundo que executa evolução e sono periodicamente
func (o *Orchestrator) StartAutonomousCycle(ctx context.Context, collection string) {
	log.Printf("🚀 [EVOLUTION] Iniciando Orquestrador de Agência Autônoma")

	// Tickers para ciclos diferentes
	// Zaratustra (Evolução de Energia): Mais frequente
	evolutionTicker := time.NewTicker(6 * time.Hour)
	// Sleep (Reconsolidação Geométrica): Diário (ciclo circadiano)
	sleepTicker := time.NewTicker(24 * time.Hour)

	go func() {
		defer evolutionTicker.Stop()
		defer sleepTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("🛑 [EVOLUTION] Orquestrador finalizado")
				return

			case <-evolutionTicker.C:
				log.Printf("🦅 [EVOLUTION] Trigger: Ciclo Zaratustra")
				o.zaratustra.RunEvolutionCycle(ctx, collection)

			case <-sleepTicker.C:
				log.Printf("💤 [EVOLUTION] Trigger: Ciclo de Sono Riemanniano")
				o.sleep.TriggerRiemannianSleep(ctx, collection)
			}
		}
	}()
}

// RunManualIntegration executa ambos os ciclos imediatamente (para fins de teste/força bruta)
func (o *Orchestrator) RunManualIntegration(ctx context.Context, collection string) {
	log.Printf("🛠️ [EVOLUTION] Executando integração manual Evolution + Sleep")
	o.zaratustra.RunEvolutionCycle(ctx, collection)
	time.Sleep(2 * time.Second)
	o.sleep.TriggerRiemannianSleep(ctx, collection)
}
