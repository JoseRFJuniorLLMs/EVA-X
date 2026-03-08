// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package notes

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"eva/internal/brainstem/database"

	"github.com/rs/zerolog/log"
)

// ClinicalNoteGenerator generates clinical insights for psychologists
type ClinicalNoteGenerator struct {
	db *database.DB
}

// NewClinicalNoteGenerator creates a new clinical note generator
func NewClinicalNoteGenerator(db *database.DB) *ClinicalNoteGenerator {
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

	// Query all memories for this patient, then filter by keywords in Go
	rows, err := g.db.QueryByLabel(ctx, "memories",
		" AND n.patient_id = $pid",
		map[string]interface{}{
			"pid": float64(patientID),
		}, 0)
	if err != nil {
		return nil, err
	}

	var memories []Memory
	for _, m := range rows {
		content := database.GetString(m, "content")
		contentLower := strings.ToLower(content)

		// Check if any keyword matches
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(contentLower, strings.ToLower(keyword)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		mem := Memory{
			ID:         database.GetInt64(m, "id"),
			Content:    content,
			SessionID:  database.GetInt64(m, "session_id"),
			Timestamp:  database.GetTime(m, "created_at"),
			Similarity: 0.7, // Placeholder
		}
		memories = append(memories, mem)

		if len(memories) >= 5 {
			break
		}
	}

	return memories, nil
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

	id, err := g.db.Insert(ctx, "clinical_notes", map[string]interface{}{
		"patient_id":        note.PatientID,
		"session_id":        note.SessionID,
		"raw_statement":     note.RawStatement,
		"possible_meanings": string(meaningsJSON),
		"related_memories":  string(memoriesJSON),
		"sentiment_delta":   note.SentimentDelta,
		"alert_level":       note.AlertLevel,
		"clinical_themes":   string(themesJSON),
		"recommended_focus": note.RecommendedFocus,
		"created_at":        note.CreatedAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return err
	}
	note.ID = id
	return nil
}

// GetNotesForSession retrieves all clinical notes for a session
func (g *ClinicalNoteGenerator) GetNotesForSession(ctx context.Context, sessionID int64) ([]*ClinicalNote, error) {
	rows, err := g.db.QueryByLabel(ctx, "clinical_notes",
		" AND n.session_id = $sid",
		map[string]interface{}{
			"sid": float64(sessionID),
		}, 0)
	if err != nil {
		return nil, err
	}

	var notes []*ClinicalNote
	for _, m := range rows {
		note := &ClinicalNote{
			ID:               database.GetInt64(m, "id"),
			PatientID:        database.GetInt64(m, "patient_id"),
			SessionID:        database.GetInt64(m, "session_id"),
			RawStatement:     database.GetString(m, "raw_statement"),
			SentimentDelta:   database.GetFloat64(m, "sentiment_delta"),
			AlertLevel:       int(database.GetInt64(m, "alert_level")),
			RecommendedFocus: database.GetString(m, "recommended_focus"),
			CreatedAt:        database.GetTime(m, "created_at"),
		}

		// Parse JSON string fields
		meaningsStr := database.GetString(m, "possible_meanings")
		if meaningsStr != "" {
			json.Unmarshal([]byte(meaningsStr), &note.PossibleMeanings)
		}
		memoriesStr := database.GetString(m, "related_memories")
		if memoriesStr != "" {
			json.Unmarshal([]byte(memoriesStr), &note.RelatedMemories)
		}
		themesStr := database.GetString(m, "clinical_themes")
		if themesStr != "" {
			json.Unmarshal([]byte(themesStr), &note.ClinicalThemes)
		}

		notes = append(notes, note)
	}

	// Sort by created_at ASC
	for i := 0; i < len(notes); i++ {
		for j := i + 1; j < len(notes); j++ {
			if notes[j].CreatedAt.Before(notes[i].CreatedAt) {
				notes[i], notes[j] = notes[j], notes[i]
			}
		}
	}

	return notes, nil
}
