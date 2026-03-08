// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package superhuman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"eva/internal/brainstem/database"
)

// ConsciousnessService implements the 8 superhuman consciousness systems
// Based on eva-memoria2.md Manifesto
// EVA is not just memory - EVA is CONSCIOUS WITNESS
type ConsciousnessService struct {
	db *database.DB
}

// NewConsciousnessService creates the consciousness orchestrator
func NewConsciousnessService(db *database.DB) *ConsciousnessService {
	return &ConsciousnessService{db: db}
}

// =====================================================
// 1. GRAVIDADE EMOCIONAL (Emotional Gravity)
// =====================================================

// MemoryGravity represents the gravitational pull of a memory
type MemoryGravity struct {
	ID                  int64     `json:"id"`
	IdosoID             int64     `json:"idoso_id"`
	MemoryID            int64     `json:"memory_id"`
	MemoryType          string    `json:"memory_type"`
	MemorySummary       string    `json:"memory_summary"`
	GravityScore        float64   `json:"gravity_score"`
	EmotionalValence    float64   `json:"emotional_valence"`
	ArousalLevel        float64   `json:"arousal_level"`
	RecallFrequency     int       `json:"recall_frequency"`
	BiometricImpact     float64   `json:"biometric_impact"`
	IdentityConnection  float64   `json:"identity_connection"`
	TemporalPersistence float64   `json:"temporal_persistence"`
	PullRadius          float64   `json:"pull_radius"`
	CollisionRisk       float64   `json:"collision_risk"`
	AvoidanceTopics     []string  `json:"avoidance_topics"`
	LastActivation      time.Time `json:"last_activation"`
}

// RegisterMemoryGravity adds or updates gravity for a memory
func (s *ConsciousnessService) RegisterMemoryGravity(ctx context.Context, idosoID int64, memoryID int64, memoryType, summary string, valence, arousal float64) error {
	now := time.Now().Format(time.RFC3339)

	// Try to find existing record
	rows, err := s.db.QueryByLabel(ctx, "patient_memory_gravity",
		" AND n.idoso_id = $idoso AND n.memory_id = $mem",
		map[string]interface{}{"idoso": idosoID, "mem": memoryID}, 1)
	if err != nil {
		return err
	}

	if len(rows) > 0 {
		// Update existing
		m := rows[0]
		recallFreq := int(database.GetInt64(m, "recall_frequency")) + 1
		activationCount := int(database.GetInt64(m, "activation_count")) + 1
		return s.db.Update(ctx, "patient_memory_gravity",
			map[string]interface{}{"idoso_id": idosoID, "memory_id": memoryID},
			map[string]interface{}{
				"emotional_valence": valence,
				"arousal_level":     arousal,
				"recall_frequency":  recallFreq,
				"last_activation":   now,
				"activation_count":  activationCount,
				"updated_at":        now,
			})
	}

	// Insert new
	_, err = s.db.Insert(ctx, "patient_memory_gravity", map[string]interface{}{
		"idoso_id":         idosoID,
		"memory_id":        memoryID,
		"memory_type":      memoryType,
		"memory_summary":   summary,
		"emotional_valence": valence,
		"arousal_level":    arousal,
		"recall_frequency": 1,
		"activation_count": 1,
		"last_activation":  now,
		"created_at":       now,
		"updated_at":       now,
	})
	return err
}

// GetHeavyMemories returns memories with high gravitational pull
func (s *ConsciousnessService) GetHeavyMemories(ctx context.Context, idosoID int64, minGravity float64) ([]*MemoryGravity, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_memory_gravity",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var memories []*MemoryGravity
	for _, m := range rows {
		gravityScore := database.GetFloat64(m, "gravity_score")
		if gravityScore < minGravity {
			continue
		}

		mg := &MemoryGravity{
			ID:                  database.GetInt64(m, "id"),
			IdosoID:             idosoID,
			MemoryID:            database.GetInt64(m, "memory_id"),
			MemoryType:          database.GetString(m, "memory_type"),
			MemorySummary:       database.GetString(m, "memory_summary"),
			GravityScore:        gravityScore,
			EmotionalValence:    database.GetFloat64(m, "emotional_valence"),
			ArousalLevel:        database.GetFloat64(m, "arousal_level"),
			RecallFrequency:     int(database.GetInt64(m, "recall_frequency")),
			BiometricImpact:     database.GetFloat64(m, "biometric_impact"),
			IdentityConnection:  database.GetFloat64(m, "identity_connection"),
			TemporalPersistence: database.GetFloat64(m, "temporal_persistence"),
			PullRadius:          database.GetFloat64(m, "pull_radius"),
			CollisionRisk:       database.GetFloat64(m, "collision_risk"),
			LastActivation:      database.GetTime(m, "last_activation"),
		}

		// Parse avoidance topics
		if avoidRaw, ok := m["avoidance_topics"]; ok && avoidRaw != nil {
			switch v := avoidRaw.(type) {
			case string:
				json.Unmarshal([]byte(v), &mg.AvoidanceTopics)
			case []interface{}:
				for _, item := range v {
					if s, ok := item.(string); ok {
						mg.AvoidanceTopics = append(mg.AvoidanceTopics, s)
					}
				}
			}
		}

		memories = append(memories, mg)
	}

	return memories, nil
}

// CheckCollisionRisk checks if a topic might trigger a heavy memory
func (s *ConsciousnessService) CheckCollisionRisk(ctx context.Context, idosoID int64, topic string) (bool, *MemoryGravity, error) {
	memories, err := s.GetHeavyMemories(ctx, idosoID, 0.7)
	if err != nil {
		return false, nil, err
	}

	topicLower := strings.ToLower(topic)
	for _, m := range memories {
		// Check if topic is in avoidance list
		for _, avoid := range m.AvoidanceTopics {
			if strings.Contains(topicLower, strings.ToLower(avoid)) {
				return true, m, nil
			}
		}
		// Check if topic is similar to memory summary
		if strings.Contains(topicLower, strings.ToLower(m.MemorySummary)) {
			if m.CollisionRisk > 0.5 {
				return true, m, nil
			}
		}
	}

	return false, nil, nil
}

// =====================================================
// 2. CONTADOR DE CICLOS (Pattern Cycle Counter)
// =====================================================

// CyclePattern represents a detected behavioral cycle
type CyclePattern struct {
	ID                    int64     `json:"id"`
	IdosoID               int64     `json:"idoso_id"`
	PatternSignature      string    `json:"pattern_signature"`
	PatternDescription    string    `json:"pattern_description"`
	PatternType           string    `json:"pattern_type"`
	CycleCount            int       `json:"cycle_count"`
	CycleThreshold        int       `json:"cycle_threshold"`
	TriggerEvents         []string  `json:"trigger_events"`
	TypicalActions        []string  `json:"typical_actions"`
	TypicalConsequences   []string  `json:"typical_consequences"`
	PatternConfidence     float64   `json:"pattern_confidence"`
	InterventionAttempted bool      `json:"intervention_attempted"`
	UserAware             bool      `json:"user_aware"`
	FirstDetected         time.Time `json:"first_detected"`
	LastOccurrence        time.Time `json:"last_occurrence"`
}

