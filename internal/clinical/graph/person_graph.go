// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"

	"github.com/rs/zerolog/log"
)

// PersonGraph manages the knowledge graph of people in a child's life
type PersonGraph struct {
	graphAdapter *nietzscheInfra.GraphAdapter
}

// NewPersonGraph creates a new person graph manager
func NewPersonGraph(graphAdapter *nietzscheInfra.GraphAdapter) *PersonGraph {
	return &PersonGraph{graphAdapter: graphAdapter}
}

// Person represents a person in the child's life
type Person struct {
	ID           string    `json:"id"`
	Names        []string  `json:"names"`        // ["mãe", "ela", "Dona Maria", "mamãe"]
	Relationship string    `json:"relationship"` // "mãe", "professora", "amigo"
	FirstMention time.Time `json:"first_mention"`
	LastMention  time.Time `json:"last_mention"`
	MentionCount int       `json:"mention_count"`

	// Sentiment tracking
	SentimentHistory []SentimentPoint `json:"sentiment_history"`
	AvgSentiment     float64          `json:"avg_sentiment"`
}

// SentimentPoint represents sentiment at a point in time
type SentimentPoint struct {
	SessionID int64     `json:"session_id"`
	Sentiment float64   `json:"sentiment"` // -1 to 1
	Timestamp time.Time `json:"timestamp"`
	Context   string    `json:"context"`
}

// CreateOrUpdatePerson creates or updates a person node in the graph
func (pg *PersonGraph) CreateOrUpdatePerson(ctx context.Context, patientID int64, name, relationship string, sentiment float64, sessionID int64) (*Person, error) {
	// Normalize name
	normalizedName := strings.ToLower(strings.TrimSpace(name))

	// Try to resolve to existing person
	existingPerson, err := pg.ResolvePerson(ctx, patientID, normalizedName)
	if err == nil && existingPerson != nil {
		// Update existing person
		return pg.updatePerson(ctx, existingPerson, normalizedName, sentiment, sessionID)
	}

	// Create new person via MergeNode
	personID := fmt.Sprintf("person_%d_%d", patientID, time.Now().Unix())

	now := nietzscheInfra.NowUnix()
	result, err := pg.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "PersonInLife",
		MatchKeys: map[string]interface{}{
			"id": personID,
		},
		OnCreateSet: map[string]interface{}{
			"id":            personID,
			"names":         normalizedName,
			"relationship":  relationship,
			"first_mention": now,
			"last_mention":  now,
			"mention_count": 1,
			"avg_sentiment": sentiment,
			"patient_id":    patientID,
		},
	})
	if err != nil {
		return nil, err
	}

	// Create KNOWS edge from patient to person
	_, err = pg.graphAdapter.MergeEdge(ctx, nietzscheInfra.MergeEdgeOpts{
		FromNodeID: fmt.Sprintf("patient_%d", patientID),
		ToNodeID:   result.NodeID,
		EdgeType:   "KNOWS",
	})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create KNOWS edge")
	}

	log.Info().
		Int64("patient_id", patientID).
		Str("person_id", personID).
		Str("name", name).
		Str("relationship", relationship).
		Msg("Created new person in graph")

	return &Person{
		ID:           personID,
		Names:        []string{normalizedName},
		Relationship: relationship,
		FirstMention: time.Now(),
		LastMention:  time.Now(),
		MentionCount: 1,
		SentimentHistory: []SentimentPoint{
			{SessionID: sessionID, Sentiment: sentiment, Timestamp: time.Now()},
		},
		AvgSentiment: sentiment,
	}, nil
}

// ResolvePerson attempts to resolve a name/pronoun to an existing person
func (pg *PersonGraph) ResolvePerson(ctx context.Context, patientID int64, name string) (*Person, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(name))

	// Handle pronouns
	if normalizedName == "ele" || normalizedName == "ela" || normalizedName == "eles" || normalizedName == "elas" {
		return pg.resolvePronouns(ctx, patientID, normalizedName)
	}

	// Search by name using NQL
	nql := `MATCH (p:PersonInLife) WHERE p.patient_id = $patientId AND p.names CONTAINS $name RETURN p LIMIT 1`

	result, err := pg.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"patientId": patientID,
		"name":      normalizedName,
	}, "")

	if err != nil || result == nil || len(result.Nodes) == 0 {
		return nil, fmt.Errorf("person not found")
	}

	node := result.Nodes[0]
	person := &Person{}
	person.ID = node.ID

	if names, ok := node.Content["names"]; ok {
		if nameStr, ok := names.(string); ok {
			person.Names = append(person.Names, nameStr)
		} else if nameList, ok := names.([]interface{}); ok {
			for _, n := range nameList {
				person.Names = append(person.Names, fmt.Sprintf("%v", n))
			}
		}
	}
	if rel, ok := node.Content["relationship"]; ok {
		person.Relationship = fmt.Sprintf("%v", rel)
	}
	if count, ok := node.Content["mention_count"]; ok {
		if c, ok := count.(float64); ok {
			person.MentionCount = int(c)
		}
	}
	if sentiment, ok := node.Content["avg_sentiment"]; ok {
		if s, ok := sentiment.(float64); ok {
			person.AvgSentiment = s
		}
	}

	return person, nil
}

