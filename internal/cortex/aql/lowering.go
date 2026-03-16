// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AQL Lowering — converts AQL Statements to NietzscheDB gRPC calls.
// Each verb is lowered to one or more SDK operations with automatic side-effects.

package aql

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"github.com/rs/zerolog/log"

	nietzsche "nietzsche-sdk"
)

// ── searchNodes — core search without side-effects ───────────────────
// Used internally by ORBIT, RESONATE, DESCEND, ASCEND, IMAGINE to find
// seed nodes without spawning energy-boost goroutines.
//
// TODO(nql-rewrite): searchNodes uses raw SDK KnnSearch/FullTextSearch which
// bypass the NQL RewriteNQL() rewriter. This is fine for KNN/FTS (they search
// by vector/text, not by node type label), but if NQL queries are added here
// in the future, they MUST be passed through nietzscheInfra.RewriteNQL() so
// custom node types (e.g. "User") are rewritten to Semantic + node_label filter.

func (e *Executor) searchNodes(ctx context.Context, query string, collection string, k int) ([]CognitiveNode, error) {
	var nodes []CognitiveNode

	// Primary: KNN vector search (if embedding function available)
	if e.embedFunc != nil && query != "" {
		vec32, err := e.embedFunc(ctx, query)
		if err != nil {
			log.Warn().Err(err).Msg("[AQL/search] embedFunc failed, falling back to FTS")
		} else {
			vec64 := float32ToFloat64(vec32)
			knnResults, knnErr := e.client.KnnSearch(ctx, vec64, uint32(k), collection)
			if knnErr != nil {
				return nil, fmt.Errorf("searchNodes KNN: %w", knnErr)
			}
			for _, r := range knnResults {
				nodes = append(nodes, CognitiveNode{
					ID:       r.ID,
					Metadata: map[string]interface{}{"distance": r.Distance},
				})
			}
		}
	}

	// Fallback: full-text search
	if len(nodes) == 0 && query != "" {
		ftResults, ftErr := e.client.FullTextSearch(ctx, query, collection, uint32(k))
		if ftErr != nil {
			return nil, fmt.Errorf("searchNodes FTS: %w", ftErr)
		}
		for _, r := range ftResults {
			nodes = append(nodes, CognitiveNode{
				ID:       r.NodeID,
				Metadata: map[string]interface{}{"fts_score": r.Score},
			})
		}
	}

	if nodes == nil {
		nodes = []CognitiveNode{}
	}
	return nodes, nil
}

// ── RECALL — semantic search (KNN + full-text fallback) ──────────────

func (e *Executor) executeRecall(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)
	limit := stmt.Limit
	if limit <= 0 {
		limit = 10
	}

	searchResults, err := e.searchNodes(ctx, stmt.Query, col, limit)
	if err != nil {
		return nil, err
	}

	// Hydrate nodes with full data (Content, Energy, NodeType) via parallel GetNode.
	// searchNodes only returns {ID, Metadata{distance}} — insufficient for display.
	nodes := make([]CognitiveNode, len(searchResults))
	if len(searchResults) > 0 {
		recallSem := make(chan struct{}, 10)
		var recallWg sync.WaitGroup
		for i, sn := range searchResults {
			recallWg.Add(1)
			recallSem <- struct{}{}
			go func(idx int, nodeID string, meta map[string]interface{}) {
				defer recallWg.Done()
				defer func() { <-recallSem }()
				cn := CognitiveNode{ID: nodeID, Metadata: meta}
				if nr, getErr := e.client.GetNode(ctx, nodeID, col); getErr == nil && nr.Found {
					cn.Content = extractContentString(nr.Content)
					cn.Energy = nr.Energy
					cn.NodeType = nr.NodeType
				}
				nodes[idx] = cn
			}(i, sn.ID, sn.Metadata)
		}
		recallWg.Wait()
	}

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	// Side-effect: boost energy of accessed nodes (async, non-blocking)
	sideEffects := []string{"BoostAccessedNodes"}
	nodeIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.ID != "" {
			nodeIDs = append(nodeIDs, n.ID)
		}
	}
	if len(nodeIDs) > 0 {
		go func(ids []string, collection string) {
			defer func() {
				if r := recover(); r != nil {
					log.Warn().Interface("panic", r).Msg("[AQL/RECALL] energy boost panic recovered")
				}
			}()
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, id := range ids {
				nr, getErr := e.client.GetNode(bgCtx, id, collection)
				if getErr != nil {
					log.Warn().Err(getErr).Str("node_id", id).Msg("[AQL/RECALL] GetNode failed")
					continue
				}
				if nr.Found {
					newEnergy := nr.Energy + 0.03
					if newEnergy > 1.0 {
						newEnergy = 1.0
					}
					if err := e.client.UpdateEnergy(bgCtx, id, newEnergy, collection); err != nil {
						log.Warn().Err(err).Str("node_id", id).Msg("[AQL/RECALL] UpdateEnergy failed")
					}
				}
			}
		}(nodeIDs, col)
	}

	result := CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: sideEffects},
	}
	return &result, nil
}

// ── RESONATE — wave diffusion / activation spreading ────────────────