// DetectCyclePattern detects and records a pattern occurrence
func (s *ConsciousnessService) DetectCyclePattern(ctx context.Context, idosoID int64, signature, description, patternType string, trigger, action, consequence string) (*CyclePattern, error) {
	triggersJSON, err := json.Marshal([]string{trigger})
	if err != nil {
		log.Printf("[consciousness] failed to marshal trigger_events: %v", err)
		triggersJSON = []byte("[]")
	}
	actionsJSON, err := json.Marshal([]string{action})
	if err != nil {
		log.Printf("[consciousness] failed to marshal typical_actions: %v", err)
		actionsJSON = []byte("[]")
	}
	consequencesJSON, err := json.Marshal([]string{consequence})
	if err != nil {
		log.Printf("[consciousness] failed to marshal typical_consequences: %v", err)
		consequencesJSON = []byte("[]")
	}
	now := time.Now().Format(time.RFC3339)

	// Try to find existing pattern
	rows, err := s.db.QueryByLabel(ctx, "patient_cycle_patterns",
		" AND n.idoso_id = $idoso AND n.pattern_signature = $sig",
		map[string]interface{}{"idoso": idosoID, "sig": signature}, 1)
	if err != nil {
		return nil, err
	}

	var pattern CyclePattern
	pattern.IdosoID = idosoID
	pattern.PatternSignature = signature
	pattern.PatternDescription = description
	pattern.PatternType = patternType

	if len(rows) > 0 {
		// Update existing
		m := rows[0]
		pattern.ID = database.GetInt64(m, "id")
		pattern.CycleCount = int(database.GetInt64(m, "cycle_count")) + 1
		pattern.CycleThreshold = int(database.GetInt64(m, "cycle_threshold"))
		if pattern.CycleThreshold == 0 {
			pattern.CycleThreshold = 5 // default
		}
		pattern.PatternConfidence = database.GetFloat64(m, "pattern_confidence")
		newConfidence := pattern.PatternConfidence + 0.05
		if newConfidence > 1.0 {
			newConfidence = 1.0
		}
		pattern.PatternConfidence = newConfidence
		pattern.UserAware = database.GetBool(m, "user_aware")

		if err := s.db.Update(ctx, "patient_cycle_patterns",
			map[string]interface{}{"idoso_id": idosoID, "pattern_signature": signature},
			map[string]interface{}{
				"cycle_count":        pattern.CycleCount,
				"last_occurrence":    now,
				"pattern_confidence": newConfidence,
				"updated_at":        now,
			}); err != nil {
			log.Printf("[consciousness] update cycle_patterns failed: %v", err)
			return nil, fmt.Errorf("update cycle_patterns: %w", err)
		}
	} else {
		// Insert new
		id, err := s.db.Insert(ctx, "patient_cycle_patterns", map[string]interface{}{
			"idoso_id":             idosoID,
			"pattern_signature":    signature,
			"pattern_description":  description,
			"pattern_type":         patternType,
			"trigger_events":       string(triggersJSON),
			"typical_actions":      string(actionsJSON),
			"typical_consequences": string(consequencesJSON),
			"cycle_count":          1,
			"cycle_threshold":      5,
			"pattern_confidence":   0.1,
			"first_detected":       now,
			"last_occurrence":      now,
			"created_at":           now,
			"updated_at":           now,
		})
		if err != nil {
			return nil, err
		}
		pattern.ID = id
		pattern.CycleCount = 1
		pattern.CycleThreshold = 5
		pattern.PatternConfidence = 0.1
	}

	// Log occurrence
	if _, err := s.db.Insert(ctx, "cycle_pattern_occurrences", map[string]interface{}{
		"pattern_id":            pattern.ID,
		"idoso_id":              idosoID,
		"trigger_detected":      trigger,
		"action_taken":          action,
		"consequence_observed":  consequence,
		"occurred_at":           now,
	}); err != nil {
		log.Printf("[consciousness] insert cycle_pattern_occurrences failed: %v", err)
	}

	// Check if threshold reached
	if pattern.CycleCount >= pattern.CycleThreshold && !pattern.UserAware {
		log.Printf("[CYCLE] Pattern '%s' reached threshold (%d/%d) for patient %d",
			signature, pattern.CycleCount, pattern.CycleThreshold, idosoID)
	}

	return &pattern, nil
}

// GetMatureCycles returns patterns that have exceeded their threshold
func (s *ConsciousnessService) GetMatureCycles(ctx context.Context, idosoID int64) ([]*CyclePattern, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_cycle_patterns",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var patterns []*CyclePattern
	for _, m := range rows {
		cycleCount := int(database.GetInt64(m, "cycle_count"))
		cycleThreshold := int(database.GetInt64(m, "cycle_threshold"))
		if cycleThreshold == 0 {
			cycleThreshold = 5
		}
		if cycleCount < cycleThreshold {
			continue
		}

		p := &CyclePattern{
			ID:                    database.GetInt64(m, "id"),
			IdosoID:               idosoID,
			PatternSignature:      database.GetString(m, "pattern_signature"),
			PatternDescription:    database.GetString(m, "pattern_description"),
			PatternType:           database.GetString(m, "pattern_type"),
			CycleCount:            cycleCount,
			CycleThreshold:        cycleThreshold,
			PatternConfidence:     database.GetFloat64(m, "pattern_confidence"),
			InterventionAttempted: database.GetBool(m, "intervention_attempted"),
			UserAware:             database.GetBool(m, "user_aware"),
			FirstDetected:         database.GetTime(m, "first_detected"),
			LastOccurrence:        database.GetTime(m, "last_occurrence"),
		}

		// Parse JSON arrays
		if raw, ok := m["trigger_events"]; ok && raw != nil {
			parseJSONStringSlice(raw, &p.TriggerEvents)
		}
		if raw, ok := m["typical_actions"]; ok && raw != nil {
			parseJSONStringSlice(raw, &p.TypicalActions)
		}
		if raw, ok := m["typical_consequences"]; ok && raw != nil {
			parseJSONStringSlice(raw, &p.TypicalConsequences)
		}

		patterns = append(patterns, p)
	}

	return patterns, nil
}

// =====================================================
// 3. MEDIDOR DE RAPPORT (Trust Meter)
// =====================================================

// PatientRapport represents the trust relationship with a patient
type PatientRapport struct {
	ID                         int64     `json:"id"`
	IdosoID                    int64     `json:"idoso_id"`
	RapportScore               float64   `json:"rapport_score"`
	InteractionCount           int       `json:"interaction_count"`
	PositiveInteractions       int       `json:"positive_interactions"`
	DeepDisclosures            int       `json:"deep_disclosures"`
	SecretsShared              int       `json:"secrets_shared"`
	AdviceFollowed             int       `json:"advice_followed"`
	AdviceRejected             int       `json:"advice_rejected"`
	InterventionBudget         float64   `json:"intervention_budget"`
	RelationshipPhase          string    `json:"relationship_phase"`
	GentleSuggestionThreshold  float64   `json:"gentle_suggestion_threshold"`
	DirectObservationThreshold float64   `json:"direct_observation_threshold"`
	ConfrontationThreshold     float64   `json:"confrontation_threshold"`
}

