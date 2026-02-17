// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingestion

import (
	"context"
	"database/sql"
	"time"

	"github.com/rs/zerolog/log"
)

// AtomicFact represents a single, indivisible piece of information
type AtomicFact struct {
	ID            int64     `json:"id"`
	Content       string    `json:"content"`
	Confidence    float64   `json:"confidence"` // 0-1: how certain the LLM is
	Source        string    `json:"source"`     // "user_stated" | "inferred" | "observed"
	Revisable     bool      `json:"revisable"`  // can this fact be corrected?
	Version       int       `json:"version"`    // for versioning
	PatientID     int64     `json:"patient_id"`
	EventTime     time.Time `json:"event_time"`     // when it happened
	IngestionTime time.Time `json:"ingestion_time"` // when it was recorded
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// FactExtractor extracts atomic facts from conversation text
type FactExtractor struct {
	db *sql.DB
}

// NewFactExtractor creates a new fact extractor
func NewFactExtractor(db *sql.DB) *FactExtractor {
	return &FactExtractor{db: db}
}

// ExtractFacts extracts atomic facts from text using LLM
func (f *FactExtractor) ExtractFacts(ctx context.Context, text string, patientID int64) ([]*AtomicFact, error) {
	// TODO: Call LLM to extract facts
	// For now, return placeholder

	facts := []*AtomicFact{
		{
			Content:       text,
			Confidence:    0.8,
			Source:        "user_stated",
			Revisable:     true,
			Version:       1,
			PatientID:     patientID,
			EventTime:     time.Now(),
			IngestionTime: time.Now(),
		},
	}

	return facts, nil
}

// StoreFacts stores atomic facts in database
func (f *FactExtractor) StoreFacts(ctx context.Context, facts []*AtomicFact) error {
	tx, err := f.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO atomic_facts (
			content, confidence, source, revisable, version,
			patient_id, event_time, ingestion_time, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		RETURNING id
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, fact := range facts {
		err = stmt.QueryRowContext(
			ctx,
			fact.Content,
			fact.Confidence,
			fact.Source,
			fact.Revisable,
			fact.Version,
			fact.PatientID,
			fact.EventTime,
			fact.IngestionTime,
		).Scan(&fact.ID)

		if err != nil {
			return err
		}

		log.Info().
			Int64("fact_id", fact.ID).
			Float64("confidence", fact.Confidence).
			Str("source", fact.Source).
			Msg("Stored atomic fact")
	}

	return tx.Commit()
}

// GetFacts retrieves facts for a patient
func (f *FactExtractor) GetFacts(ctx context.Context, patientID int64, limit int) ([]*AtomicFact, error) {
	rows, err := f.db.QueryContext(ctx, `
		SELECT 
			id, content, confidence, source, revisable, version,
			patient_id, event_time, ingestion_time, created_at, updated_at
		FROM atomic_facts
		WHERE patient_id = $1
		ORDER BY event_time DESC
		LIMIT $2
	`, patientID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []*AtomicFact
	for rows.Next() {
		var fact AtomicFact
		err := rows.Scan(
			&fact.ID,
			&fact.Content,
			&fact.Confidence,
			&fact.Source,
			&fact.Revisable,
			&fact.Version,
			&fact.PatientID,
			&fact.EventTime,
			&fact.IngestionTime,
			&fact.CreatedAt,
			&fact.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		facts = append(facts, &fact)
	}

	return facts, rows.Err()
}

// FactContradiction represents a contradiction between facts
type FactContradiction struct {
	Fact1ID    int64
	Fact2ID    int64
	Confidence float64 // how confident we are in the contradiction
	DetectedAt time.Time
}

// DetectContradictions finds contradictory facts
func (f *FactExtractor) DetectContradictions(ctx context.Context, patientID int64) ([]*FactContradiction, error) {
	// TODO: Use LLM to detect semantic contradictions
	// For now, return empty
	return []*FactContradiction{}, nil
}

// ReviseFact creates a new version of a fact
func (f *FactExtractor) ReviseFact(ctx context.Context, factID int64, newContent string, confidence float64) (*AtomicFact, error) {
	// Get current fact
	var currentFact AtomicFact
	err := f.db.QueryRowContext(ctx, `
		SELECT id, content, version, patient_id, event_time
		FROM atomic_facts
		WHERE id = $1
	`, factID).Scan(
		&currentFact.ID,
		&currentFact.Content,
		&currentFact.Version,
		&currentFact.PatientID,
		&currentFact.EventTime,
	)

	if err != nil {
		return nil, err
	}

	// Create new version
	newFact := &AtomicFact{
		Content:       newContent,
		Confidence:    confidence,
		Source:        "revised",
		Revisable:     true,
		Version:       currentFact.Version + 1,
		PatientID:     currentFact.PatientID,
		EventTime:     currentFact.EventTime, // Keep original event time
		IngestionTime: time.Now(),            // New ingestion time
	}

	err = f.db.QueryRowContext(ctx, `
		INSERT INTO atomic_facts (
			content, confidence, source, revisable, version,
			patient_id, event_time, ingestion_time, created_at, updated_at,
			previous_version_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW(), $9)
		RETURNING id
	`, newFact.Content, newFact.Confidence, newFact.Source, newFact.Revisable,
		newFact.Version, newFact.PatientID, newFact.EventTime, newFact.IngestionTime,
		factID,
	).Scan(&newFact.ID)

	if err != nil {
		return nil, err
	}

	log.Info().
		Int64("old_fact_id", factID).
		Int64("new_fact_id", newFact.ID).
		Int("new_version", newFact.Version).
		Msg("Revised fact")

	return newFact, nil
}

// FactStats returns statistics about facts
func (f *FactExtractor) FactStats(ctx context.Context, patientID int64) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total facts
	var total int
	err := f.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM atomic_facts WHERE patient_id = $1
	`, patientID).Scan(&total)

	if err != nil {
		return nil, err
	}

	stats["total_facts"] = total

	// By source
	rows, err := f.db.QueryContext(ctx, `
		SELECT source, COUNT(*) as count
		FROM atomic_facts
		WHERE patient_id = $1
		GROUP BY source
	`, patientID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bySource := make(map[string]int)
	for rows.Next() {
		var source string
		var count int
		if err := rows.Scan(&source, &count); err != nil {
			return nil, err
		}
		bySource[source] = count
	}

	stats["by_source"] = bySource

	// Average confidence
	var avgConfidence float64
	err = f.db.QueryRowContext(ctx, `
		SELECT AVG(confidence) FROM atomic_facts WHERE patient_id = $1
	`, patientID).Scan(&avgConfidence)

	if err != nil {
		return nil, err
	}

	stats["avg_confidence"] = avgConfidence

	return stats, nil
}
