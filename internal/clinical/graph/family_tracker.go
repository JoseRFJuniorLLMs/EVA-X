// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package graph

import (
	"context"
	"sort"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

// FamilyTracker tracks family structure changes
type FamilyTracker struct {
	db          *database.DB
	personGraph *PersonGraph
}

// NewFamilyTracker creates a new family tracker
func NewFamilyTracker(db *database.DB, personGraph *PersonGraph) *FamilyTracker {
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

	// Count conversations for this patient via NietzscheDB
	recentSessionCount, err := f.db.Count(ctx, "conversations", " AND n.patient_id = $pid", map[string]interface{}{
		"pid": patientID,
	})

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
	// Check if change already recorded (within last 7 days)
	rows, err := f.db.QueryByLabel(ctx, "family_changes",
		" AND n.patient_id = $pid AND n.person_NietzscheDB_id = $nid AND n.change_type = $ct",
		map[string]interface{}{
			"pid": change.PatientID,
			"nid": change.PersonID,
			"ct":  change.ChangeType,
		}, 0)

	if err == nil {
		sevenDaysAgo := time.Now().AddDate(0, 0, -7)
		for _, row := range rows {
			detectedAt := database.GetTime(row, "detected_at")
			if detectedAt.After(sevenDaysAgo) {
				// Already recorded recently
				return nil
			}
		}
	}

	// Insert new family change
	id, insertErr := f.db.Insert(ctx, "family_changes", map[string]interface{}{
		"patient_id":          change.PatientID,
		"change_type":         change.ChangeType,
		"person_NietzscheDB_id": change.PersonID,
		"person_name":         change.PersonName,
		"relationship":        change.Relationship,
		"detected_at":         change.DetectedAt.Format(time.RFC3339Nano),
	})
	if insertErr != nil {
		return insertErr
	}
	change.ID = id
	return nil
}

// GetRecentChanges retrieves recent family changes
func (f *FamilyTracker) GetRecentChanges(ctx context.Context, patientID int64, days int) ([]FamilyChange, error) {
	rows, err := f.db.QueryByLabel(ctx, "family_changes",
		" AND n.patient_id = $pid",
		map[string]interface{}{
			"pid": patientID,
		}, 0)

	if err != nil {
		return nil, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var changes []FamilyChange
	for _, row := range rows {
		detectedAt := database.GetTime(row, "detected_at")
		if detectedAt.After(cutoff) {
			changes = append(changes, FamilyChange{
				ID:           database.GetInt64(row, "id"),
				PatientID:    database.GetInt64(row, "patient_id"),
				ChangeType:   database.GetString(row, "change_type"),
				PersonID:     database.GetString(row, "person_NietzscheDB_id"),
				PersonName:   database.GetString(row, "person_name"),
				Relationship: database.GetString(row, "relationship"),
				DetectedAt:   detectedAt,
			})
		}
	}

	// Sort by detected_at DESC
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].DetectedAt.After(changes[j].DetectedAt)
	})

	return changes, nil
}