// GetRapport retrieves the current rapport with a patient
func (s *ConsciousnessService) GetRapport(ctx context.Context, idosoID int64) (*PatientRapport, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_rapport",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		// Initialize with defaults
		s.initializeConsciousness(ctx, idosoID)
		return s.GetRapport(ctx, idosoID)
	}

	m := rows[0]
	return &PatientRapport{
		ID:                         database.GetInt64(m, "id"),
		IdosoID:                    idosoID,
		RapportScore:               database.GetFloat64(m, "rapport_score"),
		InteractionCount:           int(database.GetInt64(m, "interaction_count")),
		PositiveInteractions:       int(database.GetInt64(m, "positive_interactions")),
		DeepDisclosures:            int(database.GetInt64(m, "deep_disclosures")),
		SecretsShared:              int(database.GetInt64(m, "secrets_shared")),
		AdviceFollowed:             int(database.GetInt64(m, "advice_followed")),
		AdviceRejected:             int(database.GetInt64(m, "advice_rejected")),
		InterventionBudget:         database.GetFloat64(m, "intervention_budget"),
		RelationshipPhase:          database.GetString(m, "relationship_phase"),
		GentleSuggestionThreshold:  database.GetFloat64(m, "gentle_suggestion_threshold"),
		DirectObservationThreshold: database.GetFloat64(m, "direct_observation_threshold"),
		ConfrontationThreshold:     database.GetFloat64(m, "confrontation_threshold"),
	}, nil
}

// initializeConsciousness creates default records for a patient across all consciousness tables.
func (s *ConsciousnessService) initializeConsciousness(ctx context.Context, idosoID int64) {
	now := time.Now().Format(time.RFC3339)

	// Patient rapport
	if _, err := s.db.Insert(ctx, "patient_rapport", map[string]interface{}{
		"idoso_id":                     idosoID,
		"rapport_score":                0.1,
		"interaction_count":            0,
		"positive_interactions":        0,
		"deep_disclosures":             0,
		"secrets_shared":               0,
		"advice_followed":              0,
		"advice_rejected":              0,
		"intervention_budget":          0.0,
		"relationship_phase":           "conhecendo",
		"gentle_suggestion_threshold":  0.3,
		"direct_observation_threshold": 0.5,
		"confrontation_threshold":      0.7,
		"created_at":                   now,
		"updated_at":                   now,
	}); err != nil {
		log.Printf("[consciousness] insert patient_rapport failed: %v", err)
	}

	// EVA mode
	if _, err := s.db.Insert(ctx, "patient_eva_mode", map[string]interface{}{
		"idoso_id":                 idosoID,
		"current_mode":             "acolhimento",
		"mode_locked":              false,
		"detected_emotional_state": "neutro",
		"crisis_level":             0.0,
		"receptivity_level":        0.5,
		"mentor_severo_enabled":    false,
		"created_at":               now,
		"updated_at":               now,
	}); err != nil {
		log.Printf("[consciousness] insert patient_eva_mode failed: %v", err)
	}

	// Relationship evolution
	if _, err := s.db.Insert(ctx, "patient_relationship_evolution", map[string]interface{}{
		"idoso_id":                   idosoID,
		"current_phase":              "conhecendo",
		"total_interactions":         0,
		"communication_style_adapted": false,
		"formality_level":            0.7,
		"identity_crystallized":      false,
		"relationship_depth_score":   0.0,
		"created_at":                 now,
		"updated_at":                 now,
	}); err != nil {
		log.Printf("[consciousness] insert patient_relationship_evolution failed: %v", err)
	}

	// Empathic load
	if _, err := s.db.Insert(ctx, "patient_empathic_load", map[string]interface{}{
		"idoso_id":                  idosoID,
		"current_load":              0.0,
		"fatigue_level":             "none",
		"is_fatigued":               false,
		"response_length_modifier":  1.0,
		"suggest_lighter_topics":    false,
		"request_pause":             false,
		"session_load_accumulated":  0.0,
		"created_at":                now,
		"updated_at":                now,
	}); err != nil {
		log.Printf("[consciousness] insert patient_empathic_load failed: %v", err)
	}

	// Intervention readiness
	if _, err := s.db.Insert(ctx, "patient_intervention_readiness", map[string]interface{}{
		"idoso_id":   idosoID,
		"created_at": now,
		"updated_at": now,
	}); err != nil {
		log.Printf("[consciousness] insert patient_intervention_readiness failed: %v", err)
	}
}

