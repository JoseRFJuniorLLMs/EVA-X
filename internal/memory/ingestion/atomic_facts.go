// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package ingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"eva/internal/brainstem/database"

	"github.com/google/generative-ai-go/genai"
	"github.com/rs/zerolog/log"
	"google.golang.org/api/option"
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
	db     *database.DB
	apiKey string
	model  string
}

// NewFactExtractor creates a new fact extractor
func NewFactExtractor(db *database.DB) *FactExtractor {
	return &FactExtractor{db: db}
}

// NewFactExtractorWithLLM creates a fact extractor with LLM support
func NewFactExtractorWithLLM(db *database.DB, apiKey, model string) *FactExtractor {
	return &FactExtractor{db: db, apiKey: apiKey, model: model}
}

// ExtractFacts extracts atomic facts from text using Gemini LLM
func (f *FactExtractor) ExtractFacts(ctx context.Context, text string, patientID int64) ([]*AtomicFact, error) {
	if f.apiKey == "" || strings.TrimSpace(text) == "" {
		// Fallback: return raw text as single fact
		return []*AtomicFact{{
			Content:       text,
			Confidence:    0.8,
			Source:        "user_stated",
			Revisable:     true,
			Version:       1,
			PatientID:     patientID,
			EventTime:     time.Now(),
			IngestionTime: time.Now(),
		}}, nil
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(f.apiKey))
	if err != nil {
		log.Warn().Err(err).Msg("[FACTS] Gemini client failed, falling back to raw text")
		return []*AtomicFact{{Content: text, Confidence: 0.5, Source: "user_stated", Revisable: true, Version: 1, PatientID: patientID, EventTime: time.Now(), IngestionTime: time.Now()}}, nil
	}
	defer client.Close()

	model := client.GenerativeModel(f.model)
	model.SetTemperature(0.1)

	prompt := fmt.Sprintf(`Extraia fatos atômicos (informações indivisíveis) do texto abaixo.
Retorne um JSON array com objetos contendo:
- "content": o fato atômico em uma frase clara
- "confidence": confiança de 0.0 a 1.0
- "source": "user_stated" se o paciente disse, "inferred" se foi inferido, "observed" se foi observado

Texto: "%s"

Responda APENAS o JSON array, sem markdown.`, text)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Warn().Err(err).Msg("[FACTS] Gemini extraction failed, falling back to raw text")
		return []*AtomicFact{{Content: text, Confidence: 0.5, Source: "user_stated", Revisable: true, Version: 1, PatientID: patientID, EventTime: time.Now(), IngestionTime: time.Now()}}, nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return []*AtomicFact{{Content: text, Confidence: 0.5, Source: "user_stated", Revisable: true, Version: 1, PatientID: patientID, EventTime: time.Now(), IngestionTime: time.Now()}}, nil
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var extracted []struct {
		Content    string  `json:"content"`
		Confidence float64 `json:"confidence"`
		Source     string  `json:"source"`
	}

	if err := json.Unmarshal([]byte(responseText), &extracted); err != nil {
		log.Warn().Err(err).Str("response", responseText).Msg("[FACTS] Failed to parse LLM response")
		return []*AtomicFact{{Content: text, Confidence: 0.5, Source: "user_stated", Revisable: true, Version: 1, PatientID: patientID, EventTime: time.Now(), IngestionTime: time.Now()}}, nil
	}

	facts := make([]*AtomicFact, 0, len(extracted))
	now := time.Now()
	for _, e := range extracted {
		source := e.Source
		if source == "" {
			source = "user_stated"
		}
		facts = append(facts, &AtomicFact{
			Content:       e.Content,
			Confidence:    e.Confidence,
			Source:        source,
			Revisable:     true,
			Version:       1,
			PatientID:     patientID,
			EventTime:     now,
			IngestionTime: now,
		})
	}

	if len(facts) == 0 {
		facts = append(facts, &AtomicFact{Content: text, Confidence: 0.5, Source: "user_stated", Revisable: true, Version: 1, PatientID: patientID, EventTime: now, IngestionTime: now})
	}

	log.Info().Int("facts_extracted", len(facts)).Int64("patient_id", patientID).Msg("[FACTS] Extracted atomic facts via Gemini")
	return facts, nil
}