func (e *Executor) executeResonate(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Find the seed node via lightweight search (no side-effects)
	seeds, err := e.searchNodes(ctx, stmt.Query, col, 1)
	if err != nil || len(seeds) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := seeds[0].ID
	depth := uint32(3)
	if stmt.Depth > 0 {
		depth = uint32(stmt.Depth)
	}

	// BFS from seed — returns visited node IDs
	visitedIDs, err := e.client.Bfs(ctx, seedID, nietzsche.TraversalOpts{MaxDepth: depth}, col)
	if err != nil {
		return nil, fmt.Errorf("RESONATE BFS failed: %w", err)
	}

	// Hydrate visited nodes with full data (parallel, bounded)
	type hydrateResult struct {
		idx  int
		node CognitiveNode
	}
	sem := make(chan struct{}, 10)
	ch := make(chan hydrateResult, len(visitedIDs))
	for i, id := range visitedIDs {
		sem <- struct{}{}
		go func(idx int, nodeID string) {
			defer func() { <-sem }()
			cn := CognitiveNode{ID: nodeID}
			if nr, getErr := e.client.GetNode(ctx, nodeID, col); getErr == nil && nr.Found {
				cn.Content = extractContentString(nr.Content)
				cn.Energy = nr.Energy
			}
			ch <- hydrateResult{idx: idx, node: cn}
		}(i, id)
	}
	nodes := make([]CognitiveNode, len(visitedIDs))
	for range visitedIDs {
		r := <-ch
		nodes[r.idx] = r.node
	}

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	// Side-effect: create a pattern node linking resonating nodes (async, non-blocking)
	sideEffects := []string{"RecordResonancePattern"}
	if len(nodes) > 1 {
		sideEffects = append(sideEffects, "CreatePatternNode")
		resonantIDs := make([]string, 0, len(nodes))
		for _, n := range nodes {
			if n.ID != "" {
				resonantIDs = append(resonantIDs, n.ID)
			}
		}
		go func(ids []string, collection string, query string) {
			defer func() {
				if r := recover(); r != nil {
					log.Warn().Interface("panic", r).Msg("[AQL/RESONATE] pattern node panic recovered")
				}
			}()
			patternCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Create a semantic node that represents the resonance pattern
			patternContent := map[string]interface{}{
				"node_label":    "Pattern",
				"description":   fmt.Sprintf("Resonance pattern: %s", query),
				"source_count":  len(ids),
				"pattern_type":  "resonance",
			}
			patternResult, insertErr := e.client.InsertNode(patternCtx, nietzsche.InsertNodeOpts{
				Content:    patternContent,
				Energy:     0.4,
				NodeType:   "Semantic",
				Collection: collection,
			})
			if insertErr != nil {
				log.Warn().Err(insertErr).Msg("[AQL/RESONATE] CreatePatternNode InsertNode failed")
				return
			}

			// Link pattern node to source nodes (limit to first 10 to avoid flooding)
			linkLimit := 10
			if len(ids) < linkLimit {
				linkLimit = len(ids)
			}
			for _, srcID := range ids[:linkLimit] {
				_, linkErr := e.client.InsertEdge(patternCtx, nietzsche.InsertEdgeOpts{
					From:       patternResult.ID,
					To:         srcID,
					EdgeType:   "RESONANCE_SOURCE",
					Weight:     1.0,
					Collection: collection,
				})
				if linkErr != nil {
					log.Warn().Err(linkErr).Str("to", srcID).Msg("[AQL/RESONATE] LinkSourceEpisodes InsertEdge failed")
				}
			}
		}(resonantIDs, col, stmt.Query)
	}

	return &CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: sideEffects},
	}, nil
}

// ── REFLECT — introspection / synthesis ──────────────────────────────

func (e *Executor) executeReflect(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// If node IDs provided, synthesize them
	if len(stmt.NodeIDs) >= 2 {
		synthResult, err := e.client.SynthesisMulti(ctx, stmt.NodeIDs, col)
		if err != nil {
			return nil, fmt.Errorf("REFLECT synthesis failed: %w", err)
		}
		if synthResult != nil {
			cn := CognitiveNode{
				ID:       synthResult.NearestNodeID,
				Metadata: map[string]interface{}{"nearest_distance": synthResult.NearestDistance, "synthesis_coords": synthResult.SynthesisCoords},
			}
			// Hydrate synthesis node with Content/Energy
			if nr, getErr := e.client.GetNode(ctx, synthResult.NearestNodeID, col); getErr == nil && nr.Found {
				cn.Content = extractContentString(nr.Content)
				cn.Energy = nr.Energy
				cn.NodeType = nr.NodeType
			}
			return &CognitiveResult{Nodes: []CognitiveNode{cn}}, nil
		}
	}

	// Default: get collection stats as self-reflection
	stats, err := e.client.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("REFLECT stats failed: %w", err)
	}

	return &CognitiveResult{
		Nodes: []CognitiveNode{{
			Content:  fmt.Sprintf("Collection %s: nodes=%d edges=%d", col, stats.NodeCount, stats.EdgeCount),
			Metadata: map[string]interface{}{"node_count": stats.NodeCount, "edge_count": stats.EdgeCount},
		}},
	}, nil
}

// ── TRACE — path finding (Dijkstra) ─────────────────────────────────

