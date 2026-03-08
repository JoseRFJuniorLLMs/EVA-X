// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package synthesis

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
	"eva/internal/clinical/goals"
	"eva/internal/clinical/notes"
	"eva/internal/clinical/risk"
	"eva/internal/clinical/silence"

	"github.com/rs/zerolog/log"
)

// SessionSynthesizer generates comprehensive session reports
type SessionSynthesizer struct {
	db              *database.DB
	riskDetector    *risk.PediatricRiskDetector
	noteGenerator   *notes.ClinicalNoteGenerator
	goalTracker     *goals.TreatmentGoalTracker
	silenceDetector *silence.SilenceDetector
}

// NewSessionSynthesizer creates a new session synthesizer
func NewSessionSynthesizer(
	db *database.DB,
	riskDetector *risk.PediatricRiskDetector,
	noteGenerator *notes.ClinicalNoteGenerator,
	goalTracker *goals.TreatmentGoalTracker,
	silenceDetector *silence.SilenceDetector,
) *SessionSynthesizer {
	return &SessionSynthesizer{
		db:              db,
		riskDetector:    riskDetector,
		noteGenerator:   noteGenerator,
		goalTracker:     goalTracker,
		silenceDetector: silenceDetector,
	}
}

// SessionSynthesis represents a comprehensive session report
type SessionSynthesis struct {
	ID                int64          `json:"id"`
	PatientID         int64          `json:"patient_id"`
	SessionID         int64          `json:"session_id"`
	SessionNumber     int            `json:"session_number"`
	SessionDate       time.Time      `json:"session_date"`
	Duration          int            `json:"duration_minutes"`
	MainThemes        []ThemeSummary `json:"main_themes"`
	Alerts            []AlertSummary `json:"alerts"`
	TreatmentProgress []GoalProgress `json:"treatment_progress"`
	RiskSummary       *RiskSummary   `json:"risk_summary"`
	Suggestions       []string       `json:"suggestions"`
	GeneratedAt       time.Time      `json:"generated_at"`
}

// ThemeSummary represents a theme in the session
type ThemeSummary struct {
	Theme     string  `json:"theme"`
	Frequency int     `json:"frequency"`
	Trend     string  `json:"trend"` // "↑ increasing", "↓ decreasing", "→ stable", "🆕 new"
	Sentiment float64 `json:"sentiment"`
}

// AlertSummary represents an alert
type AlertSummary struct {
	Type    string `json:"type"`  // "risk", "silence", "crisis"
	Level   string `json:"level"` // "LOW", "MODERATE", "HIGH", "CRITICAL"
	Message string `json:"message"`
	Details string `json:"details"`
}

// GoalProgress represents treatment goal progress
type GoalProgress struct {
	GoalID      int64   `json:"goal_id"`
	Description string  `json:"description"`
	Progress    float64 `json:"progress"` // 0-1
	Trend       string  `json:"trend"`
}

// RiskSummary represents risk assessment summary
type RiskSummary struct {
	Level             string   `json:"level"`
	Score             float64  `json:"score"`
	DetectedMetaphors []string `json:"detected_metaphors,omitempty"`
}

// GenerateSynthesis generates a comprehensive session synthesis
func (s *SessionSynthesizer) GenerateSynthesis(ctx context.Context, sessionID int64) (*SessionSynthesis, error) {
	// Get session info
	convNode, err := s.db.GetNodeByID(ctx, "conversations", sessionID)
	if err != nil {
		return nil, err
	}
	if convNode == nil {
		return nil, fmt.Errorf("conversation %d not found", sessionID)
	}

	patientID := database.GetInt64(convNode, "patient_id")
	sessionNumber := int(database.GetInt64(convNode, "session_number"))
	sessionDate := database.GetTime(convNode, "started_at")

	synthesis := &SessionSynthesis{
		PatientID:     patientID,
		SessionID:     sessionID,
		SessionNumber: sessionNumber,
		SessionDate:   sessionDate,
		GeneratedAt:   time.Now(),
	}

	// 1. Analyze main themes
	synthesis.MainThemes = s.analyzeThemes(ctx, sessionID, patientID)

	// 2. Collect alerts
	synthesis.Alerts = s.collectAlerts(ctx, sessionID, patientID)

	// 3. Get treatment progress
	synthesis.TreatmentProgress = s.getTreatmentProgress(ctx, patientID)

	// 4. Get risk summary
	synthesis.RiskSummary = s.getRiskSummary(ctx, sessionID)

	// 5. Generate suggestions
	synthesis.Suggestions = s.generateSuggestions(synthesis)

	// 6. Store synthesis
	err = s.storeSynthesis(ctx, synthesis)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store synthesis")
	}

	return synthesis, nil
}