// StoreFacts stores atomic facts in database via NietzscheDB
func (f *FactExtractor) StoreFacts(ctx context.Context, facts []*AtomicFact) error {
	now := time.Now()
	for _, fact := range facts {
		content := map[string]interface{}{
			"content":        fact.Content,
			"confidence":     fact.Confidence,
			"source":         fact.Source,
			"revisable":      fact.Revisable,
			"version":        fact.Version,
			"patient_id":     fact.PatientID,
			"event_time":     fact.EventTime.Format(time.RFC3339),
			"ingestion_time": fact.IngestionTime.Format(time.RFC3339),
			"created_at":     now.Format(time.RFC3339),
			"updated_at":     now.Format(time.RFC3339),
		}

		id, err := f.db.Insert(ctx, "atomic_facts", content)
		if err != nil {
			return err
		}
		fact.ID = id

		log.Info().
			Int64("fact_id", fact.ID).
			Float64("confidence", fact.Confidence).
			Str("source", fact.Source).
			Msg("Stored atomic fact")
	}

	return nil
}

// GetFacts retrieves facts for a patient via NietzscheDB
func (f *FactExtractor) GetFacts(ctx context.Context, patientID int64, limit int) ([]*AtomicFact, error) {
	rows, err := f.db.QueryByLabel(ctx, "atomic_facts",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID},
		limit,
	)
	if err != nil {
		return nil, err
	}

	var facts []*AtomicFact
	for _, row := range rows {
		fact := &AtomicFact{
			ID:            database.GetInt64(row, "pg_id"),
			Content:       database.GetString(row, "content"),
			Confidence:    database.GetFloat64(row, "confidence"),
			Source:        database.GetString(row, "source"),
			Revisable:     database.GetBool(row, "revisable"),
			Version:       int(database.GetInt64(row, "version")),
			PatientID:     database.GetInt64(row, "patient_id"),
			EventTime:     database.GetTime(row, "event_time"),
			IngestionTime: database.GetTime(row, "ingestion_time"),
			CreatedAt:     database.GetTime(row, "created_at"),
			UpdatedAt:     database.GetTime(row, "updated_at"),
		}
		if fact.ID == 0 {
			fact.ID = database.GetInt64(row, "id")
		}
		facts = append(facts, fact)
	}

	return facts, nil
}

// FactContradiction represents a contradiction between facts
type FactContradiction struct {
	Fact1ID    int64
	Fact2ID    int64
	Confidence float64 // how confident we are in the contradiction
	DetectedAt time.Time
}

