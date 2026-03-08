// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/memory"
	"eva/internal/memory/consolidation"
	"eva/internal/memory/krylov"
	nietzsche "nietzsche-sdk"

	"github.com/rs/zerolog/log"
)

// MemoryOrchestrator integrates the full memory pipeline
// Voice → FDPN → Krylov → Spectral → REM
type MemoryOrchestrator struct {
	db            *sql.DB
	graphAdapter  *nietzscheInfra.GraphAdapter  // NietzscheDB GraphAdapter
	vectorAdapter *nietzscheInfra.VectorAdapter // NietzscheDB VectorAdapter (busca vetorial)

	// Pipeline components
	fdpnEngine      *memory.FDPNEngine
	krylovManager   *krylov.KrylovMemoryManager
	remConsolidator *consolidation.REMConsolidator
}

// NewMemoryOrchestrator creates a new memory orchestrator
func NewMemoryOrchestrator(
	db *sql.DB,
	graphAdapter *nietzscheInfra.GraphAdapter,
	vectorAdapter *nietzscheInfra.VectorAdapter,
	fdpn *memory.FDPNEngine,
	krylovMgr *krylov.KrylovMemoryManager,
) *MemoryOrchestrator {
	if db == nil {
		log.Warn().Msg("⚠️ [MEMORY-ORCH] NietzscheDB unavailable — running in degraded mode")
	}
	return &MemoryOrchestrator{
		db:              db,
		graphAdapter:    graphAdapter,
		vectorAdapter:   vectorAdapter,
		fdpnEngine:      fdpn,
		krylovManager:   krylovMgr,
		remConsolidator: consolidation.NewREMConsolidator(graphAdapter, krylovMgr),
	}
}

// IngestMemory processes a new memory through the full pipeline
func (o *MemoryOrchestrator) IngestMemory(ctx context.Context, userID string, content string, embedding []float64) error {
	log.Info().
		Str("user_id", userID).
		Str("content_preview", truncate(content, 50)).
		Msg("🧠 Memory ingestion started")

	// STEP 1: FDPN Activation (streaming prime)
	// Activates relevant subgraphs based on content
	err := o.fdpnEngine.StreamingPrime(ctx, userID, content)
	if err != nil {
		log.Error().Err(err).Msg("FDPN activation failed")
		// Non-critical, continue
	} else {
		log.Debug().Msg("✅ FDPN: Subgraphs activated")
	}

	// STEP 2: Krylov Compression
	// Compress 1536D → 64D embedding
	compressedEmbedding, err := o.krylovManager.CompressVector(embedding)
	if err != nil {
		log.Error().Err(err).Msg("Krylov compression failed")
		// Fallback: use original embedding
		compressedEmbedding = embedding
	} else {
		log.Debug().
			Int("original_dim", len(embedding)).
			Int("compressed_dim", len(compressedEmbedding)).
			Msg("✅ Krylov: Embedding compressed")
	}

	// STEP 3: Update Krylov subspace with new vector
	err = o.krylovManager.UpdateSubspace(embedding)
	if err != nil {
		log.Error().Err(err).Msg("Krylov subspace update failed")
	} else {
		log.Debug().Msg("✅ Krylov: Subspace updated")
	}

	// STEP 4: Store in NietzscheDB vector (with compressed embedding)
	// TODO: Store in NietzscheDB vector with compressed embedding
	// This would require VectorAdapter integration

	// STEP 5: Store in NietzscheDB
	// TODO: Store episodic memory in database

	log.Info().
		Str("user_id", userID).
		Msg("✅ Memory ingestion complete")

	return nil
}

// RunNightlyConsolidation executes REM consolidation for all active patients
func (o *MemoryOrchestrator) RunNightlyConsolidation(ctx context.Context) error {
	log.Info().Msg("🌙 Starting nightly REM consolidation")

	startTime := time.Now()

	// Run consolidation for all active patients
	results, err := o.remConsolidator.ConsolidateAll(ctx)
	if err != nil {
		log.Error().Err(err).Msg("REM consolidation failed")
		return err
	}

	// Log results
	totalProcessed := 0
	totalClusters := 0
	totalPruned := 0

	for _, result := range results {
		totalProcessed += result.EpisodicProcessed
		totalClusters += result.CommunitiesFormed
		totalPruned += result.MemoriesPruned
	}

	duration := time.Since(startTime)

	log.Info().
		Int("patients", len(results)).
		Int("episodic_processed", totalProcessed).
		Int("clusters_formed", totalClusters).
		Int("memories_pruned", totalPruned).
		Str("duration", duration.String()).
		Msg("✅ Nightly REM consolidation complete")

	// Run Krylov memory consolidation
	o.MemoryConsolidation()

	return nil
}