func (e *Executor) executeTrace(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if stmt.From == "" || stmt.To == "" {
		return nil, fmt.Errorf("TRACE requires 'from' and 'to' node IDs")
	}

	// Dijkstra from start — returns visited IDs and costs
	visitedIDs, costs, err := e.client.Dijkstra(ctx, stmt.From, nietzsche.TraversalOpts{MaxDepth: 20}, col)
	if err != nil {
		return nil, fmt.Errorf("TRACE Dijkstra failed: %w", err)
	}

	// Build cost lookup for visited nodes
	costOf := make(map[string]float64, len(visitedIDs))
	targetFound := false
	for i, id := range visitedIDs {
		if i < len(costs) {
			costOf[id] = costs[i]
		}
		if id == stmt.To {
			targetFound = true
		}
	}

	if !targetFound {
		// Target not reachable — return empty path
		return &CognitiveResult{
			Nodes:    []CognitiveNode{},
			Metadata: ResultMetadata{SideEffects: []string{}},
		}, nil
	}

	// Reconstruct actual shortest path by backtracking from target to source.
	// At each step, find the neighbor (via BFS depth=1) with the lowest Dijkstra cost.
	path := []string{stmt.To}
	current := stmt.To
	visited := map[string]bool{stmt.To: true}
	for current != stmt.From {
		// Use CausalNeighbors("past") to follow only predecessors (incoming edges),
		// not BFS which returns both in+out neighbors and can backtrack incorrectly.
		causalEdges, causalErr := e.client.CausalNeighbors(ctx, current, "past", col)
		if causalErr != nil {
			// Fallback to BFS depth=1 if CausalNeighbors not available
			bfsNeighbors, bfsErr := e.client.Bfs(ctx, current, nietzsche.TraversalOpts{MaxDepth: 1}, col)
			if bfsErr != nil {
				break
			}
			causalEdges = nil // signal to use BFS fallback
			bestID := ""
			bestCost := math.MaxFloat64
			for _, nID := range bfsNeighbors {
				if nID == current || visited[nID] {
					continue
				}
				c, ok := costOf[nID]
				if !ok {
					continue
				}
				if c < bestCost {
					bestCost = c
					bestID = nID
				}
			}
			if bestID == "" {
				break
			}
			visited[bestID] = true
			// Prepend: backtrack goes target→source, path must be source→target
			path = append([]string{bestID}, path...)
			current = bestID
			continue
		}
		bestID := ""
		bestCost := math.MaxFloat64
		for _, edge := range causalEdges {
			nID := edge.FromNodeID // predecessor
			if nID == current || visited[nID] {
				continue
			}
			c, ok := costOf[nID]
			if !ok {
				continue
			}
			if c < bestCost {
				bestCost = c
				bestID = nID
			}
		}
		if bestID == "" {
			break // no progress, path broken
		}
		path = append([]string{bestID}, path...)
		visited[bestID] = true
		current = bestID
	}

	// Build result from reconstructed path — hydrate with Content/Energy
	var nodes []CognitiveNode
	for _, id := range path {
		cost := costOf[id]
		cn := CognitiveNode{
			ID:       id,
			Metadata: map[string]interface{}{"cost": cost},
		}
		if nr, getErr := e.client.GetNode(ctx, id, col); getErr == nil && nr.Found {
			cn.Content = extractContentString(nr.Content)
			cn.Energy = nr.Energy
			cn.NodeType = nr.NodeType
		}
		nodes = append(nodes, cn)
	}

	// Side-effect: boost all nodes on path (async, non-blocking)
	pathIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.ID != "" {
			pathIDs = append(pathIDs, n.ID)
		}
	}
	if len(pathIDs) > 0 {
		go func(ids []string, collection string) {
			defer func() {
				if r := recover(); r != nil {
					log.Warn().Interface("panic", r).Msg("[AQL/TRACE] energy boost panic recovered")
				}
			}()
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, id := range ids {
				nr, getErr := e.client.GetNode(bgCtx, id, collection)
				if getErr != nil {
					log.Warn().Err(getErr).Str("node_id", id).Msg("[AQL/TRACE] GetNode failed")
					continue
				}
				if nr.Found {
					newEnergy := nr.Energy + 0.02
					if newEnergy > 1.0 {
						newEnergy = 1.0
					}
					if err := e.client.UpdateEnergy(bgCtx, id, newEnergy, collection); err != nil {
						log.Warn().Err(err).Str("node_id", id).Msg("[AQL/TRACE] UpdateEnergy failed")
					}
				}
			}
		}(pathIDs, col)
	}

	// TODO(side-effect): TRACE could create an episodic "path traversed" node recording
	// the full path from source to target, enabling EVA to remember which routes it explored.
	// When implemented, spawn a goroutine with timeout and panic recovery.

	return &CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: []string{"BoostPathNodes"}},
	}, nil
}

// ── IMPRINT — write new knowledge ───────────────────────────────────

func (e *Executor) executeImprint(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if stmt.Content == "" {
		return nil, fmt.Errorf("IMPRINT requires 'content'")
	}

	// Determine node type and energy from epistemic type
	nodeType := "Semantic"
	energy := float32(0.5)
	if stmt.Epistemic != "" {
		nodeType = stmt.Epistemic.NietzscheNodeType()
		energy = stmt.Epistemic.InitialEnergy()
	}
	if stmt.Energy > 0 {
		energy = stmt.Energy
	}

	// Build content map
	contentMap := map[string]interface{}{
		"content":        stmt.Content,
		"epistemic_type": string(stmt.Epistemic),
	}
	for k, v := range stmt.Metadata {
		contentMap[k] = v
	}

	// Normalize node type: custom types (e.g. "User", "Clinic") become "Semantic"
	// with node_label injected into content. This matches the NQL rewriter convention
	// so queries via MATCH (n:User) → MATCH (n:Semantic) WHERE n.node_label = "User"
	// will find nodes inserted through AQL IMPRINT.
	nodeType, contentMap = nietzscheInfra.NormalizeContent(nodeType, contentMap)

	// Generate embedding for coordinates
	var coords []float64
	if e.embedFunc != nil {
		vec32, err := e.embedFunc(ctx, stmt.Content)
		if err != nil {
			log.Warn().Err(err).Msg("[AQL/IMPRINT] embedFunc failed, node will have zero coords")
		} else {
			coords = float32ToFloat64(vec32)
			// Project into Poincare ball at magnitude 0.5
			coords = projectToPoincare(coords, 0.5)
		}
	}

	insertResult, err := e.client.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Content:    contentMap,
		Coords:     coords,
		Energy:     energy,
		NodeType:   nodeType,
		Collection: col,
	})
	if err != nil {
		return nil, fmt.Errorf("IMPRINT InsertNode failed: %w", err)
	}

	sideEffects := []string{"AssociateToSessionContext"}

	// Auto-link if LinkTo specified
	if stmt.LinkTo != "" {
		edgeType := "ASSOCIATED"
		if stmt.EdgeType != "" {
			edgeType = stmt.EdgeType
		}
		_, linkErr := e.client.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:       insertResult.ID,
			To:         stmt.LinkTo,
			EdgeType:   edgeType,
			Weight:     1.0,
			Collection: col,
		})
		if linkErr != nil {
			log.Warn().Err(linkErr).Str("from", insertResult.ID).Str("to", stmt.LinkTo).Msg("[AQL/IMPRINT] InsertEdge failed")
		} else {
			sideEffects = append(sideEffects, "CreateAssociationEdge")
		}
	}

	return &CognitiveResult{
		Nodes: []CognitiveNode{{
			ID:       insertResult.ID,
			Content:  stmt.Content,
			NodeType: nodeType,
			Energy:   energy,
		}},
		Metadata: ResultMetadata{SideEffects: sideEffects},
	}, nil
}

