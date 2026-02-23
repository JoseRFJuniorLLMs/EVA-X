// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// NarrativeAdapter exposes NietzscheDB's Narrative Engine (Phase 15.7) for EVA.
// The narrative engine compiles graph state changes, energy distributions, and
// structural patterns into human-readable stories. It detects emergence, conflict,
// decay, and recurrence arcs via the NQL NARRATE command.
type NarrativeAdapter struct {
	client *Client
}

// NewNarrativeAdapter creates an adapter for narrative engine operations.
func NewNarrativeAdapter(client *Client) *NarrativeAdapter {
	return &NarrativeAdapter{client: client}
}

// ── Result Types ─────────────────────────────────────────────────────────────

// NarrativeReport is the full structured narrative from NietzscheDB.
// Mirrors the Rust NarrativeReport struct from nietzsche-narrative crate.
type NarrativeReport struct {
	Collection  string           `json:"collection"`
	WindowHours uint64           `json:"window_hours"`
	GeneratedAt int64            `json:"generated_at"`
	TotalNodes  int              `json:"total_nodes"`
	TotalEdges  int              `json:"total_edges"`
	Events      []NarrativeEvent `json:"events"`
	Statistics  NarrativeStats   `json:"statistics"`
	Summary     string           `json:"summary"`
}

// NarrativeEvent is a single detected narrative arc event.
type NarrativeEvent struct {
	EventType    string   `json:"event_type"`
	Description  string   `json:"description"`
	NodeIDs      []string `json:"node_ids"`
	Significance float64  `json:"significance"`
}

// NarrativeStats holds graph-level statistics for narrative context.
type NarrativeStats struct {
	MeanEnergy     float64            `json:"mean_energy"`
	MaxEnergy      float64            `json:"max_energy"`
	MinEnergy      float64            `json:"min_energy"`
	MeanDepth      float64            `json:"mean_depth"`
	NodeTypeCounts []NodeTypeCount    `json:"node_type_counts"`
}

// NodeTypeCount maps a node type name to its count.
type NodeTypeCount struct {
	Type  string `json:"type"`
	Count int    `json:"count"`
}

// NarrativeSummary is a lightweight text-only narrative result.
type NarrativeSummary struct {
	Summary    string `json:"summary"`
	TotalNodes int64  `json:"total_nodes"`
	TotalEdges int64  `json:"total_edges"`
	EventCount int64  `json:"event_count"`
}

// EvolutionStory combines narrative reports with graph algorithm insights
// to produce a rich description of how a collection evolved over time.
type EvolutionStory struct {
	Narrative    *NarrativeReport `json:"narrative"`
	TopNodes     []ScoredNode     `json:"top_nodes,omitempty"`
	Communities  uint64           `json:"communities"`
	BridgeNodes  []ScoredNode     `json:"bridge_nodes,omitempty"`
	StoryText    string           `json:"story_text"`
}

// ScoredNode is a node ID with a numeric score (PageRank, betweenness, etc.).
type ScoredNode struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

// ChangeSummary describes graph changes since a given timestamp.
type ChangeSummary struct {
	Collection     string           `json:"collection"`
	Since          time.Time        `json:"since"`
	WindowHours    uint64           `json:"window_hours"`
	Narrative      *NarrativeReport `json:"narrative"`
	ChangeText     string           `json:"change_text"`
}

// ── Core Narrative Operations ────────────────────────────────────────────────

// GenerateNarrative produces a full structured narrative for a collection.
// Uses NQL `NARRATE IN "col" WINDOW hours FORMAT json` to retrieve the
// NarrativeReport from the server's narrative engine.
// windowHours controls the time window (0 = all time).
func (n *NarrativeAdapter) GenerateNarrative(ctx context.Context, collection string, windowHours uint64) (*NarrativeReport, error) {
	log := logger.Nietzsche()
	log.Info().
		Str("collection", collection).
		Uint64("window_hours", windowHours).
		Msg("[Narrative] Generating narrative report")

	nql := fmt.Sprintf(`NARRATE IN "%s" WINDOW %d FORMAT json`, collection, windowHours)

	result, err := n.client.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).
			Str("collection", collection).
			Msg("[Narrative] NARRATE query failed")
		return nil, fmt.Errorf("narrative generate: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("narrative error: %s", result.Error)
	}

	// NARRATE FORMAT json returns a single scalar row with column "narrative"
	// containing the full JSON report.
	report, err := parseNarrativeJSON(result.ScalarRows)
	if err != nil {
		log.Error().Err(err).Msg("[Narrative] Failed to parse narrative JSON")
		return nil, fmt.Errorf("narrative parse: %w", err)
	}

	log.Info().
		Str("collection", collection).
		Int("total_nodes", report.TotalNodes).
		Int("total_edges", report.TotalEdges).
		Int("events", len(report.Events)).
		Msg("[Narrative] Narrative report generated")

	return report, nil
}

