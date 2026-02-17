// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notes

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

// ClinicalNoteGenerator generates clinical insights for psychologists
type ClinicalNoteGenerator struct {
	db *sql.DB
}

// NewClinicalNoteGenerator creates a new clinical note generator
func NewClinicalNoteGenerator(db *sql.DB) *ClinicalNoteGenerator {
	return &ClinicalNoteGenerator{db: db}
}

// ClinicalNote represents an auto-generated clinical insight
type ClinicalNote struct {
	ID               int64     `json:"id"`
	PatientID        int64     `json:"patient_id"`
	SessionID        int64     `json:"session_id"`
	RawStatement     string    `json:"raw_statement"`
	PossibleMeanings []string  `json:"possible_meanings"`
	RelatedMemories  []Memory  `json:"related_memories"`
	SentimentDelta   float64   `json:"sentiment_delta"` // Change from previous session
	AlertLevel       int       `json:"alert_level"`     // 0-3
	ClinicalThemes   []string  `json:"clinical_themes"`
	RecommendedFocus string    `json:"recommended_focus"`
	CreatedAt        time.Time `json:"created_at"`
}

// Memory represents a related memory
type Memory struct {
	ID         int64     `json:"id"`
	Content    string    `json:"content"`
	SessionID  int64     `json:"session_id"`
	Timestamp  time.Time `json:"timestamp"`
	Similarity float64   `json:"similarity"`
}

// GenerateNote generates a clinical note from a statement
func (g *ClinicalNoteGenerator) GenerateNote(ctx context.Context, statement string, patientID, sessionID int64) (*ClinicalNote, error) {
	note := &ClinicalNote{
		PatientID:        patientID,
		SessionID:        sessionID,
		RawStatement:     statement,
		PossibleMeanings: []string{},
		RelatedMemories:  []Memory{},
		ClinicalThemes:   []string{},
		CreatedAt:        time.Now(),
	}

	// 1. Identify possible meanings
	note.PossibleMeanings = g.identifyMeanings(statement)

	// 2. Find related memories
	relatedMemories, err := g.findRelatedMemories(ctx, patientID, statement)
	if err == nil {
		note.RelatedMemories = relatedMemories
	}

	// 3. Calculate sentiment delta
	note.SentimentDelta = g.calculateSentimentDelta(ctx, patientID, sessionID)

	// 4. Identify clinical themes
	note.ClinicalThemes = g.identifyThemes(statement, relatedMemories)

	// 5. Determine alert level
	note.AlertLevel = g.determineAlertLevel(note)

	// 6. Generate recommended focus
	note.RecommendedFocus = g.generateRecommendedFocus(note)

	// 7. Store note
	err = g.storeNote(ctx, note)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store clinical note")
	}

	return note, nil
}