// ── ASSOCIATE — create/reinforce connections ─────────────────────────

func (e *Executor) executeAssociate(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if stmt.From == "" || stmt.To == "" {
		return nil, fmt.Errorf("ASSOCIATE requires 'from' and 'to'")
	}

	edgeType := "Association"
	if stmt.EdgeType != "" {
		edgeType = stmt.EdgeType
	}
	weight := float64(1.0)
	if stmt.Weight > 0 {
		weight = float64(stmt.Weight)
	}

	edgeID, err := e.client.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       stmt.From,
		To:         stmt.To,
		EdgeType:   edgeType,
		Weight:     weight,
		Collection: col,
	})
	if err != nil {
		return nil, fmt.Errorf("ASSOCIATE InsertEdge failed: %w", err)
	}

	return &CognitiveResult{
		Edges: []CognitiveEdge{{
			Source:   stmt.From,
			Target:   stmt.To,
			EdgeType: edgeType,
			Weight:   float32(weight),
		}},
		Nodes: []CognitiveNode{{ID: edgeID}},
		Metadata: ResultMetadata{SideEffects: []string{"BoostLinkedNodes"}},
	}, nil
}

// ── DISTILL — extract patterns (Louvain community detection) ──────────
// Server-side DISTILL uses Louvain; Go side must match to ensure
// consistent results for the same AQL verb across backends.

func (e *Executor) executeDistill(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Guard: check collection size before Louvain (matches server MAX_LOUVAIN_NODES=10K)
	const maxLouvainNodes = 10000
	collections, listErr := e.client.ListCollections(ctx)
	if listErr == nil {
		for _, ci := range collections {
			if ci.Name == col && ci.NodeCount > maxLouvainNodes {
				return nil, fmt.Errorf("DISTILL: collection %q has %d nodes (max %d for Louvain)", col, ci.NodeCount, maxLouvainNodes)
			}
		}
	}

	// Run Louvain community detection (matches server-side DISTILL behavior)
	comResult, err := e.client.RunLouvain(ctx, col, 20, 1.0)
	if err != nil {
		return nil, fmt.Errorf("DISTILL Louvain failed: %w", err)
	}

	limit := stmt.Limit
	if limit <= 0 {
		limit = 10
	}

	// Group nodes by community
	communities := make(map[uint64][]string)
	for _, a := range comResult.Assignments {
		communities[a.CommunityID] = append(communities[a.CommunityID], a.NodeID)
	}

	// Pick one representative per community (first member), hydrate with GetNode
	// to get energy, then rank communities by size descending.
	type communityRep struct {
		nodeID  string
		content string
		energy  float32
		size    int
		comID   uint64
	}
	var reps []communityRep
	for comID, members := range communities {
		// Sample up to 5 members to find the highest-energy representative
		bestID := members[0]
		var bestEnergy float32
		var bestContent string
		sampleSize := 5
		if len(members) < sampleSize {
			sampleSize = len(members)
		}
		for _, id := range members[:sampleSize] {
			nr, getErr := e.client.GetNode(ctx, id, col)
			if getErr != nil || !nr.Found {
				continue
			}
			if nr.Energy > bestEnergy {
				bestEnergy = nr.Energy
				bestID = id
				bestContent = extractContentString(nr.Content)
			}
		}
		reps = append(reps, communityRep{nodeID: bestID, content: bestContent, energy: bestEnergy, size: len(members), comID: comID})
	}

	// Sort by community size descending (largest communities = most salient patterns)
	for i := 1; i < len(reps); i++ {
		key := reps[i]
		j := i - 1
		for j >= 0 && reps[j].size < key.size {
			reps[j+1] = reps[j]
			j--
		}
		reps[j+1] = key
	}

	var nodes []CognitiveNode
	for i, rep := range reps {
		if i >= limit {
			break
		}
		nodes = append(nodes, CognitiveNode{
			ID:      rep.nodeID,
			Content: rep.content,
			Energy:  rep.energy,
			Metadata: map[string]interface{}{
				"community_id":   rep.comID,
				"community_size": rep.size,
			},
		})
	}

	// TODO(side-effect): DISTILL could create summary nodes for each community cluster
	// by synthesizing the representative nodes. This is expensive (requires LLM call per
	// community) so it is deferred. When implemented, spawn a goroutine with timeout
	// and panic recovery, similar to RESONATE's CreatePatternNode.

	return &CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: []string{"CreatePatternNode", "LinkSourceEpisodes"}},
	}, nil
}

// ── FADE — intentional forgetting / decay ────────────────────────────

