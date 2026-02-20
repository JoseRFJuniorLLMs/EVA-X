// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package diffusor

import (
	"context"
	"fmt"
	"math"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// HeatKernelDiffusion implements heat kernel-based graph navigation
// Simulates "heat" diffusing through the knowledge graph to discover implicit relations
type HeatKernelDiffusion struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	timeParam    float64 // t parameter for diffusion (controls spread)
}

// NodeActivation represents activation level of a node after diffusion
type NodeActivation struct {
	NodeID     string
	Label      string
	Activation float64 // 0-1: how "hot" this node is
	Distance   int     // Hop distance from source
}

// Path represents a path between two nodes
type Path struct {
	Nodes    []string
	Edges    []string
	Strength float64 // Combined edge weights
	Hops     int
}

// Insight represents a discovered implicit relation
type Insight struct {
	NodeA      string
	NodeB      string
	Relation   string
	Confidence float64
	Evidence   []Path
}

// NewHeatKernelDiffusion creates a new heat kernel diffusion engine
func NewHeatKernelDiffusion(graphAdapter *nietzscheInfra.GraphAdapter, timeParam float64) *HeatKernelDiffusion {
	return &HeatKernelDiffusion{
		graphAdapter: graphAdapter,
		timeParam:    timeParam,
	}
}

// DiffuseFromNode simulates heat diffusion from a starting node
func (hk *HeatKernelDiffusion) DiffuseFromNode(ctx context.Context, nodeID string, maxSteps int) ([]NodeActivation, error) {
	// Initialize activation map
	activations := make(map[string]float64)
	activations[nodeID] = 1.0 // Source node starts at full activation

	// Iteratively diffuse heat
	for step := 0; step < maxSteps; step++ {
		newActivations := make(map[string]float64)

		// For each active node, spread heat to neighbors
		for currentNode, activation := range activations {
			if activation < 0.01 { // Skip very low activations
				continue
			}

			// Get neighbors from Neo4j
			neighbors, err := hk.getNeighbors(ctx, currentNode)
			if err != nil {
				continue
			}

			// Distribute heat to neighbors
			heatPerNeighbor := activation * hk.diffusionRate(step)
			for _, neighbor := range neighbors {
				newActivations[neighbor] += heatPerNeighbor
			}

			// Keep some heat at current node (decay)
			newActivations[currentNode] += activation * (1.0 - hk.diffusionRate(step))
		}

		activations = newActivations
	}

	// Convert to sorted list
	result := []NodeActivation{}
	for nodeID, activation := range activations {
		if activation > 0.05 { // Only return significant activations
			result = append(result, NodeActivation{
				NodeID:     nodeID,
				Activation: activation,
			})
		}
	}

	// Sort by activation (highest first)
	sortByActivation(result)

	return result, nil
}

// FindImplicitRelations discovers non-obvious connections between two nodes
func (hk *HeatKernelDiffusion) FindImplicitRelations(ctx context.Context, nodeA, nodeB string) ([]Path, error) {
	// Diffuse from both nodes
	activationsA, err := hk.DiffuseFromNode(ctx, nodeA, 5)
	if err != nil {
		return nil, err
	}

	activationsB, err := hk.DiffuseFromNode(ctx, nodeB, 5)
	if err != nil {
		return nil, err
	}

	// Find nodes activated by both (potential bridges)
	bridges := findCommonActivations(activationsA, activationsB)

	// For each bridge, construct paths
	paths := []Path{}
	for _, bridge := range bridges {
		// Find path A -> bridge
		pathA, err := hk.findShortestPath(ctx, nodeA, bridge.NodeID)
		if err != nil {
			continue
		}

		// Find path bridge -> B
		pathB, err := hk.findShortestPath(ctx, bridge.NodeID, nodeB)
		if err != nil {
			continue
		}

		// Combine paths
		combinedPath := combinePaths(pathA, pathB)
		combinedPath.Strength = bridge.Activation // Use activation as strength
		paths = append(paths, combinedPath)
	}

	return paths, nil
}

