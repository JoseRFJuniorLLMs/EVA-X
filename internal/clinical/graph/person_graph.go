// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package graph

import (
	"context"
	"fmt"
	"strings"
	"time"

	"eva/internal/brainstem/infrastructure/graph"

	"github.com/rs/zerolog/log"
)

// PersonGraph manages the knowledge graph of people in a child's life
type PersonGraph struct {
	neo4j *graph.Neo4jClient
}

// NewPersonGraph creates a new person graph manager
func NewPersonGraph(neo4j *graph.Neo4jClient) *PersonGraph {
	return &PersonGraph{neo4j: neo4j}
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

// CreateOrUpdatePerson creates or updates a person node in Neo4j
func (pg *PersonGraph) CreateOrUpdatePerson(ctx context.Context, patientID int64, name, relationship string, sentiment float64, sessionID int64) (*Person, error) {
	// Normalize name
	normalizedName := strings.ToLower(strings.TrimSpace(name))

	// Try to resolve to existing person
	existingPerson, err := pg.ResolvePerson(ctx, patientID, normalizedName)
	if err == nil && existingPerson != nil {
		// Update existing person
		return pg.updatePerson(ctx, existingPerson, normalizedName, sentiment, sessionID)
	}

	// Create new person
	personID := fmt.Sprintf("person_%d_%d", patientID, time.Now().Unix())

	query := `
		MATCH (patient:Person {id: $patientId})
		CREATE (p:PersonInLife {
			id: $personId,
			names: [$name],
			relationship: $relationship,
			first_mention: datetime(),
			last_mention: datetime(),
			mention_count: 1,
			avg_sentiment: $sentiment
		})
		CREATE (patient)-[:KNOWS]->(p)
		RETURN p.id AS id
	`

	_, err = pg.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId":    patientID,
		"personId":     personID,
		"name":         normalizedName,
		"relationship": relationship,
		"sentiment":    sentiment,
	})

	if err != nil {
		return nil, err
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

	// Search by name
	query := `
		MATCH (patient:Person {id: $patientId})-[:KNOWS]->(p:PersonInLife)
		WHERE $name IN p.names
		RETURN p.id AS id, p.names AS names, p.relationship AS relationship,
		       p.mention_count AS mention_count, p.avg_sentiment AS avg_sentiment
		LIMIT 1
	`

	records, err := pg.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"name":      normalizedName,
	})

	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("person not found")
	}

	rec := records[0]
	person := &Person{}

	if id, ok := rec.Get("id"); ok {
		person.ID = id.(string)
	}
	if names, ok := rec.Get("names"); ok {
		if nameList, ok := names.([]interface{}); ok {
			for _, n := range nameList {
				person.Names = append(person.Names, n.(string))
			}
		}
	}
	if rel, ok := rec.Get("relationship"); ok {
		person.Relationship = rel.(string)
	}
	if count, ok := rec.Get("mention_count"); ok {
		person.MentionCount = int(count.(int64))
	}
	if sentiment, ok := rec.Get("avg_sentiment"); ok {
		person.AvgSentiment = sentiment.(float64)
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

	query := `
		MATCH (patient:Person {id: $patientId})-[:KNOWS]->(p:PersonInLife)
		WHERE $gender = 'any' OR p.gender = $gender
		RETURN p.id AS id, p.names AS names, p.relationship AS relationship,
		       p.last_mention AS last_mention
		ORDER BY p.last_mention DESC
		LIMIT 1
	`

	records, err := pg.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"gender":    gender,
	})

	if err != nil || len(records) == 0 {
		return nil, fmt.Errorf("cannot resolve pronoun")
	}

	rec := records[0]
	person := &Person{}

	if id, ok := rec.Get("id"); ok {
		person.ID = id.(string)
	}
	if names, ok := rec.Get("names"); ok {
		if nameList, ok := names.([]interface{}); ok {
			for _, n := range nameList {
				person.Names = append(person.Names, n.(string))
			}
		}
	}
	if rel, ok := rec.Get("relationship"); ok {
		person.Relationship = rel.(string)
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

	query := `
		MATCH (p:PersonInLife {id: $personId})
		SET p.last_mention = datetime(),
		    p.mention_count = p.mention_count + 1,
		    p.avg_sentiment = (p.avg_sentiment * p.mention_count + $sentiment) / (p.mention_count + 1)
	`

	if !hasName {
		query += `, p.names = p.names + [$newName]`
	}

	query += ` RETURN p.mention_count AS mention_count, p.avg_sentiment AS avg_sentiment`

	params := map[string]interface{}{
		"personId":  person.ID,
		"sentiment": sentiment,
	}

	if !hasName {
		params["newName"] = newName
	}

	records, err := pg.neo4j.ExecuteWriteAndReturn(ctx, query, params)
	if err != nil {
		return nil, err
	}

	if len(records) > 0 {
		rec := records[0]
		if count, ok := rec.Get("mention_count"); ok {
			person.MentionCount = int(count.(int64))
		}
		if sentiment, ok := rec.Get("avg_sentiment"); ok {
			person.AvgSentiment = sentiment.(float64)
		}
	}

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
	query := `
		MATCH (patient:Person {id: $patientId})-[:KNOWS]->(p:PersonInLife)
		RETURN p.id AS id, p.names AS names, p.relationship AS relationship,
		       p.mention_count AS mention_count, p.avg_sentiment AS avg_sentiment,
		       p.first_mention AS first_mention, p.last_mention AS last_mention
		ORDER BY p.mention_count DESC
	`

	records, err := pg.neo4j.ExecuteRead(ctx, query, map[string]interface{}{
		"patientId": patientID,
	})

	if err != nil {
		return nil, err
	}

	var people []*Person
	for _, rec := range records {
		person := &Person{}

		if id, ok := rec.Get("id"); ok {
			person.ID = id.(string)
		}
		if names, ok := rec.Get("names"); ok {
			if nameList, ok := names.([]interface{}); ok {
				for _, n := range nameList {
					person.Names = append(person.Names, n.(string))
				}
			}
		}
		if rel, ok := rec.Get("relationship"); ok {
			person.Relationship = rel.(string)
		}
		if count, ok := rec.Get("mention_count"); ok {
			person.MentionCount = int(count.(int64))
		}
		if sentiment, ok := rec.Get("avg_sentiment"); ok {
			person.AvgSentiment = sentiment.(float64)
		}

		people = append(people, person)
	}

	return people, nil
}

// DetectRelationshipChanges detects significant changes in sentiment toward a person
func (pg *PersonGraph) DetectRelationshipChanges(ctx context.Context, patientID int64) ([]string, error) {
	// TODO: Implement sentiment trend analysis
	// Compare recent sentiment vs. historical average
	return []string{}, nil
}