func (e *Executor) executeFade(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if len(stmt.NodeIDs) == 0 {
		return nil, fmt.Errorf("FADE requires node_ids to target")
	}

	var faded []CognitiveNode
	for _, nodeID := range stmt.NodeIDs {
		nr, err := e.client.GetNode(ctx, nodeID, col)
		if err != nil {
			log.Warn().Err(err).Str("node_id", nodeID).Msg("[AQL/FADE] GetNode failed")
			continue
		}
		if !nr.Found {
			log.Debug().Str("node_id", nodeID).Msg("[AQL/FADE] Node not found, skipping")
			continue
		}
		newEnergy := nr.Energy - 0.2
		if newEnergy < 0.01 {
			if delErr := e.client.DeleteNode(ctx, nodeID, col); delErr != nil {
				log.Warn().Err(delErr).Str("node_id", nodeID).Msg("[AQL/FADE] DeleteNode failed")
				continue
			}
		} else {
			if upErr := e.client.UpdateEnergy(ctx, nodeID, newEnergy, col); upErr != nil {
				log.Warn().Err(upErr).Str("node_id", nodeID).Msg("[AQL/FADE] UpdateEnergy failed")
				continue
			}
		}
		faded = append(faded, CognitiveNode{
			ID:      nodeID,
			Content: extractContentString(nr.Content),
			Energy:  newEnergy,
		})
	}

	return &CognitiveResult{
		Nodes:    faded,
		Metadata: ResultMetadata{SideEffects: []string{"RecordFadeEvent"}},
	}, nil
}

// ── DESCEND — navigate deeper in hierarchy (higher magnitude) ────────

func (e *Executor) executeDescend(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Find source node via lightweight search (no side-effects)
	seeds, err := e.searchNodes(ctx, stmt.Query, col, 1)
	if err != nil || len(seeds) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := seeds[0].ID

	// Get source node magnitude for filtering
	sourceNode, err := e.client.GetNode(ctx, seedID, col)
	if err != nil {
		return nil, fmt.Errorf("DESCEND GetNode failed: %w", err)
	}
	sourceMag := vectorMagnitude(sourceNode.Embedding)

	// BFS to find neighbors, then filter to DEEPER (higher magnitude)
	depth := uint32(2)
	if stmt.Depth > 0 {
		depth = uint32(stmt.Depth)
	}
	visitedIDs, err := e.client.Bfs(ctx, seedID, nietzsche.TraversalOpts{MaxDepth: depth}, col)
	if err != nil {
		return nil, fmt.Errorf("DESCEND BFS failed: %w", err)
	}

	// Parallelize GetNode calls with semaphore to avoid N+1 sequential RPCs
	type hydratedNode struct {
		id  string
		nr  nietzsche.NodeResult
		err error
	}
	filteredIDs := make([]string, 0, len(visitedIDs))
	for _, id := range visitedIDs {
		if id != seedID {
			filteredIDs = append(filteredIDs, id)
		}
	}
	hydrated := make([]hydratedNode, len(filteredIDs))
	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup
	for i, id := range filteredIDs {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, nodeID string) {
			defer wg.Done()
			defer func() { <-sem }()
			nr, getErr := e.client.GetNode(ctx, nodeID, col)
			hydrated[idx] = hydratedNode{id: nodeID, nr: nr, err: getErr}
		}(i, id)
	}
	wg.Wait()

	var nodes []CognitiveNode
	for _, h := range hydrated {
		if h.err != nil || !h.nr.Found {
			continue
		}
		mag := vectorMagnitude(h.nr.Embedding)
		// DESCEND: keep nodes with higher magnitude (deeper in Poincaré)
		if mag > sourceMag {
			content := extractContentString(h.nr.Content)
			nodes = append(nodes, CognitiveNode{
				ID:        h.id,
				Content:   content,
				Energy:    h.nr.Energy,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "depth_delta": mag - sourceMag},
			})
		}
	}

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── ASCEND — navigate to abstractions (lower magnitude) ──────────────

func (e *Executor) executeAscend(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Find source node via lightweight search (no side-effects)
	seeds, err := e.searchNodes(ctx, stmt.Query, col, 1)
	if err != nil || len(seeds) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := seeds[0].ID
	sourceNode, err := e.client.GetNode(ctx, seedID, col)
	if err != nil {
		return nil, fmt.Errorf("ASCEND GetNode failed: %w", err)
	}
	sourceMag := vectorMagnitude(sourceNode.Embedding)

	depth := uint32(2)
	if stmt.Depth > 0 {
		depth = uint32(stmt.Depth)
	}
	visitedIDs, err := e.client.Bfs(ctx, seedID, nietzsche.TraversalOpts{MaxDepth: depth}, col)
	if err != nil {
		return nil, fmt.Errorf("ASCEND BFS failed: %w", err)
	}

	// Parallelize GetNode calls with semaphore to avoid N+1 sequential RPCs
	ascendFilteredIDs := make([]string, 0, len(visitedIDs))
	for _, id := range visitedIDs {
		if id != seedID {
			ascendFilteredIDs = append(ascendFilteredIDs, id)
		}
	}
	ascendHydrated := make([]struct {
		id  string
		nr  nietzsche.NodeResult
		err error
	}, len(ascendFilteredIDs))
	ascendSem := make(chan struct{}, 10)
	var ascendWg sync.WaitGroup
	for i, id := range ascendFilteredIDs {
		ascendWg.Add(1)
		ascendSem <- struct{}{}
		go func(idx int, nodeID string) {
			defer ascendWg.Done()
			defer func() { <-ascendSem }()
			nr, getErr := e.client.GetNode(ctx, nodeID, col)
			ascendHydrated[idx] = struct {
				id  string
				nr  nietzsche.NodeResult
				err error
			}{id: nodeID, nr: nr, err: getErr}
		}(i, id)
	}
	ascendWg.Wait()

	var nodes []CognitiveNode
	for _, h := range ascendHydrated {
		if h.err != nil || !h.nr.Found {
			continue
		}
		mag := vectorMagnitude(h.nr.Embedding)
		// ASCEND: keep nodes with lower magnitude (more abstract, closer to origin)
		if mag < sourceMag {
			content := extractContentString(h.nr.Content)
			nodes = append(nodes, CognitiveNode{
				ID:        h.id,
				Content:   content,
				Energy:    h.nr.Energy,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "depth_delta": sourceMag - mag},
			})
		}
	}

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── ORBIT — find peers at same depth ────────────────────────────────

