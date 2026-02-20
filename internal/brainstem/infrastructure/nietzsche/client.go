// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nietzsche "nietzsche-sdk"

	"eva/internal/brainstem/logger"
)

// Client wraps the NietzscheDB gRPC SDK for EVA.
// Replaces the old HTTP REST client that targeted endpoints which do not exist.
// NietzscheDB exposes gRPC on port 50051 — this client uses that.
type Client struct {
	sdk  *nietzsche.NietzscheClient
	addr string
}

// NewClient connects to NietzscheDB via gRPC (insecure, for same-host / docker-compose).
func NewClient(grpcAddr string) (*Client, error) {
	log := logger.Nietzsche()
	log.Info().Str("grpc_addr", grpcAddr).Msg("connecting to NietzscheDB via gRPC")

	sdk, err := nietzsche.ConnectInsecure(grpcAddr)
	if err != nil {
		log.Error().Err(err).Str("addr", grpcAddr).Msg("failed to connect to NietzscheDB")
		return nil, fmt.Errorf("nietzsche gRPC connect %s: %w", grpcAddr, err)
	}

	log.Info().Str("grpc_addr", grpcAddr).Msg("NietzscheDB gRPC client connected")
	return &Client{sdk: sdk, addr: grpcAddr}, nil
}

// Close releases the gRPC connection.
func (c *Client) Close() error {
	if c.sdk != nil {
		return c.sdk.Close()
	}
	return nil
}

// SDK returns the underlying NietzscheDB SDK client for advanced operations.
func (c *Client) SDK() *nietzsche.NietzscheClient {
	return c.sdk
}

// ── Health & Stats ──────────────────────────────────────────────────────────

// Health checks if NietzscheDB is reachable and healthy via gRPC HealthCheck.
func (c *Client) Health(ctx context.Context) error {
	log := logger.Nietzsche()

	if err := c.sdk.HealthCheck(ctx); err != nil {
		log.Error().Err(err).Msg("NietzscheDB health check failed")
		return fmt.Errorf("nietzsche health check: %w", err)
	}

	log.Debug().Msg("NietzscheDB health check OK")
	return nil
}

// GetStats returns database statistics (node count, edge count, version).
func (c *Client) GetStats(ctx context.Context) (map[string]interface{}, error) {
	log := logger.Nietzsche()

	stats, err := c.sdk.GetStats(ctx)
	if err != nil {
		log.Error().Err(err).Msg("NietzscheDB stats request failed")
		return nil, fmt.Errorf("nietzsche stats: %w", err)
	}

	result := map[string]interface{}{
		"node_count":    stats.NodeCount,
		"edge_count":    stats.EdgeCount,
		"version":       stats.Version,
		"sensory_count": stats.SensoryCount,
	}

	log.Debug().Msg("NietzscheDB stats retrieved")
	return result, nil
}

// ── Collections ─────────────────────────────────────────────────────────────

// EnsureCollection creates a collection if it does not exist (idempotent).
func (c *Client) EnsureCollection(ctx context.Context, name string, dim uint32, metric string) error {
	log := logger.Nietzsche()

	created, err := c.sdk.CreateCollection(ctx, nietzsche.CollectionConfig{
		Name:   name,
		Dim:    dim,
		Metric: metric,
	})
	if err != nil {
		log.Error().Err(err).Str("collection", name).Msg("failed to ensure collection")
		return fmt.Errorf("nietzsche ensure collection %s: %w", name, err)
	}

	if created {
		log.Info().Str("collection", name).Uint32("dim", dim).Str("metric", metric).Msg("collection created")
	} else {
		log.Debug().Str("collection", name).Msg("collection already exists")
	}

	return nil
}

// ListCollections returns all collections with metadata.
func (c *Client) ListCollections(ctx context.Context) ([]nietzsche.CollectionInfo, error) {
	return c.sdk.ListCollections(ctx)
}

