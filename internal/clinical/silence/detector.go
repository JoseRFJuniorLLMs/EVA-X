// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package silence

import (
	"context"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

// SilenceDetector detects when a child stops talking about a previously frequent topic
type SilenceDetector struct {
	db *database.DB
}

// NewSilenceDetector creates a new silence detector
func NewSilenceDetector(db *database.DB) *SilenceDetector {
	return &SilenceDetector{db: db}
}

// TopicFrequency represents topic mention frequency
type TopicFrequency struct {
	Topic             string    `json:"topic"`
	ExpectedFrequency float64   `json:"expected_frequency"` // mentions per session
	ActualFrequency   float64   `json:"actual_frequency"`
	SessionsSilent    int       `json:"sessions_silent"`
	AlertLevel        string    `json:"alert_level"`
	FirstDetected     time.Time `json:"first_detected"`
}

// SilenceAlert represents a silence alert
type SilenceAlert struct {
	ID                int64     `json:"id"`
	PatientID         int64     `json:"patient_id"`
	Topic             string    `json:"topic"`
	ExpectedFrequency float64   `json:"expected_frequency"`
	ActualFrequency   float64   `json:"actual_frequency"`
	SessionsSilent    int       `json:"sessions_silent"`
	AlertLevel        string    `json:"alert_level"`
	FirstDetected     time.Time `json:"first_detected"`
	Resolved          bool      `json:"resolved"`
}

// AnalyzeTopicFrequencies analyzes topic frequencies for a patient
func (s *SilenceDetector) AnalyzeTopicFrequencies(ctx context.Context, patientID int64) ([]*TopicFrequency, error) {
	// 1. Get all sessions for patient
	sessions, err := s.getRecentSessions(ctx, patientID, 10)
	if err != nil {
		return nil, err
	}

	if len(sessions) < 3 {
		// Not enough data
		return []*TopicFrequency{}, nil
	}

	// 2. Extract topics from each session
	topicCounts := make(map[string][]int) // topic -> [count_session1, count_session2, ...]

	for _, sessionID := range sessions {
		counts := s.countTopicsInSession(ctx, sessionID)
		for topic, count := range counts {
			topicCounts[topic] = append(topicCounts[topic], count)
		}
	}

	// 3. Calculate expected vs actual frequencies
	var frequencies []*TopicFrequency

	for topic, counts := range topicCounts {
		if len(counts) < 3 {
			continue
		}

		// Expected frequency = average of first N-1 sessions
		expectedSum := 0
		for i := 0; i < len(counts)-1; i++ {
			expectedSum += counts[i]
		}
		expectedFreq := float64(expectedSum) / float64(len(counts)-1)

		// Actual frequency = last session
		actualFreq := float64(counts[len(counts)-1])

		// Detect silence (expected > 2 mentions, actual = 0)
		if expectedFreq >= 2.0 && actualFreq == 0 {
			freq := &TopicFrequency{
				Topic:             topic,
				ExpectedFrequency: expectedFreq,
				ActualFrequency:   actualFreq,
				SessionsSilent:    1,
				AlertLevel:        s.calculateAlertLevel(expectedFreq, actualFreq),
				FirstDetected:     time.Now(),
			}

			frequencies = append(frequencies, freq)

			// Create alert
			s.createSilenceAlert(ctx, patientID, freq)
		}
	}

	return frequencies, nil
}

// getRecentSessions retrieves recent session IDs
func (s *SilenceDetector) getRecentSessions(ctx context.Context, patientID int64, limit int) ([]int64, error) {
	rows, err := s.db.QueryByLabel(ctx, "conversations",
		" AND n.patient_id = $pid", map[string]interface{}{
			"pid": patientID,
		}, 0)
	if err != nil {
		return nil, err
	}

	// Build sortable entries
	type convEntry struct {
		id        int64
		startedAt time.Time
	}
	var entries []convEntry
	for _, m := range rows {
		entries = append(entries, convEntry{
			id:        database.GetInt64(m, "id"),
			startedAt: database.GetTime(m, "started_at"),
		})
	}

	// Sort by started_at DESC
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].startedAt.After(entries[j].startedAt)
	})

	// Apply limit
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// Extract IDs and reverse to chronological order
	sessions := make([]int64, len(entries))
	for i, e := range entries {
		sessions[i] = e.id
	}
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	return sessions, nil
}

