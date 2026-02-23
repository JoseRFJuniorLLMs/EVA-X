// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"fmt"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// ── Named Vectors — multi-embedding-per-node for EVA ─────────────────────────
//
// EVA processes multiple modalities:
//   - "text"   — 3072D Gemini text embeddings (default)
//   - "audio"  — voice prosody embeddings from audio encoder
//   - "visual" — MedGemma image embeddings
//
// NietzscheDB's Named Vectors crate stores named vectors at the storage layer
// (CF_META prefix nvec:{node_id}:{name}), but the gRPC API does not yet expose
// dedicated RPCs for named vectors.
//
// This adapter implements named vectors at the application level using
// collection-per-vector-name conventions:
//
//   Base collection "memories" + vector name "audio"
//     → sub-collection "memories__nv_audio"
//
// Each named vector node mirrors the parent node ID and carries a back-reference
// in its content metadata. The parent node's content includes a "_named_vectors"
// list for discoverability.
//
// When NietzscheDB exposes native Named Vector RPCs, this adapter can be updated
// to use them without changing the consumer API.

// ── Well-known vector names ──────────────────────────────────────────────────

const (
	// VectorNameText is the default text embedding vector name.
	VectorNameText = "text"

	// VectorNameAudio is the voice prosody embedding vector name.
	VectorNameAudio = "audio"

	// VectorNameVisual is the image/visual embedding vector name.
	VectorNameVisual = "visual"

	// namedVectorCollectionSep separates base collection from vector name.
	namedVectorCollectionSep = "__nv_"

	// contentKeyNamedVectors is the metadata key that tracks which named vectors
	// exist on a parent node.
	contentKeyNamedVectors = "_named_vectors"

	// contentKeyParentNodeID is the back-reference key in named vector nodes.
	contentKeyParentNodeID = "_parent_node_id"

	// contentKeyVectorName identifies the vector name in the named vector node.
	contentKeyVectorName = "_vector_name"
)

// ── NamedVectorsAdapter ──────────────────────────────────────────────────────

// NamedVectorsAdapter provides multi-embedding-per-node operations for EVA.
// It wraps the NietzscheDB client to store and search named vectors using
// sub-collections keyed by vector name.
type NamedVectorsAdapter struct {
	client *Client
}

// NewNamedVectorsAdapter creates a NamedVectorsAdapter backed by a NietzscheDB client.
func NewNamedVectorsAdapter(client *Client) *NamedVectorsAdapter {
	return &NamedVectorsAdapter{client: client}
}

// SDK returns the underlying NietzscheDB SDK client.
func (nva *NamedVectorsAdapter) SDK() *nietzsche.NietzscheClient {
	return nva.client.SDK()
}

// ── Types ────────────────────────────────────────────────────────────────────

// NamedEmbedding represents a single named vector to be attached to a node.
type NamedEmbedding struct {
	Name   string    // vector name: "text", "audio", "visual", or custom
	Vector []float32 // embedding coordinates (float32 from model output)
}

// InsertNamedVectorsOpts configures a node insertion with multiple named embeddings.
type InsertNamedVectorsOpts struct {
	ID         string                 // node UUID; empty → server auto-generates
	Collection string                 // base collection; "" → "default"
	NodeType   string                 // "Semantic"|"Episodic"|"Concept"; "" → "Semantic"
	Content    map[string]interface{} // arbitrary payload
	Energy     float32                // node energy; 0 → server default (1.0)
	Vectors    []NamedEmbedding       // named embeddings to attach
}

// NamedSearchOpts configures a KNN search on a specific named vector.
type NamedSearchOpts struct {
	Collection string    // base collection
	VectorName string    // which named vector to search (e.g. "audio")
	Vector     []float32 // query embedding
	Limit      int       // max results
	UserID     int64     // optional user_id filter (0 = no filter)
}

// NamedVectorSearchResult contains a search hit with parent node context.
type NamedVectorSearchResult struct {
	ID           string                 // parent node ID
	VectorName   string                 // which named vector matched
	Distance     float64                // KNN distance score
	ParentNodeID string                 // explicit parent reference
	Payload      map[string]interface{} // named vector node content
}

// ── Collection naming ────────────────────────────────────────────────────────

