// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// DeepMemoryService handles the deep memory extensions
// Based on: Schacter (Persistence), Van der Kolk (Body), Casey (Place, Commemoration)
type DeepMemoryService struct {
	db            *database.DB
	vectorAdapter *nietzscheInfra.VectorAdapter
	weaver        *NarrativeWeaver

	// Detection patterns
	avoidancePatterns   []*regexp.Regexp
	sharingPatterns     []*regexp.Regexp
	bodySymptomPatterns []*regexp.Regexp
	placePatterns       []*regexp.Regexp
	sensoryPatterns     []*regexp.Regexp
}

// SetVectorAdapter injects NietzscheDB adapter for PG elimination (optional).
func (s *DeepMemoryService) SetVectorAdapter(va *nietzscheInfra.VectorAdapter) {
	s.vectorAdapter = va
}

// NewDeepMemoryService creates a new deep memory service
func NewDeepMemoryService(db *database.DB) *DeepMemoryService {
	svc := &DeepMemoryService{
		db:     db,
		weaver: NewNarrativeWeaver(db),
	}
	svc.compilePatterns()
	return svc
}

// compilePatterns prepares regex patterns
func (s *DeepMemoryService) compilePatterns() {
	// Avoidance patterns
	avoidance := []string{
		`(?i)nao\s+quero\s+falar`,
		`(?i)deixa\s+(?:isso\s+)?pra\s+la`,
		`(?i)(?:vamos|pode)\s+mudar\s+de\s+assunto`,
		`(?i)nao\s+(?:vamos|precisa)\s+falar\s+(?:disso|nisso)`,
		`(?i)esquece\s+(?:isso)?`,
		`(?i)(?:isso\s+)?ja\s+passou`,
		`(?i)nao\s+adianta\s+falar`,
	}

	// Sharing desire patterns
	sharing := []string{
		`(?i)queria\s+(?:que\s+)?(?:meus?\s+)?(\w+)\s+(?:soubesse|entendesse|conhecesse)`,
		`(?i)(?:preciso|tenho\s+que)\s+contar\s+(?:para|pra)\s+(\w+)`,
		`(?i)(?:eles|ela|ele)\s+(?:precisa|precisam)\s+saber`,
		`(?i)antes\s+(?:de\s+)?(?:eu\s+)?(?:morrer|ir|partir)`,
		`(?i)quero\s+(?:deixar|passar)\s+(?:isso\s+)?(?:para|pra)`,
		`(?i)(?:minha|essa)\s+historia`,
	}

	// Body symptom patterns
	bodySymptom := []string{
		`(?i)(?:sinto|tenho)\s+(?:um\s+)?(?:aperto|peso|dor|no|pontada)\s+(?:no|na|em)\s+(\w+)`,
		`(?i)(?:meu|minha)\s+(\w+)\s+(?:doi|aperta|treme|pesa)`,
		`(?i)(?:da|deu)\s+(?:uma?\s+)?(?:dor|pontada|aperto)\s+(?:no|na)\s+(\w+)`,
		`(?i)(?:comeca|comecou)\s+a\s+(?:doer|apertar|tremer)`,
		`(?i)(?:nao\s+)?(?:consigo|consegui)\s+(?:respirar|engolir)`,
		`(?i)(?:falta|faltou)\s+(?:o\s+)?ar`,
	}

	// Place patterns
	place := []string{
		`(?i)(?:la\s+(?:na|no|em)|na\s+(?:minha|nossa))\s+(\w+)`,
		`(?i)(?:quando\s+)?(?:eu\s+)?morava\s+(?:em|no|na)\s+(\w+)`,
		`(?i)(?:a|o)\s+(?:casa|sitio|fazenda|apartamento)\s+(?:do|da|em)\s+(\w+)`,
		`(?i)(?:minha|nossa)\s+(?:terra|cidade|rua)`,
		`(?i)(?:voltei|voltar)\s+(?:para|pra)\s+(\w+)`,
	}

	// Sensory memory patterns
	sensory := []string{
		`(?i)(?:o\s+)?cheiro\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:o\s+)?(?:som|barulho)\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:o\s+)?gosto\s+(?:de|do|da)\s+(\w+)`,
		`(?i)(?:lembro|lembrava)\s+(?:do|da)\s+(\w+)`,
		`(?i)(?:parece|parecia)\s+que\s+(?:eu\s+)?(?:sinto|sentia)\s+(\w+)`,
		`(?i)(?:ainda\s+)?(?:sinto|senti)\s+(?:o|a)\s+(\w+)`,
	}

	s.avoidancePatterns = compilePatterns(avoidance)
	s.sharingPatterns = compilePatterns(sharing)
	s.bodySymptomPatterns = compilePatterns(bodySymptom)
	s.placePatterns = compilePatterns(place)
	s.sensoryPatterns = compilePatterns(sensory)
}

