// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"
	"time"

	"eva/internal/memory"

	"github.com/rs/zerolog/log"
)

// MemoryScheduler schedules periodic memory operations
type MemoryScheduler struct {
	orchestrator *memory.MemoryOrchestrator
	stopChan     chan struct{}
}

// NewMemoryScheduler creates a new memory scheduler
func NewMemoryScheduler(orchestrator *memory.MemoryOrchestrator) *MemoryScheduler {
	return &MemoryScheduler{
		orchestrator: orchestrator,
		stopChan:     make(chan struct{}),
	}
}

// Start starts the memory scheduler
func (s *MemoryScheduler) Start(ctx context.Context) {
	log.Info().Msg("🌙 Memory scheduler started")

	// Schedule nightly consolidation at 3 AM
	go s.scheduleNightlyConsolidation(ctx)

	// Schedule nightly backup at 2 AM
	go s.scheduleNightlyBackup(ctx)

	// Schedule evolution (Zaratustra) every 6 hours
	go s.scheduleKrylovMaintenance(ctx)
}

// scheduleNightlyConsolidation runs REM consolidation at 3 AM daily
func (s *MemoryScheduler) scheduleNightlyConsolidation(ctx context.Context) {
	for {
		// Calculate time until next 3 AM
		now := time.Now()
		next3AM := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, now.Location())

		if now.After(next3AM) {
			// If past 3 AM today, schedule for tomorrow
			next3AM = next3AM.Add(24 * time.Hour)
		}

		duration := time.Until(next3AM)

		log.Info().
			Str("next_run", next3AM.Format("2006-01-02 15:04:05")).
			Str("time_until", duration.String()).
			Msg("📅 Next REM consolidation scheduled")

		select {
		case <-ctx.Done():
			log.Info().Msg("🛑 Nightly consolidation scheduler stopped")
			return
		case <-s.stopChan:
			log.Info().Msg("🛑 Nightly consolidation scheduler stopped")
			return
		case <-time.After(duration):
			// Run consolidation
			s.runNightlyConsolidation(ctx)

			// Wait until next day
			time.Sleep(1 * time.Hour)
		}
	}
}

// runNightlyConsolidation executes the nightly consolidation
func (s *MemoryScheduler) runNightlyConsolidation(ctx context.Context) {
	log.Info().Msg("🌙 Running nightly REM consolidation")

	consolidationCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	err := s.orchestrator.RunNightlyConsolidation(consolidationCtx)
	if err != nil {
		log.Error().Err(err).Msg("❌ Nightly consolidation failed")
	} else {
		log.Info().Msg("✅ Nightly consolidation complete")
	}

	// TASK 8.1: Trigger Sleep cycle (RIEMANN reconsolidation)
	log.Info().Msg("💤 Triggering NietzscheDB Sleep cycle")
	if err := s.orchestrator.TriggerSleep(consolidationCtx, "memories"); err != nil {
		log.Error().Err(err).Msg("❌ NietzscheDB Sleep cycle failed")
	}

	// TASK 7.5: Trigger speculative Dream Simulation
	log.Info().Msg("💤 Triggering nightly Dream Simulation")
	if err := s.orchestrator.RunDreamSimulation(consolidationCtx, "patient_graph"); err != nil {
		log.Error().Err(err).Msg("❌ Nightly Dream Simulation failed")
	}
}

// scheduleNightlyBackup runs NietzscheDB backup at 2 AM daily
func (s *MemoryScheduler) scheduleNightlyBackup(ctx context.Context) {
	for {
		// Calculate time until next 2 AM
		now := time.Now()
		next2AM := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())

		if now.After(next2AM) {
			next2AM = next2AM.Add(24 * time.Hour)
		}

		duration := time.Until(next2AM)

		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-time.After(duration):
			log.Info().Msg("💾 Running nightly NietzscheDB backup")
			backupLabel := fmt.Sprintf("nightly_%s", time.Now().Format("20060102"))
			if err := s.orchestrator.CreateBackup(ctx, backupLabel); err != nil {
				log.Error().Err(err).Msg("❌ NietzscheDB backup failed")
			}
			time.Sleep(1 * time.Hour)
		}
	}
}

// scheduleKrylovMaintenance runs Krylov reorthogonalization every 6 hours
func (s *MemoryScheduler) scheduleKrylovMaintenance(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	log.Info().Msg("🔧 Krylov maintenance scheduler started (every 6h)")

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("🛑 Krylov maintenance scheduler stopped")
			return
		case <-s.stopChan:
			log.Info().Msg("🛑 Krylov maintenance scheduler stopped")
			return
		case <-ticker.C:
			// Run maintenance
			log.Info().Msg("🔧 Running Krylov maintenance")
			s.orchestrator.MemoryConsolidation()

			// TASK 8.2: Invoke Zaratustra autonomous evolution
			log.Info().Msg("⚡ Invoking Zaratustra evolution")
			if err := s.orchestrator.InvokeZaratustra(ctx, "memories"); err != nil {
				log.Error().Err(err).Msg("❌ Zaratustra evolution failed")
			}
		}
	}
}

// Stop stops the scheduler
func (s *MemoryScheduler) Stop() {
	close(s.stopChan)
}
