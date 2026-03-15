// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AQL Lowering — converts AQL Statements to NietzscheDB gRPC calls.
// Each verb is lowered to one or more SDK operations with automatic side-effects.

package aql

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	nietzsche "nietzsche-sdk"
)

// ── RECALL — semantic search (KNN + full-text fallback) ──────────────

func (e *Executor) executeRecall(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)
	limit := stmt.Limit
	if limit <= 0 {
		limit = 10
	}

	var nodes []CognitiveNode

	// Primary: KNN vector search (if embedding function available)
	if e.embedFunc != nil && stmt.Query != "" {
		vec32, err := e.embedFunc(ctx, stmt.Query)
		if err != nil {
			log.Printf("[AQL/RECALL] embedFunc failed (falling back to FTS): %v", err)
		} else {
			vec64 := float32ToFloat64(vec32)
			knnResults, knnErr := e.client.KnnSearch(ctx, vec64, uint32(limit), col)
			if knnErr == nil {
				for _, r := range knnResults {
					nodes = append(nodes, CognitiveNode{
						ID:       r.ID,
						Metadata: map[string]interface{}{"distance": r.Distance},
					})
				}
			}
		}
	}

	// Fallback: full-text search
	if len(nodes) == 0 && stmt.Query != "" {
		ftResults, err := e.client.FullTextSearch(ctx, stmt.Query, col, uint32(limit))
		if err == nil {
			for _, r := range ftResults {
				nodes = append(nodes, CognitiveNode{
					ID:       r.NodeID,
					Metadata: map[string]interface{}{"fts_score": r.Score},
				})
			}
		}
	}

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
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, id := range ids {
				nr, getErr := e.client.GetNode(bgCtx, id, collection)
				if getErr != nil {
					log.Printf("[AQL/RECALL] GetNode %s failed: %v", id, getErr)
					continue
				}
				if nr.Found {
					newEnergy := nr.Energy + 0.03
					if newEnergy > 1.0 {
						newEnergy = 1.0
					}
					if err := e.client.UpdateEnergy(bgCtx, id, newEnergy, collection); err != nil {
						log.Printf("[AQL/RECALL] UpdateEnergy %s failed: %v", id, err)
					}
				}
			}
		}(nodeIDs, col)
	}

	if nodes == nil {
		nodes = []CognitiveNode{}
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

	// First find the seed node via RECALL
	recallStmt := &Statement{
		Verb:       VerbRecall,
		Query:      stmt.Query,
		Collection: col,
		Limit:      1,
	}
	recallResult, err := e.executeRecall(ctx, recallStmt)
	if err != nil || len(recallResult.Nodes) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := recallResult.Nodes[0].ID
	depth := uint32(3)
	if stmt.Depth > 0 {
		depth = uint32(stmt.Depth)
	}

	// BFS from seed — returns visited node IDs
	visitedIDs, err := e.client.Bfs(ctx, seedID, nietzsche.TraversalOpts{MaxDepth: depth}, col)
	if err != nil {
		return nil, fmt.Errorf("RESONATE BFS failed: %w", err)
	}

	var nodes []CognitiveNode
	for _, id := range visitedIDs {
		nodes = append(nodes, CognitiveNode{ID: id})
	}

	return &CognitiveResult{
		Nodes:    nodes,
		Metadata: ResultMetadata{SideEffects: []string{"RecordResonancePattern"}},
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
			return &CognitiveResult{
				Nodes: []CognitiveNode{{
					ID:       synthResult.NearestNodeID,
					Metadata: map[string]interface{}{"nearest_distance": synthResult.NearestDistance, "synthesis_coords": synthResult.SynthesisCoords},
				}},
			}, nil
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

	var nodes []CognitiveNode
	for i, id := range visitedIDs {
		cost := 0.0
		if i < len(costs) {
			cost = costs[i]
		}
		nodes = append(nodes, CognitiveNode{
			ID:       id,
			Metadata: map[string]interface{}{"cost": cost},
		})
		// Stop if we found the target
		if id == stmt.To {
			break
		}
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
			bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			for _, id := range ids {
				nr, getErr := e.client.GetNode(bgCtx, id, collection)
				if getErr != nil {
					log.Printf("[AQL/TRACE] GetNode %s failed: %v", id, getErr)
					continue
				}
				if nr.Found {
					newEnergy := nr.Energy + 0.02
					if newEnergy > 1.0 {
						newEnergy = 1.0
					}
					if err := e.client.UpdateEnergy(bgCtx, id, newEnergy, collection); err != nil {
						log.Printf("[AQL/TRACE] UpdateEnergy %s failed: %v", id, err)
					}
				}
			}
		}(pathIDs, col)
	}

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

	// Generate embedding for coordinates
	var coords []float64
	if e.embedFunc != nil {
		vec32, err := e.embedFunc(ctx, stmt.Content)
		if err != nil {
			log.Printf("[AQL/IMPRINT] embedFunc failed (node will have zero coords): %v", err)
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
			log.Printf("[AQL/IMPRINT] InsertEdge %s→%s failed: %v", insertResult.ID, stmt.LinkTo, linkErr)
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

// ── DISTILL — extract patterns (PageRank) ────────────────────────────

func (e *Executor) executeDistill(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Run PageRank to find most influential nodes
	prResult, err := e.client.RunPageRank(ctx, col, 0.85, 20)
	if err != nil {
		return nil, fmt.Errorf("DISTILL PageRank failed: %w", err)
	}

	limit := stmt.Limit
	if limit <= 0 {
		limit = 10
	}

	var nodes []CognitiveNode
	for i, score := range prResult.Scores {
		if i >= limit {
			break
		}
		nodes = append(nodes, CognitiveNode{
			ID:       score.NodeID,
			Energy:   float32(score.Score),
			Metadata: map[string]interface{}{"pagerank": score.Score},
		})
	}

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
			log.Printf("[AQL/FADE] GetNode %s failed: %v", nodeID, err)
			continue
		}
		if !nr.Found {
			log.Printf("[AQL/FADE] Node %s not found, skipping", nodeID)
			continue
		}
		newEnergy := nr.Energy - 0.2
		if newEnergy < 0.01 {
			if delErr := e.client.DeleteNode(ctx, nodeID, col); delErr != nil {
				log.Printf("[AQL/FADE] DeleteNode %s failed: %v", nodeID, delErr)
				continue
			}
		} else {
			if upErr := e.client.UpdateEnergy(ctx, nodeID, newEnergy, col); upErr != nil {
				log.Printf("[AQL/FADE] UpdateEnergy %s failed: %v", nodeID, upErr)
				continue
			}
		}
		faded = append(faded, CognitiveNode{ID: nodeID, Energy: newEnergy})
	}

	return &CognitiveResult{
		Nodes:    faded,
		Metadata: ResultMetadata{SideEffects: []string{"RecordFadeEvent"}},
	}, nil
}

// ── DESCEND — navigate deeper in hierarchy (higher magnitude) ────────

func (e *Executor) executeDescend(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	// Find source node via RECALL
	recallResult, err := e.executeRecall(ctx, &Statement{
		Verb: VerbRecall, Query: stmt.Query, Collection: col, Limit: 1,
	})
	if err != nil || len(recallResult.Nodes) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := recallResult.Nodes[0].ID

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

	var nodes []CognitiveNode
	for _, id := range visitedIDs {
		if id == seedID {
			continue
		}
		nr, getErr := e.client.GetNode(ctx, id, col)
		if getErr != nil || !nr.Found {
			continue
		}
		mag := vectorMagnitude(nr.Embedding)
		// DESCEND: keep nodes with higher magnitude (deeper in Poincaré)
		if mag > sourceMag {
			nodes = append(nodes, CognitiveNode{
				ID:        id,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "depth_delta": mag - sourceMag},
			})
		}
	}

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── ASCEND — navigate to abstractions (lower magnitude) ──────────────

func (e *Executor) executeAscend(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	recallResult, err := e.executeRecall(ctx, &Statement{
		Verb: VerbRecall, Query: stmt.Query, Collection: col, Limit: 1,
	})
	if err != nil || len(recallResult.Nodes) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := recallResult.Nodes[0].ID
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

	var nodes []CognitiveNode
	for _, id := range visitedIDs {
		if id == seedID {
			continue
		}
		nr, getErr := e.client.GetNode(ctx, id, col)
		if getErr != nil || !nr.Found {
			continue
		}
		mag := vectorMagnitude(nr.Embedding)
		// ASCEND: keep nodes with lower magnitude (more abstract, closer to origin)
		if mag < sourceMag {
			nodes = append(nodes, CognitiveNode{
				ID:        id,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "depth_delta": sourceMag - mag},
			})
		}
	}

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── ORBIT — find peers at same depth ────────────────────────────────

func (e *Executor) executeOrbit(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	recallResult, err := e.executeRecall(ctx, &Statement{
		Verb: VerbRecall, Query: stmt.Query, Collection: col, Limit: 1,
	})
	if err != nil || len(recallResult.Nodes) == 0 {
		return &CognitiveResult{Nodes: []CognitiveNode{}}, nil
	}

	seedID := recallResult.Nodes[0].ID
	sourceNode, err := e.client.GetNode(ctx, seedID, col)
	if err != nil {
		return nil, fmt.Errorf("ORBIT GetNode failed: %w", err)
	}
	sourceMag := vectorMagnitude(sourceNode.Embedding)

	// KNN finds nearest neighbors, then filter by similar magnitude (±20%)
	knnResult, err := e.executeRecall(ctx, &Statement{
		Verb: VerbRecall, Query: stmt.Query, Collection: col, Limit: 30,
	})
	if err != nil {
		return nil, fmt.Errorf("ORBIT search failed: %w", err)
	}

	tolerance := sourceMag * 0.2
	if tolerance < 0.05 {
		tolerance = 0.05
	}

	var nodes []CognitiveNode
	for _, n := range knnResult.Nodes {
		if n.ID == seedID {
			continue
		}
		nr, getErr := e.client.GetNode(ctx, n.ID, col)
		if getErr != nil || !nr.Found {
			continue
		}
		mag := vectorMagnitude(nr.Embedding)
		// ORBIT: keep nodes at similar magnitude (±tolerance)
		if math.Abs(mag-sourceMag) <= tolerance {
			nodes = append(nodes, CognitiveNode{
				ID:        n.ID,
				Magnitude: float32(mag),
				Metadata:  map[string]interface{}{"magnitude": mag, "mag_delta": math.Abs(mag - sourceMag)},
			})
		}
	}

	return &CognitiveResult{Nodes: nodes}, nil
}

// ── DREAM — creative recombination / sleep cycle ────────────────────

func (e *Executor) executeDream(ctx context.Context, stmt *Statement) (*CognitiveResult, error) {
	col := e.collection(stmt)

	if stmt.Topic != "" {
		// Start a dream exploration via NQL DREAM command
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
	// Counterfactual: RECALL premise nodes, then explore alternative paths
	recallResult, err := e.executeRecall(ctx, &Statement{
		Verb: VerbRecall, Query: stmt.Premise, Collection: e.collection(stmt), Limit: 5,
	})
	if err != nil {
		return nil, fmt.Errorf("IMAGINE recall failed: %w", err)
	}

	return &CognitiveResult{
		Nodes:    recallResult.Nodes,
		Metadata: ResultMetadata{SideEffects: []string{"CounterfactualBranch"}},
	}, nil
}

// ── Helpers ──────────────────────────────────────────────────────────

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

	parts := strings.Fields(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty AQL statement")
	}

	stmt := &Statement{}

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

	// Extract quoted string as query/content/topic/premise
	if idx := strings.Index(raw, "\""); idx >= 0 {
		end := strings.Index(raw[idx+1:], "\"")
		if end >= 0 {
			quoted := raw[idx+1 : idx+1+end]
			switch stmt.Verb {
			case VerbImprint:
				stmt.Content = quoted
			case VerbDream:
				stmt.Topic = quoted
			case VerbImagine:
				stmt.Premise = quoted
			default:
				stmt.Query = quoted
			}
		}
	}

	// Parse qualifier keywords
	upper := strings.ToUpper(raw)
	for i := 1; i < len(parts); i++ {
		kw := strings.ToUpper(parts[i])
		switch kw {
		case "COLLECTION":
			if i+1 < len(parts) {
				stmt.Collection = parts[i+1]
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
				stmt.Epistemic = EpistemicType(parts[i+1])
				i++
			}
		case "MOOD":
			if i+1 < len(parts) {
				stmt.Mood = MoodState(strings.ToLower(parts[i+1]))
				i++
			}
		case "FROM":
			if i+1 < len(parts) {
				stmt.From = parts[i+1]
				i++
			}
		case "TO":
			if i+1 < len(parts) {
				stmt.To = parts[i+1]
				i++
			}
		case "LINK_TO":
			if i+1 < len(parts) {
				stmt.LinkTo = parts[i+1]
				i++
			}
		case "EDGE_TYPE":
			if i+1 < len(parts) {
				stmt.EdgeType = parts[i+1]
				i++
			}
		}
	}

	// For TRACE, also check inline "FROM x TO y" in any position
	if stmt.Verb == VerbTrace && stmt.From == "" {
		if strings.Contains(upper, "FROM ") {
			rest := parts[1:] // skip verb
			for j := 0; j < len(rest)-1; j++ {
				if strings.ToUpper(rest[j]) == "FROM" {
					stmt.From = rest[j+1]
				}
				if strings.ToUpper(rest[j]) == "TO" {
					stmt.To = rest[j+1]
				}
			}
		}
	}

	return stmt, nil
}