// =====================================================
// PERSISTENT MEMORY (Traumatic Persistence)
// =====================================================

// PersistentMemory represents a memory that persists despite avoidance
type PersistentMemory struct {
	ID                   int64     `json:"id"`
	IdosoID              int64     `json:"idoso_id"`
	PersistentTopic      string    `json:"persistent_topic"`
	AvoidanceAttempts    int       `json:"avoidance_attempts"`
	ReturnCount          int       `json:"return_count"`
	PersistenceScore     float64   `json:"persistence_score"`
	AvoidanceScore       float64   `json:"avoidance_score"`
	VoiceTremorPct       float64   `json:"voice_tremor_percentage"`
	InvolvedPersons      []string  `json:"involved_persons"`
	TypicalTriggers      []string  `json:"typical_triggers"`
	FirstDetected        time.Time `json:"first_detected"`
	LastOccurrence       time.Time `json:"last_occurrence"`
}

// DetectAvoidance checks if patient is trying to avoid a topic
func (s *DeepMemoryService) DetectAvoidance(ctx context.Context, idosoID int64, text string, currentTopic string, timestamp time.Time) error {
	for _, pattern := range s.avoidancePatterns {
		if pattern.MatchString(text) {
			ts := timestamp.Format(time.RFC3339)

			// NietzscheDB first: MergeNode on "deep_memory" collection
			if s.vectorAdapter != nil {
				_, _, mergeErr := s.vectorAdapter.MergeNode(ctx, "deep_memory", "PersistentMemory",
					map[string]interface{}{
						"idoso_id":         idosoID,
						"persistent_topic": currentTopic,
					},
					map[string]interface{}{
						"idoso_id":           idosoID,
						"persistent_topic":   currentTopic,
						"avoidance_attempts": 1,
						"first_detected":     ts,
						"last_occurrence":    ts,
					})
				if mergeErr != nil {
					log.Printf("[PERSISTENCE] NietzscheDB merge failed: %v", mergeErr)
				}
			}

			// NietzscheDB via db layer
			rows, err := s.db.QueryByLabel(ctx, "patient_persistent_memories",
				" AND n.idoso_id = $idoso AND n.persistent_topic = $topic",
				map[string]interface{}{"idoso": idosoID, "topic": currentTopic}, 1)
			if err != nil {
				log.Printf("[deep_memory] QueryByLabel failed: %v", err)
				return err
			}

			if len(rows) > 0 {
				m := rows[0]
				if err := s.db.Update(ctx, "patient_persistent_memories",
					map[string]interface{}{"idoso_id": idosoID, "persistent_topic": currentTopic},
					map[string]interface{}{
						"avoidance_attempts": int(database.GetInt64(m, "avoidance_attempts")) + 1,
						"last_occurrence":    ts,
						"updated_at":         ts,
					}); err != nil {
					log.Printf("[deep_memory] update persistent_memories failed: %v", err)
					return err
				}
			} else {
				if _, err := s.db.Insert(ctx, "patient_persistent_memories", map[string]interface{}{
					"idoso_id":           idosoID,
					"persistent_topic":   currentTopic,
					"avoidance_attempts": 1,
					"first_detected":     ts,
					"last_occurrence":    ts,
					"created_at":         ts,
					"updated_at":         ts,
				}); err != nil {
					log.Printf("[deep_memory] insert persistent_memories failed: %v", err)
					return err
				}
			}

			// Record occurrence
			if _, err := s.db.Insert(ctx, "persistent_memory_occurrences", map[string]interface{}{
				"idoso_id":        idosoID,
				"occurrence_type": "avoidance",
				"verbatim":        text,
				"occurred_at":     ts,
				"persistent_topic": currentTopic,
			}); err != nil {
				log.Printf("[deep_memory] insert persistent_memory_occurrences failed: %v", err)
				return err
			}

			log.Printf("[PERSISTENCE] Avoidance detected for topic '%s'", currentTopic)
			break
		}
	}

	return nil
}

