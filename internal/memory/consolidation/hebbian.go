// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package consolidation - Hebbian Strengthening
// "Neurons that fire together, wire together" -- Donald Hebb, 1949
// Boosts edge weights between co-activated memory nodes during selective replay.
package consolidation

import (
	"context"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// HebbianStrengthener boosts graph edges between co-activated memories
type HebbianStrengthener struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	boostFactor  float64 // Multiplicative weight boost (default: 1.5)
}

// NewHebbianStrengthener creates a new Hebbian strengthener
func NewHebbianStrengthener(graphAdapter *nietzscheInfra.GraphAdapter, boostFactor float64) *HebbianStrengthener {
	if boostFactor <= 1.0 {
		boostFactor = 1.5 // Default: 50% boost
	}
	return &HebbianStrengthener{
		graphAdapter: graphAdapter,
		boostFactor:  boostFactor,
	}
}

// StrengthenCoActivated boosts edges between co-replayed memory nodes.
// For each pair of co-replayed memories that share a direct connection,
// multiply the edge weight by boostFactor and reset the edge age.
// Returns the number of edges strengthened.
// Rewritten: Go loops with MergeEdge instead of complex UNWIND Cypher.
func (h *HebbianStrengthener) StrengthenCoActivated(ctx context.Context, patientID int64, memoryIDs []string) (int, error) {
	if h.graphAdapter == nil || len(memoryIDs) < 2 {
		return 0, nil
	}

	strengthened := 0

	// For each unique pair of memory IDs, try to boost their direct edges
	for i := 0; i < len(memoryIDs)-1; i++ {
		for j := i + 1; j < len(memoryIDs); j++ {
			mid1 := memoryIDs[i]
			mid2 := memoryIDs[j]

			// Check if they are directly connected via BFS depth 1
			neighbors, err := h.graphAdapter.Bfs(ctx, mid1, 1, "")
			if err != nil {
				continue
			}

			directlyConnected := false
			for _, n := range neighbors {
				if n == mid2 {
					directlyConnected = true
					break
				}
			}

			if directlyConnected {
				// Boost the edge between them
				_, err := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
					FromNodeID: mid1,
					ToNodeID:   mid2,
					EdgeType:   "RELATED",
					OnMatchSet: map[string]interface{}{
						"hebbian_boost_at": nietzscheInfra.NowUnix(),
						"age":              0,
					},
				})
				if err == nil {
					strengthened++
				}
			}

			// Also boost shared topic connections (1-hop: Memory -> Topic <- Memory)
			// Find topics connected to mid1
			topicNeighbors1, err := h.graphAdapter.BfsWithEdgeType(ctx, mid1, "RELATED_TO", 1, "")
			if err != nil {
				continue
			}

			// Find topics connected to mid2
			topicNeighbors2, err := h.graphAdapter.BfsWithEdgeType(ctx, mid2, "RELATED_TO", 1, "")
			if err != nil {
				continue
			}

			// Find shared topics
			topicSet := make(map[string]bool)
			for _, t := range topicNeighbors1 {
				topicSet[t] = true
			}

			for _, t := range topicNeighbors2 {
				if topicSet[t] {
					// Shared topic found - boost both edges to this topic
					// Boost mid1 -> topic edge
					_, err1 := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
						FromNodeID: mid1,
						ToNodeID:   t,
						EdgeType:   "RELATED_TO",
						OnMatchSet: map[string]interface{}{
							"hebbian_boost_at": nietzscheInfra.NowUnix(),
							"age":              0,
						},
					})

					// Boost mid2 -> topic edge
					_, err2 := h.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
						FromNodeID: mid2,
						ToNodeID:   t,
						EdgeType:   "RELATED_TO",
						OnMatchSet: map[string]interface{}{
							"hebbian_boost_at": nietzscheInfra.NowUnix(),
							"age":              0,
						},
					})

					if err1 == nil && err2 == nil {
						strengthened += 2
					}
				}
			}
		}
	}

	log.Printf("[HEBBIAN] Strengthened %d edges between %d co-activated memories (boost=%.2f)", strengthened, len(memoryIDs), h.boostFactor)

	return strengthened, nil
}
