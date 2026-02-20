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
// Rewritten from complex Cypher (reduce, collect, CASE WHEN) to Go loops.
func (pm *PatternMiner) MineRecurrentPatterns(ctx context.Context, idosoID int64, minFrequency int) ([]*types.RecurrentPattern, error) {
	// 1. Find Person node
	patientNodeID := fmt.Sprintf("%d", idosoID)

	// 2. BFS from Person to find EXPERIENCED events (depth 1 for direct edges)
	nql := `MATCH (p:Person)-[:EXPERIENCED]->(e:Event) WHERE p.id = $idosoId RETURN e`
	eventsResult, err := pm.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// 3. For each event, find connected Topics
	// Build a map: topic_name -> list of event data
	type eventData struct {
		timestamp  time.Time
		emotion    string
		importance float64
	}
	topicEvents := make(map[string][]eventData)
	topicNodeIDs := make(map[string]string) // topic name -> node ID (for later use)
	_ = patientNodeID // used for context

	for _, eventNode := range eventsResult.Nodes {
		eventID := eventNode.ID

		// Find topics connected to this event
		topicNQL := `MATCH (e:Event)-[:RELATED_TO]->(t:Topic) WHERE e.id = $eventID RETURN t`
		topicResult, err := pm.graphAdapter.ExecuteNQL(ctx, topicNQL, map[string]interface{}{
			"eventID": eventID,
		}, "")
		if err != nil {
			continue
		}

		// Extract event data
		var ts time.Time
		if v, ok := eventNode.Content["timestamp"]; ok {
			if f, ok := v.(float64); ok {
				ts = time.Unix(int64(f), 0)
			}
		}
		emotion := ""
		if v, ok := eventNode.Content["emotion"]; ok {
			emotion = fmt.Sprintf("%v", v)
		}
		importance := 0.0
		if v, ok := eventNode.Content["importance"]; ok {
			switch iv := v.(type) {
			case float64:
				importance = iv
			case int64:
				importance = float64(iv)
			case int:
				importance = float64(iv)
			}
		}

		ed := eventData{
			timestamp:  ts,
			emotion:    emotion,
			importance: importance,
		}

		for _, topicNode := range topicResult.Nodes {
			topicName := ""
			if n, ok := topicNode.Content["name"]; ok {
				topicName = fmt.Sprintf("%v", n)
			}
			if topicName == "" {
				continue
			}
			topicEvents[topicName] = append(topicEvents[topicName], ed)
			topicNodeIDs[topicName] = topicNode.ID
		}
	}

	// 4. Filter by minFrequency and build patterns
	var patterns []*types.RecurrentPattern

	for topic, events := range topicEvents {
		if len(events) < minFrequency {
			continue
		}

		// Sort events by timestamp
		sort.Slice(events, func(i, j int) bool {
			return events[i].timestamp.Before(events[j].timestamp)
		})

		frequency := len(events)
		firstSeen := events[0].timestamp
		lastSeen := events[len(events)-1].timestamp

		// Collect emotions
		emotionsList := make([]string, 0, len(events))
		for _, e := range events {
			if e.emotion != "" {
				emotionsList = append(emotionsList, e.emotion)
			}
		}

		// Calculate average interval between occurrences (in days)
		avgInterval := 0.0
		if len(events) > 1 {
			var totalDays float64
			for i := 0; i < len(events)-1; i++ {
				days := events[i+1].timestamp.Sub(events[i].timestamp).Hours() / 24
				totalDays += days
			}
			avgInterval = totalDays / float64(len(events)-1)
		}

		// Calculate severity trend from importance deltas
		severityTrend := "stable"
		if len(events) > 1 {
			var deltaSum float64
			for i := 0; i < len(events)-1; i++ {
				deltaSum += events[i+1].importance - events[i].importance
			}
			avgDelta := deltaSum / float64(len(events)-1)
			if avgDelta > 0.1 {
				severityTrend = "increasing"
			} else if avgDelta < -0.1 {
				severityTrend = "decreasing"
			}
		}

		// Confidence: frequency / 10.0
		confidence := math.Min(float64(frequency)/10.0, 1.0)

		pattern := &types.RecurrentPattern{
			Topic:         topic,
			Frequency:     frequency,
			FirstSeen:     firstSeen,
			LastSeen:      lastSeen,
			AvgInterval:   avgInterval,
			Emotions:      emotionsList,
			SeverityTrend: severityTrend,
			Confidence:    confidence,
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// MineTemporalPatterns identifica quando certos topicos aparecem (hora do dia, dia da semana)
// Rewritten from complex Cypher (CASE WHEN on timestamp fields) to Go loops.
func (pm *PatternMiner) MineTemporalPatterns(ctx context.Context, idosoID int64) ([]*types.TemporalPattern, error) {
	// 1. Find events for this person
	nql := `MATCH (p:Person)-[:EXPERIENCED]->(e:Event) WHERE p.id = $idosoId RETURN e`
	eventsResult, err := pm.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"idosoId": idosoID,
	}, "")
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}

	// 2. For each event, find topics and classify time
	type temporalKey struct {
		topic     string
		timeOfDay string
		dayType   string
	}
	counts := make(map[temporalKey]int)

	for _, eventNode := range eventsResult.Nodes {
		eventID := eventNode.ID

		// Get timestamp
		var ts time.Time
		if v, ok := eventNode.Content["timestamp"]; ok {
			if f, ok := v.(float64); ok {
				ts = time.Unix(int64(f), 0)
			}
		}
		if ts.IsZero() {
			continue
		}

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
		weekday := ts.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			dayType = "weekend"
		}

		// Find topics for this event
		topicNQL := `MATCH (e:Event)-[:RELATED_TO]->(t:Topic) WHERE e.id = $eventID RETURN t`
		topicResult, err := pm.graphAdapter.ExecuteNQL(ctx, topicNQL, map[string]interface{}{
			"eventID": eventID,
		}, "")
		if err != nil {
			continue
		}

		for _, topicNode := range topicResult.Nodes {
			topicName := ""
			if n, ok := topicNode.Content["name"]; ok {
				topicName = fmt.Sprintf("%v", n)
			}
			if topicName == "" {
				continue
			}

			key := temporalKey{
				topic:     topicName,
				timeOfDay: timeOfDay,
				dayType:   dayType,
			}
			counts[key]++
		}
	}

	// 3. Filter by minimum 3 occurrences and build patterns
	var patterns []*types.TemporalPattern

	for key, count := range counts {
		if count < 3 {
			continue
		}

		pattern := &types.TemporalPattern{
			Topic:       key.topic,
			TimeOfDay:   key.timeOfDay,
			DayOfWeek:   key.dayType,
			Occurrences: count,
		}

		patterns = append(patterns, pattern)
	}

	// Sort by occurrences desc
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
				"created":            nietzscheInfra.NowUnix(),
				"frequency":          pattern.Frequency,
				"first_seen":         float64(pattern.FirstSeen.Unix()),
				"last_seen":          float64(pattern.LastSeen.Unix()),
				"avg_interval_days":  pattern.AvgInterval,
				"severity_trend":     pattern.SeverityTrend,
				"confidence":         pattern.Confidence,
			},
			OnMatchSet: map[string]interface{}{
				"updated":            nietzscheInfra.NowUnix(),
				"frequency":          pattern.Frequency,
				"last_seen":          float64(pattern.LastSeen.Unix()),
				"avg_interval_days":  pattern.AvgInterval,
				"severity_trend":     pattern.SeverityTrend,
				"confidence":         pattern.Confidence,
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