// namedVectorCollection returns the sub-collection name for a given base
// collection and vector name.
//
// Example: namedVectorCollection("memories", "audio") → "memories__nv_audio"
func namedVectorCollection(baseCollection, vectorName string) string {
	if baseCollection == "" {
		baseCollection = "default"
	}
	return baseCollection + namedVectorCollectionSep + vectorName
}

// ── Ensure sub-collections ───────────────────────────────────────────────────

// EnsureNamedVectorCollections creates sub-collections for each vector name
// under a base collection. Uses cosine metric and the specified dimension.
// This is idempotent — safe to call multiple times.
func (nva *NamedVectorsAdapter) EnsureNamedVectorCollections(ctx context.Context,
	baseCollection string, vectorNames []string, dims map[string]uint32) error {

	log := logger.Nietzsche()

	for _, name := range vectorNames {
		subCol := namedVectorCollection(baseCollection, name)
		dim := uint32(3072) // default to Gemini text dimension
		if d, ok := dims[name]; ok {
			dim = d
		}

		if err := nva.client.EnsureCollection(ctx, subCol, dim, "cosine"); err != nil {
			log.Error().Err(err).
				Str("sub_collection", subCol).
				Str("vector_name", name).
				Msg("failed to ensure named vector sub-collection")
			return fmt.Errorf("ensure named vector collection %s: %w", subCol, err)
		}

		log.Debug().
			Str("sub_collection", subCol).
			Uint32("dim", dim).
			Msg("named vector sub-collection ensured")
	}

	return nil
}

// ── Insert with named vectors ────────────────────────────────────────────────

// InsertWithNamedVectors inserts a parent node and its named vector embeddings.
//
// The parent node is inserted into the base collection with its primary content.
// Each named embedding is stored as a separate node in its sub-collection,
// sharing the same node ID for correlation.
//
// The parent node's content is augmented with a "_named_vectors" list so
// consumers can discover which embeddings are available.
func (nva *NamedVectorsAdapter) InsertWithNamedVectors(ctx context.Context,
	opts InsertNamedVectorsOpts) (string, error) {

	log := logger.Nietzsche()

	// Augment content with named vectors metadata
	content := opts.Content
	if content == nil {
		content = make(map[string]interface{})
	}
	vectorNames := make([]string, len(opts.Vectors))
	for i, v := range opts.Vectors {
		vectorNames[i] = v.Name
	}
	content[contentKeyNamedVectors] = vectorNames

	// Insert parent node (without embedding — the primary embedding goes into
	// the "text" sub-collection if present)
	nodeType := opts.NodeType
	if nodeType == "" {
		nodeType = "Semantic"
	}

	parentResult, err := nva.client.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:         opts.ID,
		Collection: opts.Collection,
		Content:    content,
		NodeType:   nodeType,
		Energy:     opts.Energy,
	})
	if err != nil {
		log.Error().Err(err).
			Str("collection", opts.Collection).
			Str("id", opts.ID).
			Msg("failed to insert parent node for named vectors")
		return "", fmt.Errorf("insert parent node %s: %w", opts.ID, err)
	}

	parentID := parentResult.ID
	if parentID == "" {
		parentID = opts.ID
	}

	// Insert each named vector into its sub-collection
	for _, nv := range opts.Vectors {
		subCol := namedVectorCollection(opts.Collection, nv.Name)

		nvContent := map[string]interface{}{
			contentKeyParentNodeID: parentID,
			contentKeyVectorName:   nv.Name,
		}

		vec64 := float32ToFloat64(nv.Vector)

		_, err := nva.client.InsertWithEmbedding(ctx, subCol, parentID, vec64, nvContent, nodeType)
		if err != nil {
			log.Error().Err(err).
				Str("sub_collection", subCol).
				Str("parent_id", parentID).
				Str("vector_name", nv.Name).
				Msg("failed to insert named vector")
			return parentID, fmt.Errorf("insert named vector %s/%s: %w", subCol, parentID, err)
		}

		log.Debug().
			Str("sub_collection", subCol).
			Str("parent_id", parentID).
			Str("vector_name", nv.Name).
			Int("dim", len(nv.Vector)).
			Msg("named vector inserted")
	}

	log.Info().
		Str("collection", opts.Collection).
		Str("parent_id", parentID).
		Int("named_vectors", len(opts.Vectors)).
		Msg("node with named vectors inserted")

	return parentID, nil
}

