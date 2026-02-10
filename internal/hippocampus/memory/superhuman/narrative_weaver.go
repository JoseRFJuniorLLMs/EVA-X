package superhuman

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// NarrativeWeaver reconstructs objective narratives from patient data
// Based on Schacter's "Searching for Memory" - memory as reconstruction
// PRINCIPLE: Weaves data temporally WITHOUT interpretation
type NarrativeWeaver struct {
	db *sql.DB
}

// NewNarrativeWeaver creates a new narrative reconstruction service
func NewNarrativeWeaver(db *sql.DB) *NarrativeWeaver {
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

	// 1. Find correlated persons
	personsQuery := `
		SELECT DISTINCT jsonb_array_elements_text(correlated_persons) as person
		FROM patient_somatic_correlations
		WHERE idoso_id = $1 AND correlated_topic = $2
		UNION
		SELECT DISTINCT jsonb_array_elements_text(correlated_persons) as person
		FROM patient_metaphors
		WHERE idoso_id = $1 AND $2 = ANY(SELECT jsonb_array_elements_text(correlated_topics))
		UNION
		SELECT DISTINCT jsonb_array_elements_text(involved_persons) as person
		FROM patient_persistent_memories
		WHERE idoso_id = $1 AND persistent_topic ILIKE '%' || $2 || '%'
	`
	rows, err := w.db.QueryContext(ctx, personsQuery, idosoID, centralTopic)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var person string
			if err := rows.Scan(&person); err == nil && person != "" {
				thread.ConnectedElements.Persons = append(thread.ConnectedElements.Persons, person)
			}
		}
	}

	// 2. Find correlated places
	placesQuery := `
		SELECT place_name, emotional_valence
		FROM patient_world_places
		WHERE idoso_id = $1
		AND (
			associated_emotions::text ILIKE '%' || $2 || '%'
			OR place_name IN (
				SELECT DISTINCT jsonb_array_elements_text(correlated_places)
				FROM patient_body_memories
				WHERE idoso_id = $1 AND strongest_correlation_topic = $2
			)
		)
	`
	placeRows, err := w.db.QueryContext(ctx, placesQuery, idosoID, centralTopic)
	if err == nil {
		defer placeRows.Close()
		for placeRows.Next() {
			var place string
			var valence sql.NullFloat64
			if err := placeRows.Scan(&place, &valence); err == nil {
				thread.ConnectedElements.Places = append(thread.ConnectedElements.Places, place)
			}
		}
	}

	// 3. Find correlated body symptoms
	symptomsQuery := `
		SELECT physical_symptom, body_location, strongest_correlation_strength
		FROM patient_body_memories
		WHERE idoso_id = $1 AND strongest_correlation_topic = $2
		ORDER BY strongest_correlation_strength DESC
	`
	symptomRows, err := w.db.QueryContext(ctx, symptomsQuery, idosoID, centralTopic)
	if err == nil {
		defer symptomRows.Close()
		for symptomRows.Next() {
			var symptom, location string
			var strength sql.NullFloat64
			if err := symptomRows.Scan(&symptom, &location, &strength); err == nil {
				symptomDesc := symptom
				if location != "" {
					symptomDesc = fmt.Sprintf("%s (%s)", symptom, location)
				}
				thread.ConnectedElements.Symptoms = append(thread.ConnectedElements.Symptoms, symptomDesc)
				if strength.Valid {
					thread.ConnectionStrength = max(thread.ConnectionStrength, strength.Float64)
				}
			}
		}
	}

	// 4. Build timeline from life markers
	markersQuery := `
		SELECT marker_description, marker_year, marker_type, emotional_valence
		FROM patient_life_markers
		WHERE idoso_id = $1
		AND (
			described_impact ILIKE '%' || $2 || '%'
			OR $2 = ANY(SELECT jsonb_array_elements_text(involved_persons))
		)
		ORDER BY marker_year
	`
	markerRows, err := w.db.QueryContext(ctx, markersQuery, idosoID, centralTopic)
	if err == nil {
		defer markerRows.Close()
		for markerRows.Next() {
			var desc, markerType string
			var year sql.NullInt32
			var valence sql.NullFloat64
			if err := markerRows.Scan(&desc, &year, &markerType, &valence); err == nil {
				event := TimelineEvent{
					Event:       desc,
					ElementType: "life_marker",
					ElementName: markerType,
				}
				if year.Valid {
					event.Year = int(year.Int32)
				}
				if valence.Valid {
					event.Valence = valence.Float64
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
	query := `
		SELECT id, marker_description, marker_year, marker_age, marker_type,
		       described_impact, emotional_valence, before_description,
		       after_description, mention_count, involved_persons
		FROM patient_life_markers
		WHERE idoso_id = $1
		ORDER BY marker_year NULLS LAST, mention_count DESC
	`

	rows, err := w.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var markers []*LifeMarker
	for rows.Next() {
		m := &LifeMarker{IdosoID: idosoID}
		var year, age sql.NullInt32
		var impact, before, after sql.NullString
		var valence sql.NullFloat64
		var personsJSON []byte

		err := rows.Scan(
			&m.ID, &m.Description, &year, &age, &m.MarkerType,
			&impact, &valence, &before, &after, &m.MentionCount, &personsJSON,
		)
		if err != nil {
			continue
		}

		if year.Valid {
			m.Year = int(year.Int32)
		}
		if age.Valid {
			m.Age = int(age.Int32)
		}
		if impact.Valid {
			m.DescribedImpact = impact.String
		}
		if valence.Valid {
			m.EmotionalValence = valence.Float64
		}
		if before.Valid {
			m.BeforeDescription = before.String
		}
		if after.Valid {
			m.AfterDescription = after.String
		}
		if len(personsJSON) > 0 {
			json.Unmarshal(personsJSON, &m.InvolvedPersons)
		}

		markers = append(markers, m)
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