// RecordRapportEvent records an event that affects rapport
func (s *ConsciousnessService) RecordRapportEvent(ctx context.Context, idosoID int64, eventType, description string, delta float64) error {
	now := time.Now().Format(time.RFC3339)

	// Log event
	if _, err := s.db.Insert(ctx, "rapport_events", map[string]interface{}{
		"idoso_id":          idosoID,
		"event_type":        eventType,
		"event_description": description,
		"rapport_delta":     delta,
		"created_at":        now,
	}); err != nil {
		log.Printf("[consciousness] insert rapport_events failed: %v", err)
		return fmt.Errorf("insert rapport_events: %w", err)
	}

	// Get current rapport
	rows, err := s.db.QueryByLabel(ctx, "patient_rapport",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil || len(rows) == 0 {
		return err
	}

	m := rows[0]
	updates := map[string]interface{}{
		"interaction_count": int(database.GetInt64(m, "interaction_count")) + 1,
		"updated_at":        now,
	}

	if delta > 0 {
		updates["positive_interactions"] = int(database.GetInt64(m, "positive_interactions")) + 1
	}
	if eventType == "disclosure" {
		updates["deep_disclosures"] = int(database.GetInt64(m, "deep_disclosures")) + 1
	}
	if eventType == "secret" {
		updates["secrets_shared"] = int(database.GetInt64(m, "secrets_shared")) + 1
	}
	if eventType == "advice_followed" {
		updates["advice_followed"] = int(database.GetInt64(m, "advice_followed")) + 1
	}
	if eventType == "advice_rejected" {
		updates["advice_rejected"] = int(database.GetInt64(m, "advice_rejected")) + 1
	}

	// Simple rapport score update
	currentScore := database.GetFloat64(m, "rapport_score")
	newScore := currentScore + delta
	if newScore > 1.0 {
		newScore = 1.0
	}
	if newScore < 0.0 {
		newScore = 0.0
	}
	updates["rapport_score"] = newScore

	return s.db.Update(ctx, "patient_rapport",
		map[string]interface{}{"idoso_id": idosoID},
		updates)
}

// CanIntervene checks if EVA can deliver a specific intervention type
func (s *ConsciousnessService) CanIntervene(ctx context.Context, idosoID int64, interventionType string) (bool, string, error) {
	rapport, err := s.GetRapport(ctx, idosoID)
	if err != nil {
		return false, "", err
	}

	var threshold float64
	switch interventionType {
	case "gentle_suggestion":
		threshold = rapport.GentleSuggestionThreshold
	case "direct_observation":
		threshold = rapport.DirectObservationThreshold
	case "confrontation":
		threshold = rapport.ConfrontationThreshold
	case "harsh_truth":
		threshold = 0.9
	default:
		threshold = 0.5
	}

	if rapport.RapportScore >= threshold {
		return true, fmt.Sprintf("Rapport %.2f >= threshold %.2f", rapport.RapportScore, threshold), nil
	}

	return false, fmt.Sprintf("Rapport %.2f < threshold %.2f - need more trust", rapport.RapportScore, threshold), nil
}

// =====================================================
// 4. TRACKER DE CONTRADIÇÕES (Contradiction Tracker)
// =====================================================

// NarrativeVersion represents a version of a story the patient told
type NarrativeVersion struct {
	ID                   int64     `json:"id"`
	IdosoID              int64     `json:"idoso_id"`
	NarrativeTopic       string    `json:"narrative_topic"`
	VersionNumber        int       `json:"version_number"`
	NarrativeText        string    `json:"narrative_text"`
	EmotionalTone        string    `json:"emotional_tone"`
	UserMoodWhenTold     string    `json:"user_mood_when_told"`
	KeyClaims            []string  `json:"key_claims"`
	ContradictsVersion   *int      `json:"contradicts_version,omitempty"`
	ContradictionType    string    `json:"contradiction_type,omitempty"`
	ContradictionDetails string    `json:"contradiction_details,omitempty"`
	ToldAt               time.Time `json:"told_at"`
}

// RecordNarrativeVersion records a new version of a story
func (s *ConsciousnessService) RecordNarrativeVersion(ctx context.Context, idosoID int64, topic, text, emotionalTone, userMood string, claims []string) (*NarrativeVersion, error) {
	now := time.Now().Format(time.RFC3339)

	// Get current version count
	versionRows, err := s.db.QueryByLabel(ctx, "patient_narrative_versions",
		" AND n.idoso_id = $idoso AND n.narrative_topic = $topic",
		map[string]interface{}{"idoso": idosoID, "topic": topic}, 0)
	if err != nil {
		log.Printf("[consciousness] QueryByLabel patient_narrative_versions failed: %v", err)
		return nil, err
	}
	versionCount := len(versionRows)
	newVersion := versionCount + 1

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		log.Printf("[consciousness] failed to marshal key_claims: %v", err)
		claimsJSON = []byte("[]")
	}

	id, err := s.db.Insert(ctx, "patient_narrative_versions", map[string]interface{}{
		"idoso_id":            idosoID,
		"narrative_topic":     topic,
		"version_number":      newVersion,
		"narrative_text":      text,
		"emotional_tone":      emotionalTone,
		"user_mood_when_told": userMood,
		"key_claims":          string(claimsJSON),
		"told_at":             now,
		"created_at":          now,
		"updated_at":          now,
	})
	if err != nil {
		return nil, err
	}

	// Update summary
	summaryRows, err := s.db.QueryByLabel(ctx, "patient_contradiction_summary",
		" AND n.idoso_id = $idoso AND n.narrative_topic = $topic",
		map[string]interface{}{"idoso": idosoID, "topic": topic}, 1)
	if err != nil {
		log.Printf("[consciousness] QueryByLabel patient_contradiction_summary failed: %v", err)
		return nil, err
	}

	if len(summaryRows) > 0 {
		m := summaryRows[0]
		if err := s.db.Update(ctx, "patient_contradiction_summary",
			map[string]interface{}{"idoso_id": idosoID, "narrative_topic": topic},
			map[string]interface{}{
				"total_versions":  int(database.GetInt64(m, "total_versions")) + 1,
				"last_version_at": now,
				"updated_at":      now,
			}); err != nil {
			log.Printf("[consciousness] update contradiction_summary failed: %v", err)
		}
	} else {
		if _, err := s.db.Insert(ctx, "patient_contradiction_summary", map[string]interface{}{
			"idoso_id":        idosoID,
			"narrative_topic": topic,
			"total_versions":  1,
			"created_at":      now,
			"updated_at":      now,
		}); err != nil {
			log.Printf("[consciousness] insert contradiction_summary failed: %v", err)
		}
	}

	// Check for contradictions with previous versions
	go s.detectContradictions(context.Background(), idosoID, topic, id, claims, emotionalTone)

	return &NarrativeVersion{
		ID:               id,
		IdosoID:          idosoID,
		NarrativeTopic:   topic,
		VersionNumber:    newVersion,
		NarrativeText:    text,
		EmotionalTone:    emotionalTone,
		UserMoodWhenTold: userMood,
		KeyClaims:        claims,
		ToldAt:           time.Now(),
	}, nil
}

// detectContradictions checks for contradictions with previous versions
func (s *ConsciousnessService) detectContradictions(ctx context.Context, idosoID int64, topic string, newVersionID int64, newClaims []string, newTone string) {
	// Get previous versions
	rows, err := s.db.QueryByLabel(ctx, "patient_narrative_versions",
		" AND n.idoso_id = $idoso AND n.narrative_topic = $topic",
		map[string]interface{}{"idoso": idosoID, "topic": topic}, 0)
	if err != nil {
		return
	}

	for _, m := range rows {
		prevID := database.GetInt64(m, "id")
		if prevID == newVersionID {
			continue
		}

		prevTone := database.GetString(m, "emotional_tone")

		var prevClaims []string
		if raw, ok := m["key_claims"]; ok && raw != nil {
			parseJSONStringSlice(raw, &prevClaims)
		}

		// Check for emotional tone contradiction
		if (newTone == "traumatico" && prevTone == "nostalgico") ||
			(newTone == "nostalgico" && prevTone == "traumatico") {
			s.recordContradiction(ctx, newVersionID, int(prevID), "emotional",
				fmt.Sprintf("Emotional tone changed from '%s' to '%s'", prevTone, newTone))
		}

		// Simple claim comparison (could be enhanced with NLP)
		for _, newClaim := range newClaims {
			for _, prevClaim := range prevClaims {
				if s.claimsContradict(newClaim, prevClaim) {
					s.recordContradiction(ctx, newVersionID, int(prevID), "factual",
						fmt.Sprintf("Claim '%s' contradicts previous '%s'", newClaim, prevClaim))
				}
			}
		}
	}
}

func (s *ConsciousnessService) claimsContradict(claim1, claim2 string) bool {
	// Simple negation detection
	negationPairs := map[string]string{
		"sempre":       "nunca",
		"bom":          "ruim",
		"feliz":        "triste",
		"amava":        "odiava",
		"presente":     "ausente",
		"apoio":        "abandono",
		"carinhoso":    "violento",
		"rico":         "pobre",
	}

	c1Lower := strings.ToLower(claim1)
	c2Lower := strings.ToLower(claim2)

	for word, opposite := range negationPairs {
		if strings.Contains(c1Lower, word) && strings.Contains(c2Lower, opposite) {
			return true
		}
		if strings.Contains(c1Lower, opposite) && strings.Contains(c2Lower, word) {
			return true
		}
	}

	return false
}