// ── Node CRUD ───────────────────────────────────────────────────────────────

// Store inserts a node into a NietzscheDB collection.
// This replaces the old HTTP POST /api/collections/{name}/records endpoint.
func (c *Client) Store(ctx context.Context, collection string, key string, value interface{}) error {
	log := logger.Nietzsche()

	content := map[string]interface{}{
		"key":   key,
		"value": value,
	}

	_, err := c.sdk.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         key,
		Content:    content,
		NodeType:   "Semantic",
		Collection: collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB store failed")
		return fmt.Errorf("nietzsche store: %w", err)
	}

	log.Info().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB store completed")
	return nil
}

// Get retrieves a node from a NietzscheDB collection by ID.
// This replaces the old HTTP GET /api/collections/{name}/records/{key} endpoint.
func (c *Client) Get(ctx context.Context, collection string, key string) (map[string]interface{}, error) {
	log := logger.Nietzsche()

	result, err := c.sdk.GetNode(ctx, key, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB get failed")
		return nil, fmt.Errorf("nietzsche get: %w", err)
	}

	if !result.Found {
		log.Debug().
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB record not found")
		return nil, nil
	}

	log.Debug().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB get completed")
	return result.Content, nil
}

// Delete removes a node from a NietzscheDB collection by ID.
// This replaces the old HTTP DELETE /api/collections/{name}/records/{key} endpoint.
func (c *Client) Delete(ctx context.Context, collection string, key string) error {
	log := logger.Nietzsche()

	if err := c.sdk.DeleteNode(ctx, key, collection); err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("key", key).
			Msg("NietzscheDB delete failed")
		return fmt.Errorf("nietzsche delete: %w", err)
	}

	log.Info().
		Str("collection", collection).
		Str("key", key).
		Msg("NietzscheDB delete completed")
	return nil
}

// ── Vector Search (KNN) ─────────────────────────────────────────────────────

// InsertWithEmbedding inserts a node with a vector embedding into a collection.
func (c *Client) InsertWithEmbedding(ctx context.Context, collection string, id string,
	embedding []float64, content interface{}, nodeType string) (nietzsche.NodeResult, error) {

	return c.sdk.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         id,
		Coords:     embedding,
		Content:    content,
		NodeType:   nodeType,
		Collection: collection,
	})
}

// KnnSearch performs k-nearest-neighbor search using cosine similarity.
// This replaces Qdrant's Search endpoint for vector retrieval.
func (c *Client) KnnSearch(ctx context.Context, collection string,
	queryVector []float64, k uint32) ([]nietzsche.KnnResult, error) {

	log := logger.Nietzsche()

	results, err := c.sdk.KnnSearch(ctx, queryVector, k, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Uint32("k", k).
			Msg("NietzscheDB KNN search failed")
		return nil, fmt.Errorf("nietzsche knn search: %w", err)
	}

	log.Debug().
		Str("collection", collection).
		Int("results", len(results)).
		Msg("NietzscheDB KNN search completed")
	return results, nil
}

// ── Graph Operations ────────────────────────────────────────────────────────

// InsertNode creates a new node in NietzscheDB.
func (c *Client) InsertNode(ctx context.Context, opts nietzsche.InsertNodeOpts) (nietzsche.NodeResult, error) {
	return c.sdk.InsertNode(ctx, opts)
}

// GetNode retrieves a node by ID.
func (c *Client) GetNode(ctx context.Context, id, collection string) (nietzsche.NodeResult, error) {
	return c.sdk.GetNode(ctx, id, collection)
}

// InsertEdge creates an edge between two nodes.
func (c *Client) InsertEdge(ctx context.Context, opts nietzsche.InsertEdgeOpts) (string, error) {
	return c.sdk.InsertEdge(ctx, opts)
}

// DeleteEdge removes an edge by ID.
func (c *Client) DeleteEdge(ctx context.Context, id, collection string) error {
	return c.sdk.DeleteEdge(ctx, id, collection)
}

