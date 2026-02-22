// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package memory

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/pkg/types"
)

type PatternMiner struct {
	graphAdapter *nietzscheInfra.GraphAdapter
}

// Structs moved to pkg/types

func NewPatternMiner(graphAdapter *nietzscheInfra.GraphAdapter) *PatternMiner {
	return &PatternMiner{graphAdapter: graphAdapter}
}

// MineRecurrentPatterns identifica topicos que aparecem multiplas vezes
// Optimized to use native NQL aggregations (GROUP BY, COUNT, MIN, MAX, AVG).
func (pm *PatternMiner) MineRecurrentPatterns(ctx context.Context, idosoID int64, minFrequency int) ([]*types.RecurrentPattern, error) {
	nql := `
		MATCH (p:Person)-[:EXPERIENCED]->(e:Event)-[:RELATED_TO]->(t:Topic)
		WHERE p.id = $idosoId
		RETURN t.name as topic, 
		       COUNT(e) as frequency, 
		       MIN(e.timestamp) as first_seen, 
		       MAX(e.timestamp) as last_seen,
		       AVG(e.importance) as avg_importance
		GROUP BY t.name
		HAVING frequency >= $minFrequency
	`

	result, err := pm.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId":      idosoID,
		"minFrequency": minFrequency,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to mine recurrent patterns via NQL: %w", err)
	}

	var patterns []*types.RecurrentPattern
	for _, row := range result.ScalarRows {
		topic := fmt.Sprintf("%v", row["topic"])
		frequency := int(row["frequency"].(int64))
		firstSeen := time.Unix(int64(row["first_seen"].(float64)), 0)
		lastSeen := time.Unix(int64(row["last_seen"].(float64)), 0)

		// In NQL, we can't easily collect emotions as an array yet,
		// so we keep the emotion list empty or do a secondary shallow query if needed.
		// For MVP, we use the metrics.

		confidence := math.Min(float64(frequency)/10.0, 1.0)

		pattern := &types.RecurrentPattern{
			Topic:       topic,
			Frequency:   frequency,
			FirstSeen:   firstSeen,
			LastSeen:    lastSeen,
			Confidence:  confidence,
			AvgInterval: lastSeen.Sub(firstSeen).Hours() / (24.0 * float64(frequency)), // Approximation
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// MineTemporalPatterns identifica quando certos topicos aparecem (hora do dia, dia da semana)
// Optimized to reduce data transfer by filtering at the database level.
func (pm *PatternMiner) MineTemporalPatterns(ctx context.Context, idosoID int64) ([]*types.TemporalPattern, error) {
	// Query all relevant event/topic pairs
	nql := `
		MATCH (p:Person)-[:EXPERIENCED]->(e:Event)-[:RELATED_TO]->(t:Topic)
		WHERE p.id = $idosoId
		RETURN t.name as topic, e.timestamp as ts
	`
	result, err := pm.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query temporal raw data: %w", err)
	}

	type temporalKey struct {
		topic     string
		timeOfDay string
		dayType   string
	}
	counts := make(map[temporalKey]int)

	for _, row := range result.ScalarRows {
		topicName := fmt.Sprintf("%v", row["topic"])
		tsFloat := row["ts"].(float64)
		ts := time.Unix(int64(tsFloat), 0)

		// Classify time of day
		hour := ts.Hour()
		var timeOfDay string
		switch {
		case hour >= 6 && hour < 12:
			timeOfDay = "morning"
		case hour >= 12 && hour < 18:
			timeOfDay = "afternoon"
		case hour >= 18 && hour < 22:
			timeOfDay = "evening"
		default:
			timeOfDay = "night"
		}

		// Classify day type
		dayType := "weekday"
		if ts.Weekday() == time.Saturday || ts.Weekday() == time.Sunday {
			dayType = "weekend"
		}

		key := temporalKey{
			topic:     topicName,
			timeOfDay: timeOfDay,
			dayType:   dayType,
		}
		counts[key]++
	}

	var patterns []*types.TemporalPattern
	for key, count := range counts {
		if count >= 3 {
			patterns = append(patterns, &types.TemporalPattern{
				Topic:       key.topic,
				TimeOfDay:   key.timeOfDay,
				DayOfWeek:   key.dayType,
				Occurrences: count,
			})
		}
	}

	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Occurrences > patterns[j].Occurrences
	})

	return patterns, nil
}

// CreatePatternNodes materializa os padroes como nos no grafo
func (pm *PatternMiner) CreatePatternNodes(ctx context.Context, idosoID int64) error {
	patterns, err := pm.MineRecurrentPatterns(ctx, idosoID, 3) // minimo 3 ocorrencias
	if err != nil {
		return err
	}

	// Find/create person node
	personResult, err := pm.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType:  "Person",
		MatchKeys: map[string]interface{}{"id": idosoID},
	})
	if err != nil {
		return fmt.Errorf("failed to merge Person node: %w", err)
	}
	personNodeID := personResult.NodeID

	for _, pattern := range patterns {
		// Merge Pattern node
		patResult, err := pm.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
			NodeType: "Pattern",
			MatchKeys: map[string]interface{}{
				"person_id": idosoID,
				"topic":     pattern.Topic,
			},
			OnCreateSet: map[string]interface{}{
				"created":           nietzscheInfra.NowUnix(),
				"frequency":         pattern.Frequency,
				"first_seen":        float64(pattern.FirstSeen.Unix()),
				"last_seen":         float64(pattern.LastSeen.Unix()),
				"avg_interval_days": pattern.AvgInterval,
				"severity_trend":    pattern.SeverityTrend,
				"confidence":        pattern.Confidence,
			},
			OnMatchSet: map[string]interface{}{
				"updated":           nietzscheInfra.NowUnix(),
				"frequency":         pattern.Frequency,
				"last_seen":         float64(pattern.LastSeen.Unix()),
				"avg_interval_days": pattern.AvgInterval,
				"severity_trend":    pattern.SeverityTrend,
				"confidence":        pattern.Confidence,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create pattern node: %w", err)
		}
		patternNodeID := patResult.NodeID

		// Person -> HAS_PATTERN -> Pattern
		_, err = pm.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
			FromNodeID: personNodeID,
			ToNodeID:   patternNodeID,
			EdgeType:   "HAS_PATTERN",
		})
		if err != nil {
			return fmt.Errorf("failed to create HAS_PATTERN edge: %w", err)
		}

		// Pattern -> REPRESENTS -> Topic (find topic node first)
		topicNQL := `MATCH (t:Topic) WHERE t.name = $topic RETURN t LIMIT 1`
		topicResult, err := pm.graphAdapter.ExecuteNQL(ctx, topicNQL, map[string]interface{}{
			"topic": pattern.Topic,
		}, "")
		if err == nil && len(topicResult.Nodes) > 0 {
			topicNodeID := topicResult.Nodes[0].ID
			_, err = pm.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
				FromNodeID: patternNodeID,
				ToNodeID:   topicNodeID,
				EdgeType:   "REPRESENTS",
			})
			if err != nil {
				return fmt.Errorf("failed to create REPRESENTS edge: %w", err)
			}
		}
	}

	return nil
}