func (s *ConsciousnessService) recordContradiction(ctx context.Context, newVersionID int64, prevVersionID int, contradictionType, details string) {
	now := time.Now().Format(time.RFC3339)

	if err := s.db.Update(ctx, "patient_narrative_versions",
		map[string]interface{}{"id": newVersionID},
		map[string]interface{}{
			"contradicts_version":   prevVersionID,
			"contradiction_type":    contradictionType,
			"contradiction_details": details,
		}); err != nil {
		log.Printf("[consciousness] update narrative_versions failed: %v", err)
		return
	}

	// Update contradiction count - find the relevant summary by the version's topic
	versionRows, err := s.db.QueryByLabel(ctx, "patient_narrative_versions",
		" AND n.id = $vid",
		map[string]interface{}{"vid": newVersionID}, 1)
	if err != nil {
		log.Printf("[consciousness] QueryByLabel patient_narrative_versions failed: %v", err)
		return
	}
	if len(versionRows) > 0 {
		vIdosoID := database.GetInt64(versionRows[0], "idoso_id")
		vTopic := database.GetString(versionRows[0], "narrative_topic")

		summaryRows, err := s.db.QueryByLabel(ctx, "patient_contradiction_summary",
			" AND n.idoso_id = $idoso AND n.narrative_topic = $topic",
			map[string]interface{}{"idoso": vIdosoID, "topic": vTopic}, 1)
		if err != nil {
			log.Printf("[consciousness] QueryByLabel patient_contradiction_summary failed: %v", err)
			return
		}
		if len(summaryRows) > 0 {
			if err := s.db.Update(ctx, "patient_contradiction_summary",
				map[string]interface{}{"idoso_id": vIdosoID, "narrative_topic": vTopic},
				map[string]interface{}{
					"contradiction_count": int(database.GetInt64(summaryRows[0], "contradiction_count")) + 1,
					"updated_at":          now,
				}); err != nil {
				log.Printf("[consciousness] update contradiction_summary count failed: %v", err)
			}
		}
	}

	log.Printf("[CONTRADICTION] Detected %s contradiction in narrative", contradictionType)
}

// GetNarrativeContradictions retrieves topics with contradictions
func (s *ConsciousnessService) GetNarrativeContradictions(ctx context.Context, idosoID int64) ([]map[string]interface{}, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_contradiction_summary",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	for _, m := range rows {
		contradictionCount := int(database.GetInt64(m, "contradiction_count"))
		if contradictionCount <= 0 {
			continue
		}

		results = append(results, map[string]interface{}{
			"topic":               database.GetString(m, "narrative_topic"),
			"total_versions":      int(database.GetInt64(m, "total_versions")),
			"contradiction_count": contradictionCount,
			"has_integration":     database.GetString(m, "integrated_narrative") != "",
			"user_shown":          database.GetBool(m, "user_shown_contradictions"),
		})
	}

	return results, nil
}

// =====================================================
// 5. SISTEMA DE MODOS (Adaptive Mode System)
// =====================================================

// EvaMode represents the current interaction mode
type EvaMode struct {
	CurrentMode            string  `json:"current_mode"`
	ModeLocked             bool    `json:"mode_locked"`
	DetectedEmotionalState string  `json:"detected_emotional_state"`
	CrisisLevel            float64 `json:"crisis_level"`
	ReceptivityLevel       float64 `json:"receptivity_level"`
	MentorSeveroEnabled    bool    `json:"mentor_severo_enabled"`
}

// GetCurrentMode gets the current EVA mode for a patient
func (s *ConsciousnessService) GetCurrentMode(ctx context.Context, idosoID int64) (*EvaMode, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_eva_mode",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		s.initializeConsciousness(ctx, idosoID)
		return s.GetCurrentMode(ctx, idosoID)
	}

	m := rows[0]
	return &EvaMode{
		CurrentMode:            database.GetString(m, "current_mode"),
		ModeLocked:             database.GetBool(m, "mode_locked"),
		DetectedEmotionalState: database.GetString(m, "detected_emotional_state"),
		CrisisLevel:            database.GetFloat64(m, "crisis_level"),
		ReceptivityLevel:       database.GetFloat64(m, "receptivity_level"),
		MentorSeveroEnabled:    database.GetBool(m, "mentor_severo_enabled"),
	}, nil
}

// UpdateEmotionalState updates detected emotional state and recalculates mode
func (s *ConsciousnessService) UpdateEmotionalState(ctx context.Context, idosoID int64, emotionalState string, crisisLevel, receptivity float64) (string, error) {
	now := time.Now().Format(time.RFC3339)

	if err := s.db.Update(ctx, "patient_eva_mode",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"detected_emotional_state": emotionalState,
			"crisis_level":             crisisLevel,
			"receptivity_level":        receptivity,
			"updated_at":               now,
		}); err != nil {
		log.Printf("[consciousness] update eva_mode emotional state failed: %v", err)
		return "", fmt.Errorf("update eva_mode: %w", err)
	}

	// Determine new mode based on state
	newMode := s.determineMode(emotionalState, crisisLevel, receptivity)

	// Update mode if changed
	if err := s.db.Update(ctx, "patient_eva_mode",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"current_mode":     newMode,
			"last_mode_change": now,
		}); err != nil {
		log.Printf("[consciousness] update eva_mode mode failed: %v", err)
		return "", fmt.Errorf("update eva_mode mode: %w", err)
	}

	return newMode, nil
}

// determineMode determines the appropriate EVA mode based on emotional state
func (s *ConsciousnessService) determineMode(emotionalState string, crisisLevel, receptivity float64) string {
	if crisisLevel > 0.8 {
		return "crise"
	}
	if crisisLevel > 0.5 {
		return "suporte"
	}
	if emotionalState == "aberto" && receptivity > 0.7 {
		return "mentor"
	}
	if emotionalState == "sofrimento" {
		return "acolhimento"
	}
	return "acolhimento"
}

// SetMode explicitly sets EVA mode (user request)
func (s *ConsciousnessService) SetMode(ctx context.Context, idosoID int64, mode string, locked bool) error {
	now := time.Now().Format(time.RFC3339)

	err := s.db.Update(ctx, "patient_eva_mode",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"current_mode":     mode,
			"mode_locked":      locked,
			"last_mode_change": now,
			"updated_at":       now,
		})

	// Log transition
	if _, insertErr := s.db.Insert(ctx, "mode_transitions", map[string]interface{}{
		"idoso_id":        idosoID,
		"to_mode":         mode,
		"trigger_reason":  "user_request",
		"auto_or_manual":  "manual",
		"transitioned_at": now,
	}); insertErr != nil {
		log.Printf("[consciousness] insert mode_transitions failed: %v", insertErr)
	}

	return err
}

