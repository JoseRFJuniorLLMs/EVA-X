// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package consolidation - NietzscheBridge
// Bridges REM memory consolidation with NietzscheDB's multi-manifold geometry.
// Replaces Euclidean centroid averaging (abstractCommunity) with proper
// Frechet mean on the Poincare ball via SynthesisMulti, preserving
// hyperbolic hierarchy: abstract concepts near center, concrete at edges.
package consolidation

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// NietzscheBridge connects REM consolidation to NietzscheDB's multi-manifold
// operations. Instead of computing Euclidean centroids (which destroy
// hyperbolic hierarchy), it delegates to SynthesisMulti for Frechet mean
// on the Poincare ball.
//
// Geometry rationale:
//   - Euclidean average: centroid falls at arbitrary point, norm is meaningless
//   - Poincare Frechet mean: centroid respects ||x|| = depth in the disk
//     (0 = center = most abstract, ~1 = edge = most concrete)
//
// Reference: Lou et al. (2020) "Differentiating through the Frechet Mean" (ICML)
type NietzscheBridge struct {
	manifold *nietzscheInfra.ManifoldAdapter
	graph    *nietzscheInfra.GraphAdapter
	client   *nietzscheInfra.Client
}

// SynthesizedConcept is the result of computing a Frechet mean over a
// community of episodic memories in hyperbolic space.
type SynthesizedConcept struct {
	// NodeID is the UUID of the synthesized semantic node (if created).
	NodeID string

	// PoincareCoords are the Frechet mean coordinates in the Poincare ball.
	// These are NOT Euclidean averages -- they respect hyperbolic geometry.
	PoincareCoords []float64

	// NearestNode is the UUID of the existing node closest to the synthesis point.
	NearestNode string

	// NearestDistance is the hyperbolic distance from the synthesis to the nearest node.
	NearestDistance float64

	// Depth is ||x|| in the Poincare disk: 0 = center (most abstract), ~1 = edge (concrete).
	// This is the key advantage over Euclidean: depth encodes abstraction level.
	Depth float64
}

// NewNietzscheBridge creates a bridge from REM consolidation to NietzscheDB manifolds.
func NewNietzscheBridge(manifold *nietzscheInfra.ManifoldAdapter,
	graph *nietzscheInfra.GraphAdapter, client *nietzscheInfra.Client) *NietzscheBridge {
	return &NietzscheBridge{
		manifold: manifold,
		graph:    graph,
		client:   client,
	}
}

// SynthesizeCommunity computes the Frechet mean of a community of episodic
// memories using NietzscheDB's Riemannian SynthesisMulti operation.
//
// Unlike abstractCommunity() which does sum(embeddings)/N + normalize (Euclidean),
// this delegates to the server's Frechet mean on the Poincare ball, ensuring:
//   - The centroid's ||x|| reflects true abstraction depth
//   - Abstract concepts naturally migrate toward the disk center
//   - Concrete memories stay near the boundary
//
// Returns a SynthesizedConcept with Poincare coords, depth, and nearest node info.
func (nb *NietzscheBridge) SynthesizeCommunity(ctx context.Context,
	memoryNodeIDs []string, collection string) (*SynthesizedConcept, error) {

	if len(memoryNodeIDs) < 2 {
		return nil, fmt.Errorf("SynthesizeCommunity requires at least 2 nodes, got %d", len(memoryNodeIDs))
	}

	// Delegate to ManifoldAdapter.SynthesizeMemories which calls SynthesisMulti
	// on the server. The server computes the Frechet mean on the Poincare ball
	// (iterative Riemannian gradient descent), NOT Euclidean average.
	result, err := nb.manifold.SynthesizeMemories(ctx, memoryNodeIDs, collection)
	if err != nil {
		return nil, fmt.Errorf("SynthesisMulti failed for %d nodes: %w", len(memoryNodeIDs), err)
	}
	if result == nil {
		return nil, fmt.Errorf("SynthesisMulti returned nil for %d nodes", len(memoryNodeIDs))
	}

	// Compute depth = ||x|| in the Poincare ball.
	// This is the L2 norm of the synthesis coordinates.
	// In hyperbolic geometry: 0 = origin (most abstract), approaching 1 = boundary (concrete).
	depth := poincareNorm(result.SynthesisCoords)

	concept := &SynthesizedConcept{
		PoincareCoords:  result.SynthesisCoords,
		NearestNode:     result.NearestNodeID,
		NearestDistance:  result.NearestDistance,
		Depth:           depth,
	}

	log.Printf("[NietzscheBridge] Synthesized community: %d memories -> depth=%.4f, nearest=%s (dist=%.4f)",
		len(memoryNodeIDs), depth, result.NearestNodeID, result.NearestDistance)

	return concept, nil
}

