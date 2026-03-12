// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package perception

import (
	"context"
	"fmt"
	"math"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	nietzsche "nietzsche-sdk"

	"github.com/rs/zerolog/log"
)

// ── Collection & Node Constants ─────────────────────────────────────────────

const (
	// CollectionName is the NietzscheDB collection for 2D perceptions.
	CollectionName = "eva_perceptions"

	// CollectionDim matches the default EVA embedding dimension.
	// Perception nodes use 128-dim coords in the Poincare ball
	// (spatial position mapped to hyperbolic space).
	CollectionDim = 128

	// CollectionMetric uses poincare for hyperbolic hierarchy.
	CollectionMetric = "poincare"

	// Node labels stored in content.node_label
	LabelScene  = "Scene2D"
	LabelObject = "Object2D"

	// Edge types
	EdgeContains     = "CONTAINS"      // Scene -> Object
	EdgeSpatialNear  = "SPATIAL_NEAR"  // Object <-> Object (proximity in frame)
	EdgeTemporalNext = "TEMPORAL_NEXT" // Scene -> Scene (consecutive frames)
	EdgeObservedBy   = "OBSERVED_BY"   // Scene -> Patient/User

	// DefaultTTL for perception nodes (30 seconds — short-term memory)
	DefaultTTL = 30 * time.Second

	// DefaultEnergy for new perception nodes (decays over time via Agency Engine)
	DefaultEnergy float32 = 0.8
)

// PerceptionStore handles all NietzscheDB operations for perception data.
type PerceptionStore struct {
	nz            *nietzscheInfra.Client
	collection    string
	prevSceneID   string // last scene node ID (for TEMPORAL_NEXT edges)
}

// NewPerceptionStore creates a store backed by the given NietzscheDB client.
func NewPerceptionStore(nzClient *nietzscheInfra.Client) *PerceptionStore {
	return &PerceptionStore{
		nz:         nzClient,
		collection: CollectionName,
	}
}

// EnsureCollection creates the eva_perceptions collection if it doesn't exist.
func (ps *PerceptionStore) EnsureCollection(ctx context.Context) error {
	return ps.nz.EnsureCollection(ctx, ps.collection, CollectionDim, CollectionMetric)
}

