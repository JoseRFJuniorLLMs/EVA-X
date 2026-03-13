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

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Client wraps the NietzscheDB gRPC SDK for EVA.
// Replaces the old HTTP REST client that targeted endpoints which do not exist.
// NietzscheDB exposes gRPC on port 50051 — this client uses that.
type Client struct {
	sdk  *nietzsche.NietzscheClient
	addr string
}

// NewClient connects to NietzscheDB via gRPC with auto-reconnect, keepalive, and retry.
// When NietzscheDB restarts (e.g., after OOM), the gRPC connection automatically reconnects
// without requiring EVA-X restart. Uses exponential backoff (500ms-5s) and keepalive pings.
func NewClient(grpcAddr string) (*Client, error) {
	log := logger.Nietzsche()
	log.Info().Str("grpc_addr", grpcAddr).Msg("connecting to NietzscheDB via gRPC (with auto-reconnect)")

	sdk, err := nietzsche.Connect(grpcAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// Keepalive: ping every 10s, timeout after 5s, allow pings without active RPCs
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		// Fast reconnect backoff: 500ms base, 1.6x multiplier, max 5s
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  500 * time.Millisecond,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   5 * time.Second,
			},
			MinConnectTimeout: 3 * time.Second,
		}),
		// Retry policy: auto-retry on UNAVAILABLE (server restart) and RESOURCE_EXHAUSTED
		grpc.WithDefaultServiceConfig(`{
			"methodConfig": [{
				"name": [{"service": "nietzschedb.NietzscheDB"}],
				"waitForReady": true,
				"retryPolicy": {
					"maxAttempts": 3,
					"initialBackoff": "0.2s",
					"maxBackoff": "2s",
					"backoffMultiplier": 2,
					"retryableStatusCodes": ["UNAVAILABLE","RESOURCE_EXHAUSTED"]
				}
			}]
		}`),
	)
	if err != nil {
		log.Error().Err(err).Str("addr", grpcAddr).Msg("failed to connect to NietzscheDB")
		return nil, fmt.Errorf("nietzsche gRPC connect %s: %w", grpcAddr, err)
	}

	log.Info().Str("grpc_addr", grpcAddr).Msg("NietzscheDB gRPC client connected (auto-reconnect enabled)")
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
// Routes through c.InsertNode (not c.sdk.InsertNode) so that NormalizeNodeType
// and node_label injection are applied consistently.
func (c *Client) InsertWithEmbedding(ctx context.Context, collection string, id string,
	embedding []float64, content interface{}, nodeType string) (nietzsche.NodeResult, error) {

	// Ensure content is never nil — empty map is safer than null in storage.
	if content == nil {
		content = map[string]interface{}{}
	}

	return c.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         id,
		Coords:     embedding,
		Content:    content,
		NodeType:   nodeType,
		Collection: collection,
	})
}

// KnnSearch performs k-nearest-neighbor search using cosine similarity.
// Performs KNN search via NietzscheDB gRPC for vector retrieval.
func (c *Client) KnnSearch(ctx context.Context, collection string,
	queryVector []float64, k uint32) ([]nietzsche.KnnResult, error) {
	return c.KnnSearchFiltered(ctx, collection, queryVector, k, nil)
}