// SimulateInsight simulates cognitive "insight" by finding unexpected connections
func (hk *HeatKernelDiffusion) SimulateInsight(ctx context.Context, query string) ([]Insight, error) {
	// Extract key concepts from query (simplified - would use NLP in production)
	concepts := extractConcepts(query)

	insights := []Insight{}

	// For each pair of concepts, find implicit relations
	for i := 0; i < len(concepts); i++ {
		for j := i + 1; j < len(concepts); j++ {
			paths, err := hk.FindImplicitRelations(ctx, concepts[i], concepts[j])
			if err != nil {
				continue
			}

			if len(paths) > 0 {
				// Generate insight
				insight := Insight{
					NodeA:      concepts[i],
					NodeB:      concepts[j],
					Relation:   inferRelation(paths),
					Confidence: calculateConfidence(paths),
					Evidence:   paths,
				}
				insights = append(insights, insight)
			}
		}
	}

	return insights, nil
}

// Helper functions

func (hk *HeatKernelDiffusion) diffusionRate(step int) float64 {
	// Heat diffusion rate decreases with time
	// Using exponential decay: e^(-t/τ)
	return math.Exp(-float64(step) / hk.timeParam)
}

func (hk *HeatKernelDiffusion) getNeighbors(ctx context.Context, nodeID string) ([]string, error) {
	// Get neighbors via BFS depth 1
	neighborIDs, err := hk.graphAdapter.Bfs(ctx, nodeID, 1, "")
	if err != nil {
		return nil, err
	}
	// Filter out the source node itself
	neighbors := make([]string, 0, len(neighborIDs))
	for _, nid := range neighborIDs {
		if nid != nodeID {
			neighbors = append(neighbors, nid)
		}
	}
	return neighbors, nil
}

func (hk *HeatKernelDiffusion) findShortestPath(ctx context.Context, startID, endID string) (Path, error) {
	// Use BFS to find path between nodes
	neighborIDs, err := hk.graphAdapter.Bfs(ctx, startID, 10, "")
	if err != nil {
		return Path{}, err
	}
	for i, nid := range neighborIDs {
		if nid == endID {
			return Path{
				Nodes: []string{startID, endID},
				Hops:  i + 1,
			}, nil
		}
	}
	return Path{
		Nodes: []string{startID, endID},
		Hops:  1,
	}, nil
}

func sortByActivation(activations []NodeActivation) {
	// Simple bubble sort (would use sort.Slice in production)
	for i := 0; i < len(activations); i++ {
		for j := i + 1; j < len(activations); j++ {
			if activations[j].Activation > activations[i].Activation {
				activations[i], activations[j] = activations[j], activations[i]
			}
		}
	}
}

func findCommonActivations(a, b []NodeActivation) []NodeActivation {
	common := []NodeActivation{}
	aMap := make(map[string]float64)

	for _, node := range a {
		aMap[node.NodeID] = node.Activation
	}

	for _, node := range b {
		if activationA, exists := aMap[node.NodeID]; exists {
			// Average activations from both sides
			avgActivation := (activationA + node.Activation) / 2.0
			common = append(common, NodeActivation{
				NodeID:     node.NodeID,
				Activation: avgActivation,
			})
		}
	}

	return common
}

func combinePaths(pathA, pathB Path) Path {
	return Path{
		Nodes: append(pathA.Nodes, pathB.Nodes...),
		Hops:  pathA.Hops + pathB.Hops,
	}
}

func extractConcepts(query string) []string {
	// Simplified concept extraction
	// In production, would use NLP/NER
	return []string{"concept1", "concept2"}
}

func inferRelation(paths []Path) string {
	// Infer type of relation from paths
	if len(paths) == 0 {
		return "unknown"
	}

	// Simplified - would analyze edge types in production
	if paths[0].Hops <= 2 {
		return "directly_related"
	} else if paths[0].Hops <= 4 {
		return "indirectly_related"
	}

	return "distantly_related"
}

func calculateConfidence(paths []Path) float64 {
	if len(paths) == 0 {
		return 0.0
	}

	// More paths = higher confidence
	// Shorter paths = higher confidence
	avgHops := 0.0
	for _, path := range paths {
		avgHops += float64(path.Hops)
	}
	avgHops /= float64(len(paths))

	// Confidence decreases with hop distance
	confidence := 1.0 / (1.0 + avgHops/3.0)

	// Boost for multiple paths
	confidence *= math.Min(1.0+float64(len(paths))*0.1, 1.5)

	return math.Min(confidence, 1.0)
}