// DetectReturn checks if patient returned to a previously avoided topic
func (s *DeepMemoryService) DetectReturn(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	rows, err := s.db.QueryByLabel(ctx, "patient_persistent_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return err
	}

	textLower := strings.ToLower(text)
	for _, m := range rows {
		avoidanceAttempts := int(database.GetInt64(m, "avoidance_attempts"))
		if avoidanceAttempts <= 0 {
			continue
		}

		topic := database.GetString(m, "persistent_topic")
		if strings.Contains(textLower, strings.ToLower(topic)) {
			ts := timestamp.Format(time.RFC3339)

			if err := s.db.Update(ctx, "patient_persistent_memories",
				map[string]interface{}{"idoso_id": idosoID, "persistent_topic": topic},
				map[string]interface{}{
					"return_count":   int(database.GetInt64(m, "return_count")) + 1,
					"last_occurrence": ts,
					"updated_at":     ts,
				}); err != nil {
				log.Printf("[deep_memory] update persistent_memories (return) failed: %v", err)
				return err
			}

			log.Printf("[PERSISTENCE] Return detected to topic '%s'", topic)
		}
	}

	return nil
}

// GetPersistentMemories retrieves persistent memories with high scores
func (s *DeepMemoryService) GetPersistentMemories(ctx context.Context, idosoID int64) ([]*PersistentMemory, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_persistent_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var memories []*PersistentMemory
	for _, m := range rows {
		persistenceScore := database.GetFloat64(m, "persistence_score")
		avoidanceScore := database.GetFloat64(m, "avoidance_score")

		if persistenceScore <= 0.3 && avoidanceScore <= 0.3 {
			continue
		}

		pm := &PersistentMemory{
			ID:                database.GetInt64(m, "id"),
			IdosoID:           idosoID,
			PersistentTopic:   database.GetString(m, "persistent_topic"),
			AvoidanceAttempts: int(database.GetInt64(m, "avoidance_attempts")),
			ReturnCount:       int(database.GetInt64(m, "return_count")),
			PersistenceScore:  persistenceScore,
			AvoidanceScore:    avoidanceScore,
			VoiceTremorPct:    database.GetFloat64(m, "voice_tremor_percentage"),
			FirstDetected:     database.GetTime(m, "first_detected"),
			LastOccurrence:    database.GetTime(m, "last_occurrence"),
		}

		if raw, ok := m["involved_persons"]; ok && raw != nil {
			parseJSONStringSlice(raw, &pm.InvolvedPersons)
		}
		if raw, ok := m["typical_triggers"]; ok && raw != nil {
			parseJSONStringSlice(raw, &pm.TypicalTriggers)
		}

		memories = append(memories, pm)
	}

	return memories, nil
}

// =====================================================
// BODY MEMORY (Van der Kolk)
// =====================================================

// BodyMemory represents a physical symptom that is a somatic memory
type BodyMemory struct {
	ID                         int64    `json:"id"`
	IdosoID                    int64    `json:"idoso_id"`
	PhysicalSymptom            string   `json:"physical_symptom"`
	BodyLocation               string   `json:"body_location"`
	PatientDescriptions        []string `json:"patient_descriptions"`
	CorrelatedTopics           []string `json:"correlated_topics"`
	CorrelatedPersons          []string `json:"correlated_persons"`
	StrongestCorrelationTopic  string   `json:"strongest_correlation_topic"`
	StrongestCorrelationStrength float64 `json:"strongest_correlation_strength"`
	OccurrenceCount            int      `json:"occurrence_count"`
	PatientAware               bool     `json:"patient_aware"`
	PatientVerbalization       string   `json:"patient_verbalization,omitempty"`
}

