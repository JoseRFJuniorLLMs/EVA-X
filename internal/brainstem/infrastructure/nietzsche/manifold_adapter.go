// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// ManifoldAdapter exposes NietzscheDB's multi-manifold operations for EVA's memory system.
// Three manifolds are available:
//   - Riemann (Poincaré ball): Synthesis/SynthesisMulti — merge concept embeddings
//   - Minkowski: CausalNeighbors/CausalChain — temporal causal reasoning
//   - Klein: KleinPath/IsOnShortestPath — decision path geodesics
type ManifoldAdapter struct {
	client *Client
}

// NewManifoldAdapter creates a multi-manifold adapter.
func NewManifoldAdapter(client *Client) *ManifoldAdapter {
	return &ManifoldAdapter{client: client}
}

// ── Riemann (Poincaré) — Memory Synthesis ───────────────────────────────────

// SynthesizeMemories computes the Riemannian midpoint of multiple memory nodes.
// The result is a "synthesized" coordinate that represents the centroid of all
// input memories in hyperbolic space — useful for creating summary memories.
func (m *ManifoldAdapter) SynthesizeMemories(ctx context.Context, nodeIDs []string, collection string) (*nietzsche.SynthesisResult, error) {
	log := logger.Nietzsche()

	if len(nodeIDs) < 2 {
		log.Warn().Int("count", len(nodeIDs)).Msg("[Manifold] SynthesisMulti needs at least 2 nodes")
		return nil, nil
	}

	result, err := m.client.SynthesisMulti(ctx, nodeIDs, collection)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Int("nodes", len(nodeIDs)).Msg("[Manifold] SynthesisMulti failed")
		return nil, err
	}

	log.Info().
		Str("collection", collection).
		Int("nodes", len(nodeIDs)).
		Str("nearest", result.NearestNodeID).
		Float64("nearest_dist", result.NearestDistance).
		Msg("[Manifold] Memory synthesis completed")
	return result, nil
}

// SynthesizePair computes the Riemannian midpoint between exactly two nodes.
func (m *ManifoldAdapter) SynthesizePair(ctx context.Context, nodeIDA, nodeIDB, collection string) (*nietzsche.SynthesisResult, error) {
	return m.client.Synthesis(ctx, nodeIDA, nodeIDB, collection)
}

// ── Minkowski — Causal Reasoning ────────────────────────────────────────────

// CausalHistory traces the causal chain of an event backwards in time.
// Returns all events in the past light-cone of the given node.
func (m *ManifoldAdapter) CausalHistory(ctx context.Context, eventNodeID, collection string) (*nietzsche.CausalChainResult, error) {
	log := logger.Nietzsche()

	result, err := m.client.CausalChain(ctx, eventNodeID, 10, "past", collection)
	if err != nil {
		log.Error().Err(err).Str("node", eventNodeID).Msg("[Manifold] CausalChain (past) failed")
		return nil, err
	}

	log.Info().
		Str("node", eventNodeID).
		Int("chain_len", len(result.ChainIDs)).
		Int("edges", len(result.Edges)).
		Msg("[Manifold] Causal history traced")
	return result, nil
}

// CausalFuture traces the causal chain of an event forwards in time.
// Returns all events in the future light-cone of the given node.
func (m *ManifoldAdapter) CausalFuture(ctx context.Context, eventNodeID, collection string) (*nietzsche.CausalChainResult, error) {
	return m.client.CausalChain(ctx, eventNodeID, 10, "future", collection)
}

// CausalNeighborsPast returns immediate past causal neighbors of a node.
func (m *ManifoldAdapter) CausalNeighborsPast(ctx context.Context, nodeID, collection string) ([]nietzsche.CausalEdge, error) {
	return m.client.CausalNeighbors(ctx, nodeID, "past", collection)
}

// CausalNeighborsFuture returns immediate future causal neighbors of a node.
func (m *ManifoldAdapter) CausalNeighborsFuture(ctx context.Context, nodeID, collection string) ([]nietzsche.CausalEdge, error) {
	return m.client.CausalNeighbors(ctx, nodeID, "future", collection)
}

// ── Klein — Decision Paths ──────────────────────────────────────────────────

// OptimalPath computes the Klein-model geodesic between two concept nodes.
// The Klein disk is the affine model of hyperbolic space, where geodesics
// are straight lines (chords). Useful for finding the most direct decision path.
func (m *ManifoldAdapter) OptimalPath(ctx context.Context, fromNodeID, toNodeID, collection string) (*nietzsche.KleinPathResult, error) {
	log := logger.Nietzsche()

	result, err := m.client.KleinPath(ctx, fromNodeID, toNodeID, collection)
	if err != nil {
		log.Error().Err(err).Str("from", fromNodeID).Str("to", toNodeID).Msg("[Manifold] KleinPath failed")
		return nil, err
	}

	log.Info().
		Str("from", fromNodeID).
		Str("to", toNodeID).
		Bool("found", result.Found).
		Int("path_len", len(result.Path)).
		Float64("cost", result.Cost).
		Msg("[Manifold] Klein optimal path computed")
	return result, nil
}

// IsConnected checks if a concept node lies on the shortest path between two others.
// Useful for testing whether a concept is a prerequisite for a decision path.
func (m *ManifoldAdapter) IsConnected(ctx context.Context, nodeIDA, nodeIDB, viaNodeID, collection string) (bool, error) {
	result, err := m.client.IsOnShortestPath(ctx, nodeIDA, nodeIDB, viaNodeID, collection)
	if err != nil {
		return false, err
	}
	return result.OnPath, nil
}
