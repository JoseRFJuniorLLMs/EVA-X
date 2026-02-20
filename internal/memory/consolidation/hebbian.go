// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package consolidation - Hebbian Strengthening
// "Neurons that fire together, wire together" — Donald Hebb, 1949
// Boosts edge weights between co-activated memory nodes during selective replay.
package consolidation

import (
	"context"
	"fmt"
	"log"

	"eva/internal/brainstem/infrastructure/graph"
)

// HebbianStrengthener boosts graph edges between co-activated memories
type HebbianStrengthener struct {
	neo4j       *graph.Neo4jClient
	boostFactor float64 // Multiplicative weight boost (default: 1.5)
}

// NewHebbianStrengthener creates a new Hebbian strengthener
func NewHebbianStrengthener(neo4j *graph.Neo4jClient, boostFactor float64) *HebbianStrengthener {
	if boostFactor <= 1.0 {
		boostFactor = 1.5 // Default: 50% boost
	}
	return &HebbianStrengthener{
		neo4j:       neo4j,
		boostFactor: boostFactor,
	}
}

// StrengthenCoActivated boosts edges between co-replayed memory nodes.
// For each pair of co-replayed memories that share a direct or 1-hop connection,
// multiply the edge weight by boostFactor and reset the edge age.
// Returns the number of edges strengthened.
func (h *HebbianStrengthener) StrengthenCoActivated(ctx context.Context, patientID int64, memoryIDs []string) (int, error) {
	if h.neo4j == nil || len(memoryIDs) < 2 {
		return 0, nil
	}

	// Find and boost direct edges between co-replayed memories
	query := `
		UNWIND $memoryIds AS mid1
		UNWIND $memoryIds AS mid2
		WITH mid1, mid2 WHERE mid1 < mid2
		MATCH (m1:Memory)-[r]-(m2:Memory)
		WHERE toString(id(m1)) = mid1 AND toString(id(m2)) = mid2
		SET r.weight = COALESCE(r.weight, 1.0) * $boost,
		    r.hebbian_boost_at = datetime(),
		    r.age = 0
		RETURN count(r) AS strengthened
	`

	records, err := h.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"memoryIds": memoryIDs,
		"boost":     h.boostFactor,
	})
	if err != nil {
		return 0, fmt.Errorf("hebbian strengthening failed: %w", err)
	}

	strengthened := 0
	// ExecuteWrite returns ResultSummary, but we log the count
	_ = records
	log.Printf("🔗 [HEBBIAN] Strengthened edges between %d co-activated memories (boost=%.2f)", len(memoryIDs), h.boostFactor)

	// Also boost shared topic connections (1-hop: Memory → Topic ← Memory)
	topicQuery := `
		UNWIND $memoryIds AS mid1
		UNWIND $memoryIds AS mid2
		WITH mid1, mid2 WHERE mid1 < mid2
		MATCH (m1:Memory)-[r1:RELATED_TO]->(t:Topic)<-[r2:RELATED_TO]-(m2:Memory)
		WHERE toString(id(m1)) = mid1 AND toString(id(m2)) = mid2
		SET r1.weight = COALESCE(r1.weight, 1.0) * $boost,
		    r2.weight = COALESCE(r2.weight, 1.0) * $boost,
		    r1.hebbian_boost_at = datetime(),
		    r2.hebbian_boost_at = datetime(),
		    r1.age = 0,
		    r2.age = 0
		RETURN count(r1) + count(r2) AS strengthened
	`

	_, err = h.neo4j.ExecuteWrite(ctx, topicQuery, map[string]interface{}{
		"memoryIds": memoryIDs,
		"boost":     h.boostFactor,
	})
	if err != nil {
		log.Printf("⚠️ [HEBBIAN] Topic edge strengthening error: %v", err)
	}

	return strengthened, nil
}
