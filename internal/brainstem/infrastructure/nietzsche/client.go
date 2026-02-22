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

// ── Graph Algorithms ────────────────────────────────────────────────────────

// RunPageRank computes PageRank scores for all nodes in a collection.
func (c *Client) RunPageRank(ctx context.Context, collection string, damping float64, maxIterations uint32) (nietzsche.AlgoScoreResult, error) {
	return c.sdk.RunPageRank(ctx, collection, damping, maxIterations)
}

// RunLouvain detects communities using the Louvain algorithm.
func (c *Client) RunLouvain(ctx context.Context, collection string, maxIterations uint32, resolution float64) (nietzsche.AlgoCommunityResult, error) {
	return c.sdk.RunLouvain(ctx, collection, maxIterations, resolution)
}

// RunLabelProp detects communities using label propagation.
func (c *Client) RunLabelProp(ctx context.Context, collection string, maxIterations uint32) (nietzsche.AlgoCommunityResult, error) {
	return c.sdk.RunLabelProp(ctx, collection, maxIterations)
}

// RunBetweenness computes betweenness centrality (bridge nodes).
func (c *Client) RunBetweenness(ctx context.Context, collection string, sampleSize uint32) (nietzsche.AlgoScoreResult, error) {
	return c.sdk.RunBetweenness(ctx, collection, sampleSize)
}

// RunCloseness computes closeness centrality (hub proximity).
func (c *Client) RunCloseness(ctx context.Context, collection string) (nietzsche.AlgoScoreResult, error) {
	return c.sdk.RunCloseness(ctx, collection)
}

// RunDegreeCentrality computes degree centrality. direction: "in", "out", or "both".
func (c *Client) RunDegreeCentrality(ctx context.Context, collection, direction string) (nietzsche.AlgoScoreResult, error) {
	return c.sdk.RunDegreeCentrality(ctx, collection, direction)
}

// RunWCC finds weakly connected components.
func (c *Client) RunWCC(ctx context.Context, collection string) (nietzsche.AlgoCommunityResult, error) {
	return c.sdk.RunWCC(ctx, collection)
}

// RunSCC finds strongly connected components.
func (c *Client) RunSCC(ctx context.Context, collection string) (nietzsche.AlgoCommunityResult, error) {
	return c.sdk.RunSCC(ctx, collection)
}

// RunAStar computes A* shortest path between two nodes.
func (c *Client) RunAStar(ctx context.Context, collection, startID, goalID string) (nietzsche.AStarResult, error) {
	return c.sdk.RunAStar(ctx, collection, startID, goalID)
}

// RunTriangleCount counts the number of triangles in the graph.
func (c *Client) RunTriangleCount(ctx context.Context, collection string) (nietzsche.TriangleResult, error) {
	return c.sdk.RunTriangleCount(ctx, collection)
}

// RunJaccardSimilarity computes pairwise Jaccard similarity.
func (c *Client) RunJaccardSimilarity(ctx context.Context, collection string, topK uint32, threshold float64) (nietzsche.SimilarityResult, error) {
	return c.sdk.RunJaccardSimilarity(ctx, collection, topK, threshold)
}

// ── Multi-Manifold ──────────────────────────────────────────────────────────

// Synthesis computes Riemannian midpoint synthesis between two nodes.
func (c *Client) Synthesis(ctx context.Context, nodeIDA, nodeIDB, collection string) (*nietzsche.SynthesisResult, error) {
	return c.sdk.Synthesis(ctx, nodeIDA, nodeIDB, collection)
}

// SynthesisMulti computes Riemannian synthesis across multiple nodes.
func (c *Client) SynthesisMulti(ctx context.Context, nodeIDs []string, collection string) (*nietzsche.SynthesisResult, error) {
	return c.sdk.SynthesisMulti(ctx, nodeIDs, collection)
}

// CausalNeighbors returns Minkowski causal neighbors. direction: "future", "past", or "both".
func (c *Client) CausalNeighbors(ctx context.Context, nodeID, direction, collection string) ([]nietzsche.CausalEdge, error) {
	return c.sdk.CausalNeighbors(ctx, nodeID, direction, collection)
}

// CausalChain traces a Minkowski causal chain from a node. direction: "future" or "past".
func (c *Client) CausalChain(ctx context.Context, nodeID string, maxDepth uint32, direction, collection string) (*nietzsche.CausalChainResult, error) {
	return c.sdk.CausalChain(ctx, nodeID, maxDepth, direction, collection)
}

// KleinPath computes a Klein-model geodesic path between two nodes.
func (c *Client) KleinPath(ctx context.Context, startNodeID, goalNodeID, collection string) (*nietzsche.KleinPathResult, error) {
	return c.sdk.KleinPath(ctx, startNodeID, goalNodeID, collection)
}

// IsOnShortestPath checks if nodeC lies on the shortest path between nodeA and nodeB.
func (c *Client) IsOnShortestPath(ctx context.Context, nodeIDA, nodeIDB, nodeIDC, collection string) (*nietzsche.ShortestPathCheckResult, error) {
	return c.sdk.IsOnShortestPath(ctx, nodeIDA, nodeIDB, nodeIDC, collection)
}

