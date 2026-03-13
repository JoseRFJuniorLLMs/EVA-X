// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package importance

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

const (
	// pageRankCacheTTL controls how long PageRank results are cached before recomputation.
	pageRankCacheTTL = 10 * time.Minute

	// pageRankDamping is the standard damping factor for PageRank.
	pageRankDamping = 0.85

	// pageRankMaxIter is the maximum number of PageRank iterations.
	pageRankMaxIter = 20

	// pageRankCollection is the NietzscheDB collection to run PageRank on.
	pageRankCollection = "eva_mind"

	// defaultCentrality is the fallback when PageRank is unavailable.
	defaultCentrality = 0.5
)

// pageRankCache holds cached PageRank scores with a TTL.
type pageRankCache struct {
	mu        sync.RWMutex
	scores    map[string]float64 // nodeUUID -> normalized score (0-1)
	updatedAt time.Time
}

// Scorer calculates importance scores for memories
type Scorer struct {
	db    *database.DB
	cache pageRankCache
}

// NewScorer creates a new importance scorer
func NewScorer(db *database.DB) *Scorer {
	return &Scorer{
		db: db,
		cache: pageRankCache{
			scores: make(map[string]float64),
		},
	}
}

// ImportanceFactors represents the components of importance
type ImportanceFactors struct {
	Frequency          float64 // How often accessed (0-1)
	Recency            float64 // How recent (0-1)
	GraphCentrality    float64 // NietzscheDB graph connections (0-1)
	EmotionalIntensity float64 // Emotional weight (0-1)
	GoalRelevance      float64 // Relevance to patient goals (0-1)
}

// MemoryImportance represents a memory with its importance score
type MemoryImportance struct {
	MemoryID     int64
	Score        float64
	Factors      ImportanceFactors
	CalculatedAt time.Time
}

// refreshPageRankIfStale recomputes PageRank scores if the cache has expired.
// Safe for concurrent access. Only one goroutine will perform the refresh.
func (s *Scorer) refreshPageRankIfStale(ctx context.Context) {
	s.cache.mu.RLock()
	fresh := time.Since(s.cache.updatedAt) < pageRankCacheTTL && len(s.cache.scores) > 0
	s.cache.mu.RUnlock()
	if fresh {
		return
	}

	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have refreshed).
	if time.Since(s.cache.updatedAt) < pageRankCacheTTL && len(s.cache.scores) > 0 {
		return
	}

	nz := s.db.NzClient()
	if nz == nil {
		log.Warn().Msg("importance: NietzscheDB client unavailable, PageRank cache not refreshed")
		return
	}

	result, err := nz.RunPageRank(ctx, pageRankCollection, pageRankDamping, pageRankMaxIter)
	if err != nil {
		log.Warn().Err(err).Msg("importance: PageRank query failed, keeping stale cache")
		return
	}

	// Find max score for normalization to [0, 1].
	var maxScore float64
	for _, ns := range result.Scores {
		if ns.Score > maxScore {
			maxScore = ns.Score
		}
	}

	scores := make(map[string]float64, len(result.Scores))
	for _, ns := range result.Scores {
		if maxScore > 0 {
			scores[ns.NodeID] = ns.Score / maxScore
		} else {
			scores[ns.NodeID] = 0
		}
	}

	s.cache.scores = scores
	s.cache.updatedAt = time.Now()

	log.Info().
		Int("nodes", len(scores)).
		Uint64("duration_ms", result.DurationMs).
		Uint32("iterations", result.Iterations).
		Msg("importance: PageRank cache refreshed")
}

// getGraphCentrality returns the PageRank-based centrality for a memory node.
// Falls back to defaultCentrality if the node is not found or PageRank failed.
func (s *Scorer) getGraphCentrality(ctx context.Context, memoryID int64) float64 {
	s.refreshPageRankIfStale(ctx)

	nodeUUID := s.db.NodeUUID("memories", memoryID)

	s.cache.mu.RLock()
	score, ok := s.cache.scores[nodeUUID]
	s.cache.mu.RUnlock()

	if !ok {
		return defaultCentrality
	}
	return score
}