// identifyMeanings identifies possible interpretations of a statement
func (g *ClinicalNoteGenerator) identifyMeanings(statement string) []string {
	meanings := []string{}
	statementLower := strings.ToLower(statement)

	// Loss patterns
	if strings.Contains(statementLower, "fugiu") || strings.Contains(statementLower, "foi embora") || strings.Contains(statementLower, "perdi") {
		meanings = append(meanings, "Real loss (pet, person, object)")
		meanings = append(meanings, "Metaphor for abandonment or separation")
		meanings = append(meanings, "Fear of loss")
	}

	// Sadness patterns
	if strings.Contains(statementLower, "triste") || strings.Contains(statementLower, "choro") {
		meanings = append(meanings, "Emotional distress")
		meanings = append(meanings, "Possible depression indicator")
		meanings = append(meanings, "Grief or mourning")
	}

	// Isolation patterns
	if strings.Contains(statementLower, "sozinho") || strings.Contains(statementLower, "ninguém") {
		meanings = append(meanings, "Social isolation")
		meanings = append(meanings, "Loneliness")
		meanings = append(meanings, "Lack of support system")
	}

	// Anger patterns
	if strings.Contains(statementLower, "raiva") || strings.Contains(statementLower, "ódio") || strings.Contains(statementLower, "bravo") {
		meanings = append(meanings, "Anger expression")
		meanings = append(meanings, "Possible trauma response")
		meanings = append(meanings, "Difficulty with emotion regulation")
	}

	// Fear patterns
	if strings.Contains(statementLower, "medo") || strings.Contains(statementLower, "assustado") {
		meanings = append(meanings, "Anxiety or fear")
		meanings = append(meanings, "Possible trauma trigger")
		meanings = append(meanings, "Safety concerns")
	}

	// School patterns
	if strings.Contains(statementLower, "escola") || strings.Contains(statementLower, "professor") {
		meanings = append(meanings, "Academic concerns")
		meanings = append(meanings, "Social dynamics at school")
		meanings = append(meanings, "Possible bullying or conflict")
	}

	// Family patterns
	if strings.Contains(statementLower, "mãe") || strings.Contains(statementLower, "pai") || strings.Contains(statementLower, "família") {
		meanings = append(meanings, "Family dynamics")
		meanings = append(meanings, "Attachment concerns")
		meanings = append(meanings, "Home environment issues")
	}

	// If no specific patterns, add generic
	if len(meanings) == 0 {
		meanings = append(meanings, "General conversation")
		meanings = append(meanings, "Requires contextual analysis")
	}

	return meanings
}

// findRelatedMemories finds semantically related memories
func (g *ClinicalNoteGenerator) findRelatedMemories(ctx context.Context, patientID int64, statement string) ([]Memory, error) {
	// Simple keyword-based search (TODO: upgrade to semantic search with embeddings)
	keywords := extractKeywords(statement)

	if len(keywords) == 0 {
		return []Memory{}, nil
	}

	// Build query with keyword search
	query := `
		SELECT m.id, m.content, c.id as session_id, m.created_at
		FROM memories m
		JOIN conversations c ON m.patient_id = c.patient_id
		WHERE m.patient_id = $1
		AND (
	`

	conditions := []string{}
	for _, keyword := range keywords {
		conditions = append(conditions, "LOWER(m.content) LIKE '%' || LOWER('"+keyword+"') || '%'")
	}
	query += strings.Join(conditions, " OR ")
	query += `) ORDER BY m.created_at DESC LIMIT 5`

	rows, err := g.db.QueryContext(ctx, query, patientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.Content, &m.SessionID, &m.Timestamp); err == nil {
			m.Similarity = 0.7 // Placeholder
			memories = append(memories, m)
		}
	}

	return memories, rows.Err()
}

// extractKeywords extracts important keywords from statement
func extractKeywords(statement string) []string {
	// Remove common words
	stopWords := map[string]bool{
		"o": true, "a": true, "de": true, "da": true, "do": true,
		"e": true, "é": true, "em": true, "um": true, "uma": true,
		"para": true, "com": true, "por": true, "que": true,
	}

	words := strings.Fields(strings.ToLower(statement))
	keywords := []string{}

	for _, word := range words {
		cleaned := strings.Trim(word, ".,!?;:")
		if len(cleaned) > 3 && !stopWords[cleaned] {
			keywords = append(keywords, cleaned)
		}
	}

	return keywords
}

// calculateSentimentDelta calculates change in sentiment from previous session
func (g *ClinicalNoteGenerator) calculateSentimentDelta(ctx context.Context, patientID, currentSessionID int64) float64 {
	// TODO: Implement actual sentiment analysis
	// For now, return placeholder
	return 0.0
}

