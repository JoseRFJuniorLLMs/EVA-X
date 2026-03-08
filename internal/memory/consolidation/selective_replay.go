// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package consolidation - Selective Replay Consolidation (SRC)
// Reference: Tadros et al. "Sleep-like Unsupervised Replay Reduces Catastrophic Forgetting"
//            Nature Communications, 2022
// Prioritizes high-activation + low-coherence (dissonant) memories for replay,
// preventing catastrophic forgetting of edge cases and contradictions.
package consolidation

import (
	"context"
	"log"
	"math"
	"sort"
	"time"

	"gonum.org/v1/gonum/mat"
)

// SelectiveReplayConfig configures the SRC algorithm
type SelectiveReplayConfig struct {
	MaxReplayBudget    int     // Maximum memories to replay per cycle (default: 100)
	DissonanceThreshold float64 // Minimum dissonance to be eligible (default: 0.1)
	HebbianBoostFactor float64 // Weight boost for co-activated edges (default: 1.5)
	KNeighbors         int     // Number of neighbors for coherence calculation (default: 5)
}

// DefaultSelectiveReplayConfig returns sensible defaults
func DefaultSelectiveReplayConfig() *SelectiveReplayConfig {
	return &SelectiveReplayConfig{
		MaxReplayBudget:    100,
		DissonanceThreshold: 0.1,
		HebbianBoostFactor: 1.5,
		KNeighbors:         5,
	}
}

// DissonanceScore holds the dissonance analysis for a single memory
type DissonanceScore struct {
	MemoryID   string  `json:"memory_id"`
	Activation float64 `json:"activation"`    // Original activation score
	Coherence  float64 `json:"coherence"`     // Avg similarity to K nearest neighbors (0-1)
	Dissonance float64 `json:"dissonance"`    // activation × (1 - coherence)
}

// ReplayResult holds the result of a selective replay cycle
type ReplayResult struct {
	TotalMemories     int           `json:"total_memories"`
	DissonantCount    int           `json:"dissonant_count"`
	ReplayedCount     int           `json:"replayed_count"`
	HebbianEdges      int           `json:"hebbian_edges_strengthened"`
	AvgDissonance     float64       `json:"avg_dissonance"`
	MaxDissonance     float64       `json:"max_dissonance"`
	Duration          time.Duration `json:"duration"`
}

// ScoNietzscheDBsonance computes dissonance for each memory.
// dissonance = activation × (1 - coherence)
// where coherence = average cosine similarity to K nearest neighbors.
// High dissonance = memory is highly activated but doesn't fit well with its neighbors.
func ScoNietzscheDBsonance(memories []EpisodicMemory, kNeighbors int) []DissonanceScore {
	if len(memories) == 0 {
		return nil
	}

	if kNeighbors <= 0 {
		kNeighbors = 5
	}
	if kNeighbors >= len(memories) {
		kNeighbors = len(memories) - 1
	}
	if kNeighbors == 0 {
		kNeighbors = 1
	}

	scores := make([]DissonanceScore, len(memories))

	for i, mem := range memories {
		scores[i].MemoryID = mem.ID
		scores[i].Activation = mem.ActivationScore

		if len(mem.Embedding) == 0 {
			scores[i].Coherence = 0.5 // neutral if no embedding
			scores[i].Dissonance = mem.ActivationScore * 0.5
			continue
		}

		// Compute similarity to all other memories
		type simPair struct {
			idx int
			sim float64
		}
		var sims []simPair
		for j, other := range memories {
			if i == j || len(other.Embedding) == 0 {
				continue
			}
			sim := cosineSim(mem.Embedding, other.Embedding)
			sims = append(sims, simPair{j, sim})
		}

		// Sort by similarity DESC, take top K
		sort.Slice(sims, func(a, b int) bool {
			return sims[a].sim > sims[b].sim
		})

		k := kNeighbors
		if k > len(sims) {
			k = len(sims)
		}

		var coherenceSum float64
		for n := 0; n < k; n++ {
			coherenceSum += sims[n].sim
		}

		if k > 0 {
			scores[i].Coherence = coherenceSum / float64(k)
		}

		scores[i].Dissonance = scores[i].Activation * (1.0 - scores[i].Coherence)
	}

	// Sort by dissonance DESC
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Dissonance > scores[j].Dissonance
	})

	return scores
}

