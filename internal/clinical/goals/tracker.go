// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package goals

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

// TreatmentGoalTracker tracks therapeutic objectives and measures progress
type TreatmentGoalTracker struct {
	db *database.DB
}

// NewTreatmentGoalTracker creates a new treatment goal tracker
func NewTreatmentGoalTracker(db *database.DB) *TreatmentGoalTracker {
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

	id, err := t.db.Insert(ctx, "treatment_goals", map[string]interface{}{
		"patient_id":         patientID,
		"description":        description,
		"target_sessions":    targetSessions,
		"sessions_completed": 0,
		"related_themes":     string(themesJSON),
		"progress_metrics":   string(metricsJSON),
		"status":             "ACTIVE",
		"created_at":         goal.CreatedAt.Format(time.RFC3339Nano),
		"updated_at":         goal.UpdatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return nil, err
	}
	goal.ID = id

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

	updates := map[string]interface{}{
		"sessions_completed": goal.SessionsCompleted,
		"progress_metrics":   string(metricsJSON),
		"progress_notes":     string(notesJSON),
		"status":             goal.Status,
		"updated_at":         time.Now().Format(time.RFC3339Nano),
	}
	if goal.CompletedAt != nil {
		updates["completed_at"] = goal.CompletedAt.Format(time.RFC3339Nano)
	}

	err = t.db.Update(ctx, "treatment_goals",
		map[string]interface{}{"id": float64(goalID)},
		updates,
	)

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
	m, err := t.db.GetNodeByID(ctx, "treatment_goals", goalID)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("treatment goal %d not found", goalID)
	}

	goal := contentToTreatmentGoal(m)
	return goal, nil
}

// GetActiveGoals retrieves all active goals for a patient
func (t *TreatmentGoalTracker) GetActiveGoals(ctx context.Context, patientID int64) ([]*TreatmentGoal, error) {
	rows, err := t.db.QueryByLabel(ctx, "treatment_goals",
		" AND n.patient_id = $pid AND n.status = $status",
		map[string]interface{}{
			"pid":    float64(patientID),
			"status": "ACTIVE",
		}, 0)
	if err != nil {
		return nil, err
	}

	var goals []*TreatmentGoal
	for _, m := range rows {
		goals = append(goals, contentToTreatmentGoal(m))
	}

	// Sort by created_at DESC
	for i := 0; i < len(goals); i++ {
		for j := i + 1; j < len(goals); j++ {
			if goals[j].CreatedAt.After(goals[i].CreatedAt) {
				goals[i], goals[j] = goals[j], goals[i]
			}
		}
	}

	return goals, nil
}

// contentToTreatmentGoal converts a NietzscheDB content map to a TreatmentGoal.
func contentToTreatmentGoal(m map[string]interface{}) *TreatmentGoal {
	goal := &TreatmentGoal{
		ID:                database.GetInt64(m, "id"),
		PatientID:         database.GetInt64(m, "patient_id"),
		Description:       database.GetString(m, "description"),
		TargetSessions:    int(database.GetInt64(m, "target_sessions")),
		SessionsCompleted: int(database.GetInt64(m, "sessions_completed")),
		Status:            database.GetString(m, "status"),
		CreatedAt:         database.GetTime(m, "created_at"),
		UpdatedAt:         database.GetTime(m, "updated_at"),
		CompletedAt:       database.GetTimePtr(m, "completed_at"),
	}

	// Parse JSON string fields
	themesStr := database.GetString(m, "related_themes")
	if themesStr != "" {
		json.Unmarshal([]byte(themesStr), &goal.RelatedThemes)
	}
	metricsStr := database.GetString(m, "progress_metrics")
	if metricsStr != "" {
		json.Unmarshal([]byte(metricsStr), &goal.ProgressMetrics)
	}
	notesStr := database.GetString(m, "progress_notes")
	if notesStr != "" {
		json.Unmarshal([]byte(notesStr), &goal.ProgressNotes)
	}

	if goal.ProgressMetrics == nil {
		goal.ProgressMetrics = make(map[string][]int)
	}
	if goal.ProgressNotes == nil {
		goal.ProgressNotes = []string{}
	}
	if goal.RelatedThemes == nil {
		goal.RelatedThemes = []string{}
	}

	return goal
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