// GenerateNarrativeText produces a lightweight text-only narrative summary.
// Uses NQL `NARRATE IN "col" WINDOW hours` (text format, the default).
func (n *NarrativeAdapter) GenerateNarrativeText(ctx context.Context, collection string, windowHours uint64) (*NarrativeSummary, error) {
	log := logger.Nietzsche()

	nql := fmt.Sprintf(`NARRATE IN "%s" WINDOW %d`, collection, windowHours)

	result, err := n.client.Query(ctx, nql, nil, collection)
	if err != nil {
		log.Error().Err(err).Str("collection", collection).Msg("[Narrative] NARRATE text query failed")
		return nil, fmt.Errorf("narrative text: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("narrative text error: %s", result.Error)
	}

	summary := parseNarrativeText(result.ScalarRows)

	log.Debug().
		Str("collection", collection).
		Int64("total_nodes", summary.TotalNodes).
		Msg("[Narrative] Text narrative generated")

	return summary, nil
}

// ── Composite Operations ─────────────────────────────────────────────────────

// GetEvolutionStory builds a rich narrative combining the narrative engine output
// with PageRank scores and community detection to describe how a collection evolved.
// This runs NARRATE + PageRank + Louvain + Betweenness in parallel.
func (n *NarrativeAdapter) GetEvolutionStory(ctx context.Context, collection string) (*EvolutionStory, error) {
	log := logger.Nietzsche()
	log.Info().Str("collection", collection).Msg("[Narrative] Building evolution story (NARRATE + PageRank + Louvain + Betweenness)")

	var (
		report     *NarrativeReport
		prScores   []ScoredNode
		bridges    []ScoredNode
		comCount   uint64

		reportErr, prErr, comErr, betErr error
		wg                               sync.WaitGroup
	)

	// Run narrative + algorithms in parallel
	wg.Add(4)

	go func() {
		defer wg.Done()
		report, reportErr = n.GenerateNarrative(ctx, collection, 0)
	}()
	go func() {
		defer wg.Done()
		pr, err := n.client.RunPageRank(ctx, collection, 0.85, 100)
		if err != nil {
			prErr = err
			return
		}
		prScores = topScoredNodes(pr.Scores, 10)
	}()
	go func() {
		defer wg.Done()
		com, err := n.client.RunLouvain(ctx, collection, 100, 1.0)
		if err != nil {
			comErr = err
			return
		}
		comCount = com.CommunityCount
	}()
	go func() {
		defer wg.Done()
		bet, err := n.client.RunBetweenness(ctx, collection, 0)
		if err != nil {
			betErr = err
			return
		}
		bridges = topScoredNodes(bet.Scores, 5)
	}()

	wg.Wait()

	// Log individual errors but don't fail completely
	if prErr != nil {
		log.Warn().Err(prErr).Msg("[Narrative] PageRank failed (non-fatal)")
	}
	if comErr != nil {
		log.Warn().Err(comErr).Msg("[Narrative] Louvain failed (non-fatal)")
	}
	if betErr != nil {
		log.Warn().Err(betErr).Msg("[Narrative] Betweenness failed (non-fatal)")
	}

	// Narrative is the critical component
	if reportErr != nil {
		log.Error().Err(reportErr).Msg("[Narrative] Narrative engine failed")
		return nil, fmt.Errorf("narrative evolution story: %w", reportErr)
	}

	story := &EvolutionStory{
		Narrative:   report,
		TopNodes:    prScores,
		Communities: comCount,
		BridgeNodes: bridges,
	}

	story.StoryText = composeEvolutionText(story)

	log.Info().
		Str("collection", collection).
		Int("top_nodes", len(prScores)).
		Uint64("communities", comCount).
		Int("bridges", len(bridges)).
		Msg("[Narrative] Evolution story complete")

	return story, nil
}

// SummarizeChanges generates a narrative focused on changes since a given time.
// Computes the window in hours from `since` to now and runs NARRATE with that window.
func (n *NarrativeAdapter) SummarizeChanges(ctx context.Context, collection string, since time.Time) (*ChangeSummary, error) {
	log := logger.Nietzsche()

	now := time.Now()
	if since.After(now) {
		return nil, fmt.Errorf("narrative: 'since' time %v is in the future", since)
	}

	windowHours := uint64(now.Sub(since).Hours())
	if windowHours == 0 {
		windowHours = 1 // minimum 1 hour window
	}

	log.Info().
		Str("collection", collection).
		Time("since", since).
		Uint64("window_hours", windowHours).
		Msg("[Narrative] Summarizing changes")

	report, err := n.GenerateNarrative(ctx, collection, windowHours)
	if err != nil {
		return nil, fmt.Errorf("narrative summarize changes: %w", err)
	}

	changeText := composeChangeText(report, since)

	log.Info().
		Str("collection", collection).
		Int("events", len(report.Events)).
		Msg("[Narrative] Change summary generated")

	return &ChangeSummary{
		Collection:  collection,
		Since:       since,
		WindowHours: windowHours,
		Narrative:   report,
		ChangeText:  changeText,
	}, nil
}

// NarrateForNodes generates a narrative and filters events to those mentioning
// the given node IDs. Useful for understanding the story around specific nodes.
func (n *NarrativeAdapter) NarrateForNodes(ctx context.Context, collection string, nodeIDs []string) (*NarrativeReport, error) {
	log := logger.Nietzsche()
	log.Info().
		Str("collection", collection).
		Int("node_count", len(nodeIDs)).
		Msg("[Narrative] Generating narrative filtered by nodes")

	// Generate full narrative (all time)
	report, err := n.GenerateNarrative(ctx, collection, 0)
	if err != nil {
		return nil, err
	}

	// Build lookup set for requested node IDs
	nodeSet := make(map[string]struct{}, len(nodeIDs))
	for _, id := range nodeIDs {
		nodeSet[id] = struct{}{}
	}

	// Filter events to those involving at least one of the requested nodes
	var filtered []NarrativeEvent
	for _, event := range report.Events {
		if eventInvolvesNodes(event, nodeSet) {
			filtered = append(filtered, event)
		}
	}

	report.Events = filtered

	log.Debug().
		Str("collection", collection).
		Int("filtered_events", len(filtered)).
		Msg("[Narrative] Node-filtered narrative complete")

	return report, nil
}

// ── Internal Helpers ─────────────────────────────────────────────────────────

// parseNarrativeJSON extracts a NarrativeReport from scalar rows returned by
// NARRATE FORMAT json. The server returns a single row with column "narrative".
func parseNarrativeJSON(rows []map[string]interface{}) (*NarrativeReport, error) {
	if len(rows) == 0 {
		return &NarrativeReport{Summary: "No narrative data available."}, nil
	}

	row := rows[0]
	narrativeVal, ok := row["narrative"]
	if !ok {
		return &NarrativeReport{Summary: "No narrative data available."}, nil
	}

	jsonStr, ok := narrativeVal.(string)
	if !ok {
		return nil, fmt.Errorf("narrative column is not a string: %T", narrativeVal)
	}

	var report NarrativeReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("narrative JSON unmarshal: %w", err)
	}

	return &report, nil
}