// SynthesizeCommunityFromMemories is a convenience method that extracts node IDs
// from EpisodicMemory structs and calls SynthesizeCommunity.
// It also populates the SynthesizedConcept with a generated NodeID and selects
// exemplars (top 3 by activation score) for backward compatibility with ProtoConcept.
func (nb *NietzscheBridge) SynthesizeCommunityFromMemories(ctx context.Context,
	comm []EpisodicMemory, collection string) (*SynthesizedConcept, *ProtoConcept, error) {

	if len(comm) < 2 {
		return nil, nil, fmt.Errorf("need at least 2 memories, got %d", len(comm))
	}

	// Extract node IDs
	nodeIDs := make([]string, 0, len(comm))
	for _, mem := range comm {
		if mem.ID != "" {
			nodeIDs = append(nodeIDs, mem.ID)
		}
	}

	if len(nodeIDs) < 2 {
		return nil, nil, fmt.Errorf("only %d valid node IDs in community of %d", len(nodeIDs), len(comm))
	}

	// Compute Frechet mean via NietzscheDB
	synth, err := nb.SynthesizeCommunity(ctx, nodeIDs, collection)
	if err != nil {
		return nil, nil, err
	}

	// Generate semantic node ID
	synth.NodeID = fmt.Sprintf("sem_%d", time.Now().UnixNano())

	// Build a ProtoConcept for backward compatibility with createSemanticNode
	concept := &ProtoConcept{
		Centroid:         synth.PoincareCoords,
		MemberCount:      len(comm),
		AbstractionLevel: depthToAbstractionLevel(synth.Depth),
	}

	// Select top 3 exemplars (same logic as abstractCommunity)
	maxExemplars := 3
	if len(comm) < maxExemplars {
		maxExemplars = len(comm)
	}
	for i := 0; i < maxExemplars; i++ {
		concept.ExemplarIDs = append(concept.ExemplarIDs, comm[i].ID)
	}

	// Label from most activated exemplar
	if len(comm) > 0 && comm[0].Content != "" {
		label := comm[0].Content
		if len(label) > 100 {
			label = label[:100]
		}
		concept.Label = label
	}

	return synth, concept, nil
}

// ClassifyManifold determines which geometric manifold best fits a set of memories.
//
// Heuristics:
//   - Sequential timestamps + causal mentions -> "minkowski" (temporal causality)
//   - IS_A / taxonomic / hierarchical relations -> "poincare" (tree hierarchy)
//   - Default: "poincare" (hyperbolic is the safest general-purpose choice)
//
// This classification can be used to route future operations on these memories
// to the appropriate manifold adapter.
func (nb *NietzscheBridge) ClassifyManifold(ctx context.Context, memories []EpisodicMemory) string {
	if len(memories) == 0 {
		return "poincare"
	}

	// Heuristic 1: check for sequential timestamps with causal language
	sequentialCount := 0
	causalMentions := 0

	causalKeywords := []string{
		"because", "caused", "led to", "resulted in", "therefore",
		"consequently", "due to", "as a result", "triggered",
		"porque", "causou", "levou a", "resultou em", "portanto",
		"consequentemente", "devido a", "como resultado",
	}

	var prevTime time.Time
	for _, mem := range memories {
		// Check temporal sequencing
		if !mem.CreatedAt.IsZero() && !prevTime.IsZero() {
			diff := mem.CreatedAt.Sub(prevTime)
			if diff > 0 && diff < 24*time.Hour {
				sequentialCount++
			}
		}
		prevTime = mem.CreatedAt

		// Check causal language in content
		contentLower := strings.ToLower(mem.Content)
		for _, keyword := range causalKeywords {
			if strings.Contains(contentLower, keyword) {
				causalMentions++
				break // count each memory only once
			}
		}
	}

	// Heuristic 2: check for taxonomic / IS_A / hierarchical patterns
	taxonomicKeywords := []string{
		"is a", "type of", "kind of", "category", "subclass",
		"instance of", "belongs to", "classified as", "taxonomy",
		"e um", "tipo de", "classe de", "categoria", "subclasse",
		"instancia de", "pertence a", "classificado como",
	}

	taxonomicCount := 0
	for _, mem := range memories {
		contentLower := strings.ToLower(mem.Content)
		for _, keyword := range taxonomicKeywords {
			if strings.Contains(contentLower, keyword) {
				taxonomicCount++
				break
			}
		}
	}

	totalMemories := len(memories)

	// Decision logic:
	// If >40% of memories have sequential timestamps AND >20% mention causality -> Minkowski
	sequentialRatio := float64(sequentialCount) / float64(totalMemories)
	causalRatio := float64(causalMentions) / float64(totalMemories)

	if sequentialRatio > 0.4 && causalRatio > 0.2 {
		log.Printf("[NietzscheBridge] ClassifyManifold: minkowski (sequential=%.0f%%, causal=%.0f%%)",
			sequentialRatio*100, causalRatio*100)
		return "minkowski"
	}

	// If >30% of memories mention taxonomic relations -> Poincare (explicit)
	taxonomicRatio := float64(taxonomicCount) / float64(totalMemories)
	if taxonomicRatio > 0.3 {
		log.Printf("[NietzscheBridge] ClassifyManifold: poincare (taxonomic=%.0f%%)",
			taxonomicRatio*100)
		return "poincare"
	}

	// Default: Poincare (hyperbolic hierarchy is the safest general assumption
	// for memory consolidation, where abstract concepts should be near the center)
	return "poincare"
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// poincareNorm computes the L2 norm of a point in the Poincare ball.
// This is ||x|| which represents depth: 0 = center (abstract), ~1 = boundary (concrete).
func poincareNorm(coords []float64) float64 {
	if len(coords) == 0 {
		return 0
	}
	sumSq := 0.0
	for _, v := range coords {
		sumSq += v * v
	}
	return math.Sqrt(sumSq)
}

// depthToAbstractionLevel maps Poincare ball depth to a discrete abstraction level.
// Depth 0.0-0.3 = level 3 (highly abstract, near center)
// Depth 0.3-0.6 = level 2 (moderately abstract)
// Depth 0.6-0.9 = level 1 (concrete)
// Depth >0.9    = level 0 (very concrete, near boundary)
func depthToAbstractionLevel(depth float64) int {
	switch {
	case depth < 0.3:
		return 3
	case depth < 0.6:
		return 2
	case depth < 0.9:
		return 1
	default:
		return 0
	}
}