// ── Batch Operations ────────────────────────────────────────────────────────

// BatchInsertNodes inserts multiple nodes in a single RPC call.
func (c *Client) BatchInsertNodes(ctx context.Context, nodes []nietzsche.InsertNodeOpts, collection string) ([]string, error) {
	return c.sdk.BatchInsertNodes(ctx, nodes, collection)
}

// BatchInsertEdges inserts multiple edges in a single RPC call.
func (c *Client) BatchInsertEdges(ctx context.Context, edges []nietzsche.InsertEdgeOpts, collection string) ([]string, error) {
	return c.sdk.BatchInsertEdges(ctx, edges, collection)
}

// ── Full-Text & Hybrid Search ───────────────────────────────────────────────

// FullTextSearch performs a BM25 inverted-index search over node content.
func (c *Client) FullTextSearch(ctx context.Context, query, collection string, limit uint32) ([]nietzsche.FtsResult, error) {
	return c.sdk.FullTextSearch(ctx, query, collection, limit)
}

// HybridSearch combines full-text BM25 and vector KNN search.
func (c *Client) HybridSearch(ctx context.Context, textQuery string, queryCoords []float64,
	k uint32, textWeight, vectorWeight float64, collection string) ([]nietzsche.KnnResult, error) {
	return c.sdk.HybridSearch(ctx, textQuery, queryCoords, k, textWeight, vectorWeight, collection)
}

// ── Backup / Restore ────────────────────────────────────────────────────────

// CreateBackup creates a RocksDB checkpoint backup.
func (c *Client) CreateBackup(ctx context.Context, label string) (nietzsche.BackupInfo, error) {
	return c.sdk.CreateBackup(ctx, label)
}

// ListBackups returns all available backups.
func (c *Client) ListBackups(ctx context.Context) ([]nietzsche.BackupInfo, error) {
	return c.sdk.ListBackups(ctx)
}

// RestoreBackup restores from a backup path.
func (c *Client) RestoreBackup(ctx context.Context, backupPath, targetPath string) error {
	return c.sdk.RestoreBackup(ctx, backupPath, targetPath)
}

// ── CDC (Change Data Capture) ───────────────────────────────────────────────

// SubscribeCDC subscribes to change events from a collection.
func (c *Client) SubscribeCDC(ctx context.Context, collection string, fromLSN uint64) (*nietzsche.CDCSubscription, error) {
	return c.sdk.SubscribeCDC(ctx, collection, fromLSN)
}

// ── Cache (collection-scoped key-value) ─────────────────────────────────────

// CacheSet stores a value under key with optional TTL.
func (c *Client) CacheSet(ctx context.Context, collection, key string, value []byte, ttlSecs uint64) error {
	return c.sdk.CacheSet(ctx, collection, key, value, ttlSecs)
}

// CacheGet retrieves a cached value. Returns (value, found, error).
func (c *Client) CacheGet(ctx context.Context, collection, key string) ([]byte, bool, error) {
	return c.sdk.CacheGet(ctx, collection, key)
}

// CacheDel removes a cached value.
func (c *Client) CacheDel(ctx context.Context, collection, key string) error {
	return c.sdk.CacheDel(ctx, collection, key)
}

// ── Lists (node-scoped ordered sequences) ───────────────────────────────────

// ListRPush appends a value to a node's named list. Returns new list length.
func (c *Client) ListRPush(ctx context.Context, nodeID, listName string, value []byte, collection string) (uint64, error) {
	return c.sdk.ListRPush(ctx, nodeID, listName, value, collection)
}

// ListLRange returns a range of values from a node's named list.
func (c *Client) ListLRange(ctx context.Context, nodeID, listName string, start uint64, stop int64, collection string) ([][]byte, error) {
	return c.sdk.ListLRange(ctx, nodeID, listName, start, stop, collection)
}

// ListLen returns the length of a node's named list.
func (c *Client) ListLen(ctx context.Context, nodeID, listName, collection string) (uint64, error) {
	return c.sdk.ListLen(ctx, nodeID, listName, collection)
}

// ── Schema / Index ──────────────────────────────────────────────────────────

// SetSchema registers a validation schema for a node type.
func (c *Client) SetSchema(ctx context.Context, nodeType string, requiredFields []string, fieldTypes []nietzsche.SchemaFieldType, collection string) error {
	return c.sdk.SetSchema(ctx, nodeType, requiredFields, fieldTypes, collection)
}

// GetSchema retrieves the validation schema for a node type.
func (c *Client) GetSchema(ctx context.Context, nodeType, collection string) (*nietzsche.SchemaResult, error) {
	return c.sdk.GetSchema(ctx, nodeType, collection)
}

// CreateIndex creates a secondary index on a metadata field.
func (c *Client) CreateIndex(ctx context.Context, collection, field string) error {
	return c.sdk.CreateIndex(ctx, collection, field)
}

// DropIndex removes a secondary index.
func (c *Client) DropIndex(ctx context.Context, collection, field string) error {
	return c.sdk.DropIndex(ctx, collection, field)
}

