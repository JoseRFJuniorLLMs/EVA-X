// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// fdpn_schrodinger.go — Schrödinger Edge integration for FDPN spreading activation.
//
// Migrates FDPN from fixed-weight associations (A→B = 0.7 always) to
// probabilistic edges where context collapses superposition:
//
//   A→B probability=0.7, context_boost="hospital"
//   Hospital context → collapses to ~1.0
//   Party context   → stays at base 0.7 (or decays)
//
// Observer = SituationalModulator (via situation.Situation)
//
// Strategy: gradual migration — existing fixed edges keep working;
// new associations are created as Schrödinger Edges; priming checks
// for probabilistic metadata and uses it when present.

package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"eva/internal/cortex/situation"
)

// ── Schrödinger-aware priming ───────────────────────────────────────────────

// PrimeWithContext performs spreading activation where Schrödinger edges
// are collapsed according to the current situational context.
//
// This is the probabilistic replacement for StreamingPrime — edges with
// "probability" metadata have their effective weight determined by context
// match (boost) or raw probability (no match). Edges without probability
// metadata fall through to the existing fixed-weight path.
//
// Performance budget: SchrodingerCollapse < 5ms per edge (NQL scalar read).
// We use batch NQL + local math to stay within budget for real-time priming.
func (e *FDPNEngine) PrimeWithContext(
	ctx context.Context,
	startNodeID string,
	sitCtx *situation.Situation,
	collection string,
) (*SubgraphActivation, error) {
	startTime := time.Now()

	if e.graphAdapter == nil {
		return nil, fmt.Errorf("graphAdapter is nil")
	}

	// Resolve collection
	col := collection
	if col == "" {
		col = "" // GraphAdapter will use its default
	}

	// 1. BFS from start node (same as original priming)
	neighborIDs, err := e.graphAdapter.Bfs(ctx, startNodeID, uint32(e.maxDepth), col)
	if err != nil {
		return nil, fmt.Errorf("BFS from %s failed: %w", startNodeID, err)
	}

	// 2. Build context string for Schrödinger collapse
	contextTag := buildContextTag(sitCtx)

	// 3. For each neighbor, query edges and compute activation
	var activatedNodes []ActivatedNode
	var totalEnergy float64

	for _, nid := range neighborIDs {
		if nid == startNodeID {
			continue
		}

		// Get edge metadata between startNode → neighbor via NQL
		activation, err := e.computeSchrodingerActivation(ctx, startNodeID, nid, contextTag, col)
		if err != nil {
			// Fallback: use fixed decay (existing behavior)
			level := estimateLevel(len(activatedNodes))
			activation = math.Pow(0.85, float64(level))
		}

		if activation < e.threshold {
			continue
		}

		// Fetch node data
		nodeResult, err := e.graphAdapter.GetNode(ctx, nid, col)
		if err != nil {
			continue
		}

		nome := extractNodeName(nodeResult.Content)
		nodeType := nodeResult.NodeType
		if nodeType == "" {
			nodeType = "Unknown"
		}

		node := ActivatedNode{
			ID:         nid,
			Name:       nome,
			Type:       nodeType,
			Activation: activation,
			Level:      estimateLevel(len(activatedNodes)),
			Properties: nodeResult.Content,
		}
		activatedNodes = append(activatedNodes, node)
		totalEnergy += activation
	}

	// 4. Entropy filter (same as original)
	filtered := e.filterEntropy(activatedNodes)

	// 5. Build result
	rootName := startNodeID
	rootResult, err := e.graphAdapter.GetNode(ctx, startNodeID, col)
	if err == nil {
		rootName = extractNodeName(rootResult.Content)
	}

	subgraph := &SubgraphActivation{
		RootNode:  rootName,
		Nodes:     filtered,
		Timestamp: time.Now(),
		Energy:    totalEnergy,
		Depth:     e.maxDepth,
	}

	elapsed := time.Since(startTime)
	if elapsed.Milliseconds() > 10 {
		log.Printf("[FDPN_SCHRODINGER] PrimeWithContext took %dms for %d nodes (context=%s)",
			elapsed.Milliseconds(), len(filtered), contextTag)
	}

	return subgraph, nil
}

