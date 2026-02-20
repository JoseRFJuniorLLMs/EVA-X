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

// GraphAdapter replaces Neo4jClient for graph operations.
// It translates Neo4j Cypher patterns (MERGE, MATCH, variable-length paths)
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

// ── MERGE (replaces Neo4j MERGE) ─────────────────────────────────────────────

// MergeNodeOpts mirrors the options for a Neo4j MERGE on a node.
type MergeNodeOpts struct {
	Collection string
	NodeType   string
	MatchKeys  map[string]interface{}
	OnCreateSet map[string]interface{}
	OnMatchSet  map[string]interface{}
}

// MergeNodeResult is the result of a MergeNode operation.
type MergeNodeResult struct {
	Created bool
	NodeID  string
	Content map[string]interface{}
}

// MergeNode finds or creates a node by type + match keys (Neo4j MERGE equivalent).
func (ga *GraphAdapter) MergeNode(ctx context.Context, opts MergeNodeOpts) (*MergeNodeResult, error) {
	log := logger.Nietzsche()

	col := opts.Collection
	if col == "" {
		col = ga.collection
	}

	result, err := ga.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection: col,
		NodeType:   opts.NodeType,
		MatchKeys:  opts.MatchKeys,
		OnCreateSet: opts.OnCreateSet,
		OnMatchSet:  opts.OnMatchSet,
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

	return &MergeNodeResult{
		Created: result.Created,
		NodeID:  result.NodeID,
		Content: result.Content,
	}, nil
}

// MergeEdgeOpts mirrors the options for a Neo4j MERGE on a relationship.
type MergeEdgeOpts struct {
	Collection string
	FromNodeID string
	ToNodeID   string
	EdgeType   string
	OnCreateSet map[string]interface{}
	OnMatchSet  map[string]interface{}
}

// MergeEdgeResult is the result of a MergeEdge operation.
type MergeEdgeResult struct {
	Created bool
	EdgeID  string
}

// MergeEdge finds or creates an edge (Neo4j MERGE on relationship).
func (ga *GraphAdapter) MergeEdge(ctx context.Context, opts MergeEdgeOpts) (*MergeEdgeResult, error) {
	log := logger.Nietzsche()

	col := opts.Collection
	if col == "" {
		col = ga.collection
	}

	result, err := ga.client.MergeEdge(ctx, nietzsche.MergeEdgeOpts{
		Collection: col,
		FromNodeID: opts.FromNodeID,
		ToNodeID:   opts.ToNodeID,
		EdgeType:   opts.EdgeType,
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

// ── MATCH + WHERE (replaces Neo4j ExecuteRead) ───────────────────────────────

// QueryResult holds the result of an NQL query.
type QueryResult = nietzsche.QueryResult

// ExecuteNQL runs an NQL query against a collection.
// Replaces Neo4j ExecuteRead for simple MATCH queries.
func (ga *GraphAdapter) ExecuteNQL(ctx context.Context, nql string,
	params map[string]interface{}, collection string) (*QueryResult, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	return ga.client.Query(ctx, nql, params, col)
}

// ── BFS (replaces Neo4j *1..N variable-length paths) ─────────────────────────

// Bfs performs breadth-first traversal from a start node.
// Replaces Neo4j patterns like: MATCH (a)-[*1..N]-(b)
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

// BfsWithEdgeType performs BFS filtered by edge type.
// Replaces: MATCH (a)-[:TYPE*1..N]-(b)
func (ga *GraphAdapter) BfsWithEdgeType(ctx context.Context, startID string,
	edgeType string, maxDepth uint32, collection string) ([]string, error) {

	col := collection
	if col == "" {
		col = ga.collection
	}

	return ga.client.Bfs(ctx, startID, nietzsche.TraversalOpts{
		MaxDepth:  maxDepth,
		EdgeLabel: edgeType,
	}, col)
}

// ── Node CRUD (replaces Neo4j CREATE/SET/DELETE) ─────────────────────────────

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

// UpdateEnergy updates a node's energy (replaces Neo4j SET n.energy = ...).
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

// ── Diffuse (replaces Neo4j spectral community analysis) ─────────────────────

// Diffuse runs heat-kernel diffusion from source nodes.
func (ga *GraphAdapter) Diffuse(ctx context.Context, sourceIDs []string,
	opts nietzsche.DiffuseOpts) ([]nietzsche.DiffusionScale, error) {
	return ga.client.Diffuse(ctx, sourceIDs, opts)
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// NowUnix returns current time as Unix float64 (replaces Neo4j datetime()).
func NowUnix() float64 {
	return float64(time.Now().Unix())
}

// DaysAgoUnix returns Unix timestamp for N days ago (replaces Neo4j duration({days: N})).
func DaysAgoUnix(days int) float64 {
	return float64(time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix())
}

// HoursAgoUnix returns Unix timestamp for N hours ago.
func HoursAgoUnix(hours int) float64 {
	return float64(time.Now().Add(-time.Duration(hours) * time.Hour).Unix())
}