// analyzeThemes analyzes main themes in session
func (s *SessionSynthesizer) analyzeThemes(ctx context.Context, sessionID, patientID int64) []ThemeSummary {
	// Get clinical notes for this session
	clinicalNotes, err := s.noteGenerator.GetNotesForSession(ctx, sessionID)
	if err != nil {
		return []ThemeSummary{}
	}

	// Aggregate themes
	themeFreq := make(map[string]int)
	themeSentiment := make(map[string]float64)

	for _, note := range clinicalNotes {
		for _, theme := range note.ClinicalThemes {
			themeFreq[theme]++
			themeSentiment[theme] += note.SentimentDelta
		}
	}

	// Get previous session themes for trend analysis
	previousThemes := s.getPreviousSessionThemes(ctx, patientID, sessionID)

	var themes []ThemeSummary
	for theme, freq := range themeFreq {
		trend := s.calculateTrend(theme, freq, previousThemes)
		avgSentiment := themeSentiment[theme] / float64(freq)

		themes = append(themes, ThemeSummary{
			Theme:     theme,
			Frequency: freq,
			Trend:     trend,
			Sentiment: avgSentiment,
		})
	}

	return themes
}

// getPreviousSessionThemes gets themes from previous session
func (s *SessionSynthesizer) getPreviousSessionThemes(ctx context.Context, patientID, currentSessionID int64) map[string]int {
	themes := make(map[string]int)

	// Get previous session: conversations for patient with id < currentSessionID, sorted DESC, limit 1
	convRows, err := s.db.QueryByLabel(ctx, "conversations",
		" AND n.patient_id = $pid", map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return themes
	}

	// Filter id < currentSessionID and sort by started_at DESC
	type convEntry struct {
		id        int64
		startedAt time.Time
	}
	var candidates []convEntry
	for _, m := range convRows {
		cid := database.GetInt64(m, "id")
		if cid < currentSessionID {
			candidates = append(candidates, convEntry{
				id:        cid,
				startedAt: database.GetTime(m, "started_at"),
			})
		}
	}

	if len(candidates) == 0 {
		return themes
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].startedAt.After(candidates[j].startedAt)
	})

	prevSessionID := candidates[0].id

	// Get themes from previous synthesis
	synthRows, err := s.db.QueryByLabel(ctx, "session_syntheses",
		" AND n.session_id = $sid", map[string]interface{}{
			"sid": prevSessionID,
		}, 1)
	if err != nil || len(synthRows) == 0 {
		return themes
	}

	themesJSON := database.GetString(synthRows[0], "main_themes")
	var prevThemes []ThemeSummary
	json.Unmarshal([]byte(themesJSON), &prevThemes)

	for _, t := range prevThemes {
		themes[t.Theme] = t.Frequency
	}

	return themes
}

// calculateTrend calculates trend for a theme
func (s *SessionSynthesizer) calculateTrend(theme string, currentFreq int, previousThemes map[string]int) string {
	prevFreq, exists := previousThemes[theme]

	if !exists {
		return "🆕 new"
	}

	if currentFreq > prevFreq {
		return "↑ increasing"
	} else if currentFreq < prevFreq {
		return "↓ decreasing"
	}

	return "→ stable"
}