// computeSchrodingerActivation computes the effective activation weight
// for an edge between fromID and toID.
//
// If the edge has Schrödinger metadata (probability, context_boost), we
// compute the effective probability with context. Otherwise, we return
// the edge weight directly (fixed-weight fallback).
func (e *FDPNEngine) computeSchrodingerActivation(
	ctx context.Context,
	fromID, toID string,
	contextTag string,
	collection string,
) (float64, error) {
	// Query edge between the two nodes, including Schrödinger metadata
	nql := `MATCH (a)-[r]-(b) WHERE a.id = $fromID AND b.id = $toID RETURN r.weight, r.probability, r.context_boost, r.boost_factor LIMIT 1`
	result, err := e.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"fromID": fromID,
		"toID":   toID,
	}, collection)
	if err != nil {
		return 0, fmt.Errorf("edge query failed: %w", err)
	}

	if result == nil || len(result.ScalarRows) == 0 {
		// No direct edge found — use distance-based decay (original behavior)
		return 0.85, nil // base activation for BFS-reachable nodes
	}

	row := result.ScalarRows[0]

	// Check for Schrödinger metadata
	prob, hasProbability := extractFloat(row, "r.probability")
	if !hasProbability {
		// Fixed-weight edge: use weight directly
		weight, hasWeight := extractFloat(row, "r.weight")
		if hasWeight && weight > 0 {
			return weight, nil
		}
		return 0.85, nil // default for edges with no weight
	}

	// Schrödinger edge: compute effective probability
	contextBoost, _ := extractString(row, "r.context_boost")
	boostFactor, hasBF := extractFloat(row, "r.boost_factor")
	if !hasBF {
		boostFactor = 1.5 // default from NietzscheDB schrodinger.rs
	}

	effectiveProb := prob

	// Apply context boost: if the current situation context matches the
	// edge's context_boost tag, multiply probability by boost_factor
	if contextBoost != "" && contextTag != "" {
		if strings.Contains(contextTag, contextBoost) {
			effectiveProb = math.Min(prob*boostFactor, 1.0)
			log.Printf("[FDPN_SCHRODINGER] Context boost: %s→%s prob %.2f→%.2f (context=%s, boost=%s)",
				fromID[:8], toID[:8], prob, effectiveProb, contextTag, contextBoost)
		}
	}

	return math.Max(0.0, math.Min(1.0, effectiveProb)), nil
}

// ── Schrödinger-aware association creation ───────────────────────────────────

// CreateSchrodingerAssociation creates a new probabilistic edge between
// two nodes. Used when the FDPN engine detects a new association during
// conversation — instead of a fixed weight, the edge gets probability
// metadata that allows context-dependent collapse.
//
// collapseContext is the context tag that boosts this edge (e.g., "hospital",
// "luto", "festa"). When the SituationalModulator detects that context,
// the effective probability is boosted.
func (e *FDPNEngine) CreateSchrodingerAssociation(
	ctx context.Context,
	fromNodeID, toNodeID string,
	baseProbability float64,
	collapseContext string,
	collection string,
) (string, error) {
	if e.graphAdapter == nil {
		return "", fmt.Errorf("graphAdapter is nil")
	}

	client := e.graphAdapter.Client()
	if client == nil {
		return "", fmt.Errorf("nietzsche client is nil")
	}

	edgeID, err := client.SchrodingerCreate(
		ctx,
		fromNodeID,
		toNodeID,
		"ASSOCIATES",
		collection,
		baseProbability,
		collapseContext,
	)
	if err != nil {
		return "", fmt.Errorf("SchrodingerCreate failed: %w", err)
	}

	log.Printf("[FDPN_SCHRODINGER] Created probabilistic edge %s: %s→%s (p=%.2f, ctx=%s)",
		edgeID, fromNodeID, toNodeID, baseProbability, collapseContext)

	return edgeID, nil
}

// ── Situational priming entry point ─────────────────────────────────────────

