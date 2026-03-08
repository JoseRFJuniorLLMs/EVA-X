// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// NarrativeWeaver reconstructs objective narratives from patient data
// Based on Schacter's "Searching for Memory" - memory as reconstruction
// PRINCIPLE: Weaves data temporally WITHOUT interpretation
type NarrativeWeaver struct {
	db *database.DB
}

// NewNarrativeWeaver creates a new narrative reconstruction service
func NewNarrativeWeaver(db *database.DB) *NarrativeWeaver {
	return &NarrativeWeaver{db: db}
}

// NarrativeThread represents a connected story from patient's data
type NarrativeThread struct {
	ID                  int64             `json:"id"`
	IdosoID             int64             `json:"idoso_id"`
	ThreadName          string            `json:"thread_name"`
	ConnectedElements   ConnectedElements `json:"connected_elements"`
	Timeline            []TimelineEvent   `json:"timeline"`
	ConnectionStrength  float64           `json:"connection_strength"`
	EvidenceCount       int               `json:"evidence_count"`
	NarrativeSummary    string            `json:"narrative_summary"`
	ConnectionType      string            `json:"connection_type"`
	GeneratedQuestions  []string          `json:"generated_questions"`
}

// ConnectedElements holds elements that form a narrative thread
type ConnectedElements struct {
	Persons  []string `json:"persons"`
	Topics   []string `json:"topics"`
	Places   []string `json:"places"`
	Symptoms []string `json:"symptoms"`
	Emotions []string `json:"emotions"`
}

// TimelineEvent represents an event in the narrative timeline
type TimelineEvent struct {
	Date        time.Time `json:"date"`
	Year        int       `json:"year,omitempty"`
	Event       string    `json:"event"`
	ElementType string    `json:"element_type"`
	ElementName string    `json:"element_name"`
	Valence     float64   `json:"valence,omitempty"`
}

// LifeMarker represents a significant life event
type LifeMarker struct {
	ID                int64     `json:"id"`
	IdosoID           int64     `json:"idoso_id"`
	Description       string    `json:"description"`
	Year              int       `json:"year"`
	Age               int       `json:"age,omitempty"`
	MarkerType        string    `json:"marker_type"`
	DescribedImpact   string    `json:"described_impact"`
	EmotionalValence  float64   `json:"emotional_valence"`
	BeforeDescription string    `json:"before_description,omitempty"`
	AfterDescription  string    `json:"after_description,omitempty"`
	MentionCount      int       `json:"mention_count"`
	InvolvedPersons   []string  `json:"involved_persons"`
}