func (e *Executor) executeOrbit(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Find seed node via KNN/FTS, then use BFS (graph traversal) to find peers.
	// This aligns with server-side ORBIT which uses BFS + magnitude filter,
	// not KNN (semantic similarity).
	seedResults, err := e.searchNodes(ctx, stmt.Query, col, 1)
	if err != nil || len(seedResults) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := seedResults[0].ID
	sourceNode, err := e.client.GetNode(ctx, seedID, col)
	if err != nil {
		return nil, fmt.Errorf("ORBIT GetNode failed: %w", err)
	}
	sourceMag := vectorMagnitude(sourceNode.Embedding)

	// RADIUS is absolute tolerance (matches server-side behavior).
	// e.g., RADIUS 0.2 means ±0.2 magnitude around the source node.
	tolerance := 0.2 // default
	if stmt.Radius > 0 {
		tolerance = float64(stmt.Radius)
	}
	if tolerance < 0.05 {
		tolerance = 0.05
	}

	// BFS from seed (matches server-side ORBIT behavior)
	depth := uint32(3)
	if stmt.Depth > 0 {
		depth = uint32(stmt.Depth)
	}
	visitedIDs, bfsErr := e.client.Bfs(ctx, seedID, nietzsche.TraversalOpts{MaxDepth: depth}, col)
	if bfsErr != nil {
		return nil, fmt.Errorf("ORBIT BFS failed: %w", bfsErr)
	}

	// Filter out seed itself
	var candidateIDs []string
	for _, id := range visitedIDs {
		if id != seedID {
			candidateIDs = append(candidateIDs, id)
		}
	}

	// Parallelize GetNode calls with semaphore to avoid N+1 sequential RPCs
	orbitHydrated := make([]struct {
		id  string
		nr  nietzsche.NodeResult
		err error
	}, len(candidateIDs))
	orbitSem := make(chan struct{}, 10)
	var orbitWg sync.WaitGroup
	for i, id := range candidateIDs {
		orbitWg.Add(1)
		orbitSem <- struct{}{}
		go func(idx int, nodeID string) {
			defer orbitWg.Done()
			defer func() { <-orbitSem }()
			nr, getErr := e.client.GetNode(ctx, nodeID, col)
			orbitHydrated[idx] = struct {
				id  string
				nr  nietzsche.NodeResult
				err error
			}{id: nodeID, nr: nr, err: getErr}
		}(i, id)
	}
	orbitWg.Wait()

	var nodes []CognitiveNode
	for _, h := range orbitHydrated {
		if h.err != nil || !h.nr.Found {
			continue
		}
		mag := vectorMagnitude(h.nr.Embedding)
		// ORBIT: keep nodes at similar magnitude (±tolerance)
		if math.Abs(mag-sourceMag) <= tolerance {
			content := extractContentString(h.nr.Content)
			nodes = append(nodes, CognitiveNode{
				ID:        h.id,
				Content:   content,
				Energy:    h.nr.Energy,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "mag_delta": math.Abs(mag - sourceMag)},
			})
		}
	}

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── DREAM — creative recombination / sleep cycle ────────────────────

func (e *Executor) executeDream(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if stmt.Topic != "" {
		// Start a dream exploration via NQL DREAM command
		// TODO(nql-rewrite): If DREAM NQL ever uses MATCH with custom types,
		// pass through nietzscheInfra.RewriteNQL() before e.client.Query().
		nql := "DREAM FROM $seed"
		if stmt.Depth > 0 {
			nql = fmt.Sprintf("DREAM FROM $seed DEPTH %d", stmt.Depth)
		}
		params := map[string]interface{}{"seed": stmt.Topic}
		qr, err := e.client.Query(ctx, nql, params, col)
		if err == nil && qr.DreamResult != nil {
			var nodes []CognitiveNode
			for _, ev := range qr.DreamResult.Events {
				nodes = append(nodes, CognitiveNode{
					ID:      ev.NodeID,
					Energy:  float32(ev.Energy),
					Content: ev.Description,
				})
			}
			return &CognitiveResult{
				Nodes: nodes,
				Metadata: ResultMetadata{
					SideEffects: []string{"DreamCycle"},
				},
			}, nil
		}
	}

	// Default: trigger sleep cycle
	sleepResult, err := e.client.TriggerSleep(ctx, nietzsche.SleepOpts{Collection: col})
	if err != nil {
		return nil, fmt.Errorf("DREAM TriggerSleep failed: %w", err)
	}

	return &CognitiveResult{
		Nodes: []CognitiveNode{{
			Metadata: map[string]interface{}{
				"committed":       sleepResult.Committed,
				"hausdorff_delta": sleepResult.HausdorffDelta,
				"nodes_perturbed": sleepResult.NodesPerturbed,
			},
		}},
	}, nil
}

// ── IMAGINE — counterfactual reasoning ──────────────────────────────

func (e *Executor) executeImagine(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)
	// Counterfactual: find premise nodes via lightweight search, then explore alternative paths
	searchResults, err := e.searchNodes(ctx, stmt.Premise, col, 5)
	if err != nil {
		return nil, fmt.Errorf("IMAGINE search failed: %w", err)
	}

	// Hydrate nodes with Content/Energy (searchNodes returns only {ID, metadata})
	nodes := make([]CognitiveNode, len(searchResults))
	imagineSem := make(chan struct{}, 10)
	var imagineWg sync.WaitGroup
	for i, sn := range searchResults {
		imagineWg.Add(1)
		imagineSem <- struct{}{}
		go func(idx int, nodeID string, meta map[string]interface{}) {
			defer imagineWg.Done()
			defer func() { <-imagineSem }()
			cn := CognitiveNode{ID: nodeID, Metadata: meta}
			if nr, getErr := e.client.GetNode(ctx, nodeID, col); getErr == nil && nr.Found {
				cn.Content = extractContentString(nr.Content)
				cn.Energy = nr.Energy
				cn.NodeType = nr.NodeType
			}
			nodes[idx] = cn
		}(i, sn.ID, sn.Metadata)
	}
	imagineWg.Wait()

	// Post-filter by confidence/recency/valence
	nodes = e.applyFilters(ctx, nodes, stmt)

	return &CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: []string{"CounterfactualBranch"}},
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────

