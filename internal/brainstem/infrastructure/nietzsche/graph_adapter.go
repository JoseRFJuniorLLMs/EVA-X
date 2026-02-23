// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"
	"time"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// GraphAdapter provides graph operations via NietzscheDB.
// It translates Cypher-style patterns (MERGE, MATCH, variable-length paths)
// into NietzscheDB SDK calls.
type GraphAdapter struct {
	client     *Client
	collection string // default collection (e.g. "patient_graph")
}

// NewGraphAdapter creates a GraphAdapter targeting a specific collection.
func NewGraphAdapter(client *Client, defaultCollection string) *GraphAdapter {
	return &GraphAdapter{client: client, collection: defaultCollection}
}

// SetCollection changes the default collection.
func (ga *GraphAdapter) SetCollection(collection string) {
	ga.collection = collection
}

// SDK returns the underlying NietzscheDB SDK client.
func (ga *GraphAdapter) SDK() *nietzsche.NietzscheClient {
	return ga.client.SDK()
}

// Client returns the EVA-level NietzscheDB Client wrapper.
// Used by services that need access to higher-level operations (e.g. Dream System).
func (ga *GraphAdapter) Client() *Client {
	return ga.client
}

// ── MERGE ────────────────────────────────────────────────────────────────────

// MergeNodeOpts defines options for a MERGE operation on a node.
type MergeNodeOpts struct {
	Collection  string
	NodeType    string
	MatchKeys   map[string]interface{}
	OnCreateSet map[string]interface{}
	OnMatchSet  map[string]interface{}
	// BLOCKER 2 fix: expose Poincaré coords and energy so nodes are placed
	// in hyperbolic space, not permanently at the ball origin.
	Coords []float64 // Poincaré ball embedding; nil → server assigns
	Energy float32   // node energy; 0 → server default (1.0)
}

// MergeNodeResult is the result of a MergeNode operation.
type MergeNodeResult struct {
	Created bool
	NodeID  string
	// BLOCKER 3 fix: expose the full NodeResult (Embedding, Energy, Depth,
	// HausdorffLocal, CreatedAt, NodeType) instead of only Content.
	Node nietzsche.NodeResult
	// Content is a convenience alias for Node.Content; kept for backward compat.
	Content map[string]interface{}
}

// MergeNode finds or creates a node by type + match keys (MERGE semantics).
func (ga *GraphAdapter) MergeNode(ctx context.Context, opts MergeNodeOpts) (*MergeNodeResult, error) {
	log := logger.Nietzsche()

	col := opts.Collection
	if col == "" {
		col = ga.collection
	}

	result, err := ga.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection:  col,
		NodeType:    opts.NodeType,
		MatchKeys:   opts.MatchKeys,
		OnCreateSet: opts.OnCreateSet,
		OnMatchSet:  opts.OnMatchSet,
		Coords:      opts.Coords, // BLOCKER 2 fix: forward Poincaré coords
		Energy:      opts.Energy, // BLOCKER 2 fix: forward energy
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", col).
			Str("node_type", opts.NodeType).
			Msg("merge node failed")
		return nil, fmt.Errorf("merge node %s/%s: %w", col, opts.NodeType, err)
	}

	log.Debug().
		Str("collection", col).
		Str("node_type", opts.NodeType).
		Bool("created", result.Created).
		Str("node_id", result.NodeID).
		Msg("merge node completed")

	// BLOCKER 3 fix: expose the full NodeResult so callers can access
	// Embedding, Energy, Depth, NodeType, CreatedAt, HausdorffLocal.
	// Content is kept as a convenience shortcut for Node.Content.
	return &MergeNodeResult{
		Created: result.Created,
		NodeID:  result.NodeID,
		Node:    result.Node,
		Content: result.Node.Content,
	}, nil
}

// MergeEdgeOpts defines options for a MERGE operation on a relationship/edge.
type MergeEdgeOpts struct {
	Collection  string
	FromNodeID  string
	ToNodeID    string
	EdgeType    string
	OnCreateSet map[string]interface{}
	OnMatchSet  map[string]interface{}
}

// MergeEdgeResult is the result of a MergeEdge operation.
type MergeEdgeResult struct {
	Created bool
	EdgeID  string
}