// WeaveNarrative constructs a narrative thread connecting multiple data points
func (w *NarrativeWeaver) WeaveNarrative(ctx context.Context, idosoID int64, centralTopic string) (*NarrativeThread, error) {
	thread := &NarrativeThread{
		IdosoID:    idosoID,
		ThreadName: centralTopic,
		ConnectedElements: ConnectedElements{
			Persons:  []string{},
			Topics:   []string{centralTopic},
			Places:   []string{},
			Symptoms: []string{},
			Emotions: []string{},
		},
		Timeline:           []TimelineEvent{},
		GeneratedQuestions: []string{},
	}

	// 1. Find correlated persons from somatic correlations
	somaticRows, err := w.db.QueryByLabel(ctx, "patient_somatic_correlations",
		" AND n.idoso_id = $idoso AND n.correlated_topic = $topic",
		map[string]interface{}{"idoso": idosoID, "topic": centralTopic}, 0)
	if err == nil {
		for _, m := range somaticRows {
			if raw, ok := m["correlated_persons"]; ok && raw != nil {
				var persons []string
				parseJSONStringSlice(raw, &persons)
				for _, p := range persons {
					if p != "" {
						thread.ConnectedElements.Persons = append(thread.ConnectedElements.Persons, p)
					}
				}
			}
		}
	}

	// Also find persons from persistent memories
	persistentRows, err := w.db.QueryByLabel(ctx, "patient_persistent_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		log.Printf("[NarrativeWeaver] QueryByLabel patient_persistent_memories failed: %v", err)
		return nil, err
	}
	for _, m := range persistentRows {
		persistentTopic := strings.ToLower(database.GetString(m, "persistent_topic"))
		if strings.Contains(persistentTopic, strings.ToLower(centralTopic)) {
			if raw, ok := m["involved_persons"]; ok && raw != nil {
				var persons []string
				parseJSONStringSlice(raw, &persons)
				for _, p := range persons {
					if p != "" {
						thread.ConnectedElements.Persons = append(thread.ConnectedElements.Persons, p)
					}
				}
			}
		}
	}

	// 2. Find correlated places
	placeRows, err := w.db.QueryByLabel(ctx, "patient_world_places",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil {
		for _, m := range placeRows {
			// Check if place is associated with topic
			content, err := json.Marshal(m)
			if err != nil {
				log.Printf("[NarrativeWeaver] json.Marshal place row failed: %v", err)
				continue
			}
			if strings.Contains(strings.ToLower(string(content)), strings.ToLower(centralTopic)) {
				place := database.GetString(m, "place_name")
				if place != "" {
					thread.ConnectedElements.Places = append(thread.ConnectedElements.Places, place)
				}
			}
		}
	}

	// 3. Find correlated body symptoms
	bodyRows, err := w.db.QueryByLabel(ctx, "patient_body_memories",
		" AND n.idoso_id = $idoso AND n.strongest_correlation_topic = $topic",
		map[string]interface{}{"idoso": idosoID, "topic": centralTopic}, 0)
	if err == nil {
		for _, m := range bodyRows {
			symptom := database.GetString(m, "physical_symptom")
			location := database.GetString(m, "body_location")
			strength := database.GetFloat64(m, "strongest_correlation_strength")

			symptomDesc := symptom
			if location != "" {
				symptomDesc = fmt.Sprintf("%s (%s)", symptom, location)
			}
			thread.ConnectedElements.Symptoms = append(thread.ConnectedElements.Symptoms, symptomDesc)
			if strength > thread.ConnectionStrength {
				thread.ConnectionStrength = strength
			}
		}
	}

	// 4. Build timeline from life markers
	markerRows, err := w.db.QueryByLabel(ctx, "patient_life_markers",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err == nil {
		for _, m := range markerRows {
			// Check if marker is related to topic
			impact := strings.ToLower(database.GetString(m, "described_impact"))
			content, err := json.Marshal(m)
			if err != nil {
				log.Printf("[NarrativeWeaver] json.Marshal marker row failed: %v", err)
				continue
			}
			isRelated := strings.Contains(impact, strings.ToLower(centralTopic)) ||
				strings.Contains(strings.ToLower(string(content)), strings.ToLower(centralTopic))

			if isRelated {
				desc := database.GetString(m, "marker_description")
				markerType := database.GetString(m, "marker_type")
				year := int(database.GetInt64(m, "marker_year"))
				valence := database.GetFloat64(m, "emotional_valence")

				event := TimelineEvent{
					Event:       desc,
					ElementType: "life_marker",
					ElementName: markerType,
					Year:        year,
					Valence:     valence,
				}
				thread.Timeline = append(thread.Timeline, event)
			}
		}
	}

	// 5. Generate narrative summary (objective, no interpretation)
	thread.NarrativeSummary = w.generateNarrativeSummary(thread, centralTopic)

	// 6. Generate questions
	thread.GeneratedQuestions = w.generateNarrativeQuestions(thread, centralTopic)

	// 7. Calculate evidence count
	thread.EvidenceCount = len(thread.ConnectedElements.Persons) +
		len(thread.ConnectedElements.Places) +
		len(thread.ConnectedElements.Symptoms) +
		len(thread.Timeline)

	// 8. Determine connection type
	thread.ConnectionType = w.determineConnectionType(thread)

	return thread, nil
}

// generateNarrativeSummary creates an objective narrative text
func (w *NarrativeWeaver) generateNarrativeSummary(thread *NarrativeThread, topic string) string {
	var parts []string

	// Topic introduction
	parts = append(parts, fmt.Sprintf("Sobre '%s':", topic))

	// Persons
	if len(thread.ConnectedElements.Persons) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Aparece conectado a: %s.",
			strings.Join(thread.ConnectedElements.Persons, ", ")))
	}

	// Timeline
	if len(thread.Timeline) > 0 {
		var timelineDescs []string
		for _, event := range thread.Timeline {
			if event.Year > 0 {
				timelineDescs = append(timelineDescs, fmt.Sprintf("%d: %s", event.Year, event.Event))
			} else {
				timelineDescs = append(timelineDescs, event.Event)
			}
		}
		parts = append(parts, fmt.Sprintf("Linha do tempo: %s.", strings.Join(timelineDescs, "; ")))
	}

	// Places
	if len(thread.ConnectedElements.Places) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Lugares associados: %s.",
			strings.Join(thread.ConnectedElements.Places, ", ")))
	}

	// Body symptoms
	if len(thread.ConnectedElements.Symptoms) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Seu corpo reage: %s.",
			strings.Join(thread.ConnectedElements.Symptoms, ", ")))
	}

	return strings.Join(parts, "\n")
}

// generateNarrativeQuestions creates questions for patient reflection
func (w *NarrativeWeaver) generateNarrativeQuestions(thread *NarrativeThread, topic string) []string {
	questions := []string{}

	// Person connection question
	if len(thread.ConnectedElements.Persons) > 0 {
		questions = append(questions, fmt.Sprintf(
			"Voce percebe a conexao entre '%s' e %s?",
			topic, thread.ConnectedElements.Persons[0]))
	}

	// Body connection question
	if len(thread.ConnectedElements.Symptoms) > 0 {
		questions = append(questions, fmt.Sprintf(
			"O que voce acha que conecta '%s' e o sintoma '%s'?",
			topic, thread.ConnectedElements.Symptoms[0]))
	}

	// Timeline question
	if len(thread.Timeline) > 1 {
		questions = append(questions,
			"Olhando essa linha do tempo, o que voce percebe?")
	}

	// General connection question
	if thread.EvidenceCount > 3 {
		questions = append(questions,
			"Voce havia percebido todas essas conexoes antes?")
	}

	return questions
}