// parseNarrativeText extracts a NarrativeSummary from scalar rows returned by
// NARRATE (text format). The server returns: summary, total_nodes, total_edges, events.
func parseNarrativeText(rows []map[string]interface{}) *NarrativeSummary {
	summary := &NarrativeSummary{}

	if len(rows) == 0 {
		summary.Summary = "No narrative data available."
		return summary
	}

	row := rows[0]

	if v, ok := row["summary"]; ok {
		if s, ok := v.(string); ok {
			summary.Summary = s
		}
	}
	if v, ok := row["total_nodes"]; ok {
		summary.TotalNodes = toInt64(v)
	}
	if v, ok := row["total_edges"]; ok {
		summary.TotalEdges = toInt64(v)
	}
	if v, ok := row["events"]; ok {
		summary.EventCount = toInt64(v)
	}

	return summary
}

// toInt64 safely converts various numeric types to int64.
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	case int:
		return int64(val)
	default:
		return 0
	}
}

// topScoredNodes extracts the top N scored nodes from an algorithm result.
func topScoredNodes(scores []nietzsche.NodeScore, limit int) []ScoredNode {
	result := make([]ScoredNode, 0, len(scores))
	for _, ns := range scores {
		result = append(result, ScoredNode{ID: ns.NodeID, Score: ns.Score})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// eventInvolvesNodes checks whether any node ID in the event is in the given set.
func eventInvolvesNodes(event NarrativeEvent, nodeSet map[string]struct{}) bool {
	for _, nid := range event.NodeIDs {
		if _, ok := nodeSet[nid]; ok {
			return true
		}
	}
	return false
}

// composeEvolutionText produces a human-readable evolution story from composite data.
func composeEvolutionText(story *EvolutionStory) string {
	var parts []string

	// Start with the narrative summary
	if story.Narrative != nil && story.Narrative.Summary != "" {
		parts = append(parts, story.Narrative.Summary)
	}

	// Add community info
	if story.Communities > 0 {
		parts = append(parts, fmt.Sprintf(
			"The graph organizes into %d distinct communities.", story.Communities))
	}

	// Add top PageRank nodes
	if len(story.TopNodes) > 0 {
		ids := make([]string, 0, len(story.TopNodes))
		for _, n := range story.TopNodes {
			if len(ids) >= 3 {
				break
			}
			ids = append(ids, n.ID)
		}
		parts = append(parts, fmt.Sprintf(
			"Most influential nodes: %s.", strings.Join(ids, ", ")))
	}

	// Add bridge nodes
	if len(story.BridgeNodes) > 0 {
		ids := make([]string, 0, len(story.BridgeNodes))
		for _, n := range story.BridgeNodes {
			if len(ids) >= 3 {
				break
			}
			ids = append(ids, n.ID)
		}
		parts = append(parts, fmt.Sprintf(
			"Key bridge nodes connecting communities: %s.", strings.Join(ids, ", ")))
	}

	// Add event highlights
	if story.Narrative != nil && len(story.Narrative.Events) > 0 {
		parts = append(parts, fmt.Sprintf(
			"%d significant events detected.", len(story.Narrative.Events)))
		for i, event := range story.Narrative.Events {
			if i >= 3 {
				break
			}
			parts = append(parts, fmt.Sprintf("  - %s", event.Description))
		}
	}

	if len(parts) == 0 {
		return "No evolution story available."
	}

	return strings.Join(parts, " ")
}

// composeChangeText produces a human-readable change summary for a time window.
func composeChangeText(report *NarrativeReport, since time.Time) string {
	var parts []string

	parts = append(parts, fmt.Sprintf(
		"Changes in '%s' since %s:",
		report.Collection, since.Format(time.RFC3339)))

	parts = append(parts, fmt.Sprintf(
		"%d nodes, %d edges in the analyzed window.",
		report.TotalNodes, report.TotalEdges))

	if report.Statistics.MeanEnergy > 0 {
		parts = append(parts, fmt.Sprintf(
			"Energy landscape: mean=%.3f, peak=%.3f.",
			report.Statistics.MeanEnergy, report.Statistics.MaxEnergy))
	}

	if len(report.Events) == 0 {
		parts = append(parts, "No significant events detected in this period.")
	} else {
		parts = append(parts, fmt.Sprintf("%d events detected:", len(report.Events)))
		for i, event := range report.Events {
			if i >= 5 {
				parts = append(parts, fmt.Sprintf("  ... and %d more.", len(report.Events)-5))
				break
			}
			parts = append(parts, fmt.Sprintf("  - [%s] %s (significance: %.2f)",
				event.EventType, event.Description, event.Significance))
		}
	}

	return strings.Join(parts, " ")
}