// ── Search by named vector ───────────────────────────────────────────────────

// SearchByNamedVector performs KNN search on a specific named vector space.
//
// This searches the sub-collection for the given vector name, then hydrates
// results with parent node content from the base collection.
func (nva *NamedVectorsAdapter) SearchByNamedVector(ctx context.Context,
	opts NamedSearchOpts) ([]NamedVectorSearchResult, error) {

	log := logger.Nietzsche()

	subCol := namedVectorCollection(opts.Collection, opts.VectorName)
	vec64 := float32ToFloat64(opts.Vector)

	// Search in the named vector sub-collection
	results, err := nva.client.KnnSearch(ctx, subCol, vec64, uint32(opts.Limit*2))
	if err != nil {
		log.Error().Err(err).
			Str("sub_collection", subCol).
			Str("vector_name", opts.VectorName).
			Msg("named vector search failed")
		return nil, fmt.Errorf("named vector search %s/%s: %w", opts.Collection, opts.VectorName, err)
	}

	// Hydrate results with parent node content
	var out []NamedVectorSearchResult
	for _, r := range results {
		// The named vector node ID is the same as the parent node ID
		parentID := r.ID

		// Fetch parent node content for filtering and payload
		parentNode, err := nva.client.GetNode(ctx, parentID, opts.Collection)
		if err != nil || !parentNode.Found {
			// Parent node may have been deleted; skip
			continue
		}

		// Filter by user_id if specified
		if opts.UserID > 0 {
			if uid, ok := parentNode.Content["user_id"]; ok {
				switch v := uid.(type) {
				case float64:
					if int64(v) != opts.UserID {
						continue
					}
				case int64:
					if v != opts.UserID {
						continue
					}
				}
			}
		}

		out = append(out, NamedVectorSearchResult{
			ID:           parentID,
			VectorName:   opts.VectorName,
			Distance:     r.Distance,
			ParentNodeID: parentID,
			Payload:      parentNode.Content,
		})

		if len(out) >= opts.Limit {
			break
		}
	}

	log.Debug().
		Str("collection", opts.Collection).
		Str("vector_name", opts.VectorName).
		Int("results", len(out)).
		Msg("named vector search completed")

	return out, nil
}

// ── Upsert individual named vectors ──────────────────────────────────────────

// UpsertNamedVector updates (or inserts) a single named vector for an existing node.
//
// This is useful when a new modality becomes available for an existing node
// (e.g., a text node gets an audio embedding added later).
//
// The parent node's "_named_vectors" metadata is also updated to include the
// new vector name if not already present.
func (nva *NamedVectorsAdapter) UpsertNamedVector(ctx context.Context,
	baseCollection string, nodeID string, embedding NamedEmbedding) error {

	log := logger.Nietzsche()

	subCol := namedVectorCollection(baseCollection, embedding.Name)
	vec64 := float32ToFloat64(embedding.Vector)

	nvContent := map[string]interface{}{
		contentKeyParentNodeID: nodeID,
		contentKeyVectorName:   embedding.Name,
	}

	// Try insert first; if it fails (duplicate ID), fall back to merge
	_, err := nva.client.InsertWithEmbedding(ctx, subCol, nodeID, vec64, nvContent, "Semantic")
	if err != nil {
		// Node may already exist in sub-collection — use merge to update
		_, mergeErr := nva.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
			Collection: subCol,
			NodeType:   "Semantic",
			MatchKeys:  map[string]interface{}{"id": nodeID},
			OnMatchSet: nvContent,
			Coords:     vec64,
		})
		if mergeErr != nil {
			log.Error().Err(mergeErr).
				Str("sub_collection", subCol).
				Str("node_id", nodeID).
				Str("vector_name", embedding.Name).
				Msg("failed to upsert named vector")
			return fmt.Errorf("upsert named vector %s/%s/%s: %w", baseCollection, nodeID, embedding.Name, err)
		}
	}

	// Update parent node's _named_vectors metadata
	if err := nva.ensureNamedVectorMetadata(ctx, baseCollection, nodeID, embedding.Name); err != nil {
		log.Warn().Err(err).
			Str("collection", baseCollection).
			Str("node_id", nodeID).
			Str("vector_name", embedding.Name).
			Msg("failed to update parent _named_vectors metadata (non-fatal)")
	}

	log.Debug().
		Str("collection", baseCollection).
		Str("node_id", nodeID).
		Str("vector_name", embedding.Name).
		Int("dim", len(embedding.Vector)).
		Msg("named vector upserted")

	return nil
}

