// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package diffusor

import (
	"context"
	"math"
	"sort"

	nietzsche "nietzsche-sdk"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// HeatKernelDiffusion implements heat kernel-based graph navigation using
// NietzscheDB's native Diffuse RPC. Instead of reimplementing BFS + manual
// heat calculation locally, this delegates to the server's Chebyshev-accelerated
// heat-kernel solver for correctness and performance.
type HeatKernelDiffusion struct {
	graphAdapter *nietzscheInfra.GraphAdapter
	timeParam    float64 // t parameter for diffusion (controls spread)
	collection   string  // NietzscheDB collection for diffusion
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

// NewHeatKernelDiffusion creates a new heat kernel diffusion engine.
// The collection parameter specifies which NietzscheDB collection to run
// diffusion on. If empty, the GraphAdapter's default collection is used.
func NewHeatKernelDiffusion(graphAdapter *nietzscheInfra.GraphAdapter, timeParam float64) *HeatKernelDiffusion {
	return &HeatKernelDiffusion{
		graphAdapter: graphAdapter,
		timeParam:    timeParam,
	}
}

// SetCollection overrides the collection used for Diffuse RPC calls.
func (hk *HeatKernelDiffusion) SetCollection(collection string) {
	hk.collection = collection
}

// DiffuseFromNode runs heat-kernel diffusion from a starting node using
// NietzscheDB's native Diffuse RPC. The maxSteps parameter is translated
// into multiple diffusion time scales: the server computes e^{-tL} using
// Chebyshev polynomial approximation, which is both faster and more accurate
// than the old local BFS-based iteration.
func (hk *HeatKernelDiffusion) DiffuseFromNode(ctx context.Context, nodeID string, maxSteps int) ([]NodeActivation, error) {
	// Build diffusion time values from timeParam and maxSteps.
	// We sample multiple time scales to capture both local and global structure.
	tValues := hk.buildTValues(maxSteps)

	scales, err := hk.graphAdapter.Diffuse(ctx, []string{nodeID}, nietzsche.DiffuseOpts{
		TValues:    tValues,
		KChebyshev: 0, // server default (10)
		Collection: hk.collection,
	})
	if err != nil {
		return nil, err
	}

	// Merge results across all time scales into a single activation map.
	// For nodes appearing at multiple scales, take the maximum activation.
	activations := make(map[string]float64)
	for _, scale := range scales {
		for i, nid := range scale.NodeIDs {
			if i < len(scale.Scores) {
				if scale.Scores[i] > activations[nid] {
					activations[nid] = scale.Scores[i]
				}
			}
		}
	}

	// Convert to sorted list, filtering out insignificant activations
	result := make([]NodeActivation, 0, len(activations))
	for nid, activation := range activations {
		if activation > 0.05 { // Only return significant activations
			result = append(result, NodeActivation{
				NodeID:     nid,
				Activation: activation,
			})
		}
	}

	// Sort by activation (highest first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Activation > result[j].Activation
	})

	return result, nil
}

// FindImplicitRelations discovers non-obvious connections between two nodes
// by running bidirectional diffusion and finding bridge nodes activated from
// both sides.
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

// ── Internal helpers ─────────────────────────────────────────────────────────

// buildTValues generates diffusion time values from timeParam and maxSteps.
// Produces a logarithmic spread of time values capturing multiple resolution
// scales: small t for local neighbourhood, large t for global structure.
func (hk *HeatKernelDiffusion) buildTValues(maxSteps int) []float64 {
	if maxSteps <= 0 {
		maxSteps = 5
	}

	// Generate 3 time scales: local (t/10), medium (t), global (t*10)
	// This mirrors how the old iterative method explored increasingly
	// distant neighbourhoods with each step.
	baseT := hk.timeParam
	if baseT <= 0 {
		baseT = 1.0
	}

	tValues := []float64{
		baseT * 0.1,  // local neighbourhood
		baseT,        // medium range
		baseT * 10.0, // global structure
	}

	// For high maxSteps, add intermediate scales for finer resolution
	if maxSteps > 5 {
		tValues = append(tValues, baseT*0.5, baseT*5.0)
	}

	return tValues
}

// findShortestPath uses BFS to find a path between two nodes.
func (hk *HeatKernelDiffusion) findShortestPath(ctx context.Context, startID, endID string) (Path, error) {
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
