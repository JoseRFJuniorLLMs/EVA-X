package memory

import (
	"context"
	"eva-mind/internal/brainstem/infrastructure/graph"
	"eva-mind/pkg/types"
	"fmt"
	"time"
)

type PatternMiner struct {
	neo4j *graph.Neo4jClient
}

// Structs moved to pkg/types

func NewPatternMiner(neo4j *graph.Neo4jClient) *PatternMiner {
	return &PatternMiner{neo4j: neo4j}
}

// MineRecurrentPatterns identifica tópicos que aparecem múltiplas vezes
func (pm *PatternMiner) MineRecurrentPatterns(ctx context.Context, idosoID int64, minFrequency int) ([]*types.RecurrentPattern, error) {
	query := `
        MATCH (p:Person {id: $idosoId})-[:EXPERIENCED]->(e:Event)-[:RELATED_TO]->(t:Topic)
        WITH t, e
        ORDER BY e.timestamp
        WITH t, 
             count(e) as frequency,
             collect(e.timestamp) as timestamps,
             collect(e.emotion) as emotions,
             collect(e.importance) as importances
        WHERE frequency >= $minFrequency
        
        // Calcular intervalo médio entre ocorrências
        WITH t, frequency, timestamps, emotions, importances,
             [i IN range(0, size(timestamps)-2) | 
              duration.between(timestamps[i], timestamps[i+1]).days] as intervals
        
        // Detectar tendência de severidade (importância crescente/decrescente)
        WITH t, frequency, timestamps, emotions, importances, intervals,
             [i IN range(0, size(importances)-2) | 
              importances[i+1] - importances[i]] as severity_deltas
        
        RETURN 
            t.name as topic,
            frequency,
            timestamps[0] as first_seen,
            timestamps[size(timestamps)-1] as last_seen,
            CASE 
                WHEN size(intervals) > 0 THEN reduce(sum = 0.0, x IN intervals | sum + x) / size(intervals)
                ELSE 0.0
            END as avg_interval,
            emotions,
            CASE 
                WHEN size(severity_deltas) > 0 AND (reduce(sum = 0.0, d IN severity_deltas | sum + d) / size(severity_deltas)) > 0.1 THEN 'increasing'
                WHEN size(severity_deltas) > 0 AND (reduce(sum = 0.0, d IN severity_deltas | sum + d) / size(severity_deltas)) < -0.1 THEN 'decreasing'
                ELSE 'stable'
            END as severity_trend,
            toFloat(frequency) / 10.0 as confidence
    `

	params := map[string]interface{}{
		"idosoId":      idosoID,
		"minFrequency": minFrequency,
	}

	records, err := pm.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to mine patterns: %w", err)
	}

	var patterns []*types.RecurrentPattern

	for _, record := range records {
		topic, _ := record.Get("topic")
		frequency, _ := record.Get("frequency")
		firstSeen, _ := record.Get("first_seen")
		lastSeen, _ := record.Get("last_seen")
		avgInterval, _ := record.Get("avg_interval")
		emotions, _ := record.Get("emotions")
		severityTrend, _ := record.Get("severity_trend")
		confidence, _ := record.Get("confidence")

		// Parse emotions (vem como []interface{})
		emotionsList := []string{}
		if emList, ok := emotions.([]interface{}); ok {
			for _, em := range emList {
				if emStr, ok := em.(string); ok {
					emotionsList = append(emotionsList, emStr)
				}
			}
		}

		var firstSeenTime, lastSeenTime time.Time
		if t, ok := firstSeen.(time.Time); ok {
			firstSeenTime = t
		}
		if t, ok := lastSeen.(time.Time); ok {
			lastSeenTime = t
		}

		var avgIntervalVal float64
		if v, ok := avgInterval.(float64); ok {
			avgIntervalVal = v
		}

		pattern := &types.RecurrentPattern{
			Topic:         topic.(string),
			Frequency:     int(frequency.(int64)),
			FirstSeen:     firstSeenTime,
			LastSeen:      lastSeenTime,
			AvgInterval:   avgIntervalVal,
			Emotions:      emotionsList,
			SeverityTrend: severityTrend.(string),
			Confidence:    confidence.(float64),
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// MineTemporalPatterns identifica quando certos tópicos aparecem (hora do dia, dia da semana)
func (pm *PatternMiner) MineTemporalPatterns(ctx context.Context, idosoID int64) ([]*types.TemporalPattern, error) {
	query := `
        MATCH (p:Person {id: $idosoId})-[:EXPERIENCED]->(e:Event)-[:RELATED_TO]->(t:Topic)
        WITH t, e,
             CASE 
                WHEN e.timestamp.hour >= 6 AND e.timestamp.hour < 12 THEN 'morning'
                WHEN e.timestamp.hour >= 12 AND e.timestamp.hour < 18 THEN 'afternoon'
                WHEN e.timestamp.hour >= 18 AND e.timestamp.hour < 22 THEN 'evening'
                ELSE 'night'
             END as time_of_day,
             CASE 
                WHEN e.timestamp.dayOfWeek IN [6, 7] THEN 'weekend'
                ELSE 'weekday'
             END as day_type
        
        WITH t.name as topic, time_of_day, day_type, count(*) as occurrences
        WHERE occurrences >= 3
        
        RETURN topic, time_of_day, day_type, occurrences
        ORDER BY occurrences DESC
    `

	params := map[string]interface{}{
		"idosoId": idosoID,
	}

	records, err := pm.neo4j.ExecuteRead(ctx, query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to mine temporal patterns: %w", err)
	}

	var patterns []*types.TemporalPattern

	for _, record := range records {
		topic, _ := record.Get("topic")
		timeOfDay, _ := record.Get("time_of_day")
		dayType, _ := record.Get("day_type")
		occurrences, _ := record.Get("occurrences")

		pattern := &types.TemporalPattern{
			Topic:       topic.(string),
			TimeOfDay:   timeOfDay.(string),
			DayOfWeek:   dayType.(string),
			Occurrences: int(occurrences.(int64)),
		}

		patterns = append(patterns, pattern)
	}

	return patterns, nil
}

// CreatePatternNodes materializa os padrões como nós no grafo
func (pm *PatternMiner) CreatePatternNodes(ctx context.Context, idosoID int64) error {
	patterns, err := pm.MineRecurrentPatterns(ctx, idosoID, 3) // mínimo 3 ocorrências
	if err != nil {
		return err
	}

	for _, pattern := range patterns {
		query := `
            MATCH (p:Person {id: $idosoId})
            MERGE (pat:Pattern {
                person_id: $idosoId,
                topic: $topic
            })
            ON CREATE SET 
                pat.created = datetime(),
                pat.frequency = $frequency,
                pat.first_seen = datetime($firstSeen),
                pat.last_seen = datetime($lastSeen),
                pat.avg_interval_days = $avgInterval,
                pat.severity_trend = $severityTrend,
                pat.confidence = $confidence
            ON MATCH SET
                pat.updated = datetime(),
                pat.frequency = $frequency,
                pat.last_seen = datetime($lastSeen),
                pat.avg_interval_days = $avgInterval,
                pat.severity_trend = $severityTrend,
                pat.confidence = $confidence
            
            MERGE (p)-[:HAS_PATTERN]->(pat)
            
            // Conectar ao tópico original
            WITH pat
            MATCH (t:Topic {name: $topic})
            MERGE (pat)-[:REPRESENTS]->(t)
        `

		params := map[string]interface{}{
			"idosoId":       idosoID,
			"topic":         pattern.Topic,
			"frequency":     pattern.Frequency,
			"firstSeen":     pattern.FirstSeen.Format(time.RFC3339),
			"lastSeen":      pattern.LastSeen.Format(time.RFC3339),
			"avgInterval":   pattern.AvgInterval,
			"severityTrend": pattern.SeverityTrend,
			"confidence":    pattern.Confidence,
		}

		if _, err := pm.neo4j.ExecuteWrite(ctx, query, params); err != nil {
			return fmt.Errorf("failed to create pattern node: %w", err)
		}
	}

	return nil
}
