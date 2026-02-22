// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// VectorAdapter replaces QdrantClient for vector search and upsert operations.
// It wraps NietzscheDB's KNN search and InsertNode with the same interface patterns
// that EVA's consumer code expects.
type VectorAdapter struct {
	client *Client
}

// NewVectorAdapter creates a VectorAdapter backed by a NietzscheDB client.
func NewVectorAdapter(client *Client) *VectorAdapter {
	return &VectorAdapter{client: client}
}

// VectorSearchResult mirrors what Qdrant returns — ID, score, and payload.
type VectorSearchResult struct {
	ID      string
	Score   float64
	Payload map[string]interface{}
}

// Search performs KNN vector search in a collection.
// Replaces QdrantClient.Search() and QdrantClient.SearchWithScore().
// Vectors are float32 from Gemini embeddings, converted to float64 for NietzscheDB.
func (va *VectorAdapter) Search(ctx context.Context, collection string,
	vector []float32, limit int, userID int64) ([]VectorSearchResult, error) {

	log := logger.Nietzsche()

	// float32 -> float64 (lossless)
	vec64 := float32ToFloat64(vector)

	results, err := va.client.KnnSearch(ctx, collection, vec64, uint32(limit*2))
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("vector search failed")
		return nil, fmt.Errorf("vector search %s: %w", collection, err)
	}

	// Fetch node content for each result and filter by user_id if needed
	var out []VectorSearchResult
	for _, r := range results {
		node, err := va.client.GetNode(ctx, r.ID, collection)
		if err != nil || !node.Found {
			continue
		}

		// Filter by user_id if specified (replaces Qdrant payload filter)
		if userID > 0 {
			if uid, ok := node.Content["user_id"]; ok {
				switch v := uid.(type) {
				case float64:
					if int64(v) != userID {
						continue
					}
				case int64:
					if v != userID {
						continue
					}
				}
			}
		}

		out = append(out, VectorSearchResult{
			ID:      r.ID,
			Score:   r.Distance,
			Payload: node.Content,
		})

		if len(out) >= limit {
			break
		}
	}

	log.Debug().
		Str("collection", collection).
		Int("results", len(out)).
		Msg("vector search completed")
	return out, nil
}

// Upsert inserts or updates a vector node in a collection.
// Replaces QdrantClient.Upsert() for single-point operations.
func (va *VectorAdapter) Upsert(ctx context.Context, collection string,
	id string, vector []float32, payload map[string]interface{}) error {

	log := logger.Nietzsche()

	vec64 := float32ToFloat64(vector)

	nodeType := "Semantic"
	if nt, ok := payload["node_type"].(string); ok {
		nodeType = nt
	}

	_, err := va.client.InsertWithEmbedding(ctx, collection, id, vec64, payload, nodeType)
	if err != nil {
		// Try merge if insert fails (node may already exist)
		_, mergeErr := va.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
			Collection: collection,
			NodeType:   nodeType,
			MatchKeys:  map[string]interface{}{"id": id},
			OnMatchSet: payload,
		})
		if mergeErr != nil {
			log.Error().Err(err).Str("collection", collection).Str("id", id).Msg("vector upsert failed")
			return fmt.Errorf("vector upsert %s/%s: %w", collection, id, err)
		}
	}

	log.Debug().
		Str("collection", collection).
		Str("id", id).
		Msg("vector upsert completed")
	return nil
}

// BatchUpsert inserts multiple vectors via a single BatchInsertNodes RPC.
// Falls back to sequential upserts if the batch call fails.
func (va *VectorAdapter) BatchUpsert(ctx context.Context, collection string,
	items []BatchVectorItem) error {

	log := logger.Nietzsche()

	// Build batch of InsertNodeOpts
	opts := make([]nietzsche.InsertNodeOpts, len(items))
	for i, item := range items {
		nodeType := "Semantic"
		if nt, ok := item.Payload["node_type"].(string); ok {
			nodeType = nt
		}
		opts[i] = nietzsche.InsertNodeOpts{
			ID:       item.ID,
			Coords:   float32ToFloat64(item.Vector),
			Content:  item.Payload,
			NodeType: nodeType,
		}
	}

	_, err := va.client.BatchInsertNodes(ctx, opts, collection)
	if err != nil {
		// Fallback to sequential upserts (handles duplicates via merge)
		log.Warn().Err(err).Int("count", len(items)).Msg("batch insert failed, falling back to sequential upsert")
		for _, item := range items {
			if err := va.Upsert(ctx, collection, item.ID, item.Vector, item.Payload); err != nil {
				return err
			}
		}
		return nil
	}

	log.Debug().Str("collection", collection).Int("count", len(items)).Msg("batch insert completed")
	return nil
}

