// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package personality

import (
	"context"
	"fmt"
	"log"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// EnergyOverlay projects personality traits as energy vectors in NietzscheDB.
// Existing SQL storage remains the ground truth; NietzscheDB acts as an energy
// overlay that enables Zaratustra evolution and elite-trait selection.
type EnergyOverlay struct {
	graph      *nietzscheInfra.GraphAdapter
	collection string // "eva_core"
}

// NewEnergyOverlay creates the overlay targeting the eva_core collection.
func NewEnergyOverlay(graph *nietzscheInfra.GraphAdapter) *EnergyOverlay {
	return &EnergyOverlay{
		graph:      graph,
		collection: "eva_core",
	}
}

// SyncTraitNode merges a single personality trait into NietzscheDB.
// It creates the node on first call (MERGE semantics) and updates energy on
// subsequent calls. The energy field mirrors the trait value (0-1).
func (eo *EnergyOverlay) SyncTraitNode(ctx context.Context, traitName string, value float64, isBigFive bool) error {
	if eo.graph == nil {
		return nil // NietzscheDB not configured; silently skip
	}

	category := "enneagram"
	if isBigFive {
		category = "big_five"
	}

	energy := float32(value)
	if energy < 0 {
		energy = 0
	}
	if energy > 1 {
		energy = 1
	}

	_, err := eo.graph.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		Collection: eo.collection,
		NodeType:   "PersonalityTrait",
		MatchKeys: map[string]interface{}{
			"name": traitName,
		},
		OnCreateSet: map[string]interface{}{
			"name":     traitName,
			"category": category,
			"value":    value,
		},
		OnMatchSet: map[string]interface{}{
			"value": value,
		},
		Energy: energy,
	})
	if err != nil {
		return fmt.Errorf("sync trait %s: %w", traitName, err)
	}

	return nil
}

// SyncBigFive projects all five OCEAN dimensions into eva_core.
func (eo *EnergyOverlay) SyncBigFive(ctx context.Context, profile BigFiveProfile) error {
	traits := map[string]float64{
		"openness":          profile.Openness,
		"conscientiousness": profile.Conscientiousness,
		"extraversion":      profile.Extraversion,
		"agreeableness":     profile.Agreeableness,
		"neuroticism":       profile.Neuroticism,
	}

	for name, val := range traits {
		if err := eo.SyncTraitNode(ctx, name, val, true); err != nil {
			log.Printf("[ENERGY_OVERLAY] failed to sync Big Five trait %s: %v", name, err)
			// continue syncing the rest
		}
	}

	return nil
}

// SyncEnneagramDistribution projects the 9-type probability distribution.
// Each type becomes a node whose energy equals the probability weight.
func (eo *EnergyOverlay) SyncEnneagramDistribution(ctx context.Context, dist *PersonalityDistribution) error {
	if dist == nil {
		return nil
	}

	for i, weight := range dist.Types {
		name := fmt.Sprintf("enneagram_type_%d", i+1)
		if err := eo.SyncTraitNode(ctx, name, weight, false); err != nil {
			log.Printf("[ENERGY_OVERLAY] failed to sync Enneagram type %d: %v", i+1, err)
		}
	}

	return nil
}

// UpdateTraitEnergy adjusts a trait node's energy after a session observation.
// This is called after session processing when trait values change.
func (eo *EnergyOverlay) UpdateTraitEnergy(ctx context.Context, traitName string, newValue float64) error {
	if eo.graph == nil {
		return nil
	}

	energy := float32(newValue)
	if energy < 0 {
		energy = 0
	}
	if energy > 1 {
		energy = 1
	}

	nodeID := fmt.Sprintf("PersonalityTrait:%s", traitName)
	return eo.graph.UpdateEnergy(ctx, nodeID, energy, eo.collection)
}

// EliteTraits queries eva_core for personality traits with energy > threshold.
// After Zaratustra runs, elite traits (energy > 0.8) become dominant for the
// next session's personality profile.
func (eo *EnergyOverlay) EliteTraits(ctx context.Context, threshold float64) ([]EliteTrait, error) {
	if eo.graph == nil {
		return nil, nil
	}

	nql := `MATCH (n:PersonalityTrait) WHERE n.energy > $threshold RETURN n`
	result, err := eo.graph.ExecuteNQL(ctx, nql, map[string]interface{}{
		"threshold": threshold,
	}, eo.collection)
	if err != nil {
		return nil, fmt.Errorf("elite traits query: %w", err)
	}

	var elites []EliteTrait
	for _, node := range result.Nodes {
		name, _ := node.Content["name"].(string)
		category, _ := node.Content["category"].(string)
		value, _ := node.Content["value"].(float64)

		if name == "" {
			continue
		}

		elites = append(elites, EliteTrait{
			Name:     name,
			Category: category,
			Value:    value,
			Energy:   float64(node.Energy),
		})
	}

	return elites, nil
}

// EliteTrait represents a personality trait that reached elite energy status
// after Zaratustra evolution. These traits dominate the next session.
type EliteTrait struct {
	Name     string  // e.g. "openness", "enneagram_type_2"
	Category string  // "big_five" or "enneagram"
	Value    float64 // current trait value (0-1)
	Energy   float64 // NietzscheDB energy post-Zaratustra
}