// SelectTopDissonant returns the top N most dissonant memories within budget
func SelectTopDissonant(scores []DissonanceScore, budget int, threshold float64) []DissonanceScore {
	var selected []DissonanceScore
	for _, s := range scores {
		if s.Dissonance < threshold {
			continue
		}
		selected = append(selected, s)
		if len(selected) >= budget {
			break
		}
	}
	return selected
}

// ExecuteSelectiveReplay runs the full SRC cycle:
// 1. Score dissonance for all memories
// 2. Select top dissonant within budget
// 3. Replay through Krylov subspace
// 4. Apply Hebbian strengthening
func (r *REMConsolidator) ExecuteSelectiveReplay(
	ctx context.Context,
	patientID int64,
	memories []EpisodicMemory,
	cfg *SelectiveReplayConfig,
	hebbian *HebbianStrengthener,
) *ReplayResult {
	start := time.Now()
	result := &ReplayResult{TotalMemories: len(memories)}

	if cfg == nil {
		cfg = DefaultSelectiveReplayConfig()
	}

	// 1. Score dissonance
	scores := ScoNietzscheDBsonance(memories, cfg.KNeighbors)
	result.DissonantCount = len(scores)

	if len(scores) > 0 {
		result.MaxDissonance = scores[0].Dissonance
		var totalDiss float64
		for _, s := range scores {
			totalDiss += s.Dissonance
		}
		result.AvgDissonance = totalDiss / float64(len(scores))
	}

	// 2. Select within budget
	selected := SelectTopDissonant(scores, cfg.MaxReplayBudget, cfg.DissonanceThreshold)

	// 3. Replay: re-process dissonant memories through Krylov (prioritized)
	memoryMap := make(map[string]EpisodicMemory, len(memories))
	for _, m := range memories {
		memoryMap[m.ID] = m
	}

	var replayedIDs []string
	for _, s := range selected {
		mem, ok := memoryMap[s.MemoryID]
		if !ok || len(mem.Embedding) == 0 {
			continue
		}

		if r.krylov != nil {
			_ = r.krylov.UpdateSubspace(mem.Embedding)
		}
		replayedIDs = append(replayedIDs, s.MemoryID)
		result.ReplayedCount++
	}

	// 4. Hebbian strengthening: boost edges between co-replayed memories
	if hebbian != nil && len(replayedIDs) >= 2 {
		strengthened, err := hebbian.StrengthenCoActivated(ctx, patientID, replayedIDs)
		if err != nil {
			log.Printf("⚠️ [SRC] Hebbian strengthening error: %v", err)
		} else {
			result.HebbianEdges = strengthened
		}
	}

	result.Duration = time.Since(start)

	log.Printf("🧠 [SRC] Patient %d: %d/%d dissonant, %d replayed, %d Hebbian edges (%.2f avg dissonance) in %v",
		patientID, result.DissonantCount, result.TotalMemories,
		result.ReplayedCount, result.HebbianEdges,
		result.AvgDissonance, result.Duration)

	return result
}

// cosineSim computes cosine similarity between two float64 vectors
func cosineSim(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	va := mat.NewVecDense(len(a), a)
	vb := mat.NewVecDense(len(b), b)

	dot := mat.Dot(va, vb)
	normA := mat.Norm(va, 2)
	normB := mat.Norm(vb, 2)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}

// cosineSim32 computes cosine similarity between two float32 vectors
func cosineSim32(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}