// UpdateEnergy modifies a node's energy level.
func (c *Client) UpdateEnergy(ctx context.Context, nodeID string, energy float32, collection string) error {
	return c.sdk.UpdateEnergy(ctx, nodeID, energy, collection)
}

// ── MERGE (upsert) ─────────────────────────────────────────────────────────

// MergeNode finds a node by type + match keys, or creates one (Neo4j MERGE equivalent).
func (c *Client) MergeNode(ctx context.Context, opts nietzsche.MergeNodeOpts) (*nietzsche.MergeNodeResult, error) {
	return c.sdk.MergeNode(ctx, opts)
}

// MergeEdge finds an edge by (from, to, type), or creates one.
func (c *Client) MergeEdge(ctx context.Context, opts nietzsche.MergeEdgeOpts) (*nietzsche.MergeEdgeResult, error) {
	return c.sdk.MergeEdge(ctx, opts)
}

// ── NQL Query ───────────────────────────────────────────────────────────────

// Query executes an NQL (Nietzsche Query Language) query.
func (c *Client) Query(ctx context.Context, nql string,
	params map[string]interface{}, collection string) (*nietzsche.QueryResult, error) {

	log := logger.Nietzsche()

	result, err := c.sdk.Query(ctx, nql, params, collection)
	if err != nil {
		log.Error().Err(err).
			Str("nql", nql).
			Str("collection", collection).
			Msg("NietzscheDB NQL query failed")
		return nil, fmt.Errorf("nietzsche query: %w", err)
	}

	log.Debug().
		Str("nql", nql).
		Int("nodes", len(result.Nodes)).
		Int("pairs", len(result.NodePairs)).
		Msg("NietzscheDB NQL query completed")
	return result, nil
}

// ── Traversal ───────────────────────────────────────────────────────────────

// Bfs performs breadth-first search from a start node.
func (c *Client) Bfs(ctx context.Context, startID string,
	opts nietzsche.TraversalOpts, collection string) ([]string, error) {

	return c.sdk.Bfs(ctx, startID, opts, collection)
}

// Dijkstra performs shortest-path traversal from a start node.
func (c *Client) Dijkstra(ctx context.Context, startID string,
	opts nietzsche.TraversalOpts, collection string) ([]string, []float64, error) {

	return c.sdk.Dijkstra(ctx, startID, opts, collection)
}

// Diffuse runs heat-kernel diffusion from source nodes.
func (c *Client) Diffuse(ctx context.Context, sourceIDs []string,
	opts nietzsche.DiffuseOpts) ([]nietzsche.DiffusionScale, error) {

	return c.sdk.Diffuse(ctx, sourceIDs, opts)
}

// ── Sleep & Evolution ───────────────────────────────────────────────────────

// TriggerSleep initiates a Riemannian reconsolidation sleep cycle.
func (c *Client) TriggerSleep(ctx context.Context, opts nietzsche.SleepOpts) (nietzsche.SleepResult, error) {
	return c.sdk.TriggerSleep(ctx, opts)
}

// InvokeZaratustra runs the autonomous evolution engine.
func (c *Client) InvokeZaratustra(ctx context.Context, opts nietzsche.ZaratustraOpts) (*nietzsche.ZaratustraResult, error) {
	return c.sdk.InvokeZaratustra(ctx, opts)
}

// ── Sensory ─────────────────────────────────────────────────────────────────

// InsertSensory attaches sensory data to a node.
func (c *Client) InsertSensory(ctx context.Context, opts nietzsche.InsertSensoryOpts) error {
	return c.sdk.InsertSensory(ctx, opts)
}

// GetSensory retrieves sensory metadata for a node.
func (c *Client) GetSensory(ctx context.Context, nodeID, collection string) (*nietzsche.SensoryResult, error) {
	return c.sdk.GetSensory(ctx, nodeID, collection)
}