// countTopicsInSession counts topic mentions in a session
func (s *SilenceDetector) countTopicsInSession(ctx context.Context, sessionID int64) map[string]int {
	// Predefined topics to track
	topics := []string{
		"escola", "professor", "aula", "colega",
		"mãe", "pai", "família", "irmão",
		"amigo", "amiga",
		"medo", "triste", "feliz", "raiva",
		"brincar", "jogar",
	}

	counts := make(map[string]int)

	// Get all messages in session
	msgRows, err := s.db.QueryByLabel(ctx, "messages",
		" AND n.conversation_id = $cid", map[string]interface{}{
			"cid": sessionID,
		}, 0)
	if err != nil {
		return counts
	}

	var allContent strings.Builder
	for _, m := range msgRows {
		content := database.GetString(m, "content")
		allContent.WriteString(" ")
		allContent.WriteString(strings.ToLower(content))
	}

	text := allContent.String()

	// Count each topic
	for _, topic := range topics {
		counts[topic] = strings.Count(text, topic)
	}

	return counts
}

// calculateAlertLevel determines alert level based on expected vs actual
func (s *SilenceDetector) calculateAlertLevel(expected, actual float64) string {
	if expected >= 5.0 && actual == 0 {
		return "HIGH"
	}
	if expected >= 3.0 && actual == 0 {
		return "MODERATE"
	}
	return "LOW"
}

// createSilenceAlert creates a silence alert in database
func (s *SilenceDetector) createSilenceAlert(ctx context.Context, patientID int64, freq *TopicFrequency) error {
	// Check for existing unresolved alert for this patient+topic (upsert logic)
	existing, err := s.db.QueryByLabel(ctx, "silence_alerts",
		" AND n.patient_id = $pid AND n.topic = $topic AND n.resolved = $resolved",
		map[string]interface{}{
			"pid":      patientID,
			"topic":    freq.Topic,
			"resolved": false,
		}, 1)
	if err != nil {
		return err
	}

	if len(existing) > 0 {
		// Update existing: increment sessions_silent
		currentSilent := int(database.GetInt64(existing[0], "sessions_silent"))
		err = s.db.Update(ctx, "silence_alerts",
			map[string]interface{}{
				"patient_id": patientID,
				"topic":      freq.Topic,
				"resolved":   false,
			},
			map[string]interface{}{
				"sessions_silent": currentSilent + 1,
				"last_checked":    time.Now().Format(time.RFC3339),
			})
	} else {
		// Insert new alert
		_, err = s.db.Insert(ctx, "silence_alerts", map[string]interface{}{
			"patient_id":         patientID,
			"topic":              freq.Topic,
			"expected_frequency": freq.ExpectedFrequency,
			"actual_frequency":   freq.ActualFrequency,
			"sessions_silent":    freq.SessionsSilent,
			"alert_level":        freq.AlertLevel,
			"first_detected":     freq.FirstDetected.Format(time.RFC3339),
			"resolved":           false,
		})
	}

	if err == nil {
		log.Warn().
			Int64("patient_id", patientID).
			Str("topic", freq.Topic).
			Float64("expected", freq.ExpectedFrequency).
			Str("alert_level", freq.AlertLevel).
			Msg("SILENCE ALERT: Topic disappeared")
	}

	return err
}

// GetActiveAlerts retrieves active silence alerts for a patient
func (s *SilenceDetector) GetActiveAlerts(ctx context.Context, patientID int64) ([]*SilenceAlert, error) {
	rows, err := s.db.QueryByLabel(ctx, "silence_alerts",
		" AND n.patient_id = $pid AND n.resolved = $resolved",
		map[string]interface{}{
			"pid":      patientID,
			"resolved": false,
		}, 0)
	if err != nil {
		return nil, err
	}

	var alerts []*SilenceAlert
	for _, m := range rows {
		alert := &SilenceAlert{
			ID:                database.GetInt64(m, "id"),
			PatientID:         database.GetInt64(m, "patient_id"),
			Topic:             database.GetString(m, "topic"),
			ExpectedFrequency: database.GetFloat64(m, "expected_frequency"),
			ActualFrequency:   database.GetFloat64(m, "actual_frequency"),
			SessionsSilent:    int(database.GetInt64(m, "sessions_silent")),
			AlertLevel:        database.GetString(m, "alert_level"),
			FirstDetected:     database.GetTime(m, "first_detected"),
			Resolved:          database.GetBool(m, "resolved"),
		}
		alerts = append(alerts, alert)
	}

	// Sort by alert_level DESC (CRITICAL > HIGH > MODERATE > LOW), then first_detected DESC
	alertPriority := map[string]int{"CRITICAL": 4, "HIGH": 3, "MODERATE": 2, "LOW": 1}
	sort.Slice(alerts, func(i, j int) bool {
		pi := alertPriority[alerts[i].AlertLevel]
		pj := alertPriority[alerts[j].AlertLevel]
		if pi != pj {
			return pi > pj
		}
		return alerts[i].FirstDetected.After(alerts[j].FirstDetected)
	})

	return alerts, nil
}

// ResolveAlert marks a silence alert as resolved
func (s *SilenceDetector) ResolveAlert(ctx context.Context, alertID int64) error {
	return s.db.Update(ctx, "silence_alerts",
		map[string]interface{}{"id": alertID},
		map[string]interface{}{"resolved": true},
	)
}