// collectAlerts collects all alerts for the session
func (s *SessionSynthesizer) collectAlerts(ctx context.Context, sessionID, patientID int64) []AlertSummary {
	var alerts []AlertSummary

	// 1. Risk alerts
	riskAssessments, err := s.riskDetector.GetRecentAssessments(ctx, patientID, 10)
	if err == nil {
		for _, assessment := range riskAssessments {
			if assessment.SessionID == sessionID && (assessment.RiskLevel == "HIGH" || assessment.RiskLevel == "CRITICAL") {
				alerts = append(alerts, AlertSummary{
					Type:    "risk",
					Level:   assessment.RiskLevel,
					Message: fmt.Sprintf("Risk detected: %s", assessment.RiskLevel),
					Details: fmt.Sprintf("Metaphors: %s", strings.Join(assessment.DetectedMetaphors, ", ")),
				})
			}
		}
	}

	// 2. Silence alerts
	silenceAlerts, err := s.silenceDetector.GetActiveAlerts(ctx, patientID)
	if err == nil {
		for _, alert := range silenceAlerts {
			alerts = append(alerts, AlertSummary{
				Type:    "silence",
				Level:   alert.AlertLevel,
				Message: fmt.Sprintf("Topic '%s' not mentioned", alert.Topic),
				Details: fmt.Sprintf("Expected %.1f mentions, got %.1f", alert.ExpectedFrequency, alert.ActualFrequency),
			})
		}
	}

	return alerts
}

// getTreatmentProgress gets treatment goal progress
func (s *SessionSynthesizer) getTreatmentProgress(ctx context.Context, patientID int64) []GoalProgress {
	activeGoals, err := s.goalTracker.GetActiveGoals(ctx, patientID)
	if err != nil {
		return []GoalProgress{}
	}

	var progress []GoalProgress
	for _, goal := range activeGoals {
		progressPct := s.goalTracker.CalculateProgress(goal)

		// Determine trend
		trend := "→"
		if len(goal.ProgressMetrics) > 0 {
			// Check if metrics are improving
			improving := 0
			for _, values := range goal.ProgressMetrics {
				if len(values) >= 2 {
					if values[len(values)-1] < values[0] {
						improving++
					}
				}
			}
			if improving > 0 {
				trend = "↑ improving"
			}
		}

		progress = append(progress, GoalProgress{
			GoalID:      goal.ID,
			Description: goal.Description,
			Progress:    progressPct,
			Trend:       trend,
		})
	}

	return progress
}

// getRiskSummary gets risk assessment summary
func (s *SessionSynthesizer) getRiskSummary(ctx context.Context, sessionID int64) *RiskSummary {
	// Get highest risk assessment for this session
	rows, err := s.db.QueryByLabel(ctx, "risk_detections",
		" AND n.session_id = $sid", map[string]interface{}{
			"sid": sessionID,
		}, 0)
	if err != nil || len(rows) == 0 {
		return &RiskSummary{Level: "NONE", Score: 0}
	}

	// Sort by risk_score DESC and take the first
	sort.Slice(rows, func(i, j int) bool {
		return database.GetFloat64(rows[i], "risk_score") > database.GetFloat64(rows[j], "risk_score")
	})

	best := rows[0]
	level := database.GetString(best, "risk_level")
	score := database.GetFloat64(best, "risk_score")

	var metaphors []string
	json.Unmarshal([]byte(database.GetString(best, "detected_metaphors")), &metaphors)

	return &RiskSummary{
		Level:             level,
		Score:             score,
		DetectedMetaphors: metaphors,
	}
}

// generateSuggestions generates suggestions for next session
func (s *SessionSynthesizer) generateSuggestions(synthesis *SessionSynthesis) []string {
	var suggestions []string

	// 1. Suggestions from alerts
	for _, alert := range synthesis.Alerts {
		if alert.Type == "silence" {
			suggestions = append(suggestions, fmt.Sprintf("Check in about '%s' (not mentioned recently)", extractTopic(alert.Message)))
		}
		if alert.Type == "risk" && alert.Level == "HIGH" {
			suggestions = append(suggestions, "Follow up on risk indicators from previous session")
		}
	}

	// 2. Suggestions from new themes
	for _, theme := range synthesis.MainThemes {
		if theme.Trend == "🆕 new" {
			suggestions = append(suggestions, fmt.Sprintf("Explore new theme: %s", theme.Theme))
		}
		if theme.Trend == "↑ increasing" && theme.Sentiment < -0.3 {
			suggestions = append(suggestions, fmt.Sprintf("Address increasing negative theme: %s", theme.Theme))
		}
	}

	// 3. Suggestions from treatment goals
	for _, goal := range synthesis.TreatmentProgress {
		if goal.Progress > 0.8 {
			suggestions = append(suggestions, fmt.Sprintf("Goal '%s' near completion - consider new objectives", goal.Description))
		}
	}

	// Default suggestion
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "Continue building rapport and monitoring progress")
	}

	return suggestions
}