// MemoryConsolidation runs Krylov memory consolidation
func (o *MemoryOrchestrator) MemoryConsolidation() {
	if o.krylovManager != nil {
		o.krylovManager.MemoryConsolidation()
	}
}

// TriggerSleep initiates a Riemannian reconsolidation cycle in NietzscheDB
func (o *MemoryOrchestrator) TriggerSleep(ctx context.Context, collection string) error {
	if o.graphAdapter == nil || o.graphAdapter.Client() == nil {
		return nil
	}
	log.Info().Str("collection", collection).Msg("💤 Triggering NietzscheDB Sleep cycle")
	_, err := o.graphAdapter.Client().TriggerSleep(ctx, nietzsche.SleepOpts{
		Collection: collection,
	})
	return err
}

// InvokeZaratustra runs the autonomous evolution engine in NietzscheDB
func (o *MemoryOrchestrator) InvokeZaratustra(ctx context.Context, collection string) error {
	if o.graphAdapter == nil || o.graphAdapter.Client() == nil {
		return nil
	}
	log.Info().Str("collection", collection).Msg("⚡ Invoking Zaratustra evolution")
	_, err := o.graphAdapter.Client().InvokeZaratustra(ctx, nietzsche.ZaratustraOpts{
		Collection: collection,
	})
	return err
}

// CreateBackup creates a point-in-time backup of the NietzscheDB storage
func (o *MemoryOrchestrator) CreateBackup(ctx context.Context, label string) error {
	if o.graphAdapter == nil || o.graphAdapter.Client() == nil {
		return nil
	}
	log.Info().Str("label", label).Msg("💾 Creating NietzscheDB backup")
	_, err := o.graphAdapter.Client().CreateBackup(ctx, label)
	return err
}

// RunDreamSimulation executes speculative exploration (DREAM FROM) on hot nodes
func (o *MemoryOrchestrator) RunDreamSimulation(ctx context.Context, collection string) error {
	if o.graphAdapter == nil || o.graphAdapter.Client() == nil {
		return nil
	}

	log.Info().Str("collection", collection).Msg("💤 Starting nightly Dream Simulation")

	// 1. Find hot nodes to use as dream seeds
	// Evaluation: Energy > 0.7, top 5
	nql := `MATCH (n) WHERE n.energy > 0.7 RETURN n.id, n.energy ORDER BY n.energy DESC LIMIT 5`
	result, err := o.graphAdapter.ExecuteNQL(ctx, nql, nil, collection)
	if err != nil {
		return fmt.Errorf("failed to find dream seeds: %w", err)
	}

	for _, row := range result.ScalarRows {
		seedID, _ := row["n.id"].(string)
		if seedID == "" {
			continue
		}

		log.Debug().Str("seed", seedID).Msg("🌠 Starting dream from seed")

		// 2. Start speculative dream
		dream, err := o.graphAdapter.Client().StartDream(ctx, collection, seedID, 5, 0.05)
		if err != nil {
			log.Error().Err(err).Str("seed", seedID).Msg("❌ Dream simulation failed")
			continue
		}

		// 3. Evaluate dream result
		// Heuristic: If dream produced > 2 events (anomalies), it found something interesting
		if len(dream.Events) > 2 {
			log.Info().
				Str("dream_id", dream.DreamID).
				Int("events", len(dream.Events)).
				Msg("✅ Interesting dream detected, applying changes")

			if err := o.graphAdapter.Client().ApplyDream(ctx, collection, dream.DreamID); err != nil {
				log.Error().Err(err).Str("dream_id", dream.DreamID).Msg("❌ Failed to apply dream")
			}
		} else {
			log.Debug().Str("dream_id", dream.DreamID).Msg("😴 Uninteresting dream, rejecting")
			if err := o.graphAdapter.Client().RejectDream(ctx, collection, dream.DreamID); err != nil {
				log.Error().Err(err).Str("dream_id", dream.DreamID).Msg("❌ Failed to reject dream")
			}
		}
	}

	return nil
}

// GetPipelineStatus returns status of all pipeline components
func (o *MemoryOrchestrator) GetPipelineStatus() map[string]interface{} {
	return map[string]interface{}{
		"krylov": o.krylovManager.GetStatistics(),
		"rem":    o.remConsolidator.GetStatistics(),
		"fdpn":   "active", // FDPN doesn't have statistics yet
	}
}

// truncate truncates a string to maxLen
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