// resolvePronouns resolves pronouns to the most recently mentioned person
func (pg *PersonGraph) resolvePronouns(ctx context.Context, patientID int64, pronoun string) (*Person, error) {
	// Get most recently mentioned person matching gender
	gender := "any"
	if pronoun == "ele" || pronoun == "eles" {
		gender = "male"
	} else if pronoun == "ela" || pronoun == "elas" {
		gender = "female"
	}

	nql := `MATCH (p:PersonInLife) WHERE p.patient_id = $patientId RETURN p`
	if gender != "any" {
		nql = `MATCH (p:PersonInLife) WHERE p.patient_id = $patientId AND p.gender = $gender RETURN p`
	}

	result, err := pg.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"patientId": patientID,
		"gender":    gender,
	}, "")

	if err != nil || result == nil || len(result.Nodes) == 0 {
		return nil, fmt.Errorf("cannot resolve pronoun")
	}

	node := result.Nodes[0]
	person := &Person{}
	person.ID = node.ID

	if names, ok := node.Content["names"]; ok {
		if nameStr, ok := names.(string); ok {
			person.Names = append(person.Names, nameStr)
		} else if nameList, ok := names.([]interface{}); ok {
			for _, n := range nameList {
				person.Names = append(person.Names, fmt.Sprintf("%v", n))
			}
		}
	}
	if rel, ok := node.Content["relationship"]; ok {
		person.Relationship = fmt.Sprintf("%v", rel)
	}

	log.Info().
		Str("pronoun", pronoun).
		Str("resolved_to", person.ID).
		Strs("names", person.Names).
		Msg("Resolved pronoun")

	return person, nil
}

// updatePerson updates an existing person with new mention
func (pg *PersonGraph) updatePerson(ctx context.Context, person *Person, newName string, sentiment float64, sessionID int64) (*Person, error) {
	// Add new name if not already in list
	hasName := false
	for _, name := range person.Names {
		if name == newName {
			hasName = true
			break
		}
	}

	// Calculate new averages
	newMentionCount := person.MentionCount + 1
	newAvgSentiment := (person.AvgSentiment*float64(person.MentionCount) + sentiment) / float64(newMentionCount)

	now := nietzscheInfra.NowUnix()
	onMatchSet := map[string]interface{}{
		"last_mention":  now,
		"mention_count": newMentionCount,
		"avg_sentiment": newAvgSentiment,
	}
	if !hasName {
		// Add the new name to the names field
		allNames := strings.Join(append(person.Names, newName), ",")
		onMatchSet["names"] = allNames
	}

	_, err := pg.graphAdapter.MergeNode(ctx, nietzscheInfra.MergeNodeOpts{
		NodeType: "PersonInLife",
		MatchKeys: map[string]interface{}{
			"id": person.ID,
		},
		OnMatchSet: onMatchSet,
	})
	if err != nil {
		return nil, err
	}

	person.MentionCount = newMentionCount
	person.AvgSentiment = newAvgSentiment

	if !hasName {
		person.Names = append(person.Names, newName)
	}

	log.Info().
		Str("person_id", person.ID).
		Str("new_name", newName).
		Int("mention_count", person.MentionCount).
		Float64("avg_sentiment", person.AvgSentiment).
		Msg("Updated person in graph")

	return person, nil
}

// GetAllPeople retrieves all people in a patient's life
func (pg *PersonGraph) GetAllPeople(ctx context.Context, patientID int64) ([]*Person, error) {
	nql := `MATCH (p:PersonInLife) WHERE p.patient_id = $patientId RETURN p`

	result, err := pg.graphAdapter.ExecuteNQL(ctx, nql, map[string]interface{}{
		"patientId": patientID,
	}, "")

	if err != nil {
		return nil, err
	}

	var people []*Person
	if result != nil {
		for _, node := range result.Nodes {
			person := &Person{}
			person.ID = node.ID

			if names, ok := node.Content["names"]; ok {
				if nameStr, ok := names.(string); ok {
					person.Names = append(person.Names, nameStr)
				} else if nameList, ok := names.([]interface{}); ok {
					for _, n := range nameList {
						person.Names = append(person.Names, fmt.Sprintf("%v", n))
					}
				}
			}
			if rel, ok := node.Content["relationship"]; ok {
				person.Relationship = fmt.Sprintf("%v", rel)
			}
			if count, ok := node.Content["mention_count"]; ok {
				if c, ok := count.(float64); ok {
					person.MentionCount = int(c)
				}
			}
			if sentiment, ok := node.Content["avg_sentiment"]; ok {
				if s, ok := sentiment.(float64); ok {
					person.AvgSentiment = s
				}
			}

			people = append(people, person)
		}
	}

	return people, nil
}

// DetectRelationshipChanges detects significant changes in sentiment toward a person
func (pg *PersonGraph) DetectRelationshipChanges(ctx context.Context, patientID int64) ([]string, error) {
	// TODO: Implement sentiment trend analysis
	// Compare recent sentiment vs. historical average
	return []string{}, nil
}
