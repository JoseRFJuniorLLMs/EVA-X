// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package scheduler

import (
	"context"
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

	// Schedule Krylov reorthogonalization every 6 hours
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
			// TODO: Call Krylov reorthogonalization
			// s.orchestrator.krylovManager.MemoryConsolidation()
		}
	}
}

// Stop stops the scheduler
func (s *MemoryScheduler) Stop() {
	close(s.stopChan)
}
