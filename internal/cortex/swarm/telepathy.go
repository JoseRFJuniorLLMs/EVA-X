// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package swarm

import (
	"context"
	"fmt"
	"log"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// SwarmMessage represents a message sent between EVA cognitive modules.
// Instead of natural language strings, agents exchange Poincaré embeddings
// that represent multidimensional clinical concepts.
type SwarmMessage struct {
	SourceID    string    // agent name (e.g. "scholar", "clinical", "kids")
	TargetID    string    // agent name (e.g. "orchestrator")
	Latent      []float64 // 64D feature vector
	Timestamp   int64
	ContextNode string // NietzscheDB node ID this message relates to (optional)
}

// TelepathyNode is an agent capable of sending/receiving latent messages.
type TelepathyNode struct {
	ID            string
	graphAdapter  *nietzscheInfra.GraphAdapter
	vectorAdapter *nietzscheInfra.VectorAdapter
	collection    string
}

// NewTelepathyNode creates a new swarm agent node.
func NewTelepathyNode(id string, ga *nietzscheInfra.GraphAdapter, va *nietzscheInfra.VectorAdapter, col string) *TelepathyNode {
	return &TelepathyNode{
		ID:            id,
		graphAdapter:  ga,
		vectorAdapter: va,
		collection:    col,
	}
}

// Send produces a latent message based on a text concept.
// It uses NietzscheDB's search to find the nearest concept node and returns its embedding.
func (n *TelepathyNode) Send(ctx context.Context, targetID string, concept string) (*SwarmMessage, error) {
	// 1. Convert concept to its nearest latent representation (64D projection)
	// In production, this would use the GNN to project the current state.
	// For the MVP, we use full-text search to find a semantic anchor.
	knn, err := n.vectorAdapter.FullTextSearch(ctx, n.collection, concept, 1)
	if err != nil || len(knn) == 0 {
		return nil, fmt.Errorf("telepathy: could not find semantic anchor for '%s': %v", concept, err)
	}

	// 2. Retrieve the node's Poincaré embedding
	node, err := n.graphAdapter.GetNode(ctx, knn[0].ID, n.collection)
	if err != nil {
		return nil, fmt.Errorf("telepathy: node lookup failed: %v", err)
	}

	// 3. Compress/Project to 64D (assuming the node embedding is 3072D)
	// For this simulation, we take the first 64 dimensions.
	latent := make([]float64, 64)
	copy(latent, node.Embedding[:64])

	log.Printf("[SWARM-TELEPATHY] Agent '%s' sending latent concept '%s' to '%s' (anchor: %s)",
		n.ID, concept, targetID, node.ID)

	return &SwarmMessage{
		SourceID:    n.ID,
		TargetID:    targetID,
		Latent:      latent,
		Timestamp:   time.Now().Unix(),
		ContextNode: node.ID,
	}, nil
}

// Receive decodes a latent message back into its nearest semantic label.
func (n *TelepathyNode) Receive(ctx context.Context, msg *SwarmMessage) (string, error) {
	// 1. Search for the nearest node in the vector space using the latent vector.
	// Since the latent is a projection, we search for the anchor ID first.
	if msg.ContextNode != "" {
		node, err := n.graphAdapter.GetNode(ctx, msg.ContextNode, n.collection)
		if err == nil {
			// Return node content description
			if label, ok := node.Content["label"].(string); ok {
				return label, nil
			}
			return fmt.Sprintf("node:%s", node.ID), nil
		}
	}

	// Fallback: search by latent embedding if context node is lost
	// (Requires vector search to support 64D sub-indexes, here we just stub)
	return "unknown_latent_concept", nil
}