// BatchUpsertNamedVectors upserts multiple named vectors for a single node.
// This is the common case when processing a multi-modal input (text + audio + image).
func (nva *NamedVectorsAdapter) BatchUpsertNamedVectors(ctx context.Context,
	baseCollection string, nodeID string, embeddings []NamedEmbedding) error {

	log := logger.Nietzsche()

	for _, emb := range embeddings {
		if err := nva.UpsertNamedVector(ctx, baseCollection, nodeID, emb); err != nil {
			return err
		}
	}

	log.Info().
		Str("collection", baseCollection).
		Str("node_id", nodeID).
		Int("vectors", len(embeddings)).
		Msg("batch named vectors upserted")

	return nil
}

// ── Delete named vectors ─────────────────────────────────────────────────────

// DeleteNamedVector removes a single named vector from a node.
func (nva *NamedVectorsAdapter) DeleteNamedVector(ctx context.Context,
	baseCollection string, nodeID string, vectorName string) error {

	log := logger.Nietzsche()

	subCol := namedVectorCollection(baseCollection, vectorName)
	if err := nva.client.Delete(ctx, subCol, nodeID); err != nil {
		log.Error().Err(err).
			Str("sub_collection", subCol).
			Str("node_id", nodeID).
			Str("vector_name", vectorName).
			Msg("failed to delete named vector")
		return fmt.Errorf("delete named vector %s/%s/%s: %w", baseCollection, nodeID, vectorName, err)
	}

	log.Debug().
		Str("collection", baseCollection).
		Str("node_id", nodeID).
		Str("vector_name", vectorName).
		Msg("named vector deleted")

	return nil
}

// DeleteAllNamedVectors removes all named vectors for a node across all known
// vector names. Also cleans up the parent node's metadata.
func (nva *NamedVectorsAdapter) DeleteAllNamedVectors(ctx context.Context,
	baseCollection string, nodeID string) error {

	log := logger.Nietzsche()

	// Read parent node to find which named vectors exist
	parentNode, err := nva.client.GetNode(ctx, nodeID, baseCollection)
	if err != nil || !parentNode.Found {
		return nil // nothing to delete
	}

	vectorNames := extractNamedVectorNames(parentNode.Content)
	for _, name := range vectorNames {
		subCol := namedVectorCollection(baseCollection, name)
		if err := nva.client.Delete(ctx, subCol, nodeID); err != nil {
			log.Warn().Err(err).
				Str("sub_collection", subCol).
				Str("node_id", nodeID).
				Msg("failed to delete named vector (non-fatal)")
		}
	}

	log.Debug().
		Str("collection", baseCollection).
		Str("node_id", nodeID).
		Int("deleted", len(vectorNames)).
		Msg("all named vectors deleted")

	return nil
}

// ── Query named vectors ──────────────────────────────────────────────────────

// GetNamedVector retrieves a specific named vector for a node.
// Returns the embedding coordinates and whether it was found.
func (nva *NamedVectorsAdapter) GetNamedVector(ctx context.Context,
	baseCollection string, nodeID string, vectorName string) ([]float64, bool, error) {

	subCol := namedVectorCollection(baseCollection, vectorName)
	node, err := nva.client.GetNode(ctx, nodeID, subCol)
	if err != nil {
		return nil, false, fmt.Errorf("get named vector %s/%s/%s: %w", baseCollection, nodeID, vectorName, err)
	}

	if !node.Found {
		return nil, false, nil
	}

	return node.Embedding, true, nil
}

// ListNamedVectors returns which named vectors exist for a node by reading
// the parent node's "_named_vectors" metadata.
func (nva *NamedVectorsAdapter) ListNamedVectors(ctx context.Context,
	baseCollection string, nodeID string) ([]string, error) {

	parentNode, err := nva.client.GetNode(ctx, nodeID, baseCollection)
	if err != nil {
		return nil, fmt.Errorf("list named vectors %s/%s: %w", baseCollection, nodeID, err)
	}

	if !parentNode.Found {
		return nil, nil
	}

	return extractNamedVectorNames(parentNode.Content), nil
}

