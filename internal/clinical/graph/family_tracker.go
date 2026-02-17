// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package graph

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog/log"
)

// FamilyTracker tracks family structure changes
type FamilyTracker struct {
	db          *sql.DB
	personGraph *PersonGraph
}

// NewFamilyTracker creates a new family tracker
func NewFamilyTracker(db *sql.DB, personGraph *PersonGraph) *FamilyTracker {
	return &FamilyTracker{
		db:          db,
		personGraph: personGraph,
	}
}

// FamilyChange represents a change in family structure
type FamilyChange struct {
	ID           int64     `json:"id"`
	PatientID    int64     `json:"patient_id"`
	ChangeType   string    `json:"change_type"` // NEW_PERSON, PERSON_LEFT, RELATIONSHIP_CHANGE
	PersonID     string    `json:"person_id"`
	PersonName   string    `json:"person_name"`
	Relationship string    `json:"relationship"`
	DetectedAt   time.Time `json:"detected_at"`
}

// DetectChanges detects family structure changes
func (f *FamilyTracker) DetectChanges(ctx context.Context, patientID int64) ([]FamilyChange, error) {
	var changes []FamilyChange

	// Get all people in patient's life
	people, err := f.personGraph.GetAllPeople(ctx, patientID)
	if err != nil {
		return nil, err
	}

	// Check for new people (first mention in last 2 sessions)
	for _, person := range people {
		if person.MentionCount == 1 {
			// New person detected
			change := FamilyChange{
				PatientID:    patientID,
				ChangeType:   "NEW_PERSON",
				PersonID:     person.ID,
				PersonName:   person.Names[0],
				Relationship: person.Relationship,
				DetectedAt:   person.FirstMention,
			}

			// Store change
			err := f.storeChange(ctx, &change)
			if err == nil {
				changes = append(changes, change)

				log.Info().
					Int64("patient_id", patientID).
					Str("person_name", person.Names[0]).
					Str("relationship", person.Relationship).
					Msg("👨‍👩‍👧 NEW PERSON detected in family")
			}
		}
	}

	// Check for people who left (not mentioned in last N sessions)
	missingPeople := f.detectMissingPeople(ctx, patientID, people)
	for _, person := range missingPeople {
		change := FamilyChange{
			PatientID:    patientID,
			ChangeType:   "PERSON_LEFT",
			PersonID:     person.ID,
			PersonName:   person.Names[0],
			Relationship: person.Relationship,
			DetectedAt:   time.Now(),
		}

		err := f.storeChange(ctx, &change)
		if err == nil {
			changes = append(changes, change)

			log.Warn().
				Int64("patient_id", patientID).
				Str("person_name", person.Names[0]).
				Str("relationship", person.Relationship).
				Msg("👤 PERSON LEFT detected (not mentioned recently)")
		}
	}

	return changes, nil
}

// detectMissingPeople detects people not mentioned in last N sessions
func (f *FamilyTracker) detectMissingPeople(ctx context.Context, patientID int64, allPeople []*Person) []*Person {
	const sessionsThreshold = 3

	// Get last N sessions
	var recentSessionCount int
	err := f.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM conversations
		WHERE patient_id = $1
		ORDER BY started_at DESC
		LIMIT $2
	`, patientID, sessionsThreshold).Scan(&recentSessionCount)

	if err != nil || recentSessionCount < sessionsThreshold {
		return []*Person{}
	}

	// Check which people haven't been mentioned
	var missing []*Person
	for _, person := range allPeople {
		// Check if person was mentioned in last N sessions
		daysSinceLastMention := time.Since(person.LastMention).Hours() / 24

		// Rough estimate: if not mentioned in last 30 days and had 3+ sessions
		if daysSinceLastMention > 30 && person.MentionCount >= 3 {
			missing = append(missing, person)
		}
	}

	return missing
}

// storeChange stores a family change in database
func (f *FamilyTracker) storeChange(ctx context.Context, change *FamilyChange) error {
	// Check if change already recorded
	var exists bool
	err := f.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM family_changes
			WHERE patient_id = $1 AND person_neo4j_id = $2 AND change_type = $3
			AND detected_at > NOW() - INTERVAL '7 days'
		)
	`, change.PatientID, change.PersonID, change.ChangeType).Scan(&exists)

	if err == nil && exists {
		// Already recorded recently
		return nil
	}

	return f.db.QueryRowContext(ctx, `
		INSERT INTO family_changes (
			patient_id, change_type, person_neo4j_id, person_name, relationship, detected_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, change.PatientID, change.ChangeType, change.PersonID, change.PersonName,
		change.Relationship, change.DetectedAt,
	).Scan(&change.ID)
}

// GetRecentChanges retrieves recent family changes
func (f *FamilyTracker) GetRecentChanges(ctx context.Context, patientID int64, days int) ([]FamilyChange, error) {
	rows, err := f.db.QueryContext(ctx, `
		SELECT id, patient_id, change_type, person_neo4j_id, person_name, relationship, detected_at
		FROM family_changes
		WHERE patient_id = $1 AND detected_at > NOW() - INTERVAL '$2 days'
		ORDER BY detected_at DESC
	`, patientID, days)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []FamilyChange
	for rows.Next() {
		var change FamilyChange
		err := rows.Scan(
			&change.ID, &change.PatientID, &change.ChangeType, &change.PersonID,
			&change.PersonName, &change.Relationship, &change.DetectedAt,
		)
		if err == nil {
			changes = append(changes, change)
		}
	}

	return changes, rows.Err()
}