// DetectContradictions finds contradictory facts using Gemini LLM
func (f *FactExtractor) DetectContradictions(ctx context.Context, patientID int64) ([]*FactContradiction, error) {
	// Get recent facts for this patient
	facts, err := f.GetFacts(ctx, patientID, 50)
	if err != nil {
		return nil, err
	}
	if len(facts) < 2 || f.apiKey == "" {
		return []*FactContradiction{}, nil
	}

	// Build facts list for LLM
	var factsText strings.Builder
	for _, fact := range facts {
		factsText.WriteString(fmt.Sprintf("ID=%d: %s\n", fact.ID, fact.Content))
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(f.apiKey))
	if err != nil {
		log.Warn().Err(err).Msg("[FACTS] Gemini client failed for contradiction detection")
		return []*FactContradiction{}, nil
	}
	defer client.Close()

	model := client.GenerativeModel(f.model)
	model.SetTemperature(0.1)

	prompt := fmt.Sprintf(`Analise os fatos abaixo e identifique contradições semânticas.
Retorne um JSON array com objetos contendo:
- "fact1_id": ID do primeiro fato
- "fact2_id": ID do segundo fato contraditório
- "confidence": confiança na contradição (0.0 a 1.0)

Se não houver contradições, retorne [].

Fatos:
%s

Responda APENAS o JSON array, sem markdown.`, factsText.String())

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		log.Warn().Err(err).Msg("[FACTS] Contradiction detection failed")
		return []*FactContradiction{}, nil
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return []*FactContradiction{}, nil
	}

	responseText := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])
	responseText = strings.TrimSpace(responseText)
	responseText = strings.TrimPrefix(responseText, "```json")
	responseText = strings.TrimPrefix(responseText, "```")
	responseText = strings.TrimSuffix(responseText, "```")
	responseText = strings.TrimSpace(responseText)

	var detected []struct {
		Fact1ID    int64   `json:"fact1_id"`
		Fact2ID    int64   `json:"fact2_id"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(responseText), &detected); err != nil {
		log.Warn().Err(err).Str("response", responseText).Msg("[FACTS] Failed to parse contradiction response")
		return []*FactContradiction{}, nil
	}

	contradictions := make([]*FactContradiction, 0, len(detected))
	now := time.Now()
	for _, d := range detected {
		contradictions = append(contradictions, &FactContradiction{
			Fact1ID:    d.Fact1ID,
			Fact2ID:    d.Fact2ID,
			Confidence: d.Confidence,
			DetectedAt: now,
		})
	}

	log.Info().Int("contradictions", len(contradictions)).Int64("patient_id", patientID).Msg("[FACTS] Contradiction detection complete")
	return contradictions, nil
}

// ReviseFact creates a new version of a fact via NietzscheDB
func (f *FactExtractor) ReviseFact(ctx context.Context, factID int64, newContent string, confidence float64) (*AtomicFact, error) {
	// Get current fact from NietzscheDB
	node, err := f.db.GetNodeByID(ctx, "atomic_facts", factID)
	if err != nil {
		return nil, err
	}
	if node == nil {
		return nil, fmt.Errorf("fact %d not found", factID)
	}

	currentVersion := int(database.GetInt64(node, "version"))
	currentPatientID := database.GetInt64(node, "patient_id")
	currentEventTime := database.GetTime(node, "event_time")

	// Create new version
	now := time.Now()
	newFact := &AtomicFact{
		Content:       newContent,
		Confidence:    confidence,
		Source:        "revised",
		Revisable:     true,
		Version:       currentVersion + 1,
		PatientID:     currentPatientID,
		EventTime:     currentEventTime, // Keep original event time
		IngestionTime: now,              // New ingestion time
	}

	content := map[string]interface{}{
		"content":             newFact.Content,
		"confidence":          newFact.Confidence,
		"source":              newFact.Source,
		"revisable":           newFact.Revisable,
		"version":             newFact.Version,
		"patient_id":          newFact.PatientID,
		"event_time":          newFact.EventTime.Format(time.RFC3339),
		"ingestion_time":      newFact.IngestionTime.Format(time.RFC3339),
		"created_at":          now.Format(time.RFC3339),
		"updated_at":          now.Format(time.RFC3339),
		"previous_version_id": factID,
	}

	newID, err := f.db.Insert(ctx, "atomic_facts", content)
	if err != nil {
		return nil, err
	}
	newFact.ID = newID

	log.Info().
		Int64("old_fact_id", factID).
		Int64("new_fact_id", newFact.ID).
		Int("new_version", newFact.Version).
		Msg("Revised fact")

	return newFact, nil
}

// FactStats returns statistics about facts via NietzscheDB
func (f *FactExtractor) FactStats(ctx context.Context, patientID int64) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total facts
	total, err := f.db.Count(ctx, "atomic_facts",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID},
	)
	if err != nil {
		return nil, err
	}
	stats["total_facts"] = total

	// By source + average confidence: fetch all rows and compute in Go
	rows, err := f.db.QueryByLabel(ctx, "atomic_facts",
		" AND n.patient_id = $patient_id",
		map[string]interface{}{"patient_id": patientID},
		0, // no limit
	)
	if err != nil {
		return nil, err
	}

	bySource := make(map[string]int)
	var totalConfidence float64
	for _, row := range rows {
		source := database.GetString(row, "source")
		if source != "" {
			bySource[source]++
		}
		totalConfidence += database.GetFloat64(row, "confidence")
	}
	stats["by_source"] = bySource

	var avgConfidence float64
	if len(rows) > 0 {
		avgConfidence = totalConfidence / float64(len(rows))
	}
	stats["avg_confidence"] = avgConfidence

	return stats, nil
}