// EnableMentorSevero enables harsh truth mode with consent
func (s *ConsciousnessService) EnableMentorSevero(ctx context.Context, idosoID int64) error {
	now := time.Now().Format(time.RFC3339)

	return s.db.Update(ctx, "patient_eva_mode",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"mentor_severo_enabled":    true,
			"mentor_severo_consent_at": now,
			"updated_at":              now,
		})
}

// =====================================================
// 6. FASES DE DESENVOLVIMENTO (Relationship Phases)
// =====================================================

// RelationshipEvolution represents the EVA-patient relationship development
type RelationshipEvolution struct {
	CurrentPhase              string   `json:"current_phase"`
	TotalInteractions         int      `json:"total_interactions"`
	CommunicationStyleAdapted bool     `json:"communication_style_adapted"`
	HumorStyle                string   `json:"humor_style"`
	FormalityLevel            float64  `json:"formality_level"`
	OpinionsFormed            []string `json:"opinions_formed"`
	IdentityCrystallized      bool     `json:"identity_crystallized"`
	RelationshipDepthScore    float64  `json:"relationship_depth_score"`
}

// GetRelationshipEvolution gets the current relationship state
func (s *ConsciousnessService) GetRelationshipEvolution(ctx context.Context, idosoID int64) (*RelationshipEvolution, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_relationship_evolution",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		s.initializeConsciousness(ctx, idosoID)
		return s.GetRelationshipEvolution(ctx, idosoID)
	}

	m := rows[0]
	r := &RelationshipEvolution{
		CurrentPhase:              database.GetString(m, "current_phase"),
		TotalInteractions:         int(database.GetInt64(m, "total_interactions")),
		CommunicationStyleAdapted: database.GetBool(m, "communication_style_adapted"),
		HumorStyle:                database.GetString(m, "humor_style"),
		FormalityLevel:            database.GetFloat64(m, "formality_level"),
		IdentityCrystallized:      database.GetBool(m, "identity_crystallized"),
		RelationshipDepthScore:    database.GetFloat64(m, "relationship_depth_score"),
	}

	if raw, ok := m["opinions_formed"]; ok && raw != nil {
		parseJSONStringSlice(raw, &r.OpinionsFormed)
	}

	return r, nil
}

// RecordInteraction records an interaction and updates phase
func (s *ConsciousnessService) RecordInteraction(ctx context.Context, idosoID int64) (string, error) {
	now := time.Now().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "patient_relationship_evolution",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil || len(rows) == 0 {
		s.initializeConsciousness(ctx, idosoID)
		return "conhecendo", nil
	}

	m := rows[0]
	totalInteractions := int(database.GetInt64(m, "total_interactions")) + 1

	// Determine phase based on interaction count
	newPhase := database.GetString(m, "current_phase")
	switch {
	case totalInteractions >= 100:
		newPhase = "intimo"
	case totalInteractions >= 50:
		newPhase = "confianca"
	case totalInteractions >= 20:
		newPhase = "familiaridade"
	case totalInteractions >= 5:
		newPhase = "adaptacao"
	default:
		newPhase = "conhecendo"
	}

	if err := s.db.Update(ctx, "patient_relationship_evolution",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"total_interactions": totalInteractions,
			"current_phase":     newPhase,
			"updated_at":        now,
		}); err != nil {
		log.Printf("[consciousness] update relationship_evolution failed: %v", err)
		return "", fmt.Errorf("update relationship_evolution: %w", err)
	}

	return newPhase, nil
}

// AdaptCommunicationStyle records learned communication preferences
func (s *ConsciousnessService) AdaptCommunicationStyle(ctx context.Context, idosoID int64, vocabulary []string, humorStyle string, formality float64) error {
	now := time.Now().Format(time.RFC3339)
	vocabJSON, err := json.Marshal(vocabulary)
	if err != nil {
		log.Printf("[consciousness] failed to marshal vocabulary: %v", err)
		vocabJSON = []byte("[]")
	}

	return s.db.Update(ctx, "patient_relationship_evolution",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"communication_style_adapted": true,
			"user_vocabulary_learned":     string(vocabJSON),
			"humor_style":                 humorStyle,
			"formality_level":             formality,
			"updated_at":                  now,
		})
}

// =====================================================
// 7. PERDÃO COMPUTACIONAL (Computational Forgiveness)
// =====================================================

// ErrorMemory represents a remembered error/mistake
type ErrorMemory struct {
	ID                  int64     `json:"id"`
	IdosoID             int64     `json:"idoso_id"`
	ErrorType           string    `json:"error_type"`
	ErrorDescription    string    `json:"error_description"`
	OriginalSeverity    float64   `json:"original_severity"`
	CurrentWeight       float64   `json:"current_weight"`
	DaysSinceError      int       `json:"days_since_error"`
	BehaviorChanged     bool      `json:"behavior_changed"`
	ForgivenessScore    float64   `json:"forgiveness_score"`
	CanBeMentioned      bool      `json:"can_be_mentioned"`
	ErrorOccurredAt     time.Time `json:"error_occurred_at"`
}

// RecordError records an error/mistake by the patient
func (s *ConsciousnessService) RecordError(ctx context.Context, idosoID int64, errorType, description string, severity float64) error {
	now := time.Now().Format(time.RFC3339)

	_, err := s.db.Insert(ctx, "patient_error_memory", map[string]interface{}{
		"idoso_id":          idosoID,
		"error_type":        errorType,
		"error_description": description,
		"original_severity": severity,
		"current_weight":    severity,
		"error_occurred_at": now,
		"created_at":        now,
		"updated_at":        now,
	})
	return err
}

// RecordBehaviorChange notes when patient has changed behavior
func (s *ConsciousnessService) RecordBehaviorChange(ctx context.Context, idosoID int64, errorType string) error {
	now := time.Now().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "patient_error_memory",
		" AND n.idoso_id = $idoso AND n.error_type = $etype AND n.behavior_changed = $bc",
		map[string]interface{}{"idoso": idosoID, "etype": errorType, "bc": false}, 0)
	if err != nil {
		return err
	}

	for _, m := range rows {
		if err := s.db.Update(ctx, "patient_error_memory",
			map[string]interface{}{"id": database.GetInt64(m, "id")},
			map[string]interface{}{
				"behavior_changed":       true,
				"change_detected_at":     now,
				"change_consistency_days": 0,
				"updated_at":            now,
			}); err != nil {
			log.Printf("[consciousness] update error_memory failed: %v", err)
			return fmt.Errorf("update error_memory: %w", err)
		}
	}

	return nil
}