// StoreScene persists a full SceneAnalysis as a graph structure:
// - 1 Scene2D node (parent)
// - N Object2D nodes (children)
// - N CONTAINS edges (Scene -> Object)
// - M SPATIAL_NEAR edges (nearby objects)
// - 1 TEMPORAL_NEXT edge (previous scene -> this scene)
func (ps *PerceptionStore) StoreScene(ctx context.Context, scene *SceneAnalysis, userID int64) (string, error) {
	sceneID := fmt.Sprintf("scene_%d_%d", userID, scene.Timestamp)

	// 1. Insert Scene2D node
	sceneCoords := sceneToHyperbolicCoords(scene)
	_, err := ps.nz.InsertNode(ctx, nietzsche.InsertNodeOpts{
		ID:       sceneID,
		NodeType: "Semantic",
		Content: map[string]interface{}{
			"node_label":    LabelScene,
			"user_id":       userID,
			"scene_type":    scene.SceneType,
			"lighting":      scene.Lighting,
			"activity":      scene.Activity,
			"people_count":  scene.PeopleCount,
			"risk_factors":  scene.RiskFactors,
			"object_count":  len(scene.Objects),
			"timestamp":     scene.Timestamp,
			"frame_hash":    scene.FrameHash,
			"ttl_expires_at": time.Now().Add(DefaultTTL).Unix(),
		},
		Coords:     sceneCoords,
		Energy:     DefaultEnergy,
		Collection: ps.collection,
	})
	if err != nil {
		log.Error().Err(err).Str("scene_id", sceneID).Msg("[PERCEPTION] Failed to store scene node")
		return "", fmt.Errorf("perception store scene: %w", err)
	}

	// 2. Insert Object2D nodes + CONTAINS edges
	objectIDs := make([]string, 0, len(scene.Objects))
	for i, obj := range scene.Objects {
		objID := fmt.Sprintf("obj_%s_%d_%d_%d", obj.Label, userID, scene.Timestamp, i)
		objCoords := objectToHyperbolicCoords(&obj, sceneCoords)

		_, err := ps.nz.InsertNode(ctx, nietzsche.InsertNodeOpts{
			ID:       objID,
			NodeType: "Semantic",
			Content: map[string]interface{}{
				"node_label":     LabelObject,
				"label":          obj.Label,
				"category":       obj.Category,
				"confidence":     obj.Confidence,
				"x":              obj.X,
				"y":              obj.Y,
				"width":          obj.Width,
				"height":         obj.Height,
				"user_id":        userID,
				"scene_id":       sceneID,
				"timestamp":      scene.Timestamp,
				"ttl_expires_at": time.Now().Add(DefaultTTL).Unix(),
			},
			Coords:     objCoords,
			Energy:     float32(obj.Confidence) * DefaultEnergy,
			Collection: ps.collection,
		})
		if err != nil {
			log.Warn().Err(err).Str("obj", obj.Label).Msg("[PERCEPTION] Failed to store object node")
			continue
		}
		objectIDs = append(objectIDs, objID)

		// CONTAINS edge: Scene -> Object
		_, err = ps.nz.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:       sceneID,
			To:         objID,
			EdgeType:   EdgeContains,
			Weight:     obj.Confidence,
			Collection: ps.collection,
		})
		if err != nil {
			log.Warn().Err(err).Msg("[PERCEPTION] Failed to create CONTAINS edge")
		}
	}

	// 3. SPATIAL_NEAR edges between nearby objects
	ps.createSpatialEdges(ctx, scene.Objects, objectIDs, userID, scene.Timestamp)

	// 4. TEMPORAL_NEXT edge from previous scene
	if ps.prevSceneID != "" {
		_, err = ps.nz.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
			From:       ps.prevSceneID,
			To:         sceneID,
			EdgeType:   EdgeTemporalNext,
			Weight:     1.0,
			Collection: ps.collection,
		})
		if err != nil {
			log.Warn().Err(err).Msg("[PERCEPTION] Failed to create TEMPORAL_NEXT edge")
		}
	}
	ps.prevSceneID = sceneID

	log.Info().
		Str("scene_id", sceneID).
		Int("objects", len(objectIDs)).
		Str("scene_type", scene.SceneType).
		Msg("[PERCEPTION] Scene stored in NietzscheDB")

	return sceneID, nil
}

// StoreHebbianLink creates a cross-modal edge between a perception node
// and a concept/episodic node from another collection.
// This implements "nodes that fire together, wire together" — when the user
// talks about something while the camera sees it, they get linked.
func (ps *PerceptionStore) StoreHebbianLink(ctx context.Context, perceptionNodeID, conceptNodeID, conceptCollection string) error {
	_, err := ps.nz.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
		From:       perceptionNodeID,
		To:         conceptNodeID,
		EdgeType:   "HEBBIAN_VISUAL",
		Weight:     0.5, // starts at 0.5, strengthened by repetition via Agency Engine
		Collection: ps.collection,
	})
	if err != nil {
		log.Warn().Err(err).Msg("[PERCEPTION] Hebbian link failed")
		return fmt.Errorf("perception hebbian link: %w", err)
	}

	log.Debug().
		Str("perception", perceptionNodeID).
		Str("concept", conceptNodeID).
		Msg("[PERCEPTION] Hebbian cross-modal link created")
	return nil
}

// GetCurrentScene returns the most recent scene analysis for a user,
// or nil if no perception data exists.
func (ps *PerceptionStore) GetCurrentScene(ctx context.Context, userID int64) (map[string]interface{}, error) {
	if ps.prevSceneID == "" {
		return nil, nil
	}
	return ps.nz.Get(ctx, ps.collection, ps.prevSceneID)
}

// createSpatialEdges links objects that are spatially close in the 2D frame.
func (ps *PerceptionStore) createSpatialEdges(ctx context.Context, objects []DetectedObject, objectIDs []string, userID int64, ts int64) {
	const proximityThreshold = 0.25 // normalized distance threshold

	for i := 0; i < len(objectIDs); i++ {
		for j := i + 1; j < len(objectIDs); j++ {
			if i >= len(objects) || j >= len(objects) {
				break
			}
			dist := euclideanDist2D(objects[i].X, objects[i].Y, objects[j].X, objects[j].Y)
			if dist < proximityThreshold {
				weight := 1.0 - (dist / proximityThreshold) // closer = stronger
				_, err := ps.nz.InsertEdge(ctx, nietzsche.InsertEdgeOpts{
					From:       objectIDs[i],
					To:         objectIDs[j],
					EdgeType:   EdgeSpatialNear,
					Weight:     weight,
					Collection: ps.collection,
				})
				if err != nil {
					log.Warn().Err(err).Msg("[PERCEPTION] Failed to create SPATIAL_NEAR edge")
				}
			}
		}
	}
}

