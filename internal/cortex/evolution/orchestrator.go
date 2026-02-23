// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package evolution

import (
	"context"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// Orchestrator coordena a agência autônoma da EVA (Zaratustra + Sono + Sonhos)
type Orchestrator struct {
	zaratustra *ZaratustraService
	sleep      *SleepService
	dream      *DreamService
	client     *nietzscheInfra.GraphAdapter
}

// NewOrchestrator cria um novo coordenador de evolução
func NewOrchestrator(client *nietzscheInfra.GraphAdapter) *Orchestrator {
	return &Orchestrator{
		zaratustra: NewZaratustraService(client),
		sleep:      NewSleepService(client),
		dream:      NewDreamService(client),
		client:     client,
	}
}

// Dream returns the DreamService for external callers that need direct access.
func (o *Orchestrator) Dream() *DreamService {
	return o.dream
}

// StartAutonomousCycle inicia um loop de fundo que executa evolução, sono e sonhos periodicamente
func (o *Orchestrator) StartAutonomousCycle(ctx context.Context, collection string) {
	log.Printf("[EVOLUTION] Iniciando Orquestrador de Agencia Autonoma (Zaratustra + Sleep + Dream + Backup)")

	// Tickers para ciclos diferentes
	// Zaratustra (Evolução de Energia): Mais frequente
	evolutionTicker := time.NewTicker(6 * time.Hour)
	// Dream (Exploracao Especulativa): A cada 8 horas (durante o "dia" da EVA)
	dreamTicker := time.NewTicker(8 * time.Hour)
	// Sleep (Reconsolidação Geométrica): Diário (ciclo circadiano)
	sleepTicker := time.NewTicker(24 * time.Hour)
	// Backup (RocksDB checkpoint): Diário, offset ~1h do Sleep (executa na marca de ~23h)
	// Initial delay of 23h ensures the first backup is offset from the first sleep cycle.
	backupTicker := time.NewTicker(24 * time.Hour)

	go func() {
		defer evolutionTicker.Stop()
		defer dreamTicker.Stop()
		defer sleepTicker.Stop()
		defer backupTicker.Stop()

		// Offset backup from sleep by 1h: wait 23h before first backup tick fires
		// This ensures backup runs ~1h before each sleep cycle, not simultaneously.
		backupOffsetTimer := time.NewTimer(23 * time.Hour)
		defer backupOffsetTimer.Stop()

		backupStarted := false

		for {
			select {
			case <-ctx.Done():
				log.Printf("[EVOLUTION] Orquestrador finalizado")
				return

			case <-evolutionTicker.C:
				log.Printf("[EVOLUTION] Trigger: Ciclo Zaratustra")
				o.zaratustra.RunEvolutionCycle(ctx, collection)

			case <-dreamTicker.C:
				log.Printf("[EVOLUTION] Trigger: Ciclo de Sonho Especulativo")
				// Dream from high-energy nodes (>0.7 energy, up to 3 dreams per cycle)
				o.dream.RunDreamFromHighEnergy(ctx, collection, 0.7, 3)

			case <-sleepTicker.C:
				log.Printf("[EVOLUTION] Trigger: Ciclo de Sono Riemanniano")
				o.sleep.TriggerRiemannianSleep(ctx, collection)

			case <-backupOffsetTimer.C:
				// First backup after 23h offset — then the 24h ticker takes over
				if !backupStarted {
					backupStarted = true
					log.Printf("[EVOLUTION] Trigger: Backup NietzscheDB (offset inicial)")
					o.runBackup(ctx)
				}

			case <-backupTicker.C:
				if backupStarted {
					log.Printf("[EVOLUTION] Trigger: Backup NietzscheDB (diario)")
					o.runBackup(ctx)
				}
			}
		}
	}()
}

// runBackup creates a RocksDB checkpoint backup with a date label.
func (o *Orchestrator) runBackup(ctx context.Context) {
	label := time.Now().Format("2006-01-02")
	_, err := o.client.Client().CreateBackup(ctx, label)
	if err != nil {
		log.Printf("[EVOLUTION] Backup falhou: %v", err)
	} else {
		log.Printf("[EVOLUTION] Backup concluido: %s", label)
	}
}

// RunManualIntegration executa ambos os ciclos imediatamente (para fins de teste/força bruta)
func (o *Orchestrator) RunManualIntegration(ctx context.Context, collection string) {
	log.Printf("[EVOLUTION] Executando integracao manual Evolution + Sleep")
	o.zaratustra.RunEvolutionCycle(ctx, collection)
	time.Sleep(2 * time.Second)
	o.sleep.TriggerRiemannianSleep(ctx, collection)
}

// ── L-System Daemon Setup ────────────────────────────────────────────────────

// SetupLSystemDaemon creates a Wiederkehr daemon that acts as an L-System rule:
// when a Condition node is inserted, it auto-creates symptom sub-nodes via DIFFUSE.
// This only runs on small collections (< 1000 nodes) to avoid CPU explosion on
// large graphs (L-System ticks on 14K+ nodes take 10+ min at 90% CPU).
//
// Uses NQL CREATE DAEMON via the GraphAdapter's Query method:
//
//	CREATE DAEMON symptom_expander ON (n:Condition)
//	  WHEN n.energy > 0.5
//	  THEN DIFFUSE FROM n WITH t=[0.1, 0.5] MAX_HOPS 3
//	  EVERY INTERVAL("2h") ENERGY 0.7
func (o *Orchestrator) SetupLSystemDaemon(ctx context.Context, collection string) error {
	// Guard: only create daemon for small collections to avoid CPU explosion
	sdk := o.client.SDK()
	collections, err := sdk.ListCollections(ctx)
	if err != nil {
		log.Printf("[L-SYSTEM] Failed to list collections: %v", err)
		return err
	}

	var nodeCount uint64
	for _, col := range collections {
		if col.Name == collection {
			nodeCount = col.NodeCount
			break
		}
	}

	if nodeCount >= 1000 {
		log.Printf("[L-SYSTEM] Skipping daemon for %s (%d nodes >= 1000 threshold)", collection, nodeCount)
		return nil
	}

	// Create the L-System daemon via NQL CREATE DAEMON
	nql := `CREATE DAEMON symptom_expander ON (n:Condition) WHEN n.energy > 0.5 THEN DIFFUSE FROM n WITH t=[0.1, 0.5] MAX_HOPS 3 EVERY INTERVAL("2h") ENERGY 0.7`
	result, nqlErr := o.client.ExecuteNQL(ctx, nql, nil, collection)
	if nqlErr != nil {
		log.Printf("[L-SYSTEM] CREATE DAEMON failed: %v", nqlErr)
		return nqlErr
	}
	if result != nil && result.Error != "" {
		log.Printf("[L-SYSTEM] CREATE DAEMON error: %s", result.Error)
		// Non-fatal: daemon may already exist
	} else {
		log.Printf("[L-SYSTEM] Daemon 'symptom_expander' created for %s (%d nodes)", collection, nodeCount)
	}

	return nil
}