// ── Post-filters — apply Confidence / Recency / Valence after search ──

// applyFilters hydrates nodes with data from NietzscheDB and filters by
// confidence (energy floor), recency (creation time), and valence (content field).
// Nodes that cannot be hydrated are kept (fail-open for robustness).
func (e *Executor) applyFilters(ctx context.Context, nodes []CognitiveNode, stmt *Statement) []CognitiveNode {
	needsHydrate := stmt.Confidence > 0 || stmt.Recency != "" || stmt.Valence != ""
	if !needsHydrate || len(nodes) == 0 {
		return nodes
	}

	col := e.collection(stmt)
	now := time.Now()

	// Determine recency cutoff
	var recencyCutoff time.Time
	switch stmt.Recency {
	case RecencyFresh:
		recencyCutoff = now.Add(-5 * time.Minute)
	case RecencyRecent:
		recencyCutoff = now.Add(-1 * time.Hour)
	case RecencyDistant:
		recencyCutoff = now.Add(-24 * time.Hour)
	case RecencyAncient:
		// no time filter
	}

	filtered := make([]CognitiveNode, 0, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		if n.ID == "" {
			filtered = append(filtered, *n)
			continue
		}

		// Only hydrate via GetNode if node is missing filter-relevant fields.
		// Nodes already hydrated by RECALL/DESCEND/ASCEND/ORBIT/RESONATE
		// skip the redundant RPC (avoids double-hydration).
		// Use Content or NodeType presence as hydration indicator (Energy can legitimately be 0.0)
		alreadyHydrated := n.NodeType != "" || n.Content != ""
		if !alreadyHydrated {
			nr, err := e.client.GetNode(ctx, n.ID, col)
			if err != nil || !nr.Found {
				// Can't hydrate — keep node (fail-open)
				filtered = append(filtered, *n)
				continue
			}

			// Populate CognitiveNode fields from hydrated data
			n.Energy = nr.Energy
			n.NodeType = nr.NodeType
			if n.Content == "" {
				n.Content = extractContentString(nr.Content)
			}
			if nr.CreatedAt > 0 {
				t := time.Unix(nr.CreatedAt, 0)
				n.CreatedAt = &t
			}

			// Extract valence from content if present
			if v, ok := nr.Content["valence"]; ok {
				switch vt := v.(type) {
				case float64:
					n.Valence = float32(vt)
				case float32:
					n.Valence = vt
				}
			}
		}

		// Filter: confidence (energy floor)
		if stmt.Confidence > 0 && n.Energy < stmt.Confidence {
			continue
		}

		// Filter: recency (creation time)
		if !recencyCutoff.IsZero() && n.CreatedAt != nil {
			if n.CreatedAt.Before(recencyCutoff) {
				continue
			}
		}

		// Filter: valence polarity
		if stmt.Valence != "" {
			switch stmt.Valence {
			case ValencePositive:
				if n.Valence <= 0 {
					continue
				}
			case ValenceNegative:
				if n.Valence >= 0 {
					continue
				}
			case ValenceNeutral:
				if n.Valence > 0.1 || n.Valence < -0.1 {
					continue
				}
			}
		}

		filtered = append(filtered, *n)
	}

	return filtered
}

// vectorMagnitude computes the Euclidean norm of a Poincaré embedding.
// In the Poincaré ball, magnitude ∝ depth in hierarchy.
func vectorMagnitude(coords []float64) float64 {
	var sumSq float64
	for _, c := range coords {
		sumSq += c * c
	}
	return math.Sqrt(sumSq)
}

func float32ToFloat64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, f := range v {
		out[i] = float64(f)
	}
	return out
}

func projectToPoincare(coords []float64, targetMag float64) []float64 {
	var normSq float64
	for _, c := range coords {
		normSq += c * c
	}
	norm := math.Sqrt(normSq)
	if norm == 0 {
		return coords
	}
	scale := targetMag / norm
	out := make([]float64, len(coords))
	for i, c := range coords {
		out[i] = c * scale
	}
	return out
}

