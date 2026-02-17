// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package goals

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rs/zerolog/log"
)

// TreatmentGoalTracker tracks therapeutic objectives and measures progress
type TreatmentGoalTracker struct {
	db *sql.DB
}

// NewTreatmentGoalTracker creates a new treatment goal tracker
func NewTreatmentGoalTracker(db *sql.DB) *TreatmentGoalTracker {
	return &TreatmentGoalTracker{db: db}
}

// TreatmentGoal represents a therapeutic objective
type TreatmentGoal struct {
	ID                int64            `json:"id"`
	PatientID         int64            `json:"patient_id"`
	Description       string           `json:"description"`
	TargetSessions    int              `json:"target_sessions"`
	SessionsCompleted int              `json:"sessions_completed"`
	RelatedThemes     []string         `json:"related_themes"`
	ProgressMetrics   map[string][]int `json:"progress_metrics"` // "choro": [5,3,1,0]
	ProgressNotes     []string         `json:"progress_notes"`
	Status            string           `json:"status"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
	CompletedAt       *time.Time       `json:"completed_at,omitempty"`
}

// CreateGoal creates a new treatment goal
func (t *TreatmentGoalTracker) CreateGoal(ctx context.Context, patientID int64, description string, targetSessions int, relatedThemes []string) (*TreatmentGoal, error) {
	themesJSON, _ := json.Marshal(relatedThemes)
	metricsJSON, _ := json.Marshal(map[string][]int{})

	goal := &TreatmentGoal{
		PatientID:         patientID,
		Description:       description,
		TargetSessions:    targetSessions,
		SessionsCompleted: 0,
		RelatedThemes:     relatedThemes,
		ProgressMetrics:   make(map[string][]int),
		ProgressNotes:     []string{},
		Status:            "ACTIVE",
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	err := t.db.QueryRowContext(ctx, `
		INSERT INTO treatment_goals (
			patient_id, description, target_sessions, sessions_completed,
			related_themes, progress_metrics, status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, patientID, description, targetSessions, 0, themesJSON, metricsJSON,
		"ACTIVE", goal.CreatedAt, goal.UpdatedAt,
	).Scan(&goal.ID)

	if err != nil {
		return nil, err
	}

	log.Info().
		Int64("goal_id", goal.ID).
		Int64("patient_id", patientID).
		Str("description", description).
		Msg("Created treatment goal")

	return goal, nil
}

// UpdateProgress updates progress for a goal after a session
func (t *TreatmentGoalTracker) UpdateProgress(ctx context.Context, goalID int64, sessionMetrics map[string]int, note string) error {
	// Get current goal
	goal, err := t.GetGoal(ctx, goalID)
	if err != nil {
		return err
	}

	// Update metrics
	for theme, count := range sessionMetrics {
		if _, exists := goal.ProgressMetrics[theme]; !exists {
			goal.ProgressMetrics[theme] = []int{}
		}
		goal.ProgressMetrics[theme] = append(goal.ProgressMetrics[theme], count)
	}

	// Add note
	if note != "" {
		goal.ProgressNotes = append(goal.ProgressNotes, note)
	}

	// Increment sessions completed
	goal.SessionsCompleted++

	// Check if goal is completed
	if goal.SessionsCompleted >= goal.TargetSessions {
		goal.Status = "COMPLETED"
		now := time.Now()
		goal.CompletedAt = &now
	}

	// Update database
	metricsJSON, _ := json.Marshal(goal.ProgressMetrics)
	notesJSON, _ := json.Marshal(goal.ProgressNotes)

	_, err = t.db.ExecContext(ctx, `
		UPDATE treatment_goals
		SET sessions_completed = $1,
		    progress_metrics = $2,
		    progress_notes = $3,
		    status = $4,
		    completed_at = $5,
		    updated_at = NOW()
		WHERE id = $6
	`, goal.SessionsCompleted, metricsJSON, notesJSON, goal.Status, goal.CompletedAt, goalID)

	if err == nil {
		log.Info().
			Int64("goal_id", goalID).
			Int("sessions_completed", goal.SessionsCompleted).
			Int("target_sessions", goal.TargetSessions).
			Str("status", goal.Status).
			Msg("Updated treatment goal progress")
	}

	return err
}