// MergeEdge finds or creates an edge (MERGE semantics on relationship).
func (ga *GraphAdapter) MergeEdge(ctx context.Context, opts MergeEdgeOpts) (*MergeEdgeResult, error) {
	log := logger.Nietzsche()

	col := opts.Collection
	if col == "" {
		col = ga.collection
	}

	result, err := ga.client.MergeEdge(ctx, nietzsche.MergeEdgeOpts{
		Collection:  col,
		FromNodeID:  opts.FromNodeID,
		ToNodeID:    opts.ToNodeID,
		EdgeType:    opts.EdgeType,
		OnCreateSet: opts.OnCreateSet,
		OnMatchSet:  opts.OnMatchSet,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", col).
			Str("edge_type", opts.EdgeType).
			Msg("merge edge failed")
		return nil, fmt.Errorf("merge edge %s: %w", opts.EdgeType, err)
	}

	return &MergeEdgeResult{
		Created: result.Created,
		EdgeID:  result.EdgeID,
	}, nil
}

// ── MATCH + WHERE ────────────────────────────────────────────────────────────

// QueryResult holds the result of an NQL query.
type QueryResult = nietzsche.QueryResult

// ExecuteNQL runs an NQL query against a collection.
// Executes NQL MATCH queries against NietzscheDB graph.
func (ga *GraphAdapter) ExecuteNQL(ctx context.Context, nql string,
	params map[string]interface{}, collection string) (*QueryResult, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	return ga.client.Query(ctx, nql, params, col)
}

// ── BFS (variable-length path traversal) ─────────────────────────────────────

// Bfs performs breadth-first traversal from a start node.
// Implements variable-length path traversal: (a)-[*1..N]-(b)
func (ga *GraphAdapter) Bfs(ctx context.Context, startID string, maxDepth uint32,
	collection string) ([]string, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	return ga.client.Bfs(ctx, startID, nietzsche.TraversalOpts{
		MaxDepth: maxDepth,
	}, col)
}

// Dijkstra finds the shortest path between two nodes using edge weights.
func (ga *GraphAdapter) Dijkstra(ctx context.Context, startID string,
	collection string) ([]string, []float64, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	return ga.client.Dijkstra(ctx, startID, nietzsche.TraversalOpts{}, col)
}

// RunAStar finds the shortest path between two nodes using A* algorithm.
func (ga *GraphAdapter) RunAStar(ctx context.Context, startID, goalID string,
	collection string) ([]string, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	res, err := ga.client.RunAStar(ctx, col, startID, goalID)
	if err != nil {
		return nil, err
	}
	return res.Path, nil
}

// BfsWithEdgeType performs BFS filtered by edge type.
// Replaces: MATCH (a)-[:TYPE*1..N]-(b)
//
// BLOCKER 1 optimized: the SDK's TraversalOpts has no EdgeType field so BFS
// is always type-agnostic. We batch the entire frontier per depth level into
// a single NQL call using WHERE a.id IN [...] — one RPC per depth step.
func (ga *GraphAdapter) BfsWithEdgeType(ctx context.Context, startID string,
	edgeType string, maxDepth uint32, collection string) ([]string, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	visited := map[string]bool{startID: true}
	frontier := []string{startID}
	var result []string

	for depth := uint32(0); depth < maxDepth && len(frontier) > 0; depth++ {
		// Batched NQL: find all neighbors of the frontier in a single query.
		// edgeType is an internal constant, not user input — fmt.Sprintf is safe.
		nql := fmt.Sprintf(`MATCH (a)-[r:%s]-(b) WHERE a.id IN $frontierIDs RETURN b`, edgeType)
		qr, err := ga.client.Query(ctx, nql, map[string]interface{}{
			"frontierIDs": frontier,
		}, col)
		if err != nil {
			break
		}

		var nextFrontier []string
		// Collect from result.Nodes (RETURN b → single-node projection)
		for _, n := range qr.Nodes {
			if n.ID != "" && !visited[n.ID] {
				visited[n.ID] = true
				nextFrontier = append(nextFrontier, n.ID)
				result = append(result, n.ID)
			}
		}
		// Also handle NodePairs in case the executor uses pairs for MATCH queries
		for _, pair := range qr.NodePairs {
			for _, id := range []string{pair.From.ID, pair.To.ID} {
				if id != "" && !visited[id] {
					visited[id] = true
					nextFrontier = append(nextFrontier, id)
					result = append(result, id)
				}
			}
		}
		frontier = nextFrontier
	}

	return result, nil
}

// ── Node CRUD (CREATE/SET/DELETE) ─────────────────────────────────────────────

// InsertNode creates a new node.
func (ga *GraphAdapter) InsertNode(ctx context.Context, opts nietzsche.InsertNodeOpts) (nietzsche.NodeResult, error) {
	if opts.Collection == "" {
		opts.Collection = ga.collection
	}
	return ga.client.InsertNode(ctx, opts)
}

// GetNode retrieves a node by ID.
func (ga *GraphAdapter) GetNode(ctx context.Context, id string, collection string) (nietzsche.NodeResult, error) {
	col := collection
	if col == "" {
		col = ga.collection
	}
	return ga.client.GetNode(ctx, id, col)
}

// DeleteNode removes a node by ID.
func (ga *GraphAdapter) DeleteNode(ctx context.Context, id string, collection string) error {
	col := collection
	if col == "" {
		col = ga.collection
	}
	return ga.client.Delete(ctx, col, id)
}

// UpdateEnergy updates a node's energy field via NietzscheDB.
func (ga *GraphAdapter) UpdateEnergy(ctx context.Context, nodeID string, energy float32, collection string) error {
	col := collection
	if col == "" {
		col = ga.collection
	}
	return ga.client.UpdateEnergy(ctx, nodeID, energy, col)
}

// ── Edge CRUD ────────────────────────────────────────────────────────────────

// InsertEdge creates a new edge.
func (ga *GraphAdapter) InsertEdge(ctx context.Context, opts nietzsche.InsertEdgeOpts) (string, error) {
	if opts.Collection == "" {
		opts.Collection = ga.collection
	}
	return ga.client.InsertEdge(ctx, opts)
}

// DeleteEdge removes an edge by ID.
func (ga *GraphAdapter) DeleteEdge(ctx context.Context, id string, collection string) error {
	col := collection
	if col == "" {
		col = ga.collection
	}
	return ga.client.DeleteEdge(ctx, id, col)
}

// ── Diffuse (spectral community analysis) ────────────────────────────────────

// Diffuse runs heat-kernel diffusion from source nodes.
func (ga *GraphAdapter) Diffuse(ctx context.Context, sourceIDs []string,
	opts nietzsche.DiffuseOpts) ([]nietzsche.DiffusionScale, error) {
	return ga.client.Diffuse(ctx, sourceIDs, opts)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// NowUnix returns current time as Unix float64.
func NowUnix() float64 {
	return float64(time.Now().Unix())
}

// DaysAgoUnix returns Unix timestamp for N days ago.
func DaysAgoUnix(days int) float64 {
	return float64(time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix())
}

// HoursAgoUnix returns Unix timestamp for N hours ago.
func HoursAgoUnix(hours int) float64 {
	return float64(time.Now().Add(-time.Duration(hours) * time.Hour).Unix())
}