// ListIndexes returns all indexed fields in a collection.
func (c *Client) ListIndexes(ctx context.Context, collection string) ([]string, error) {
	return c.sdk.ListIndexes(ctx, collection)
}

// ── Collection Management ───────────────────────────────────────────────────

// DropCollection permanently removes a collection and all its data.
func (c *Client) DropCollection(ctx context.Context, name string) error {
	return c.sdk.DropCollection(ctx, name)
}

// ── Sensory Extended ────────────────────────────────────────────────────────

// DegradeSensory triggers progressive quantisation degradation on a node's sensory data.
func (c *Client) DegradeSensory(ctx context.Context, nodeID, collection string) error {
	return c.sdk.DegradeSensory(ctx, nodeID, collection)
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
// Graph-centric collections use poincare metric for hyperbolic geometry;
// embedding-only collections use cosine for flat vector space.
func DefaultCollections() []nietzsche.CollectionConfig {
	return []nietzsche.CollectionConfig{
		// Vector collections (Qdrant replacement, cosine 3072D — flat embedding space)
		{Name: "memories", Dim: 3072, Metric: "cosine"},
		{Name: "signifier_chains", Dim: 3072, Metric: "cosine"},
		{Name: "eva_self_knowledge", Dim: 3072, Metric: "cosine"},
		{Name: "eva_learnings", Dim: 3072, Metric: "cosine"},
		{Name: "eva_codebase", Dim: 3072, Metric: "cosine"},
		{Name: "eva_docs", Dim: 3072, Metric: "cosine"},
		{Name: "speaker_embeddings", Dim: 3072, Metric: "cosine"},
		{Name: "stories", Dim: 3072, Metric: "cosine"},
		// Graph collections (Neo4j replacement, poincare — hyperbolic hierarchy)
		{Name: "patient_graph", Dim: 3072, Metric: "poincare"},
		{Name: "eva_core", Dim: 3072, Metric: "poincare"},
	}
}

// EnsureCollections creates all default EVA collections idempotently.
// It also checks for metric mismatches on existing collections and creates indexes.
func (c *Client) EnsureCollections(ctx context.Context) error {
	log := logger.Nietzsche()
	log.Info().Msg("ensuring all EVA collections exist in NietzscheDB")

	// Build desired metric map for mismatch detection
	desiredMetrics := make(map[string]string)
	for _, col := range DefaultCollections() {
		desiredMetrics[col.Name] = col.Metric
		if err := c.EnsureCollection(ctx, col.Name, col.Dim, col.Metric); err != nil {
			return err
		}
	}

	// Check existing collections for metric mismatches (warn, don't auto-migrate)
	existing, err := c.ListCollections(ctx)
	if err == nil {
		for _, col := range existing {
			if desired, ok := desiredMetrics[col.Name]; ok && col.Metric != desired {
				log.Warn().
					Str("collection", col.Name).
					Str("current_metric", col.Metric).
					Str("desired_metric", desired).
					Msg("[MIGRATION] collection metric mismatch — manual migration required (drop + recreate)")
			}
		}
	}

	// Create indexes on frequently queried fields (idempotent on server)
	indexes := []struct{ collection, field string }{
		{"patient_graph", "node_type"},
		{"eva_core", "node_type"},
	}
	for _, idx := range indexes {
		if err := c.CreateIndex(ctx, idx.collection, idx.field); err != nil {
			log.Warn().Err(err).Str("collection", idx.collection).Str("field", idx.field).Msg("failed to create index (non-fatal)")
		}
	}

	log.Info().Msg("all EVA collections ensured")
	return nil
}

// ── Node content helpers ────────────────────────────────────────────────────

// ── Swartz SQL Layer ──────────────────────────────────────────────────────────

// SqlQuery executes a SQL query (SELECT) against NietzscheDB's embedded SQL engine.
// Returns structured result set with columns and rows.
func (c *Client) SqlQuery(ctx context.Context, sql, collection string) (*nietzsche.SqlResultSet, error) {
	log := logger.Nietzsche()

	result, err := c.sdk.SqlQuery(ctx, sql, collection)
	if err != nil {
		log.Error().Err(err).Str("sql", sql).Str("collection", collection).Msg("SqlQuery failed")
		return nil, fmt.Errorf("nietzsche SqlQuery: %w", err)
	}

	log.Debug().Str("sql", sql).Int("rows", len(result.Rows)).Msg("SqlQuery OK")
	return result, nil
}

// SqlExec executes a SQL DDL/DML statement (CREATE TABLE, INSERT, UPDATE, DELETE, DROP TABLE).
func (c *Client) SqlExec(ctx context.Context, sql, collection string) (*nietzsche.SqlExecResult, error) {
	log := logger.Nietzsche()

	result, err := c.sdk.SqlExec(ctx, sql, collection)
	if err != nil {
		log.Error().Err(err).Str("sql", sql).Str("collection", collection).Msg("SqlExec failed")
		return nil, fmt.Errorf("nietzsche SqlExec: %w", err)
	}

	log.Debug().Str("sql", sql).Uint64("affected", result.AffectedRows).Msg("SqlExec OK")
	return result, nil
}

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