// CalculateImportance computes the importance score for a memory
func (s *Scorer) CalculateImportance(ctx context.Context, memoryID int64) (*MemoryImportance, error) {
	factors := ImportanceFactors{}

	// 1. Frequency: How often this memory was accessed (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	accessLogs, err := s.db.QueryByLabel(ctx, "memory_access_log",
		" AND n.memory_id = $mid",
		map[string]interface{}{"mid": memoryID}, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query access log: %w", err)
	}

	accessCount := 0
	for _, row := range accessLogs {
		accessedAt := database.GetTime(row, "accessed_at")
		if !accessedAt.Before(thirtyDaysAgo) {
			accessCount++
		}
	}

	// Normalize: 0 accesses = 0, 10+ accesses = 1.0
	factors.Frequency = math.Min(float64(accessCount)/10.0, 1.0)

	// 2. Recency: How recent is this memory
	memory, err := s.db.GetNodeByID(ctx, "memories", memoryID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory %d: %w", memoryID, err)
	}
	if memory == nil {
		return nil, fmt.Errorf("memory %d not found", memoryID)
	}

	createdAt := database.GetTime(memory, "created_at")
	if createdAt.IsZero() {
		createdAt = time.Now() // fallback
	}

	daysSinceCreation := time.Since(createdAt).Hours() / 24.0
	// Exponential decay: recent = 1.0, 30 days ago = 0.5, 90 days = 0.1
	factors.Recency = math.Exp(-daysSinceCreation / 30.0)

	// 3. Graph Centrality: PageRank from NietzscheDB (cached, refreshed every 10 min)
	factors.GraphCentrality = s.getGraphCentrality(ctx, memoryID)

	// 4. Emotional Intensity: Extract from metadata
	emotionalIntensity := database.GetFloat64(memory, "emotional_intensity")
	factors.EmotionalIntensity = math.Min(emotionalIntensity, 1.0)

	// 5. Goal Relevance: How relevant to patient's current goals
	// TODO: Implement goal matching
	factors.GoalRelevance = 0.5

	// Weighted combination
	// Frequency: 20%, Recency: 25%, Centrality: 20%, Emotion: 20%, Goals: 15%
	totalScore := factors.Frequency*0.20 + factors.Recency*0.25 + factors.GraphCentrality*0.20 + factors.EmotionalIntensity*0.20 + factors.GoalRelevance*0.15

	return &MemoryImportance{
		MemoryID:     memoryID,
		Score:        totalScore,
		Factors:      factors,
		CalculatedAt: time.Now(),
	}, nil
}

// BatchCalculateImportance calculates importance for multiple memories
func (s *Scorer) BatchCalculateImportance(ctx context.Context, memoryIDs []int64) ([]*MemoryImportance, error) {
	// Pre-warm the PageRank cache once before processing the batch.
	s.refreshPageRankIfStale(ctx)

	results := make([]*MemoryImportance, 0, len(memoryIDs))

	for _, id := range memoryIDs {
		importance, err := s.CalculateImportance(ctx, id)
		if err != nil {
			log.Warn().Err(err).Int64("memory_id", id).Msg("Failed to calculate importance")
			continue
		}
		results = append(results, importance)
	}

	return results, nil
}

// UpdateImportanceScores updates importance scores in database
func (s *Scorer) UpdateImportanceScores(ctx context.Context, scores []*MemoryImportance) error {
	now := time.Now().Format(time.RFC3339)

	for _, score := range scores {
		err := s.db.Update(ctx, "memories",
			map[string]interface{}{"id": score.MemoryID},
			map[string]interface{}{
				"importance_score":      score.Score,
				"importance_updated_at": now,
			})
		if err != nil {
			return fmt.Errorf("failed to update importance for memory %d: %w", score.MemoryID, err)
		}
	}

	return nil
}

// GetLowImportanceMemories returns memories with low importance scores
func (s *Scorer) GetLowImportanceMemories(ctx context.Context, threshold float64, limit int) ([]int64, error) {
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	rows, err := s.db.QueryByLabel(ctx, "memories", "", nil, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to query memories: %w", err)
	}

	// Filter and collect qualifying memories
	type scoredMemory struct {
		id    int64
		score float64
	}
	var candidates []scoredMemory

	for _, row := range rows {
		score := database.GetFloat64(row, "importance_score")
		if score >= threshold {
			continue
		}

		createdAt := database.GetTime(row, "created_at")
		if createdAt.IsZero() || !createdAt.Before(sevenDaysAgo) {
			continue
		}

		candidates = append(candidates, scoredMemory{
			id:    database.GetInt64(row, "id"),
			score: score,
		})
	}

	// Sort by score ascending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score < candidates[j].score
	})

	// Apply limit
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	memoryIDs := make([]int64, len(candidates))
	for i, c := range candidates {
		memoryIDs[i] = c.id
	}

	return memoryIDs, nil
}
