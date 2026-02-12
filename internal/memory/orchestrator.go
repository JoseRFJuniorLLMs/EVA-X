package memory

import (
	"context"
	"database/sql"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/internal/brainstem/infrastructure/vector"
	"eva-mind/internal/hippocampus/memory"
	"eva-mind/internal/memory/consolidation"

	"github.com/rs/zerolog/log"
)

// MemoryOrchestrator integrates the full memory pipeline
// Voice → FDPN → Krylov → Spectral → REM
type MemoryOrchestrator struct {
	db     *sql.DB
	neo4j  *graph.Neo4jClient
	qdrant *vector.QdrantClient

	// Pipeline components
	fdpnEngine      *memory.FDPNEngine
	krylovManager   *KrylovMemoryManager
	remConsolidator *consolidation.REMConsolidator
}

// NewMemoryOrchestrator creates a new memory orchestrator
func NewMemoryOrchestrator(
	db *sql.DB,
	neo4j *graph.Neo4jClient,
	qdrant *vector.QdrantClient,
	fdpn *memory.FDPNEngine,
	krylov *KrylovMemoryManager,
) *MemoryOrchestrator {
	return &MemoryOrchestrator{
		db:              db,
		neo4j:           neo4j,
		qdrant:          qdrant,
		fdpnEngine:      fdpn,
		krylovManager:   krylov,
		remConsolidator: consolidation.NewREMConsolidator(neo4j, krylov),
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

	// STEP 4: Store in Qdrant (with compressed embedding)
	// TODO: Store in Qdrant with compressed embedding
	// This would require Qdrant integration

	// STEP 5: Store in PostgreSQL
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
		totalClusters += result.ClustersFormed
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
	o.krylovManager.MemoryConsolidation()

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