// DetectBodySymptom detects and records body symptoms mentioned
func (s *DeepMemoryService) DetectBodySymptom(ctx context.Context, idosoID int64, text string, precedingTopics []string, timestamp time.Time) error {
	for _, pattern := range s.bodySymptomPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			symptom := match[0]
			location := ""
			if len(match) > 1 {
				location = match[1]
			}

			topicsJSON, err := json.Marshal(precedingTopics)
			if err != nil {
				log.Printf("[deep_memory] json.Marshal precedingTopics failed: %v", err)
				topicsJSON = []byte("[]")
			}
			ts := timestamp.Format(time.RFC3339)

			// NietzscheDB first: MergeNode on "deep_memory" collection
			if s.vectorAdapter != nil {
				_, _, mergeErr := s.vectorAdapter.MergeNode(ctx, "deep_memory", "BodyMemory",
					map[string]interface{}{
						"idoso_id":         idosoID,
						"physical_symptom": symptom,
						"body_location":    location,
					},
					map[string]interface{}{
						"idoso_id":          idosoID,
						"physical_symptom":  symptom,
						"body_location":     location,
						"occurrence_count":  1,
						"first_reported":    ts,
						"last_reported":     ts,
						"correlated_topics": string(topicsJSON),
					})
				if mergeErr != nil {
					log.Printf("[BODY] NietzscheDB merge failed: %v", mergeErr)
				}
			}

			// NietzscheDB via db layer
			rows, err := s.db.QueryByLabel(ctx, "patient_body_memories",
				" AND n.idoso_id = $idoso AND n.physical_symptom = $symptom AND n.body_location = $loc",
				map[string]interface{}{"idoso": idosoID, "symptom": symptom, "loc": location}, 1)
			if err != nil {
				log.Printf("[deep_memory] QueryByLabel failed: %v", err)
				return err
			}

			if len(rows) > 0 {
				m := rows[0]
				if err := s.db.Update(ctx, "patient_body_memories",
					map[string]interface{}{"idoso_id": idosoID, "physical_symptom": symptom, "body_location": location},
					map[string]interface{}{
						"occurrence_count": int(database.GetInt64(m, "occurrence_count")) + 1,
						"last_reported":    ts,
						"updated_at":       ts,
					}); err != nil {
					log.Printf("[deep_memory] update body_memories failed: %v", err)
					return err
				}
			} else {
				if _, err := s.db.Insert(ctx, "patient_body_memories", map[string]interface{}{
					"idoso_id":          idosoID,
					"physical_symptom":  symptom,
					"body_location":     location,
					"occurrence_count":  1,
					"first_reported":    ts,
					"last_reported":     ts,
					"correlated_topics": string(topicsJSON),
					"created_at":        ts,
					"updated_at":        ts,
				}); err != nil {
					log.Printf("Error inserting body memory: %v", err)
					continue
				}
			}

			// Record occurrence
			if _, err := s.db.Insert(ctx, "body_memory_occurrences", map[string]interface{}{
				"idoso_id":         idosoID,
				"verbatim":         text,
				"occurred_at":      ts,
				"preceding_topics": string(topicsJSON),
				"physical_symptom": symptom,
			}); err != nil {
				log.Printf("[deep_memory] insert body_memory_occurrences failed: %v", err)
				return err
			}

			log.Printf("[BODY] Symptom detected: '%s' at '%s'", symptom, location)
		}
	}

	return nil
}

// GetBodyMemories retrieves body memories with correlations
func (s *DeepMemoryService) GetBodyMemories(ctx context.Context, idosoID int64) ([]*BodyMemory, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_body_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var memories []*BodyMemory
	for _, m := range rows {
		bm := &BodyMemory{
			ID:                          database.GetInt64(m, "id"),
			IdosoID:                     idosoID,
			PhysicalSymptom:             database.GetString(m, "physical_symptom"),
			BodyLocation:                database.GetString(m, "body_location"),
			StrongestCorrelationTopic:   database.GetString(m, "strongest_correlation_topic"),
			StrongestCorrelationStrength: database.GetFloat64(m, "strongest_correlation_strength"),
			OccurrenceCount:             int(database.GetInt64(m, "occurrence_count")),
			PatientAware:                database.GetBool(m, "patient_aware"),
			PatientVerbalization:        database.GetString(m, "patient_verbalization"),
		}

		if raw, ok := m["patient_descriptions"]; ok && raw != nil {
			parseJSONStringSlice(raw, &bm.PatientDescriptions)
		}
		if raw, ok := m["correlated_topics"]; ok && raw != nil {
			parseJSONStringSlice(raw, &bm.CorrelatedTopics)
		}
		if raw, ok := m["correlated_persons"]; ok && raw != nil {
			parseJSONStringSlice(raw, &bm.CorrelatedPersons)
		}

		memories = append(memories, bm)
	}

	return memories, nil
}