// identifyThemes identifies clinical themes
func (g *ClinicalNoteGenerator) identifyThemes(statement string, relatedMemories []Memory) []string {
	themes := []string{}
	statementLower := strings.ToLower(statement)

	// Check for recurring themes in related memories
	themeKeywords := map[string][]string{
		"loss":       {"perdi", "fugiu", "foi embora", "morreu"},
		"sadness":    {"triste", "choro", "chorar", "tristeza"},
		"isolation":  {"sozinho", "ninguém", "sem amigos"},
		"fear":       {"medo", "assustado", "terror"},
		"anger":      {"raiva", "ódio", "bravo", "irritado"},
		"family":     {"mãe", "pai", "família", "irmão"},
		"school":     {"escola", "professor", "aula", "colega"},
		"self-worth": {"burro", "inútil", "não sirvo", "bobão"},
	}

	for theme, keywords := range themeKeywords {
		count := 0
		for _, keyword := range keywords {
			if strings.Contains(statementLower, keyword) {
				count++
			}
			// Check related memories
			for _, mem := range relatedMemories {
				if strings.Contains(strings.ToLower(mem.Content), keyword) {
					count++
				}
			}
		}
		if count >= 2 {
			themes = append(themes, theme)
		}
	}

	return themes
}

// determineAlertLevel determines clinical alert level (0-3)
func (g *ClinicalNoteGenerator) determineAlertLevel(note *ClinicalNote) int {
	level := 0

	// High-risk themes
	for _, theme := range note.ClinicalThemes {
		if theme == "self-worth" || theme == "isolation" {
			level++
		}
	}

	// Negative sentiment delta
	if note.SentimentDelta < -0.3 {
		level++
	}

	// Multiple related memories (pattern)
	if len(note.RelatedMemories) >= 3 {
		level++
	}

	// Cap at 3
	if level > 3 {
		level = 3
	}

	return level
}

// generateRecommendedFocus generates recommended focus for psychologist
func (g *ClinicalNoteGenerator) generateRecommendedFocus(note *ClinicalNote) string {
	if note.AlertLevel >= 2 {
		return "High priority: Explore " + strings.Join(note.ClinicalThemes, ", ") + ". Consider risk assessment."
	}

	if len(note.ClinicalThemes) > 0 {
		return "Explore themes: " + strings.Join(note.ClinicalThemes, ", ")
	}

	return "Continue building rapport and monitoring"
}

// storeNote stores clinical note in database
func (g *ClinicalNoteGenerator) storeNote(ctx context.Context, note *ClinicalNote) error {
	meaningsJSON, _ := json.Marshal(note.PossibleMeanings)
	memoriesJSON, _ := json.Marshal(note.RelatedMemories)
	themesJSON, _ := json.Marshal(note.ClinicalThemes)

	return g.db.QueryRowContext(ctx, `
		INSERT INTO clinical_notes (
			patient_id, session_id, raw_statement, possible_meanings,
			related_memories, sentiment_delta, alert_level, clinical_themes,
			recommended_focus, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, note.PatientID, note.SessionID, note.RawStatement, meaningsJSON,
		memoriesJSON, note.SentimentDelta, note.AlertLevel, themesJSON,
		note.RecommendedFocus, note.CreatedAt,
	).Scan(&note.ID)
}

// GetNotesForSession retrieves all clinical notes for a session
func (g *ClinicalNoteGenerator) GetNotesForSession(ctx context.Context, sessionID int64) ([]*ClinicalNote, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT id, patient_id, session_id, raw_statement, possible_meanings,
		       related_memories, sentiment_delta, alert_level, clinical_themes,
		       recommended_focus, created_at
		FROM clinical_notes
		WHERE session_id = $1
		ORDER BY created_at ASC
	`, sessionID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []*ClinicalNote
	for rows.Next() {
		var note ClinicalNote
		var meaningsJSON, memoriesJSON, themesJSON []byte

		err := rows.Scan(
			&note.ID, &note.PatientID, &note.SessionID, &note.RawStatement,
			&meaningsJSON, &memoriesJSON, &note.SentimentDelta, &note.AlertLevel,
			&themesJSON, &note.RecommendedFocus, &note.CreatedAt,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(meaningsJSON, &note.PossibleMeanings)
		json.Unmarshal(memoriesJSON, &note.RelatedMemories)
		json.Unmarshal(themesJSON, &note.ClinicalThemes)

		notes = append(notes, &note)
	}

	return notes, rows.Err()
}