// determineConnectionType classifies the type of narrative connection
func (w *NarrativeWeaver) determineConnectionType(thread *NarrativeThread) string {
	if len(thread.ConnectedElements.Symptoms) > 0 {
		return "body_mind"
	}
	if len(thread.Timeline) > 2 {
		return "causal_sequence"
	}
	if len(thread.ConnectedElements.Persons) > 0 && len(thread.ConnectedElements.Places) > 0 {
		return "place_identity"
	}
	if len(thread.ConnectedElements.Persons) > 1 {
		return "person_topic"
	}
	return "emotional_cluster"
}

// GetLifeMarkers retrieves significant life markers
func (w *NarrativeWeaver) GetLifeMarkers(ctx context.Context, idosoID int64) ([]*LifeMarker, error) {
	rows, err := w.db.QueryByLabel(ctx, "patient_life_markers",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var markers []*LifeMarker
	for _, m := range rows {
		marker := &LifeMarker{
			ID:                database.GetInt64(m, "id"),
			IdosoID:           idosoID,
			Description:       database.GetString(m, "marker_description"),
			Year:              int(database.GetInt64(m, "marker_year")),
			Age:               int(database.GetInt64(m, "marker_age")),
			MarkerType:        database.GetString(m, "marker_type"),
			DescribedImpact:   database.GetString(m, "described_impact"),
			EmotionalValence:  database.GetFloat64(m, "emotional_valence"),
			BeforeDescription: database.GetString(m, "before_description"),
			AfterDescription:  database.GetString(m, "after_description"),
			MentionCount:      int(database.GetInt64(m, "mention_count")),
		}

		if raw, ok := m["involved_persons"]; ok && raw != nil {
			parseJSONStringSlice(raw, &marker.InvolvedPersons)
		}

		markers = append(markers, marker)
	}

	return markers, nil
}

// BuildLifeNarrative constructs a complete life narrative from markers
func (w *NarrativeWeaver) BuildLifeNarrative(ctx context.Context, idosoID int64) (*MirrorOutput, error) {
	markers, err := w.GetLifeMarkers(ctx, idosoID)
	if err != nil || len(markers) == 0 {
		return nil, err
	}

	// Sort by year
	sort.Slice(markers, func(i, j int) bool {
		return markers[i].Year < markers[j].Year
	})

	dataPoints := []string{}

	// Build narrative from markers
	for _, m := range markers {
		var desc string
		if m.Year > 0 {
			desc = fmt.Sprintf("%d", m.Year)
			if m.Age > 0 {
				desc += fmt.Sprintf(" (aos %d anos)", m.Age)
			}
			desc += ": " + m.Description
		} else {
			desc = m.Description
		}

		if m.DescribedImpact != "" {
			desc += fmt.Sprintf(" - Voce disse: \"%s\"", m.DescribedImpact)
		}

		dataPoints = append(dataPoints, desc)
	}

	// Find patterns
	var positiveCount, negativeCount int
	for _, m := range markers {
		if m.EmotionalValence > 0.3 {
			positiveCount++
		} else if m.EmotionalValence < -0.3 {
			negativeCount++
		}
	}

	if positiveCount > 0 || negativeCount > 0 {
		dataPoints = append(dataPoints, fmt.Sprintf(
			"Dos %d marcos, %d sao lembrados positivamente, %d negativamente.",
			len(markers), positiveCount, negativeCount))
	}

	return &MirrorOutput{
		Type:       "life_narrative",
		DataPoints: dataPoints,
		Question:   "Olhando para essa linha da sua vida, o que voce percebe?",
		RawData: map[string]interface{}{
			"markers":        markers,
			"total_markers":  len(markers),
			"positive_count": positiveCount,
			"negative_count": negativeCount,
		},
	}, nil
}

// GenerateNarrativeMirror creates a mirror output from a narrative thread
func (w *NarrativeWeaver) GenerateNarrativeMirror(thread *NarrativeThread) *MirrorOutput {
	if thread == nil || thread.EvidenceCount < 2 {
		return nil
	}

	dataPoints := strings.Split(thread.NarrativeSummary, "\n")

	// Add connection strength
	if thread.ConnectionStrength > 0 {
		dataPoints = append(dataPoints, fmt.Sprintf(
			"Forca da conexao: %.0f%%", thread.ConnectionStrength*100))
	}

	question := "O que voce acha que conecta todas essas coisas?"
	if len(thread.GeneratedQuestions) > 0 {
		question = thread.GeneratedQuestions[0]
	}

	return &MirrorOutput{
		Type:       "narrative_thread",
		DataPoints: dataPoints,
		Frequency:  &thread.EvidenceCount,
		Question:   question,
		RawData: map[string]interface{}{
			"thread_name":        thread.ThreadName,
			"connected_elements": thread.ConnectedElements,
			"timeline":           thread.Timeline,
			"connection_type":    thread.ConnectionType,
		},
	}
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
