// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later
//
// AQL (Agent Query Language) — Go types for EVA integration.
// Maps AQL verbs, epistemic types, and cognitive results to Go structs.

package aql

import "time"

// Verb represents one of the 13 AQL cognitive verbs.
type Verb string

const (
	VerbRecall    Verb = "RECALL"
	VerbResonate  Verb = "RESONATE"
	VerbReflect   Verb = "REFLECT"
	VerbTrace     Verb = "TRACE"
	VerbImprint   Verb = "IMPRINT"
	VerbAssociate Verb = "ASSOCIATE"
	VerbDistill   Verb = "DISTILL"
	VerbFade      Verb = "FADE"
	VerbDescend   Verb = "DESCEND"
	VerbAscend    Verb = "ASCEND"
	VerbOrbit     Verb = "ORBIT"
	VerbDream     Verb = "DREAM"
	VerbImagine   Verb = "IMAGINE"
)

// EpistemicType classifies knowledge according to AQL's epistemic model.
type EpistemicType string

const (
	Belief     EpistemicType = "Belief"
	Experience EpistemicType = "Experience"
	Pattern    EpistemicType = "Pattern"
	Signal     EpistemicType = "Signal"
	Intention  EpistemicType = "Intention"
)

// InitialEnergy returns the default energy for an epistemic type.
func (e EpistemicType) InitialEnergy() float32 {
	switch e {
	case Belief:
		return 0.6
	case Experience:
		return 0.5
	case Pattern:
		return 0.8
	case Signal:
		return 0.3
	case Intention:
		return 0.7
	default:
		return 0.5
	}
}

// NietzscheNodeType maps the epistemic type to a NietzscheDB NodeType string.
func (e EpistemicType) NietzscheNodeType() string {
	switch e {
	case Belief:
		return "Semantic"
	case Experience:
		return "Episodic"
	case Pattern:
		return "Semantic"
	case Signal:
		return "Semantic"
	case Intention:
		return "Concept"
	default:
		return "Semantic"
	}
}

// MoodState modifies planner behavior globally.
type MoodState string

const (
	MoodCreative     MoodState = "creative"
	MoodAnalytical   MoodState = "analytical"
	MoodAnxious      MoodState = "anxious"
	MoodFocused      MoodState = "focused"
	MoodExploratory  MoodState = "exploratory"
	MoodConservative MoodState = "conservative"
)

// ValenceSpec describes emotional polarity.
type ValenceSpec string

const (
	ValencePositive ValenceSpec = "positive"
	ValenceNegative ValenceSpec = "negative"
	ValenceNeutral  ValenceSpec = "neutral"
)

// RecencyDegree represents temporal proximity.
type RecencyDegree string

const (
	RecencyFresh   RecencyDegree = "fresh"   // < 5 min
	RecencyRecent  RecencyDegree = "recent"  // < 1 hour
	RecencyDistant RecencyDegree = "distant" // < 24 hours
	RecencyAncient RecencyDegree = "ancient" // no limit
)

// ── Statement (AQL input) ──────────────────────────────────────────

// Statement represents a single AQL statement to be executed.
type Statement struct {
	Verb       Verb              `json:"verb"`
	Query      string            `json:"query,omitempty"`       // RECALL, RESONATE, DESCEND, ASCEND, ORBIT
	Content    string            `json:"content,omitempty"`     // IMPRINT
	From       string            `json:"from,omitempty"`        // TRACE, ASSOCIATE
	To         string            `json:"to,omitempty"`          // TRACE, ASSOCIATE
	Topic      string            `json:"topic,omitempty"`       // DREAM
	Premise    string            `json:"premise,omitempty"`     // IMAGINE
	Collection string            `json:"collection,omitempty"`  // target collection
	Limit      int               `json:"limit,omitempty"`
	Confidence float32           `json:"confidence,omitempty"`  // epistemic confidence floor (filters nodes with energy < threshold)
	Mood       MoodState         `json:"mood,omitempty"`        // TODO: implement mood-based planner behavior
	Valence    ValenceSpec       `json:"valence,omitempty"`     // filter by emotional polarity (positive/negative/neutral)
	Recency    RecencyDegree     `json:"recency,omitempty"`     // filter by temporal proximity (fresh/recent/distant/ancient)
	Epistemic  EpistemicType     `json:"epistemic,omitempty"`   // IMPRINT type
	Energy     float32           `json:"energy,omitempty"`      // IMPRINT initial energy
	Depth      int               `json:"depth,omitempty"`       // DESCEND, ASCEND, RESONATE, TRACE
	Radius     float32           `json:"radius,omitempty"`      // ORBIT        // TODO: implement filtering
	EdgeType   string            `json:"edge_type,omitempty"`   // ASSOCIATE
	Weight     float32           `json:"weight,omitempty"`      // ASSOCIATE
	LinkTo     string            `json:"link_to,omitempty"`     // IMPRINT → auto-link
	NodeIDs    []string          `json:"node_ids,omitempty"`    // REFLECT (multi), FADE (targeted)
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// ── Result types ────────────────────────────────────────────────────

// CognitiveNode represents a node returned by AQL.
type CognitiveNode struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	NodeType  string                 `json:"node_type"`
	Energy    float32                `json:"energy"`
	Magnitude float32                `json:"magnitude,omitempty"`
	Valence   float32                `json:"valence,omitempty"`
	Arousal   float32                `json:"arousal,omitempty"`
	CreatedAt *time.Time             `json:"created_at,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// CognitiveEdge represents an edge returned by AQL.
type CognitiveEdge struct {
	Source   string  `json:"source"`
	Target   string  `json:"target"`
	EdgeType string  `json:"edge_type"`
	Weight   float32 `json:"weight"`
}

// ResultMetadata holds aggregate stats about a cognitive result.
type ResultMetadata struct {
	Count          int    `json:"count"`
	AvgEnergy      float32 `json:"avg_energy"`
	MaxEnergy      float32 `json:"max_energy"`
	ExecutionMs    int64  `json:"execution_ms"`
	Backend        string `json:"backend"`
	Verb           string `json:"verb"`
	SideEffects    []string `json:"side_effects,omitempty"`
}

// CognitiveResult is the unified return type for all AQL operations.
type CognitiveResult struct {
	Nodes    []CognitiveNode `json:"nodes"`
	Edges    []CognitiveEdge `json:"edges,omitempty"`
	Metadata ResultMetadata  `json:"metadata"`
}

