// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package silence

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// SilenceDetector detects when a child stops talking about a previously frequent topic
type SilenceDetector struct {
	db *sql.DB
}

// NewSilenceDetector creates a new silence detector
func NewSilenceDetector(db *sql.DB) *SilenceDetector {
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT id 
		FROM conversations 
		WHERE patient_id = $1 
		ORDER BY started_at DESC 
		LIMIT $2
	`, patientID, limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			sessions = append(sessions, id)
		}
	}

	// Reverse to get chronological order
	for i, j := 0, len(sessions)-1; i < j; i, j = i+1, j-1 {
		sessions[i], sessions[j] = sessions[j], sessions[i]
	}

	return sessions, rows.Err()
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
	rows, err := s.db.QueryContext(ctx, `
		SELECT content 
		FROM messages 
		WHERE conversation_id = $1
	`, sessionID)

	if err != nil {
		return counts
	}
	defer rows.Close()

	var allContent strings.Builder
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err == nil {
			allContent.WriteString(" ")
			allContent.WriteString(strings.ToLower(content))
		}
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
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO silence_alerts (
			patient_id, topic, expected_frequency, actual_frequency,
			sessions_silent, alert_level, first_detected
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (patient_id, topic) WHERE NOT resolved
		DO UPDATE SET
			sessions_silent = silence_alerts.sessions_silent + 1,
			last_checked = NOW()
	`, patientID, freq.Topic, freq.ExpectedFrequency, freq.ActualFrequency,
		freq.SessionsSilent, freq.AlertLevel, freq.FirstDetected)

	if err == nil {
		log.Warn().
			Int64("patient_id", patientID).
			Str("topic", freq.Topic).
			Float64("expected", freq.ExpectedFrequency).
			Str("alert_level", freq.AlertLevel).
			Msg("🔇 SILENCE ALERT: Topic disappeared")
	}

	return err
}

// GetActiveAlerts retrieves active silence alerts for a patient
func (s *SilenceDetector) GetActiveAlerts(ctx context.Context, patientID int64) ([]*SilenceAlert, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, patient_id, topic, expected_frequency, actual_frequency,
		       sessions_silent, alert_level, first_detected, resolved
		FROM silence_alerts
		WHERE patient_id = $1 AND NOT resolved
		ORDER BY alert_level DESC, first_detected DESC
	`, patientID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*SilenceAlert
	for rows.Next() {
		var alert SilenceAlert
		err := rows.Scan(
			&alert.ID, &alert.PatientID, &alert.Topic, &alert.ExpectedFrequency,
			&alert.ActualFrequency, &alert.SessionsSilent, &alert.AlertLevel,
			&alert.FirstDetected, &alert.Resolved,
		)
		if err == nil {
			alerts = append(alerts, &alert)
		}
	}

	return alerts, rows.Err()
}

// ResolveAlert marks a silence alert as resolved
func (s *SilenceDetector) ResolveAlert(ctx context.Context, alertID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE silence_alerts 
		SET resolved = TRUE 
		WHERE id = $1
	`, alertID)

	return err
}