// BatchVectorItem represents a single vector to upsert in a batch.
type BatchVectorItem struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}

// Delete removes a vector node by ID.
// Replaces QdrantClient.Delete().
func (va *VectorAdapter) Delete(ctx context.Context, collection string, id string) error {
	return va.client.Delete(ctx, collection, id)
}

// ── Full-Text & Hybrid Search ───────────────────────────────────────────────

// FullTextSearch performs a BM25 inverted-index search over node content.
func (va *VectorAdapter) FullTextSearch(ctx context.Context, collection, query string, limit int) ([]VectorSearchResult, error) {
	log := logger.Nietzsche()

	results, err := va.client.FullTextSearch(ctx, query, collection, uint32(limit))
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("full-text search failed")
		return nil, fmt.Errorf("fts %s: %w", collection, err)
	}

	out := make([]VectorSearchResult, len(results))
	for i, r := range results {
		out[i] = VectorSearchResult{ID: r.NodeID, Score: r.Score}
	}

	log.Debug().Str("collection", collection).Int("results", len(out)).Msg("full-text search completed")
	return out, nil
}

// HybridSearch combines BM25 text search with vector KNN search.
func (va *VectorAdapter) HybridSearch(ctx context.Context, collection string,
	textQuery string, vector []float32, k int) ([]VectorSearchResult, error) {

	log := logger.Nietzsche()

	vec64 := float32ToFloat64(vector)
	results, err := va.client.HybridSearch(ctx, textQuery, vec64, uint32(k), 0.5, 0.5, collection)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("hybrid search failed")
		return nil, fmt.Errorf("hybrid search %s: %w", collection, err)
	}

	out := make([]VectorSearchResult, len(results))
	for i, r := range results {
		out[i] = VectorSearchResult{ID: r.ID, Score: r.Distance}
	}

	log.Debug().Str("collection", collection).Int("results", len(out)).Msg("hybrid search completed")
	return out, nil
}

// GetNodeByID retrieves a node's full content by ID from a collection.
// Used to hydrate Memory objects without going through PostgreSQL.
func (va *VectorAdapter) GetNodeByID(ctx context.Context, collection, id string) (map[string]interface{}, bool, error) {
	result, err := va.client.GetNode(ctx, id, collection)
	if err != nil {
		return nil, false, err
	}
	return result.Content, result.Found, nil
}

// ExecuteNQL runs a NQL query against a collection.
// Returns the raw QueryResult from NietzscheDB.
func (va *VectorAdapter) ExecuteNQL(ctx context.Context, nql string, params map[string]interface{}, collection string) ([]map[string]interface{}, error) {
	result, err := va.client.Query(ctx, nql, params, collection)
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	for _, node := range result.Nodes {
		out = append(out, node.Content)
	}
	return out, nil
}

// MergeNode performs an upsert (INSERT ... ON CONFLICT DO UPDATE) using NietzscheDB.
// matchKeys define the conflict columns, onMatchSet defines the SET clause.
func (va *VectorAdapter) MergeNode(ctx context.Context, collection, nodeType string,
	matchKeys map[string]interface{}, onMatchSet map[string]interface{}) (string, bool, error) {

	log := logger.Nietzsche()

	result, err := va.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection: collection,
		NodeType:   nodeType,
		MatchKeys:  matchKeys,
		OnMatchSet: onMatchSet,
	})
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("merge node failed")
		return "", false, fmt.Errorf("merge node %s: %w", collection, err)
	}

	log.Debug().Str("collection", collection).Str("id", result.NodeID).
		Bool("created", result.Created).Msg("merge node completed")
	return result.NodeID, result.Created, nil
}

// InsertNodeContent inserts a node without embedding vector.
// Used for non-semantic data (habits, logs, etc.) that don't need KNN search.
func (va *VectorAdapter) InsertNodeContent(ctx context.Context, collection, id string,
	content map[string]interface{}, nodeType string) error {

	log := logger.Nietzsche()

	_, err := va.client.InsertNode(ctx, nietzsche.InsertNodeOpts{
		Collection: collection,
		ID:         id,
		Content:    content,
		NodeType:   nodeType,
	})
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Str("id", id).Msg("insert node content failed")
		return fmt.Errorf("insert node %s/%s: %w", collection, id, err)
	}

	log.Debug().Str("collection", collection).Str("id", id).Msg("insert node content completed")
	return nil
}

// float32ToFloat64 converts a float32 slice to float64 (lossless).
func float32ToFloat64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, f := range v {
		out[i] = float64(f)
	}
	return out
}
