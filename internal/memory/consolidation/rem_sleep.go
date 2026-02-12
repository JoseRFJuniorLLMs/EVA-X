package consolidation

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog/log"
)

// REMConsolidator implements REM sleep-like memory consolidation
type REMConsolidator struct {
	db                     *sql.DB
	replaySpeed            float64 // 10x = 10 times faster
	consolidationThreshold int     // Consolidate every N memories
}

// NewREMConsolidator creates a new REM consolidator
func NewREMConsolidator(db *sql.DB) *REMConsolidator {
	return &REMConsolidator{
		db:                     db,
		replaySpeed:            10.0, // 10x speed
		consolidationThreshold: 100,
	}
}

// HotMemory represents a frequently accessed memory
type HotMemory struct {
	ID          int64
	Content     string
	Embedding   []float64
	AccessCount int
	LastAccess  time.Time
	Importance  float64
}

// ProtoConcept represents an abstracted concept from clustered memories
type ProtoConcept struct {
	Centroid         []float64
	CommonSignifiers []string
	Examples         []int64 // Memory IDs
	AbstractionLevel int
	CreatedAt        time.Time
}

// ConsolidateNightly performs nightly memory consolidation
func (r *REMConsolidator) ConsolidateNightly(ctx context.Context) error {
	log.Info().Msg("🌙 Starting REM sleep consolidation...")

	// 1. Identify "hot" memories (high activation in last 24h)
	hotMemories, err := r.getHotMemories(ctx, 24*time.Hour)
	if err != nil {
		return err
	}

	log.Info().Int("count", len(hotMemories)).Msg("Found hot memories")

	// 2. REPLAY: Re-process embeddings in batch (simulates "dreaming")
	for _, mem := range hotMemories {
		// TODO: Re-activate in Krylov space
		log.Debug().Int64("memory_id", mem.ID).Msg("Replaying memory")
	}

	// 3. CLUSTERING: Group similar memories (spectral clustering)
	communities, err := r.clusterMemories(ctx, hotMemories)
	if err != nil {
		return err
	}

	log.Info().Int("communities", len(communities)).Msg("Detected memory communities")

	// 4. ABSTRACTION: Create proto-concepts for each community
	for _, comm := range communities {
		protoConcept := r.abstractCommunity(comm)

		// 5. TRANSFER: Create semantic node (episodic → semantic)
		err = r.createSemanticNode(ctx, protoConcept)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create semantic node")
			continue
		}
	}

	// 6. PRUNING: Remove redundant memories within communities
	err = r.pruneRedundantMemories(ctx, communities)
	if err != nil {
		return err
	}

	log.Info().Msg("✅ REM consolidation complete")
	return nil
}

// getHotMemories retrieves frequently accessed memories
func (r *REMConsolidator) getHotMemories(ctx context.Context, window time.Duration) ([]*HotMemory, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT 
			m.id,
			m.content,
			COUNT(mal.id) as access_count,
			MAX(mal.accessed_at) as last_access,
			m.importance_score
		FROM memories m
		JOIN memory_access_log mal ON m.id = mal.memory_id
		WHERE mal.accessed_at > NOW() - $1::interval
		GROUP BY m.id
		HAVING COUNT(mal.id) >= 3
		ORDER BY COUNT(mal.id) DESC
		LIMIT 100
	`, window)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*HotMemory
	for rows.Next() {
		var mem HotMemory
		err := rows.Scan(&mem.ID, &mem.Content, &mem.AccessCount, &mem.LastAccess, &mem.Importance)
		if err != nil {
			return nil, err
		}
		memories = append(memories, &mem)
	}

	return memories, rows.Err()
}

// MemoryCommunity represents a cluster of related memories
type MemoryCommunity struct {
	Members  []*HotMemory
	Centroid []float64
}

// clusterMemories groups similar memories using spectral clustering
func (r *REMConsolidator) clusterMemories(ctx context.Context, memories []*HotMemory) ([]*MemoryCommunity, error) {
	// TODO: Implement spectral clustering
	// For now, return single community
	return []*MemoryCommunity{
		{Members: memories},
	}, nil
}

// abstractCommunity creates a proto-concept from a memory community
func (r *REMConsolidator) abstractCommunity(comm *MemoryCommunity) *ProtoConcept {
	// TODO: Compute centroid in Krylov space
	// TODO: Extract common signifiers

	// Take top 3 examples
	examples := make([]int64, 0, 3)
	for i := 0; i < 3 && i < len(comm.Members); i++ {
		examples = append(examples, comm.Members[i].ID)
	}

	return &ProtoConcept{
		Examples:         examples,
		AbstractionLevel: 1,
		CreatedAt:        time.Now(),
	}
}

// createSemanticNode creates a semantic memory node in Neo4j
func (r *REMConsolidator) createSemanticNode(ctx context.Context, concept *ProtoConcept) error {
	// TODO: Create node in Neo4j
	log.Info().Int("examples", len(concept.Examples)).Msg("Created semantic node")
	return nil
}

// pruneRedundantMemories removes redundant episodic memories
func (r *REMConsolidator) pruneRedundantMemories(ctx context.Context, communities []*MemoryCommunity) error {
	// Within each community, keep only the most important memories
	// Remove the rest (they're now represented by the proto-concept)

	for _, comm := range communities {
		if len(comm.Members) <= 3 {
			continue // Keep small communities intact
		}

		// Sort by importance
		// TODO: Sort and prune bottom 70%

		log.Info().Int("community_size", len(comm.Members)).Msg("Pruned redundant memories")
	}

	return nil
}

// ShouldConsolidate checks if consolidation should run
func (r *REMConsolidator) ShouldConsolidate(ctx context.Context) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM memories 
		WHERE created_at > NOW() - INTERVAL '24 hours'
	`).Scan(&count)

	if err != nil {
		return false, err
	}

	return count >= r.consolidationThreshold, nil
}