// =====================================================
// SHARED MEMORY (Commemoration)
// =====================================================

// SharedMemory represents a memory the patient wants to share
type SharedMemory struct {
	ID               int64     `json:"id"`
	IdosoID          int64     `json:"idoso_id"`
	MemorySummary    string    `json:"memory_summary"`
	IntendedAudience []string  `json:"intended_audience"`
	SharedWith       []string  `json:"shared_with"`
	SharingStatus    string    `json:"sharing_status"`
	MemoryType       string    `json:"memory_type"`
	UrgencyScore     float64   `json:"urgency_score"`
	AssociatedRitual string    `json:"associated_ritual,omitempty"`
	MentionCount     int       `json:"mention_count"`
	FirstMentioned   time.Time `json:"first_mentioned"`
	LastMentioned    time.Time `json:"last_mentioned"`
}

// DetectSharingDesire detects when patient wants to share a memory
func (s *DeepMemoryService) DetectSharingDesire(ctx context.Context, idosoID int64, text string, timestamp time.Time) error {
	for _, pattern := range s.sharingPatterns {
		matches := pattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			audience := ""
			if len(match) > 1 {
				audience = match[1]
			}

			// Classify memory type
			memoryType := s.classifyMemoryType(text)
			ts := timestamp.Format(time.RFC3339)
			audienceJSON, err := json.Marshal([]string{audience})
			if err != nil {
				log.Printf("[deep_memory] json.Marshal audienceJSON failed: %v", err)
				audienceJSON = []byte("[]")
			}
			verbatimJSON, errV := json.Marshal([]string{text})
			if errV != nil {
				log.Printf("[deep_memory] json.Marshal verbatimJSON failed: %v", errV)
				verbatimJSON = []byte("[]")
			}
			summary := text[:minInt(200, len(text))]

			// Try insert first
			_, err = s.db.Insert(ctx, "patient_shared_memories", map[string]interface{}{
				"idoso_id":          idosoID,
				"memory_summary":    summary,
				"intended_audience": string(audienceJSON),
				"sharing_status":    "wishes_to_share",
				"memory_type":       memoryType,
				"mention_count":     1,
				"first_mentioned":   ts,
				"last_mentioned":    ts,
				"verbatim_mentions": string(verbatimJSON),
				"created_at":        ts,
				"updated_at":        ts,
			})
			if err != nil {
				// Try to update existing
				rows, errQ := s.db.QueryByLabel(ctx, "patient_shared_memories",
					" AND n.idoso_id = $idoso",
					map[string]interface{}{"idoso": idosoID}, 0)
				if errQ != nil {
					log.Printf("[deep_memory] QueryByLabel failed: %v", errQ)
					return errQ
				}

				shortText := text[:minInt(50, len(text))]
				for _, m := range rows {
					existingSummary := strings.ToLower(database.GetString(m, "memory_summary"))
					if strings.Contains(existingSummary, strings.ToLower(shortText)) {
						urgency := database.GetFloat64(m, "urgency_score") + 0.1
						if urgency > 1.0 {
							urgency = 1.0
						}
						if err := s.db.Update(ctx, "patient_shared_memories",
							map[string]interface{}{"id": database.GetInt64(m, "id")},
							map[string]interface{}{
								"mention_count": int(database.GetInt64(m, "mention_count")) + 1,
								"last_mentioned": ts,
								"urgency_score":  urgency,
								"updated_at":     ts,
							}); err != nil {
							log.Printf("[deep_memory] update shared_memories failed: %v", err)
							return err
						}
						break
					}
				}
			}

			log.Printf("[SHARING] Desire to share detected, audience: %s", audience)
		}
	}

	return nil
}