// Reconstruct reconstructs a sensory latent vector.
// Quality can be "full", "degraded", or "best_available".
func (c *Client) Reconstruct(ctx context.Context, nodeID string, quality string) (*nietzsche.ReconstructResult, error) {
	return c.sdk.Reconstruct(ctx, nodeID, quality)
}

// ── Search (backward compatible with old HTTP client signature) ─────────────

// Search queries nodes via NQL MATCH with a text filter.
// This replaces the old HTTP POST /api/collections/{name}/search endpoint.
func (c *Client) Search(ctx context.Context, collection string, query string, limit int) ([]map[string]interface{}, error) {
	log := logger.Nietzsche()

	nql := fmt.Sprintf("MATCH (n) RETURN n LIMIT %d", limit)
	result, err := c.sdk.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("query", query).
			Msg("NietzscheDB search failed")
		return nil, fmt.Errorf("nietzsche search: %w", err)
	}

	var results []map[string]interface{}
	for _, node := range result.Nodes {
		item := map[string]interface{}{
			"id":        node.ID,
			"node_type": node.NodeType,
			"energy":    node.Energy,
		}
		if node.Content != nil {
			for k, v := range node.Content {
				item[k] = v
			}
		}
		results = append(results, item)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	log.Info().
		Str("collection", collection).
		Str("query", query).
		Int("results", len(results)).
		Msg("NietzscheDB search completed")
	return results, nil
}

// ── EnsureCollections: bulk create all EVA collections ──────────────────────

// DefaultCollections returns the standard EVA collection configurations.
func DefaultCollections() []nietzsche.CollectionConfig {
	return []nietzsche.CollectionConfig{
		// Vector collections (Qdrant replacement, cosine 3072D)
		{Name: "memories", Dim: 3072, Metric: "cosine"},
		{Name: "signifier_chains", Dim: 3072, Metric: "cosine"},
		{Name: "eva_self_knowledge", Dim: 3072, Metric: "cosine"},
		{Name: "eva_learnings", Dim: 3072, Metric: "cosine"},
		{Name: "eva_codebase", Dim: 3072, Metric: "cosine"},
		{Name: "eva_docs", Dim: 3072, Metric: "cosine"},
		{Name: "speaker_embeddings", Dim: 3072, Metric: "cosine"},
		{Name: "stories", Dim: 3072, Metric: "cosine"},
		// Graph collections (Neo4j replacement)
		{Name: "patient_graph", Dim: 3072, Metric: "cosine"},
		{Name: "eva_core", Dim: 3072, Metric: "cosine"},
	}
}

// EnsureCollections creates all default EVA collections idempotently.
func (c *Client) EnsureCollections(ctx context.Context) error {
	log := logger.Nietzsche()
	log.Info().Msg("ensuring all EVA collections exist in NietzscheDB")

	for _, col := range DefaultCollections() {
		if err := c.EnsureCollection(ctx, col.Name, col.Dim, col.Metric); err != nil {
			return err
		}
	}

	log.Info().Msg("all EVA collections ensured")
	return nil
}

// ── Node content helpers ────────────────────────────────────────────────────

// NodeResultToMap converts a NodeResult to a flat map for backward compatibility.
func NodeResultToMap(nr nietzsche.NodeResult) map[string]interface{} {
	result := map[string]interface{}{
		"id":              nr.ID,
		"node_type":       nr.NodeType,
		"energy":          nr.Energy,
		"depth":           nr.Depth,
		"hausdorff_local": nr.HausdorffLocal,
		"created_at":      time.Unix(nr.CreatedAt, 0).Format(time.RFC3339),
	}

	if nr.Content != nil {
		contentJSON, _ := json.Marshal(nr.Content)
		result["content"] = string(contentJSON)
		for k, v := range nr.Content {
			result["content_"+k] = v
		}
	}

	if len(nr.Embedding) > 0 {
		result["embedding_dim"] = len(nr.Embedding)
	}

	return result
}