// ── Multi-vector search (fusion) ─────────────────────────────────────────────

// MultiVectorSearchOpts configures a search across multiple named vector spaces.
type MultiVectorSearchOpts struct {
	Collection string           // base collection
	Queries    []NamedEmbedding // query vectors per modality
	Limit      int              // max final results
	UserID     int64            // optional user_id filter
}

// MultiVectorSearch searches across multiple named vector spaces and merges
// results using Reciprocal Rank Fusion (RRF).
//
// This enables cross-modal retrieval: find nodes that are similar in both
// text and audio embedding spaces simultaneously.
func (nva *NamedVectorsAdapter) MultiVectorSearch(ctx context.Context,
	opts MultiVectorSearchOpts) ([]NamedVectorSearchResult, error) {

	log := logger.Nietzsche()

	// Collect results from each vector space
	type rankedResult struct {
		result NamedVectorSearchResult
		rank   int
	}

	// nodeID → accumulated RRF score
	scores := make(map[string]float64)
	// nodeID → best result (for payload)
	bestResult := make(map[string]NamedVectorSearchResult)

	const rrfK = 60.0 // RRF constant

	for _, query := range opts.Queries {
		results, err := nva.SearchByNamedVector(ctx, NamedSearchOpts{
			Collection: opts.Collection,
			VectorName: query.Name,
			Vector:     query.Vector,
			Limit:      opts.Limit * 2, // over-fetch for fusion
			UserID:     opts.UserID,
		})
		if err != nil {
			log.Warn().Err(err).
				Str("vector_name", query.Name).
				Msg("multi-vector search: sub-search failed, skipping modality")
			continue
		}

		for rank, r := range results {
			scores[r.ID] += 1.0 / (rrfK + float64(rank+1))
			if _, exists := bestResult[r.ID]; !exists {
				bestResult[r.ID] = r
			}
		}
	}

	// Sort by RRF score (descending)
	type scoredEntry struct {
		id    string
		score float64
	}
	entries := make([]scoredEntry, 0, len(scores))
	for id, score := range scores {
		entries = append(entries, scoredEntry{id: id, score: score})
	}

	// Simple insertion sort (result sets are small)
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].score > entries[j-1].score; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}

	// Build final results
	var out []NamedVectorSearchResult
	for _, entry := range entries {
		if len(out) >= opts.Limit {
			break
		}
		r := bestResult[entry.id]
		r.Distance = entry.score // replace distance with RRF score
		out = append(out, r)
	}

	log.Debug().
		Str("collection", opts.Collection).
		Int("modalities", len(opts.Queries)).
		Int("results", len(out)).
		Msg("multi-vector search completed")

	return out, nil
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// ensureNamedVectorMetadata updates the parent node's "_named_vectors" list
// to include the given vector name if not already present.
func (nva *NamedVectorsAdapter) ensureNamedVectorMetadata(ctx context.Context,
	baseCollection string, nodeID string, vectorName string) error {

	parentNode, err := nva.client.GetNode(ctx, nodeID, baseCollection)
	if err != nil || !parentNode.Found {
		return err
	}

	existing := extractNamedVectorNames(parentNode.Content)
	for _, name := range existing {
		if name == vectorName {
			return nil // already present
		}
	}

	// Add the new vector name
	updated := append(existing, vectorName)
	_, err = nva.client.MergeNode(ctx, nietzsche.MergeNodeOpts{
		Collection: baseCollection,
		NodeType:   parentNode.NodeType,
		MatchKeys:  map[string]interface{}{"id": nodeID},
		OnMatchSet: map[string]interface{}{
			contentKeyNamedVectors: updated,
		},
	})
	return err
}

// extractNamedVectorNames reads the "_named_vectors" field from node content.
func extractNamedVectorNames(content map[string]interface{}) []string {
	if content == nil {
		return nil
	}

	raw, ok := content[contentKeyNamedVectors]
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case []interface{}:
		names := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				names = append(names, s)
			}
		}
		return names
	case []string:
		return v
	default:
		return nil
	}
}