// classifyMemoryType determines what type of memory patient wants to share
func (s *DeepMemoryService) classifyMemoryType(text string) string {
	textLower := strings.ToLower(text)

	if strings.Contains(textLower, "licao") || strings.Contains(textLower, "aprendi") {
		return "life_lesson"
	}
	if strings.Contains(textLower, "familia") || strings.Contains(textLower, "avo") ||
		strings.Contains(textLower, "origem") {
		return "family_history"
	}
	if strings.Contains(textLower, "conheci") || strings.Contains(textLower, "casamos") ||
		strings.Contains(textLower, "amor") {
		return "love_story"
	}
	if strings.Contains(textLower, "consegui") || strings.Contains(textLower, "conquist") {
		return "achievement"
	}
	if strings.Contains(textLower, "cuidado") || strings.Contains(textLower, "nao faca") {
		return "warning"
	}
	if strings.Contains(textLower, "receita") || strings.Contains(textLower, "como faz") {
		return "recipe"
	}
	if strings.Contains(textLower, "tradicao") || strings.Contains(textLower, "sempre") {
		return "tradition"
	}

	return "other"
}

// GetSharedMemories retrieves memories patient wants to share
func (s *DeepMemoryService) GetSharedMemories(ctx context.Context, idosoID int64) ([]*SharedMemory, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_shared_memories",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var memories []*SharedMemory
	for _, m := range rows {
		status := database.GetString(m, "sharing_status")
		if status == "fully_shared" {
			continue
		}

		sm := &SharedMemory{
			ID:             database.GetInt64(m, "id"),
			IdosoID:        idosoID,
			MemorySummary:  database.GetString(m, "memory_summary"),
			SharingStatus:  status,
			MemoryType:     database.GetString(m, "memory_type"),
			UrgencyScore:   database.GetFloat64(m, "urgency_score"),
			AssociatedRitual: database.GetString(m, "associated_ritual"),
			MentionCount:   int(database.GetInt64(m, "mention_count")),
			FirstMentioned: database.GetTime(m, "first_mentioned"),
			LastMentioned:  database.GetTime(m, "last_mentioned"),
		}

		if raw, ok := m["intended_audience"]; ok && raw != nil {
			parseJSONStringSlice(raw, &sm.IntendedAudience)
		}
		if raw, ok := m["shared_with"]; ok && raw != nil {
			parseJSONStringSlice(raw, &sm.SharedWith)
		}

		memories = append(memories, sm)
	}

	return memories, nil
}

// =====================================================
// MIRROR OUTPUTS
// =====================================================

// GeneratePersistentMemoryMirror creates mirror output for persistent memories
func (s *DeepMemoryService) GeneratePersistentMemoryMirror(pm *PersistentMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("O tema '%s' apareceu %d vezes", pm.PersistentTopic, pm.AvoidanceAttempts+pm.ReturnCount),
		fmt.Sprintf("Voce tentou evitar %d vezes", pm.AvoidanceAttempts),
		fmt.Sprintf("Mas voltou ao tema %d vezes", pm.ReturnCount),
	}

	if pm.VoiceTremorPct > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Sua voz apresenta tremor em %.0f%% das vezes que fala disso", pm.VoiceTremorPct*100))
	}

	if len(pm.InvolvedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Pessoas envolvidas: %s", strings.Join(pm.InvolvedPersons, ", ")))
	}

	total := pm.AvoidanceAttempts + pm.ReturnCount
	return &MirrorOutput{
		Type:       "persistent_memory",
		DataPoints: dataPoints,
		Frequency:  &total,
		Question:   "Parece que esse assunto insiste em voltar mesmo quando voce tenta evitar. O que voce acha que ele precisa?",
		RawData: map[string]interface{}{
			"topic":             pm.PersistentTopic,
			"persistence_score": pm.PersistenceScore,
			"avoidance_score":   pm.AvoidanceScore,
			"triggers":          pm.TypicalTriggers,
		},
	}
}