// GetGoal retrieves a treatment goal
func (t *TreatmentGoalTracker) GetGoal(ctx context.Context, goalID int64) (*TreatmentGoal, error) {
	var goal TreatmentGoal
	var themesJSON, metricsJSON, notesJSON []byte
	var completedAt sql.NullTime

	err := t.db.QueryRowContext(ctx, `
		SELECT id, patient_id, description, target_sessions, sessions_completed,
		       related_themes, progress_metrics, progress_notes, status,
		       created_at, updated_at, completed_at
		FROM treatment_goals
		WHERE id = $1
	`, goalID).Scan(
		&goal.ID, &goal.PatientID, &goal.Description, &goal.TargetSessions,
		&goal.SessionsCompleted, &themesJSON, &metricsJSON, &notesJSON,
		&goal.Status, &goal.CreatedAt, &goal.UpdatedAt, &completedAt,
	)

	if err != nil {
		return nil, err
	}

	json.Unmarshal(themesJSON, &goal.RelatedThemes)
	json.Unmarshal(metricsJSON, &goal.ProgressMetrics)
	json.Unmarshal(notesJSON, &goal.ProgressNotes)

	if completedAt.Valid {
		goal.CompletedAt = &completedAt.Time
	}

	return &goal, nil
}

// GetActiveGoals retrieves all active goals for a patient
func (t *TreatmentGoalTracker) GetActiveGoals(ctx context.Context, patientID int64) ([]*TreatmentGoal, error) {
	rows, err := t.db.QueryContext(ctx, `
		SELECT id, patient_id, description, target_sessions, sessions_completed,
		       related_themes, progress_metrics, progress_notes, status,
		       created_at, updated_at, completed_at
		FROM treatment_goals
		WHERE patient_id = $1 AND status = 'ACTIVE'
		ORDER BY created_at DESC
	`, patientID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []*TreatmentGoal
	for rows.Next() {
		var goal TreatmentGoal
		var themesJSON, metricsJSON, notesJSON []byte
		var completedAt sql.NullTime

		err := rows.Scan(
			&goal.ID, &goal.PatientID, &goal.Description, &goal.TargetSessions,
			&goal.SessionsCompleted, &themesJSON, &metricsJSON, &notesJSON,
			&goal.Status, &goal.CreatedAt, &goal.UpdatedAt, &completedAt,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(themesJSON, &goal.RelatedThemes)
		json.Unmarshal(metricsJSON, &goal.ProgressMetrics)
		json.Unmarshal(notesJSON, &goal.ProgressNotes)

		if completedAt.Valid {
			goal.CompletedAt = &completedAt.Time
		}

		goals = append(goals, &goal)
	}

	return goals, rows.Err()
}

// CalculateProgress calculates progress percentage for a goal
func (t *TreatmentGoalTracker) CalculateProgress(goal *TreatmentGoal) float64 {
	if goal.TargetSessions == 0 {
		return 0
	}

	// Session progress (50%)
	sessionProgress := float64(goal.SessionsCompleted) / float64(goal.TargetSessions)

	// Metric improvement (50%)
	metricProgress := 0.0
	metricCount := 0

	for _, values := range goal.ProgressMetrics {
		if len(values) >= 2 {
			first := float64(values[0])
			last := float64(values[len(values)-1])

			// Calculate improvement (assuming lower is better for negative themes)
			if first > 0 {
				improvement := (first - last) / first
				if improvement > 0 {
					metricProgress += improvement
					metricCount++
				}
			}
		}
	}

	if metricCount > 0 {
		metricProgress /= float64(metricCount)
	}

	// Combined progress
	totalProgress := (sessionProgress * 0.5) + (metricProgress * 0.5)

	if totalProgress > 1.0 {
		totalProgress = 1.0
	}

	return totalProgress
}

// GetProgressSummary generates a progress summary
func (t *TreatmentGoalTracker) GetProgressSummary(goal *TreatmentGoal) string {
	progress := t.CalculateProgress(goal)
	progressPercent := int(progress * 100)

	summary := ""
	summary += "Progress: " + string(rune(progressPercent)) + "%\n"
	summary += "Sessions: " + string(rune(goal.SessionsCompleted)) + "/" + string(rune(goal.TargetSessions)) + "\n"

	if len(goal.ProgressMetrics) > 0 {
		summary += "\nMetrics:\n"
		for theme, values := range goal.ProgressMetrics {
			if len(values) > 0 {
				trend := "→"
				if len(values) >= 2 {
					if values[len(values)-1] < values[0] {
						trend = "↓ improving"
					} else if values[len(values)-1] > values[0] {
						trend = "↑ worsening"
					}
				}
				summary += "  " + theme + ": " + trend + "\n"
			}
		}
	}

	return summary
}