// StreamingPrimeWithSchrodinger is the full pipeline: infer situation,
// then prime with Schrödinger edge collapse. This is the recommended
// entry point for new code paths.
//
// Falls back to StreamingPrime (fixed-weight) if modulator is nil or
// situation inference fails.
func (e *FDPNEngine) StreamingPrimeWithSchrodinger(
	ctx context.Context,
	userID string,
	text string,
	recentEvents []situation.Event,
	modulator *situation.SituationalModulator,
) (map[string]*SubgraphActivation, error) {
	keywords := e.extractKeywords(text)
	if len(keywords) == 0 {
		return make(map[string]*SubgraphActivation), nil
	}

	// 1. Infer situation (observer)
	var sitCtx *situation.Situation
	if modulator != nil {
		sit, err := modulator.Infer(ctx, userID, text, recentEvents)
		if err != nil {
			log.Printf("[FDPN_SCHRODINGER] Situation inference failed: %v, using nil context", err)
		} else {
			sitCtx = &sit
		}
	}

	result := make(map[string]*SubgraphActivation)

	for _, kw := range keywords {
		// Check L1 cache first
		cacheKey := fmt.Sprintf("%s:schrodinger:%s", userID, kw)
		if cached, ok := e.localCache.Load(cacheKey); ok {
			if sg, ok := cached.(*SubgraphActivation); ok {
				result[kw] = sg
				continue
			}
		}

		// Find root node for keyword
		nql := `MATCH (n) WHERE n.nome CONTAINS $keyword OR n.content CONTAINS $keyword RETURN n LIMIT 1`
		qr, err := e.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
			"keyword": strings.ToLower(kw),
		}, "")
		if err != nil || len(qr.Nodes) == 0 {
			continue
		}

		rootID := qr.Nodes[0].ID

		// Prime with Schrödinger context
		subgraph, err := e.PrimeWithContext(ctx, rootID, sitCtx, "")
		if err != nil {
			log.Printf("[FDPN_SCHRODINGER] PrimeWithContext failed for '%s': %v, falling back", kw, err)
			// Fallback: standard priming
			if primeErr := e.primeKeyword(ctx, userID, kw); primeErr != nil {
				log.Printf("[FDPN_SCHRODINGER] Fallback priming also failed for '%s': %v", kw, primeErr)
			}
			continue
		}

		result[kw] = subgraph

		// Cache in L1 (memory)
		e.localCache.Store(cacheKey, subgraph)

		// Cache in L2 (CacheStore, 5min TTL)
		if e.cacheStore != nil {
			data, _ := json.Marshal(subgraph)
			if cacheErr := e.cacheStore.Set(context.Background(), cacheKey, string(data), 5*time.Minute); cacheErr != nil {
				log.Printf("[FDPN_SCHRODINGER] L2 cache write failed: %v", cacheErr)
			}
		}
	}

	return result, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

// buildContextTag converts a Situation into a context tag string for
// Schrödinger edge matching. The tag concatenates stressors and social
// context so that edge context_boost fields can match via substring.
//
// Example: Situation{Stressors: ["hospital", "doenca"], SocialContext: "familia"}
//   → "hospital:doenca:familia"
func buildContextTag(sit *situation.Situation) string {
	if sit == nil {
		return ""
	}
	parts := make([]string, 0, len(sit.Stressors)+1)
	parts = append(parts, sit.Stressors...)
	if sit.SocialContext != "" {
		parts = append(parts, sit.SocialContext)
	}
	return strings.Join(parts, ":")
}

// estimateLevel estimates the BFS depth level based on position in result list.
// Same heuristic as the original fdpn_engine.go.
func estimateLevel(activatedSoFar int) int {
	if activatedSoFar > 15 {
		return 3
	}
	if activatedSoFar > 5 {
		return 2
	}
	return 1
}

// extractNodeName gets a human-readable name from node content.
func extractNodeName(content map[string]interface{}) string {
	if nome, ok := content["nome"]; ok {
		return fmt.Sprintf("%v", nome)
	}
	if c, ok := content["content"]; ok {
		return fmt.Sprintf("%v", c)
	}
	return "Unnamed"
}

// extractFloat safely extracts a float64 from a scalar row map.
func extractFloat(row map[string]interface{}, key string) (float64, bool) {
	val, ok := row[key]
	if !ok || val == nil {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

// extractString safely extracts a string from a scalar row map.
func extractString(row map[string]interface{}, key string) (string, bool) {
	val, ok := row[key]
	if !ok || val == nil {
		return "", false
	}
	if s, ok := val.(string); ok {
		return s, true
	}
	return "", false
}