// ParseStatement parses a raw AQL string into a Statement.
// Supports: VERB "query" [COLLECTION col] [LIMIT n] [CONFIDENCE f]
// [AS type] [ENERGY f] [MOOD m] [DEPTH d] [LINK_TO id] [FROM id TO id]
func ParseStatement(raw string) (*Statement, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty AQL statement")
	}

	stmt := &Statement{}

	// 1. Extract all quoted strings and replace with placeholders.
	//    This prevents keywords inside quotes (e.g. "FROM server TO client")
	//    from being parsed as AQL keywords.
	var quotedStrings []string
	sanitized := extractQuotedStrings(raw, &quotedStrings)

	parts := strings.Fields(sanitized)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty AQL statement")
	}

	// First token is the verb
	verbStr := strings.ToUpper(parts[0])
	switch verbStr {
	case "RECALL":
		stmt.Verb = VerbRecall
	case "RESONATE":
		stmt.Verb = VerbResonate
	case "REFLECT":
		stmt.Verb = VerbReflect
	case "TRACE":
		stmt.Verb = VerbTrace
	case "IMPRINT":
		stmt.Verb = VerbImprint
	case "ASSOCIATE":
		stmt.Verb = VerbAssociate
	case "DISTILL":
		stmt.Verb = VerbDistill
	case "FADE":
		stmt.Verb = VerbFade
	case "DESCEND":
		stmt.Verb = VerbDescend
	case "ASCEND":
		stmt.Verb = VerbAscend
	case "ORBIT":
		stmt.Verb = VerbOrbit
	case "DREAM":
		stmt.Verb = VerbDream
	case "IMAGINE":
		stmt.Verb = VerbImagine
	default:
		return nil, fmt.Errorf("unknown AQL verb: %s", verbStr)
	}

	// 2. Assign first quoted string as query/content/topic/premise
	if len(quotedStrings) > 0 {
		switch stmt.Verb {
		case VerbImprint:
			stmt.Content = quotedStrings[0]
		case VerbDream:
			stmt.Topic = quotedStrings[0]
		case VerbImagine:
			stmt.Premise = quotedStrings[0]
		default:
			stmt.Query = quotedStrings[0]
		}
	}

	// 3. Parse qualifier keywords on the sanitized (placeholder) version
	for i := 1; i < len(parts); i++ {
		kw := strings.ToUpper(parts[i])
		switch kw {
		case "COLLECTION":
			if i+1 < len(parts) {
				stmt.Collection = restoreQuoted(parts[i+1], quotedStrings)
				i++
			}
		case "LIMIT":
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%d", &stmt.Limit)
				i++
			}
		case "CONFIDENCE":
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%f", &stmt.Confidence)
				i++
			}
		case "DEPTH":
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%d", &stmt.Depth)
				i++
			}
		case "ENERGY":
			if i+1 < len(parts) {
				fmt.Sscanf(parts[i+1], "%f", &stmt.Energy)
				i++
			}
		case "AS":
			if i+1 < len(parts) {
				stmt.Epistemic = EpistemicType(restoreQuoted(parts[i+1], quotedStrings))
				i++
			}
		case "MOOD":
			if i+1 < len(parts) {
				stmt.Mood = MoodState(strings.ToLower(restoreQuoted(parts[i+1], quotedStrings)))
				i++
			}
		case "FROM":
			if i+1 < len(parts) {
				stmt.From = restoreQuoted(parts[i+1], quotedStrings)
				i++
			}
		case "TO":
			if i+1 < len(parts) {
				stmt.To = restoreQuoted(parts[i+1], quotedStrings)
				i++
			}
		case "LINK_TO":
			if i+1 < len(parts) {
				stmt.LinkTo = restoreQuoted(parts[i+1], quotedStrings)
				i++
			}
		case "EDGE_TYPE":
			if i+1 < len(parts) {
				stmt.EdgeType = restoreQuoted(parts[i+1], quotedStrings)
				i++
			}
		case "RECENCY":
			if i+1 < len(parts) {
				stmt.Recency = RecencyDegree(strings.ToLower(restoreQuoted(parts[i+1], quotedStrings)))
				i++
			}
		case "VALENCE":
			if i+1 < len(parts) {
				stmt.Valence = ValenceSpec(strings.ToLower(restoreQuoted(parts[i+1], quotedStrings)))
				i++
			}
		}
	}

	// For TRACE, also check inline "FROM x TO y" in any position
	upper := strings.ToUpper(sanitized)
	if stmt.Verb == VerbTrace && stmt.From == "" {
		if strings.Contains(upper, "FROM ") {
			rest := parts[1:] // skip verb
			for j := 0; j < len(rest)-1; j++ {
				if strings.ToUpper(rest[j]) == "FROM" {
					stmt.From = restoreQuoted(rest[j+1], quotedStrings)
				}
				if strings.ToUpper(rest[j]) == "TO" {
					stmt.To = restoreQuoted(rest[j+1], quotedStrings)
				}
			}
		}
	}

	return stmt, nil
}

// extractQuotedStrings replaces all quoted strings in raw with placeholders
// like __Q0__, __Q1__, etc. and appends the originals to the slice.
func extractQuotedStrings(raw string, out *[]string) string {
	var result strings.Builder
	i := 0
	idx := 0
	for i < len(raw) {
		if raw[i] == '"' {
			// Escape-aware: scan for unescaped closing quote
			var quoted strings.Builder
			i++ // skip opening quote
			for i < len(raw) {
				if raw[i] == '\\' && i+1 < len(raw) {
					// Escaped char: keep the actual character
					quoted.WriteByte(raw[i+1])
					i += 2
				} else if raw[i] == '"' {
					i++ // skip closing quote
					break
				} else {
					quoted.WriteByte(raw[i])
					i++
				}
			}
			*out = append(*out, quoted.String())
			fmt.Fprintf(&result, "__Q%d__", idx)
			idx++
			continue
		}
		result.WriteByte(raw[i])
		i++
	}
	return result.String()
}

// restoreQuoted checks if s is a placeholder like __Q0__ and returns the
// original quoted string. Otherwise returns s unchanged.
func restoreQuoted(s string, quoted []string) string {
	if strings.HasPrefix(s, "__Q") && strings.HasSuffix(s, "__") {
		var idx int
		if _, err := fmt.Sscanf(s, "__Q%d__", &idx); err == nil && idx < len(quoted) {
			return quoted[idx]
		}
	}
	return s
}

// extractContentString gets a display string from a node's Content map.
// Handles both "description" (galaxy scripts) and "content" (IMPRINT) keys.
// Falls back to first non-empty string value if neither key is present.
func extractContentString(content map[string]interface{}) string {
	// Primary: "description" (galaxy scripts, manual inserts)
	if desc, ok := content["description"]; ok {
		if s := fmt.Sprintf("%v", desc); s != "" {
			return s
		}
	}
	// Secondary: "content" (IMPRINT verb stores here)
	if c, ok := content["content"]; ok {
		if s := fmt.Sprintf("%v", c); s != "" {
			return s
		}
	}
	// Fallback: first non-empty string value
	for _, v := range content {
		if s := fmt.Sprintf("%v", v); s != "" && s != "<nil>" {
			return s
		}
	}
	return ""
}