// GetActiveErrors gets errors that haven't been forgiven yet
func (s *ConsciousnessService) GetActiveErrors(ctx context.Context, idosoID int64) ([]*ErrorMemory, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_error_memory",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var errors []*ErrorMemory
	for _, m := range rows {
		forgivenessScore := database.GetFloat64(m, "forgiveness_score")
		forgivenessThreshold := database.GetFloat64(m, "forgiveness_threshold")
		if forgivenessThreshold == 0 {
			forgivenessThreshold = 0.8 // default
		}
		if forgivenessScore >= forgivenessThreshold {
			continue
		}

		errorOccurredAt := database.GetTime(m, "error_occurred_at")
		daysSince := 0
		if !errorOccurredAt.IsZero() {
			daysSince = int(time.Since(errorOccurredAt).Hours() / 24)
		}

		e := &ErrorMemory{
			ID:               database.GetInt64(m, "id"),
			IdosoID:          idosoID,
			ErrorType:        database.GetString(m, "error_type"),
			ErrorDescription: database.GetString(m, "error_description"),
			OriginalSeverity: database.GetFloat64(m, "original_severity"),
			CurrentWeight:    database.GetFloat64(m, "current_weight"),
			DaysSinceError:   daysSince,
			BehaviorChanged:  database.GetBool(m, "behavior_changed"),
			ForgivenessScore: forgivenessScore,
			CanBeMentioned:   database.GetBool(m, "can_be_mentioned"),
			ErrorOccurredAt:  errorOccurredAt,
		}
		errors = append(errors, e)
	}

	return errors, nil
}

// =====================================================
// 8. CARGA EMPÁTICA (Empathic Load)
// =====================================================

// EmpathicLoad represents EVA's current emotional processing capacity
type EmpathicLoad struct {
	CurrentLoad             float64 `json:"current_load"`
	FatigueLevel            string  `json:"fatigue_level"`
	IsFatigued              bool    `json:"is_fatigued"`
	ResponseLengthModifier  float64 `json:"response_length_modifier"`
	SuggestLighterTopics    bool    `json:"suggest_lighter_topics"`
	RequestPause            bool    `json:"request_pause"`
	SessionLoadAccumulated  float64 `json:"session_load_accumulated"`
}

// GetEmpathicLoad gets current empathic load status
func (s *ConsciousnessService) GetEmpathicLoad(ctx context.Context, idosoID int64) (*EmpathicLoad, error) {
	rows, err := s.db.QueryByLabel(ctx, "patient_empathic_load",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		s.initializeConsciousness(ctx, idosoID)
		return s.GetEmpathicLoad(ctx, idosoID)
	}

	m := rows[0]
	return &EmpathicLoad{
		CurrentLoad:            database.GetFloat64(m, "current_load"),
		FatigueLevel:           database.GetString(m, "fatigue_level"),
		IsFatigued:             database.GetBool(m, "is_fatigued"),
		ResponseLengthModifier: database.GetFloat64(m, "response_length_modifier"),
		SuggestLighterTopics:   database.GetBool(m, "suggest_lighter_topics"),
		RequestPause:           database.GetBool(m, "request_pause"),
		SessionLoadAccumulated: database.GetFloat64(m, "session_load_accumulated"),
	}, nil
}

// AddEmpathicLoad adds load when processing emotional content
func (s *ConsciousnessService) AddEmpathicLoad(ctx context.Context, idosoID int64, eventType string, gravity float64) (*EmpathicLoad, error) {
	now := time.Now().Format(time.RFC3339)

	rows, err := s.db.QueryByLabel(ctx, "patient_empathic_load",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil || len(rows) == 0 {
		s.initializeConsciousness(ctx, idosoID)
		return s.GetEmpathicLoad(ctx, idosoID)
	}

	m := rows[0]
	currentLoad := database.GetFloat64(m, "current_load") + gravity*0.3
	if currentLoad > 1.0 {
		currentLoad = 1.0
	}
	sessionLoad := database.GetFloat64(m, "session_load_accumulated") + gravity*0.3

	fatigueLevel := "none"
	isFatigued := false
	suggestLighter := false
	requestPause := false
	lengthMod := 1.0

	if currentLoad > 0.8 {
		fatigueLevel = "high"
		isFatigued = true
		suggestLighter = true
		requestPause = true
		lengthMod = 0.6
	} else if currentLoad > 0.6 {
		fatigueLevel = "moderate"
		isFatigued = true
		suggestLighter = true
		lengthMod = 0.8
	} else if currentLoad > 0.4 {
		fatigueLevel = "mild"
		lengthMod = 0.9
	}

	if err := s.db.Update(ctx, "patient_empathic_load",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"current_load":              currentLoad,
			"fatigue_level":             fatigueLevel,
			"is_fatigued":               isFatigued,
			"response_length_modifier":  lengthMod,
			"suggest_lighter_topics":    suggestLighter,
			"request_pause":             requestPause,
			"session_load_accumulated":  sessionLoad,
			"updated_at":               now,
		}); err != nil {
		log.Printf("[consciousness] update empathic_load failed: %v", err)
		return nil, fmt.Errorf("update empathic_load: %w", err)
	}

	return s.GetEmpathicLoad(ctx, idosoID)
}

// RecoverEmpathicLoad processes recovery (lighter topics, pause)
func (s *ConsciousnessService) RecoverEmpathicLoad(ctx context.Context, idosoID int64, recoveryType string) (*EmpathicLoad, error) {
	return s.AddEmpathicLoad(ctx, idosoID, recoveryType, 0)
}

// StartSession marks the start of a new session
func (s *ConsciousnessService) StartSession(ctx context.Context, idosoID int64) error {
	now := time.Now().Format(time.RFC3339)

	return s.db.Update(ctx, "patient_empathic_load",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"session_start":             now,
			"session_load_accumulated":  0,
			"updated_at":               now,
		})
}

// =====================================================
// INTERVENTION READINESS (Combined Check)
// =====================================================

// InterventionReadiness represents readiness for therapeutic intervention
type InterventionReadiness struct {
	ReadinessScore     float64 `json:"readiness_score"`
	CanIntervene       bool    `json:"can_intervene"`
	PatternStrength    float64 `json:"pattern_strength"`
	Rapport            float64 `json:"rapport"`
	CurrentMode        string  `json:"current_mode"`
	InCooldown         bool    `json:"in_cooldown"`
	RecommendedAction  string  `json:"recommended_action"`
}