// ── Hyperbolic Coordinate Mapping ───────────────────────────────────────────
//
// Maps 2D frame positions to 128-dim Poincare ball coordinates.
// Scene nodes sit near the origin (parent/root), objects deeper (higher magnitude).
// This respects the hyperbolic hierarchy: magnitude = depth.

// sceneToHyperbolicCoords generates 128-dim coords for a Scene2D node.
// Scenes are near the origin (low magnitude) as they are parent nodes.
func sceneToHyperbolicCoords(scene *SceneAnalysis) []float64 {
	coords := make([]float64, CollectionDim)

	// Scene hash spreads scenes across different angular positions
	hash := simpleHash(scene.FrameHash)

	// Low magnitude (0.1-0.3) — scenes are parents
	magnitude := 0.15

	// Encode scene properties into first few dimensions
	coords[0] = magnitude * math.Cos(float64(hash)*0.01)
	coords[1] = magnitude * math.Sin(float64(hash)*0.01)
	coords[2] = float64(scene.PeopleCount) * 0.05
	coords[3] = float64(len(scene.Objects)) * 0.02

	// Scene type encoding (dim 4-10)
	sceneTypeIdx := sceneTypeIndex(scene.SceneType)
	if sceneTypeIdx < CollectionDim {
		coords[4+sceneTypeIdx] = 0.1
	}

	return coords
}

// objectToHyperbolicCoords generates 128-dim coords for an Object2D node.
// Objects sit deeper than their parent scene (higher magnitude).
func objectToHyperbolicCoords(obj *DetectedObject, sceneCoords []float64) []float64 {
	coords := make([]float64, CollectionDim)

	// Higher magnitude than scene (0.3-0.6) — objects are children
	magnitude := 0.3 + obj.Confidence*0.3

	// Use object's 2D position to determine angular position
	angle := math.Atan2(obj.Y-0.5, obj.X-0.5)
	coords[0] = magnitude * math.Cos(angle)
	coords[1] = magnitude * math.Sin(angle)

	// Size encoding
	coords[2] = obj.Width * 0.5
	coords[3] = obj.Height * 0.5

	// Category encoding (dim 10-20)
	catIdx := categoryIndex(obj.Category)
	if 10+catIdx < CollectionDim {
		coords[10+catIdx] = magnitude
	}

	// Inherit scene direction (weak parent influence)
	for i := 0; i < min(8, len(sceneCoords)); i++ {
		coords[i] += sceneCoords[i] * 0.1
	}

	// Clamp to Poincare ball (norm < 1.0)
	norm := vectorNorm(coords)
	if norm >= 0.95 {
		scale := 0.9 / norm
		for i := range coords {
			coords[i] *= scale
		}
	}

	return coords
}

// ── Utility functions ───────────────────────────────────────────────────────

func euclideanDist2D(x1, y1, x2, y2 float64) float64 {
	dx := x1 - x2
	dy := y1 - y2
	return math.Sqrt(dx*dx + dy*dy)
}

func vectorNorm(v []float64) float64 {
	sum := 0.0
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}

func simpleHash(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}

func sceneTypeIndex(st string) int {
	types := map[string]int{
		"bedroom": 0, "kitchen": 1, "living_room": 2, "bathroom": 3,
		"office": 4, "outdoor": 5, "unknown": 6,
	}
	if idx, ok := types[st]; ok {
		return idx
	}
	return 6
}

func categoryIndex(cat string) int {
	cats := map[string]int{
		"person": 0, "furniture": 1, "medication": 2, "food": 3, "drink": 4,
		"device": 5, "clothing": 6, "pet": 7, "hazard": 8, "other": 9,
	}
	if idx, ok := cats[cat]; ok {
		return idx
	}
	return 9
}