// extractTopic extracts topic from message
func extractTopic(message string) string {
	// Simple extraction: "Topic 'X' not mentioned" -> "X"
	start := strings.Index(message, "'")
	end := strings.LastIndex(message, "'")
	if start != -1 && end != -1 && start < end {
		return message[start+1 : end]
	}
	return ""
}

// storeSynthesis stores synthesis in database
func (s *SessionSynthesizer) storeSynthesis(ctx context.Context, synthesis *SessionSynthesis) error {
	themesJSON, _ := json.Marshal(synthesis.MainThemes)
	alertsJSON, _ := json.Marshal(synthesis.Alerts)
	progressJSON, _ := json.Marshal(synthesis.TreatmentProgress)
	riskJSON, _ := json.Marshal(synthesis.RiskSummary)
	suggestionsJSON, _ := json.Marshal(synthesis.Suggestions)

	id, err := s.db.Insert(ctx, "session_syntheses", map[string]interface{}{
		"patient_id":         synthesis.PatientID,
		"session_id":         synthesis.SessionID,
		"main_themes":        string(themesJSON),
		"alerts":             string(alertsJSON),
		"treatment_progress": string(progressJSON),
		"risk_summary":       string(riskJSON),
		"suggestions":        string(suggestionsJSON),
		"generated_at":       synthesis.GeneratedAt.Format(time.RFC3339),
	})
	if err != nil {
		return err
	}
	synthesis.ID = id
	return nil
}

// FormatAsMarkdown formats synthesis as markdown report
func (s *SessionSynthesizer) FormatAsMarkdown(synthesis *SessionSynthesis) string {
	var report strings.Builder

	report.WriteString(fmt.Sprintf("# Session #%d Summary\n", synthesis.SessionNumber))
	report.WriteString(fmt.Sprintf("**Date**: %s\n", synthesis.SessionDate.Format("2006-01-02")))
	report.WriteString(fmt.Sprintf("**Patient ID**: %d\n\n", synthesis.PatientID))

	// Main themes
	report.WriteString("## Main Themes\n")
	for _, theme := range synthesis.MainThemes {
		emoji := ""
		if strings.Contains(theme.Trend, "new") {
			emoji = "🆕"
		}
		report.WriteString(fmt.Sprintf("- %s **%s** (%s, %d mentions)\n", emoji, theme.Theme, theme.Trend, theme.Frequency))
	}
	report.WriteString("\n")

	// Alerts
	if len(synthesis.Alerts) > 0 {
		report.WriteString("## Alerts\n")
		for _, alert := range synthesis.Alerts {
			icon := "⚠️"
			if alert.Level == "CRITICAL" {
				icon = "🚨"
			}
			report.WriteString(fmt.Sprintf("- %s **%s**: %s\n", icon, alert.Level, alert.Message))
			if alert.Details != "" {
				report.WriteString(fmt.Sprintf("  - %s\n", alert.Details))
			}
		}
		report.WriteString("\n")
	}

	// Treatment progress
	if len(synthesis.TreatmentProgress) > 0 {
		report.WriteString("## Treatment Progress\n")
		for _, goal := range synthesis.TreatmentProgress {
			progressPct := int(goal.Progress * 100)
			report.WriteString(fmt.Sprintf("- **Goal**: %s\n", goal.Description))
			report.WriteString(fmt.Sprintf("  - Progress: %d%% (%s)\n", progressPct, goal.Trend))
		}
		report.WriteString("\n")
	}

	// Risk assessment
	report.WriteString("## Risk Assessment\n")
	report.WriteString(fmt.Sprintf("- Risk Level: **%s**\n", synthesis.RiskSummary.Level))
	if len(synthesis.RiskSummary.DetectedMetaphors) > 0 {
		report.WriteString(fmt.Sprintf("- Detected Metaphors: %s\n", strings.Join(synthesis.RiskSummary.DetectedMetaphors, ", ")))
	}
	report.WriteString("\n")

	// Suggestions
	report.WriteString("## Suggestions for Next Session\n")
	for i, suggestion := range synthesis.Suggestions {
		report.WriteString(fmt.Sprintf("%d. %s\n", i+1, suggestion))
	}

	return report.String()
}