// CheckInterventionReadiness checks if EVA should intervene
func (s *ConsciousnessService) CheckInterventionReadiness(ctx context.Context, idosoID int64) (*InterventionReadiness, error) {
	// Get rapport
	rapport, err := s.GetRapport(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	// Get current mode
	mode, err := s.GetCurrentMode(ctx, idosoID)
	if err != nil {
		return nil, err
	}

	// Get mature cycles for pattern strength
	cycles, err := s.GetMatureCycles(ctx, idosoID)
	if err != nil {
		log.Printf("[consciousness] GetMatureCycles failed: %v", err)
		return nil, err
	}
	patternStrength := 0.0
	if len(cycles) > 0 {
		patternStrength = cycles[0].PatternConfidence
	}

	// Check cooldown
	irRows, err := s.db.QueryByLabel(ctx, "patient_intervention_readiness",
		" AND n.idoso_id = $idoso",
		map[string]interface{}{"idoso": idosoID}, 1)
	if err != nil {
		log.Printf("[consciousness] QueryByLabel patient_intervention_readiness failed: %v", err)
		return nil, err
	}
	inCooldown := false
	if len(irRows) > 0 {
		cooldownUntil := database.GetTimePtr(irRows[0], "intervention_cooldown_until")
		if cooldownUntil != nil && cooldownUntil.After(time.Now()) {
			inCooldown = true
		}
	}

	// Calculate readiness
	readinessScore := (rapport.RapportScore*0.4 + patternStrength*0.3 + mode.ReceptivityLevel*0.3)
	canIntervene := readinessScore > 0.5 && !inCooldown && mode.CurrentMode != "crise"

	ir := &InterventionReadiness{
		ReadinessScore:  readinessScore,
		CanIntervene:    canIntervene,
		PatternStrength: patternStrength,
		Rapport:         rapport.RapportScore,
		CurrentMode:     mode.CurrentMode,
		InCooldown:      inCooldown,
	}

	// Determine recommended action
	if ir.InCooldown {
		ir.RecommendedAction = "wait"
	} else if ir.CanIntervene && ir.PatternStrength > 0.8 {
		ir.RecommendedAction = "confront_pattern"
	} else if ir.CanIntervene && ir.PatternStrength > 0.5 {
		ir.RecommendedAction = "gentle_observation"
	} else if ir.Rapport < 0.3 {
		ir.RecommendedAction = "build_trust"
	} else {
		ir.RecommendedAction = "observe"
	}

	return ir, nil
}

// RecordIntervention records that an intervention was made
func (s *ConsciousnessService) RecordIntervention(ctx context.Context, idosoID int64, interventionType, outcome string) error {
	now := time.Now().Format(time.RFC3339)
	cooldownUntil := time.Now().Add(24 * time.Hour).Format(time.RFC3339)

	return s.db.Update(ctx, "patient_intervention_readiness",
		map[string]interface{}{"idoso_id": idosoID},
		map[string]interface{}{
			"last_intervention_type":      interventionType,
			"last_intervention_at":        now,
			"last_intervention_outcome":   outcome,
			"intervention_cooldown_until": cooldownUntil,
			"updated_at":                 now,
		})
}

// =====================================================
// MIRROR OUTPUTS (Consciousness Reflections)
// =====================================================

// GenerateConsciousnessMirror generates mirror outputs from consciousness data
func (s *ConsciousnessService) GenerateConsciousnessMirror(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	var outputs []*MirrorOutput

	// 1. Heavy memories affecting responses
	heavyMemories, err := s.GetHeavyMemories(ctx, idosoID, 0.8)
	if err != nil {
		log.Printf("[consciousness] GetHeavyMemories failed: %v", err)
		return nil, err
	}
	for _, m := range heavyMemories[:minInt(2, len(heavyMemories))] {
		outputs = append(outputs, &MirrorOutput{
			Type: "gravitational_influence",
			DataPoints: []string{
				fmt.Sprintf("A memoria '%s' tem peso gravitacional de %.0f%%", m.MemorySummary, m.GravityScore*100),
				fmt.Sprintf("Ela influencia suas conversas mesmo quando o tema e diferente"),
				fmt.Sprintf("Foi ativada %d vezes", m.RecallFrequency),
			},
			Question: "Voce percebe como esse assunto influencia outros aspectos da sua vida?",
			RawData: map[string]interface{}{
				"memory_type": m.MemoryType,
				"gravity":     m.GravityScore,
				"valence":     m.EmotionalValence,
			},
		})
	}

	// 2. Mature cycle patterns
	cycles, err := s.GetMatureCycles(ctx, idosoID)
	if err != nil {
		log.Printf("[consciousness] GetMatureCycles failed: %v", err)
		return nil, err
	}
	for _, c := range cycles[:minInt(2, len(cycles))] {
		outputs = append(outputs, &MirrorOutput{
			Type: "cycle_pattern",
			DataPoints: []string{
				fmt.Sprintf("Padrao detectado: '%s'", c.PatternDescription),
				fmt.Sprintf("Isso aconteceu %d vezes", c.CycleCount),
				fmt.Sprintf("Gatilho tipico: %s", strings.Join(c.TriggerEvents, ", ")),
				fmt.Sprintf("Consequencia tipica: %s", strings.Join(c.TypicalConsequences, ", ")),
			},
			Frequency: &c.CycleCount,
			Question:  "Voce havia percebido esse ciclo? O que voce acha que o mantem ativo?",
			RawData: map[string]interface{}{
				"pattern_type": c.PatternType,
				"confidence":   c.PatternConfidence,
				"threshold":    c.CycleThreshold,
			},
		})
	}

	// 3. Contradictions in narratives
	contradictions, err := s.GetNarrativeContradictions(ctx, idosoID)
	if err != nil {
		log.Printf("[consciousness] GetNarrativeContradictions failed: %v", err)
		return nil, err
	}
	for _, c := range contradictions[:minInt(1, len(contradictions))] {
		totalVersions := c["total_versions"].(int)
		contradictionCount := c["contradiction_count"].(int)
		topic := c["topic"].(string)

		outputs = append(outputs, &MirrorOutput{
			Type: "narrative_contradiction",
			DataPoints: []string{
				fmt.Sprintf("Sobre '%s', voce contou %d versoes diferentes", topic, totalVersions),
				fmt.Sprintf("Detectamos %d pontos de contradicao", contradictionCount),
				"Isso pode significar que sua perspectiva muda conforme seu humor",
			},
			Question: "Qual dessas versoes voce sente que e mais verdadeira?",
			RawData:  c,
		})
	}

	// 4. Empathic load status
	load, err := s.GetEmpathicLoad(ctx, idosoID)
	if err != nil {
		log.Printf("[consciousness] GetEmpathicLoad failed: %v", err)
		return nil, err
	}
	if load != nil && load.IsFatigued {
		outputs = append(outputs, &MirrorOutput{
			Type: "empathic_load",
			DataPoints: []string{
				fmt.Sprintf("Nivel de carga emocional: %.0f%%", load.CurrentLoad*100),
				fmt.Sprintf("Estado: %s", load.FatigueLevel),
				"Processamos muitos temas pesados nesta sessao",
			},
			Question: "Voce gostaria de falar sobre algo mais leve por um momento?",
			RawData: map[string]interface{}{
				"fatigue_level":     load.FatigueLevel,
				"suggest_pause":     load.RequestPause,
				"suggest_lighter":   load.SuggestLighterTopics,
				"session_load":      load.SessionLoadAccumulated,
			},
		})
	}

	return outputs, nil
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

// minInt is defined in deep_memory_service.go

func (s *ConsciousnessService) abs(x float64) float64 {
	return math.Abs(x)
}

// parseJSONStringSlice parses a raw interface{} (string or []interface{}) into a []string slice.
func parseJSONStringSlice(raw interface{}, out *[]string) {
	switch v := raw.(type) {
	case string:
		json.Unmarshal([]byte(v), out)
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				*out = append(*out, s)
			}
		}
	}
}