// KnnSearchFiltered performs a KNN search with server-side metadata filters.
// Filters use AND semantics. Pass nil for unfiltered search.
func (c *Client) KnnSearchFiltered(ctx context.Context, collection string,
	queryVector []float64, k uint32, filters []nietzsche.KnnFilter) ([]nietzsche.KnnResult, error) {

	log := logger.Nietzsche()

	results, err := c.sdk.KnnSearchFiltered(ctx, queryVector, k, collection, filters)
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
// Custom node types (e.g. "Person") are normalized to "Semantic" with a
// node_label content field injected automatically.
func (c *Client) InsertNode(ctx context.Context, opts nietzsche.InsertNodeOpts) (nietzsche.NodeResult, error) {
	// Guard against nil content — storing null in NietzscheDB makes nodes
	// invisible to full-text search and metadata filters.
	if opts.Content == nil {
		opts.Content = map[string]interface{}{}
	}

	normalized, isCustom := NormalizeNodeType(opts.NodeType)
	if isCustom {
		log := logger.Nietzsche()
		log.Debug().
			Str("original_type", opts.NodeType).
			Str("normalized_type", normalized).
			Msg("InsertNode: custom type normalized to Semantic + node_label")

		originalType := opts.NodeType
		opts.NodeType = normalized

		// Inject node_label into content (Content is interface{})
		switch c := opts.Content.(type) {
		case map[string]interface{}:
			c["node_label"] = originalType
			opts.Content = c
		case nil:
			opts.Content = map[string]interface{}{"node_label": originalType}
		default:
			// Content is a struct or other type — wrap in map
			opts.Content = map[string]interface{}{
				"node_label": originalType,
				"_data":      c,
			}
		}
	}
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

// MergeNode finds a node by type + match keys, or creates one (MERGE semantics).
// Custom node types (e.g. "Person") are normalized to "Semantic" with node_label
// injected into MatchKeys and OnCreateSet automatically.
func (c *Client) MergeNode(ctx context.Context, opts nietzsche.MergeNodeOpts) (*nietzsche.MergeNodeResult, error) {
	normalized, isCustom := NormalizeNodeType(opts.NodeType)
	if isCustom {
		log := logger.Nietzsche()
		log.Debug().
			Str("original_type", opts.NodeType).
			Str("normalized_type", normalized).
			Msg("MergeNode: custom type normalized to Semantic + node_label")

		originalType := opts.NodeType
		opts.NodeType = normalized

		// Inject node_label into MatchKeys so MERGE can find the right node
		if opts.MatchKeys == nil {
			opts.MatchKeys = make(map[string]interface{})
		}
		opts.MatchKeys["node_label"] = originalType

		// Inject node_label into OnCreateSet so new nodes get the label
		if opts.OnCreateSet == nil {
			opts.OnCreateSet = make(map[string]interface{})
		}
		opts.OnCreateSet["node_label"] = originalType
	}
	return c.sdk.MergeNode(ctx, opts)
}

// MergeEdge finds an edge by (from, to, type), or creates one.
func (c *Client) MergeEdge(ctx context.Context, opts nietzsche.MergeEdgeOpts) (*nietzsche.MergeEdgeResult, error) {
	return c.sdk.MergeEdge(ctx, opts)
}

// IncrementEdgeMeta atomically increments a numeric metadata field on an edge.
func (c *Client) IncrementEdgeMeta(ctx context.Context, opts nietzsche.IncrementEdgeMetaOpts) (float64, error) {
	return c.sdk.IncrementEdgeMeta(ctx, opts)
}

// ── Wiederkehr Daemons ─────────────────────────────────────────────────────

// CreateDaemon registers a new Wiederkehr Daemon.
func (c *Client) CreateDaemon(ctx context.Context, opts nietzsche.CreateDaemonOpts) error {
	return c.sdk.CreateDaemon(ctx, opts)
}

// ListDaemons returns all daemons for a collection.
func (c *Client) ListDaemons(ctx context.Context, collection string) ([]nietzsche.DaemonInfo, error) {
	return c.sdk.ListDaemons(ctx, collection)
}

// DropDaemon removes a registered daemon.
func (c *Client) DropDaemon(ctx context.Context, collection, label string) error {
	return c.sdk.DropDaemon(ctx, collection, label)
}

// ── NQL Query ───────────────────────────────────────────────────────────────

// Query executes an NQL (Nietzsche Query Language) query.
// Custom node type labels (e.g. Person, Zettel) are transparently rewritten
// to Semantic + node_label filter via RewriteNQL.
func (c *Client) Query(ctx context.Context, nql string,
	params map[string]interface{}, collection string) (*nietzsche.QueryResult, error) {

	log := logger.Nietzsche()

	// Transparently rewrite custom types → Semantic + node_label WHERE
	rewritten := RewriteNQL(nql)
	if rewritten != nql {
		log.Debug().
			Str("original", nql).
			Str("rewritten", rewritten).
			Msg("NQL rewriter transformed custom types")
	}

	result, err := c.sdk.Query(ctx, rewritten, params, collection)
	if err != nil {
		log.Error().Err(err).
			Str("nql", rewritten).
			Str("original_nql", nql).
			Str("collection", collection).
			Msg("NietzscheDB NQL query failed")
		return nil, fmt.Errorf("nietzsche query: %w", err)
	}

	// Check result-level error (server returned OK but query had semantic error)
	if result.Error != "" {
		log.Warn().
			Str("nql", rewritten).
			Str("error", result.Error).
			Msg("NietzscheDB NQL query returned error")
	}

	log.Debug().
		Str("nql", rewritten).
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

// ── Neural Operations (Phase 5) ──────────────────────────────────────────

// GnnInfer performs a GNN inference on a list of nodes.
func (c *Client) GnnInfer(ctx context.Context, opts nietzsche.GnnInferOpts) (nietzsche.GnnInferResult, error) {
	return c.sdk.GnnInfer(ctx, opts)
}

// MctsSearch performs a Monte Carlo Tree Search for the best action.
func (c *Client) MctsSearch(ctx context.Context, opts nietzsche.MctsOpts) (nietzsche.MctsResult, error) {
	return c.sdk.MctsSearch(ctx, opts)
}

// CalculateFidelity computes quantum fidelity (Bloch sphere entanglement proxy) between two groups of nodes.
func (c *Client) CalculateFidelity(ctx context.Context, opts nietzsche.FidelityOpts) (nietzsche.FidelityResult, error) {
	return c.sdk.CalculateFidelity(ctx, opts)
}

// ── Dream System ─────────────────────────────────────────────────────────────

// DreamResult holds the result of a DREAM FROM query.
type DreamResult struct {
	DreamID    string                   // dream session ID (e.g. "dream_abc123")
	SeedNodeID string                   // original seed node ID
	Events     []map[string]interface{} // detected events (energy spikes, curvature anomalies)
	Nodes      []map[string]interface{} // nodes discovered/modified during dream
	Raw        *nietzsche.QueryResult   // full NQL result for advanced inspection
}

// StartDream initiates a speculative exploration from a seed node via NQL DREAM FROM.
// depth controls how many hops to explore (0 = server default 5).
// noise controls perturbation amplitude (0 = server default 0.05).
func (c *Client) StartDream(ctx context.Context, collection string, seedNodeID string, depth int, noise float64) (*DreamResult, error) {
	log := logger.Nietzsche()

	nql := fmt.Sprintf("DREAM FROM $seed DEPTH %d NOISE %.4f", depth, noise)
	if depth <= 0 {
		nql = "DREAM FROM $seed"
		if noise > 0 {
			nql = fmt.Sprintf("DREAM FROM $seed NOISE %.4f", noise)
		}
	} else if noise <= 0 {
		nql = fmt.Sprintf("DREAM FROM $seed DEPTH %d", depth)
	}

	params := map[string]interface{}{
		"seed": seedNodeID,
	}

	result, err := c.sdk.Query(ctx, nql, params, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("seed", seedNodeID).
			Msg("NietzscheDB DREAM FROM failed")
		return nil, fmt.Errorf("nietzsche dream from: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("nietzsche dream error: %s", result.Error)
	}

	dreamResult := &DreamResult{
		DreamID:    "",
		SeedNodeID: seedNodeID,
		Raw:        result,
	}

	// Extract dream ID and event data from scalar rows
	for _, row := range result.ScalarRows {
		if id, ok := row["dream_id"]; ok {
			if idStr, ok := id.(string); ok {
				dreamResult.DreamID = idStr
			}
		}
		dreamResult.Events = append(dreamResult.Events, row)
	}

	// Extract discovered nodes
	for _, node := range result.Nodes {
		dreamResult.Nodes = append(dreamResult.Nodes, NodeResultToMap(node))
	}

	log.Info().
		Str("collection", collection).
		Str("seed", seedNodeID).
		Str("dream_id", dreamResult.DreamID).
		Int("events", len(dreamResult.Events)).
		Msg("NietzscheDB dream started")

	return dreamResult, nil
}

// ApplyDream commits a pending dream session, persisting energy changes to the graph.
func (c *Client) ApplyDream(ctx context.Context, collection string, dreamID string) error {
	log := logger.Nietzsche()

	nql := fmt.Sprintf(`APPLY DREAM "%s"`, dreamID)
	result, err := c.sdk.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("dream_id", dreamID).
			Msg("NietzscheDB APPLY DREAM failed")
		return fmt.Errorf("nietzsche apply dream %s: %w", dreamID, err)
	}

	if result.Error != "" {
		return fmt.Errorf("nietzsche apply dream error: %s", result.Error)
	}

	log.Info().
		Str("collection", collection).
		Str("dream_id", dreamID).
		Msg("NietzscheDB dream applied")
	return nil
}

// RejectDream discards a pending dream session without modifying the graph.
func (c *Client) RejectDream(ctx context.Context, collection string, dreamID string) error {
	log := logger.Nietzsche()

	nql := fmt.Sprintf(`REJECT DREAM "%s"`, dreamID)
	result, err := c.sdk.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Str("dream_id", dreamID).
			Msg("NietzscheDB REJECT DREAM failed")
		return fmt.Errorf("nietzsche reject dream %s: %w", dreamID, err)
	}

	if result.Error != "" {
		return fmt.Errorf("nietzsche reject dream error: %s", result.Error)
	}

	log.Info().
		Str("collection", collection).
		Str("dream_id", dreamID).
		Msg("NietzscheDB dream rejected")
	return nil
}

// ShowDreams lists all pending dream sessions for a collection.
func (c *Client) ShowDreams(ctx context.Context, collection string) ([]map[string]interface{}, error) {
	log := logger.Nietzsche()

	result, err := c.sdk.Query(ctx, "SHOW DREAMS", nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Msg("NietzscheDB SHOW DREAMS failed")
		return nil, fmt.Errorf("nietzsche show dreams: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("nietzsche show dreams error: %s", result.Error)
	}

	var dreams []map[string]interface{}
	for _, row := range result.ScalarRows {
		dreams = append(dreams, row)
	}
	if dreams == nil {
		dreams = []map[string]interface{}{}
	}

	log.Debug().
		Str("collection", collection).
		Int("pending_dreams", len(dreams)).
		Msg("NietzscheDB show dreams completed")

	return dreams, nil
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
func (c *Client) Reconstruct(ctx context.Context, nodeID string, quality string, collection string) (*nietzsche.ReconstructResult, error) {
	return c.sdk.Reconstruct(ctx, nodeID, quality, collection)
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

// FullTextSearchRich performs FTS + parallel GetNode in a single call.
func (c *Client) FullTextSearchRich(ctx context.Context, query, collection string, limit uint32) ([]nietzsche.RichFtsResult, error) {
	return c.sdk.FullTextSearchRich(ctx, query, collection, limit)
}

// ExecuteAql sends an AQL query to NietzscheDB for server-side execution.
func (c *Client) ExecuteAql(ctx context.Context, query, collection string) ([]nietzsche.AqlResult, string, error) {
	return c.sdk.ExecuteAql(ctx, query, collection)
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

	// Use FullTextSearch when a query is provided; fall back to NQL scan otherwise.
	if query != "" {
		ftsResults, ftsErr := c.FullTextSearch(ctx, query, collection, uint32(limit))
		if ftsErr == nil {
			var results []map[string]interface{}
			for _, r := range ftsResults {
				node, err := c.GetNode(ctx, r.NodeID, collection)
				if err != nil || !node.Found {
					continue
				}
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
				Msg("NietzscheDB search completed (FTS)")
			return results, nil
		}
		log.Warn().Err(ftsErr).Msg("FTS unavailable, falling back to NQL scan")
	}

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
		// Vector collections (cosine 3072D — flat embedding space)
		{Name: "memories", Dim: 3072, Metric: "cosine"},
		{Name: "signifier_chains", Dim: 3072, Metric: "cosine"},
		{Name: "eva_self_knowledge", Dim: 3072, Metric: "cosine"},
		{Name: "eva_learnings", Dim: 3072, Metric: "cosine"},
		{Name: "eva_codebase", Dim: 3072, Metric: "cosine"},
		{Name: "eva_docs", Dim: 3072, Metric: "cosine"},
		{Name: "speaker_embeddings", Dim: 192, Metric: "cosine"}, // ECAPA-TDNN = 192D
		{Name: "stories", Dim: 3072, Metric: "cosine"},
		// Graph collections (poincare — hyperbolic hierarchy)
		{Name: "patient_graph", Dim: 3072, Metric: "poincare"},
		{Name: "eva_core", Dim: 3072, Metric: "poincare"},
		// Main EVA mind collection (graph + vector hybrid)
		{Name: "eva_mind", Dim: 3072, Metric: "poincare"},
		// Cache collection (key-value TTL cache, not vector storage)
		{Name: "eva_cache", Dim: 2, Metric: "cosine"},
		// Perception collection (2D semantic perception — camera/vision)
		{Name: "eva_perceptions", Dim: 128, Metric: "poincare"},
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
		// patient_graph: graph queries filter heavily by these fields
		{"patient_graph", "node_type"},
		{"patient_graph", "node_label"},
		{"patient_graph", "patient_id"},
		{"patient_graph", "created_at"},
		{"patient_graph", "label"},
		{"patient_graph", "importance"},
		// eva_core: EVA's self-model graph
		{"eva_core", "node_type"},
		{"eva_core", "node_label"},
		{"eva_core", "label"},
		{"eva_core", "created_at"},
		{"eva_core", "category"},
		{"eva_core", "trait_name"},
		// memories: vector collection with metadata filters
		{"memories", "user_id"},
		{"memories", "importance"},
		{"memories", "created_at"},
		// eva_self_knowledge: introspection queries
		{"eva_self_knowledge", "category"},
		{"eva_self_knowledge", "created_at"},
		// eva_mind: main EVA collection
		{"eva_mind", "node_label"},
		{"eva_mind", "node_type"},
		{"eva_mind", "created_at"},
		{"eva_mind", "patient_id"},
		// eva_perceptions: 2D semantic perception
		{"eva_perceptions", "node_label"},
		{"eva_perceptions", "user_id"},
		{"eva_perceptions", "timestamp"},
		{"eva_perceptions", "scene_type"},
		{"eva_perceptions", "category"},
	}
	for _, idx := range indexes {
		if err := c.CreateIndex(ctx, idx.collection, idx.field); err != nil {
			log.Warn().Err(err).Str("collection", idx.collection).Str("field", idx.field).Msg("failed to create index (non-fatal)")
		}
	}

	// Register node-type schemas for validation (non-fatal errors — server may
	// already have the schema or not support SetSchema on older versions).
	schemas := []struct {
		nodeType   string
		required   []string
		collection string
	}{
		{"Condition", []string{"name"}, "patient_graph"},
		{"Medication", []string{"name", "dosage"}, "patient_graph"},
		{"EpisodicMemory", []string{"content", "patient_id"}, "patient_graph"},
		{"PersonalityTrait", []string{"name"}, "eva_core"},
	}
	for _, s := range schemas {
		if err := c.SetSchema(ctx, s.nodeType, s.required, nil, s.collection); err != nil {
			log.Warn().Err(err).
				Str("collection", s.collection).
				Str("node_type", s.nodeType).
				Msg("failed to set schema (non-fatal)")
		} else {
			log.Debug().
				Str("collection", s.collection).
				Str("node_type", s.nodeType).
				Int("required_fields", len(s.required)).
				Msg("schema registered")
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

// ── Schrödinger Edges ────────────────────────────────────────────────────────
// Probabilistic edges with Markov transition probabilities. Edges are
// "superpositions" that collapse at MATCH time — metadata fields:
//   probability ∈ [0,1], decay_rate, context_boost (tag), boost_factor.

// SchrodingerCreate creates a probabilistic (Schrödinger) edge between two nodes.
// The edge is stored as a regular edge with probabilistic metadata fields:
// probability, decay_rate, and optional collapse_when / context_boost.
func (c *Client) SchrodingerCreate(ctx context.Context, fromID, toID, edgeType,
	collection string, probability float64, collapseCondition string) (string, error) {

	log := logger.Nietzsche()

	// Build NQL CREATE-style via InsertEdge — Schrödinger metadata is stored in
	// edge.metadata which is exposed through InsertEdge content/weight fields.
	// Since the Go SDK InsertEdge only takes basic fields, we use NQL to set
	// the metadata properties on the edge after creation.
	edgeID, err := c.sdk.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       fromID,
		To:         toID,
		EdgeType:   edgeType,
		Weight:     probability, // weight doubles as initial probability
		Collection: collection,
	})
	if err != nil {
		log.Error().Err(err).
			Str("from", fromID).Str("to", toID).
			Str("edge_type", edgeType).
			Msg("SchrodingerCreate: InsertEdge failed")
		return "", fmt.Errorf("nietzsche schrodinger create: %w", err)
	}

	// Set probabilistic metadata via NQL MATCH SET on the new edge
	nql := `MATCH (a)-[r]->(b) WHERE r.id = $edge_id SET r.probability = $prob, r.decay_rate = 0.01`
	params := map[string]interface{}{
		"edge_id": edgeID,
		"prob":    probability,
	}
	if collapseCondition != "" {
		nql = `MATCH (a)-[r]->(b) WHERE r.id = $edge_id SET r.probability = $prob, r.decay_rate = 0.01, r.collapse_when = $cond`
		params["cond"] = collapseCondition
	}

	result, err := c.sdk.Query(ctx, nql, params, collection)
	if err != nil {
		log.Warn().Err(err).Str("edge_id", edgeID).
			Msg("SchrodingerCreate: metadata SET failed (edge created but metadata incomplete)")
	} else if result.Error != "" {
		log.Warn().Str("error", result.Error).Str("edge_id", edgeID).
			Msg("SchrodingerCreate: metadata SET returned error")
	}

	log.Info().
		Str("edge_id", edgeID).
		Str("from", fromID).Str("to", toID).
		Float64("probability", probability).
		Str("collapse_when", collapseCondition).
		Msg("SchrodingerCreate: probabilistic edge created")
	return edgeID, nil
}

// SchrodingerCollapse collapses a Schrödinger edge under the given context.
// It reads the edge's probability, evaluates context-dependent boost, and
// performs a probabilistic collapse (random sample vs effective probability).
// Returns whether the edge collapsed into existence and the final probability used.
func (c *Client) SchrodingerCollapse(ctx context.Context, edgeID, collection string,
	contextData map[string]interface{}) (collapsed bool, finalProb float64, err error) {

	log := logger.Nietzsche()

	// Read edge metadata via NQL
	nql := `MATCH (a)-[r]->(b) WHERE r.id = $edge_id RETURN r.probability, r.context_boost, r.boost_factor`
	params := map[string]interface{}{
		"edge_id": edgeID,
	}

	result, err := c.sdk.Query(ctx, nql, params, collection)
	if err != nil {
		log.Error().Err(err).Str("edge_id", edgeID).Msg("SchrodingerCollapse: query failed")
		return false, 0, fmt.Errorf("nietzsche schrodinger collapse query: %w", err)
	}
	if result.Error != "" {
		return false, 0, fmt.Errorf("nietzsche schrodinger collapse: %s", result.Error)
	}

	// Extract probability from scalar rows
	prob := 1.0
	boostFactor := 1.5
	contextBoost := ""

	if len(result.ScalarRows) > 0 {
		row := result.ScalarRows[0]
		if p, ok := row["r.probability"]; ok {
			if pf, ok := p.(float64); ok {
				prob = pf
			}
		}
		if cb, ok := row["r.context_boost"]; ok {
			if cbs, ok := cb.(string); ok {
				contextBoost = cbs
			}
		}
		if bf, ok := row["r.boost_factor"]; ok {
			if bff, ok := bf.(float64); ok {
				boostFactor = bff
			}
		}
	}

	// Apply context boost if context matches
	effectiveProb := prob
	if contextBoost != "" && contextData != nil {
		if ctxVal, ok := contextData["context"]; ok {
			if ctxStr, ok := ctxVal.(string); ok {
				if ctxStr == contextBoost {
					effectiveProb = prob * boostFactor
					if effectiveProb > 1.0 {
						effectiveProb = 1.0
					}
				}
			}
		}
	}

	// Probabilistic collapse: pseudo-random based on edge_id hash for reproducibility
	// Use a simple deterministic approach: hash of edgeID + current time modulo
	h := uint64(0)
	for _, ch := range edgeID {
		h = h*31 + uint64(ch)
	}
	sample := float64(h%10000) / 10000.0
	collapsed = sample < effectiveProb
	finalProb = effectiveProb

	log.Info().
		Str("edge_id", edgeID).
		Float64("base_prob", prob).
		Float64("effective_prob", effectiveProb).
		Bool("collapsed", collapsed).
		Msg("SchrodingerCollapse: edge collapse evaluated")

	return collapsed, finalProb, nil
}

// SchrodingerObserve reads a Schrödinger edge's current probability and state
// without collapsing it. Returns the base probability and a human-readable state.
func (c *Client) SchrodingerObserve(ctx context.Context, edgeID, collection string) (probability float64, state string, err error) {
	log := logger.Nietzsche()

	nql := `MATCH (a)-[r]->(b) WHERE r.id = $edge_id RETURN r.probability, r.decay_rate, r.context_boost, r.boost_factor`
	params := map[string]interface{}{
		"edge_id": edgeID,
	}

	result, err := c.sdk.Query(ctx, nql, params, collection)
	if err != nil {
		log.Error().Err(err).Str("edge_id", edgeID).Msg("SchrodingerObserve: query failed")
		return 0, "", fmt.Errorf("nietzsche schrodinger observe: %w", err)
	}
	if result.Error != "" {
		return 0, "", fmt.Errorf("nietzsche schrodinger observe: %s", result.Error)
	}

	if len(result.ScalarRows) == 0 {
		log.Debug().Str("edge_id", edgeID).Msg("SchrodingerObserve: edge not found or no probability metadata")
		return 1.0, "deterministic", nil // no probability metadata = deterministic edge
	}

	row := result.ScalarRows[0]
	prob := 1.0
	if p, ok := row["r.probability"]; ok {
		if pf, ok := p.(float64); ok {
			prob = pf
		}
	}

	// Determine state label
	switch {
	case prob >= 1.0:
		state = "deterministic"
	case prob <= 0.0:
		state = "collapsed_absent"
	case prob >= 0.8:
		state = "superposition_strong"
	case prob >= 0.4:
		state = "superposition_medium"
	default:
		state = "superposition_weak"
	}

	log.Debug().
		Str("edge_id", edgeID).
		Float64("probability", prob).
		Str("state", state).
		Msg("SchrodingerObserve: edge observed")

	return prob, state, nil
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
