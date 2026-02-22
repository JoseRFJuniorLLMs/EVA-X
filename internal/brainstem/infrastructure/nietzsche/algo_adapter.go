// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"sync"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// AlgoAdapter exposes NietzscheDB's 11 graph algorithms for EVA's intelligence layer.
type AlgoAdapter struct {
	client *Client
}

// NewAlgoAdapter creates an adapter for graph algorithm operations.
func NewAlgoAdapter(client *Client) *AlgoAdapter {
	return &AlgoAdapter{client: client}
}

// ── Composite Analysis ──────────────────────────────────────────────────────

// NetworkAnalysis is a composite result from multiple graph algorithms.
type NetworkAnalysis struct {
	PageRank    nietzsche.AlgoScoreResult
	Communities nietzsche.AlgoCommunityResult
	Bridges     nietzsche.AlgoScoreResult // betweenness centrality
}

// AnalyzeNetwork runs PageRank + Louvain + Betweenness in parallel on a collection.
// Returns a composite analysis of node importance, community structure, and bridge nodes.
func (a *AlgoAdapter) AnalyzeNetwork(ctx context.Context, collection string) (*NetworkAnalysis, error) {
	log := logger.Nietzsche()
	log.Info().Str("collection", collection).Msg("[Algo] Starting network analysis (PageRank + Louvain + Betweenness)")

	var (
		pr  nietzsche.AlgoScoreResult
		com nietzsche.AlgoCommunityResult
		bet nietzsche.AlgoScoreResult

		prErr, comErr, betErr error
		wg                    sync.WaitGroup
	)

	wg.Add(3)
	go func() {
		defer wg.Done()
		pr, prErr = a.client.RunPageRank(ctx, collection, 0.85, 100)
	}()
	go func() {
		defer wg.Done()
		com, comErr = a.client.RunLouvain(ctx, collection, 100, 1.0)
	}()
	go func() {
		defer wg.Done()
		bet, betErr = a.client.RunBetweenness(ctx, collection, 0)
	}()
	wg.Wait()

	// Log individual errors but don't fail completely
	if prErr != nil {
		log.Warn().Err(prErr).Msg("[Algo] PageRank failed")
	}
	if comErr != nil {
		log.Warn().Err(comErr).Msg("[Algo] Louvain failed")
	}
	if betErr != nil {
		log.Warn().Err(betErr).Msg("[Algo] Betweenness failed")
	}

	// Return results even if some failed (partial analysis)
	result := &NetworkAnalysis{PageRank: pr, Communities: com, Bridges: bet}
	log.Info().
		Str("collection", collection).
		Int("pr_scores", len(pr.Scores)).
		Uint64("communities", com.CommunityCount).
		Int("bridges", len(bet.Scores)).
		Msg("[Algo] Network analysis complete")
	return result, nil
}

// ── Community Detection ─────────────────────────────────────────────────────

// CommunityAnalysis holds results from multiple community detection algorithms.
type CommunityAnalysis struct {
	WeakComponents   nietzsche.AlgoCommunityResult
	StrongComponents nietzsche.AlgoCommunityResult
	LabelProp        nietzsche.AlgoCommunityResult
}

// DetectCommunities runs WCC + SCC + LabelProp to find community structure.
func (a *AlgoAdapter) DetectCommunities(ctx context.Context, collection string) (*CommunityAnalysis, error) {
	log := logger.Nietzsche()
	log.Info().Str("collection", collection).Msg("[Algo] Detecting communities (WCC + SCC + LabelProp)")

	var (
		wcc, scc, lp             nietzsche.AlgoCommunityResult
		wccErr, sccErr, lpErr    error
		wg                       sync.WaitGroup
	)

	wg.Add(3)
	go func() { defer wg.Done(); wcc, wccErr = a.client.RunWCC(ctx, collection) }()
	go func() { defer wg.Done(); scc, sccErr = a.client.RunSCC(ctx, collection) }()
	go func() { defer wg.Done(); lp, lpErr = a.client.RunLabelProp(ctx, collection, 100) }()
	wg.Wait()

	if wccErr != nil {
		log.Warn().Err(wccErr).Msg("[Algo] WCC failed")
	}
	if sccErr != nil {
		log.Warn().Err(sccErr).Msg("[Algo] SCC failed")
	}
	if lpErr != nil {
		log.Warn().Err(lpErr).Msg("[Algo] LabelProp failed")
	}

	result := &CommunityAnalysis{WeakComponents: wcc, StrongComponents: scc, LabelProp: lp}
	log.Info().
		Str("collection", collection).
		Uint64("wcc_count", wcc.CommunityCount).
		Uint64("scc_count", scc.CommunityCount).
		Uint64("lp_count", lp.CommunityCount).
		Msg("[Algo] Community detection complete")
	return result, nil
}

// ── Centrality Analysis ─────────────────────────────────────────────────────

// CentralityAnalysis holds results from centrality algorithms.
type CentralityAnalysis struct {
	Closeness nietzsche.AlgoScoreResult
	InDegree  nietzsche.AlgoScoreResult
	OutDegree nietzsche.AlgoScoreResult
	Triangles nietzsche.TriangleResult
}

// CalculateCentrality runs Closeness + Degree + TriangleCount.
func (a *AlgoAdapter) CalculateCentrality(ctx context.Context, collection string) (*CentralityAnalysis, error) {
	log := logger.Nietzsche()
	log.Info().Str("collection", collection).Msg("[Algo] Calculating centrality metrics")

	var (
		cl    nietzsche.AlgoScoreResult
		inD   nietzsche.AlgoScoreResult
		outD  nietzsche.AlgoScoreResult
		tri   nietzsche.TriangleResult
		clErr, inErr, outErr, triErr error
		wg    sync.WaitGroup
	)

	wg.Add(4)
	go func() { defer wg.Done(); cl, clErr = a.client.RunCloseness(ctx, collection) }()
	go func() { defer wg.Done(); inD, inErr = a.client.RunDegreeCentrality(ctx, collection, "in") }()
	go func() { defer wg.Done(); outD, outErr = a.client.RunDegreeCentrality(ctx, collection, "out") }()
	go func() { defer wg.Done(); tri, triErr = a.client.RunTriangleCount(ctx, collection) }()
	wg.Wait()

	if clErr != nil {
		log.Warn().Err(clErr).Msg("[Algo] Closeness failed")
	}
	if inErr != nil {
		log.Warn().Err(inErr).Msg("[Algo] InDegree failed")
	}
	if outErr != nil {
		log.Warn().Err(outErr).Msg("[Algo] OutDegree failed")
	}
	if triErr != nil {
		log.Warn().Err(triErr).Msg("[Algo] TriangleCount failed")
	}

	return &CentralityAnalysis{Closeness: cl, InDegree: inD, OutDegree: outD, Triangles: tri}, nil
}

// ── Single-purpose wrappers ─────────────────────────────────────────────────

// FindSimilar computes Jaccard similarity between nodes in a collection.
func (a *AlgoAdapter) FindSimilar(ctx context.Context, collection string, topK uint32) (nietzsche.SimilarityResult, error) {
	return a.client.RunJaccardSimilarity(ctx, collection, topK, 0.1)
}

// ShortestPath computes A* path between two nodes.
func (a *AlgoAdapter) ShortestPath(ctx context.Context, collection, fromID, toID string) (nietzsche.AStarResult, error) {
	return a.client.RunAStar(ctx, collection, fromID, toID)
}