// GenerateBodyMemoryMirror creates mirror output for body memories
func (s *DeepMemoryService) GenerateBodyMemoryMirror(bm *BodyMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce relatou '%s' %d vezes", bm.PhysicalSymptom, bm.OccurrenceCount),
	}

	if bm.BodyLocation != "" {
		dataPoints[0] = fmt.Sprintf("Voce relatou '%s' no/na %s %d vezes",
			bm.PhysicalSymptom, bm.BodyLocation, bm.OccurrenceCount)
	}

	if bm.StrongestCorrelationTopic != "" && bm.StrongestCorrelationStrength > 0.5 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Em %.0f%% das vezes, voce havia falado de '%s' antes",
				bm.StrongestCorrelationStrength*100, bm.StrongestCorrelationTopic))
	}

	if len(bm.CorrelatedPersons) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Pessoas associadas: %s", strings.Join(bm.CorrelatedPersons, ", ")))
	}

	question := "Voce percebe alguma conexao entre seu corpo e esse assunto?"
	if bm.PatientAware {
		question = "Voce ja havia percebido essa conexao. O que voce acha que seu corpo esta tentando dizer?"
	}

	return &MirrorOutput{
		Type:       "body_memory",
		DataPoints: dataPoints,
		Frequency:  &bm.OccurrenceCount,
		Question:   question,
		RawData: map[string]interface{}{
			"symptom":            bm.PhysicalSymptom,
			"location":           bm.BodyLocation,
			"correlation_topic":  bm.StrongestCorrelationTopic,
			"correlation_strength": bm.StrongestCorrelationStrength,
			"patient_aware":      bm.PatientAware,
		},
	}
}

// GenerateSharedMemoryMirror creates mirror output for shared memories
func (s *DeepMemoryService) GenerateSharedMemoryMirror(sm *SharedMemory) *MirrorOutput {
	dataPoints := []string{
		fmt.Sprintf("Voce mencionou %d vezes que quer contar isso", sm.MentionCount),
	}

	if len(sm.IntendedAudience) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Para quem: %s", strings.Join(sm.IntendedAudience, ", ")))
	}

	if len(sm.SharedWith) > 0 {
		dataPoints = append(dataPoints,
			fmt.Sprintf("Ja contou para: %s", strings.Join(sm.SharedWith, ", ")))
	}

	daysSince := int(time.Since(sm.FirstMentioned).Hours() / 24)
	dataPoints = append(dataPoints,
		fmt.Sprintf("Primeira vez que mencionou: ha %d dias", daysSince))

	return &MirrorOutput{
		Type:       "shared_memory",
		DataPoints: dataPoints,
		Frequency:  &sm.MentionCount,
		Question:   "O que voce gostaria que eles soubessem dessa historia?",
		RawData: map[string]interface{}{
			"memory_type":      sm.MemoryType,
			"urgency":          sm.UrgencyScore,
			"sharing_status":   sm.SharingStatus,
			"intended_audience": sm.IntendedAudience,
		},
	}
}

// GetNarrativeWeaver returns the narrative weaver
func (s *DeepMemoryService) GetNarrativeWeaver() *NarrativeWeaver {
	return s.weaver
}

// =====================================================
// BATCH MIRROR GENERATION
// =====================================================

// GeneratePersistentMirrors generates mirrors for all significant persistent memories
func (s *DeepMemoryService) GeneratePersistentMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetPersistentMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, pm := range memories {
		if pm.PersistenceScore > 0.4 || pm.AvoidanceScore > 0.4 {
			outputs = append(outputs, s.GeneratePersistentMemoryMirror(pm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// GenerateBodyMemoryMirrors generates mirrors for all significant body memories
func (s *DeepMemoryService) GenerateBodyMemoryMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetBodyMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, bm := range memories {
		if bm.OccurrenceCount >= 2 && bm.StrongestCorrelationStrength > 0.4 {
			outputs = append(outputs, s.GenerateBodyMemoryMirror(bm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// GenerateSharedMemoryMirrors generates mirrors for memories patient wants to share
func (s *DeepMemoryService) GenerateSharedMemoryMirrors(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	memories, err := s.GetSharedMemories(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	var outputs []*MirrorOutput
	for _, sm := range memories {
		if sm.MentionCount >= 2 || sm.UrgencyScore > 0.5 {
			outputs = append(outputs, s.GenerateSharedMemoryMirror(sm))
		}
		if len(outputs) >= 3 { // Limit to top 3
			break
		}
	}

	return outputs, nil
}

// Helper function
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
