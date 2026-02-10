package superhuman

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// ConsciousnessService implements the 8 superhuman consciousness systems
// Based on eva-memoria2.md Manifesto
// EVA is not just memory - EVA is CONSCIOUS WITNESS
type ConsciousnessService struct {
	db *sql.DB
}

// NewConsciousnessService creates the consciousness orchestrator
func NewConsciousnessService(db *sql.DB) *ConsciousnessService {
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
	query := `
		INSERT INTO patient_memory_gravity
		(idoso_id, memory_id, memory_type, memory_summary, emotional_valence, arousal_level)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (idoso_id, memory_id) DO UPDATE SET
			emotional_valence = $5,
			arousal_level = $6,
			recall_frequency = patient_memory_gravity.recall_frequency + 1,
			last_activation = NOW(),
			activation_count = patient_memory_gravity.activation_count + 1,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, memoryID, memoryType, summary, valence, arousal)
	if err != nil {
		return err
	}

	// Recalculate gravity
	_, err = s.db.ExecContext(ctx, "SELECT calculate_memory_gravity($1, $2)", idosoID, memoryID)
	return err
}

// GetHeavyMemories returns memories with high gravitational pull
func (s *ConsciousnessService) GetHeavyMemories(ctx context.Context, idosoID int64, minGravity float64) ([]*MemoryGravity, error) {
	query := `
		SELECT id, memory_id, memory_type, memory_summary, gravity_score,
		       emotional_valence, arousal_level, recall_frequency, biometric_impact,
		       identity_connection, temporal_persistence, pull_radius, collision_risk,
		       avoidance_topics, last_activation
		FROM patient_memory_gravity
		WHERE idoso_id = $1 AND gravity_score >= $2
		ORDER BY gravity_score DESC
	`
	rows, err := s.db.QueryContext(ctx, query, idosoID, minGravity)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*MemoryGravity
	for rows.Next() {
		m := &MemoryGravity{IdosoID: idosoID}
		var avoidJSON []byte
		var lastAct sql.NullTime

		err := rows.Scan(
			&m.ID, &m.MemoryID, &m.MemoryType, &m.MemorySummary, &m.GravityScore,
			&m.EmotionalValence, &m.ArousalLevel, &m.RecallFrequency, &m.BiometricImpact,
			&m.IdentityConnection, &m.TemporalPersistence, &m.PullRadius, &m.CollisionRisk,
			&avoidJSON, &lastAct,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(avoidJSON, &m.AvoidanceTopics)
		if lastAct.Valid {
			m.LastActivation = lastAct.Time
		}

		memories = append(memories, m)
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
	triggersJSON, _ := json.Marshal([]string{trigger})
	actionsJSON, _ := json.Marshal([]string{action})
	consequencesJSON, _ := json.Marshal([]string{consequence})

	query := `
		INSERT INTO patient_cycle_patterns
		(idoso_id, pattern_signature, pattern_description, pattern_type,
		 trigger_events, typical_actions, typical_consequences)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idoso_id, pattern_signature) DO UPDATE SET
			cycle_count = patient_cycle_patterns.cycle_count + 1,
			last_occurrence = NOW(),
			pattern_confidence = LEAST(1.0, patient_cycle_patterns.pattern_confidence + 0.05),
			updated_at = NOW()
		RETURNING id, cycle_count, cycle_threshold, pattern_confidence, user_aware
	`

	var pattern CyclePattern
	pattern.IdosoID = idosoID
	pattern.PatternSignature = signature
	pattern.PatternDescription = description
	pattern.PatternType = patternType

	err := s.db.QueryRowContext(ctx, query, idosoID, signature, description, patternType,
		string(triggersJSON), string(actionsJSON), string(consequencesJSON)).Scan(
		&pattern.ID, &pattern.CycleCount, &pattern.CycleThreshold,
		&pattern.PatternConfidence, &pattern.UserAware,
	)
	if err != nil {
		return nil, err
	}

	// Log occurrence
	occQuery := `
		INSERT INTO cycle_pattern_occurrences
		(pattern_id, idoso_id, trigger_detected, action_taken, consequence_observed)
		VALUES ($1, $2, $3, $4, $5)
	`
	s.db.ExecContext(ctx, occQuery, pattern.ID, idosoID, trigger, action, consequence)

	// Check if threshold reached
	if pattern.CycleCount >= pattern.CycleThreshold && !pattern.UserAware {
		log.Printf("🔄 [CYCLE] Pattern '%s' reached threshold (%d/%d) for patient %d",
			signature, pattern.CycleCount, pattern.CycleThreshold, idosoID)
	}

	return &pattern, nil
}

// GetMatureCycles returns patterns that have exceeded their threshold
func (s *ConsciousnessService) GetMatureCycles(ctx context.Context, idosoID int64) ([]*CyclePattern, error) {
	query := `
		SELECT id, pattern_signature, pattern_description, pattern_type,
		       cycle_count, cycle_threshold, trigger_events, typical_actions,
		       typical_consequences, pattern_confidence, intervention_attempted,
		       user_aware, first_detected, last_occurrence
		FROM patient_cycle_patterns
		WHERE idoso_id = $1 AND cycle_count >= cycle_threshold
		ORDER BY cycle_count DESC
	`
	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var patterns []*CyclePattern
	for rows.Next() {
		p := &CyclePattern{IdosoID: idosoID}
		var triggersJSON, actionsJSON, consequencesJSON []byte

		err := rows.Scan(
			&p.ID, &p.PatternSignature, &p.PatternDescription, &p.PatternType,
			&p.CycleCount, &p.CycleThreshold, &triggersJSON, &actionsJSON,
			&consequencesJSON, &p.PatternConfidence, &p.InterventionAttempted,
			&p.UserAware, &p.FirstDetected, &p.LastOccurrence,
		)
		if err != nil {
			continue
		}

		json.Unmarshal(triggersJSON, &p.TriggerEvents)
		json.Unmarshal(actionsJSON, &p.TypicalActions)
		json.Unmarshal(consequencesJSON, &p.TypicalConsequences)

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
	query := `
		SELECT id, rapport_score, interaction_count, positive_interactions,
		       deep_disclosures, secrets_shared, advice_followed, advice_rejected,
		       intervention_budget, relationship_phase,
		       gentle_suggestion_threshold, direct_observation_threshold,
		       confrontation_threshold
		FROM patient_rapport
		WHERE idoso_id = $1
	`

	r := &PatientRapport{IdosoID: idosoID}
	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&r.ID, &r.RapportScore, &r.InteractionCount, &r.PositiveInteractions,
		&r.DeepDisclosures, &r.SecretsShared, &r.AdviceFollowed, &r.AdviceRejected,
		&r.InterventionBudget, &r.RelationshipPhase,
		&r.GentleSuggestionThreshold, &r.DirectObservationThreshold,
		&r.ConfrontationThreshold,
	)
	if err == sql.ErrNoRows {
		// Initialize
		s.db.ExecContext(ctx, "SELECT initialize_superhuman_consciousness($1)", idosoID)
		return s.GetRapport(ctx, idosoID)
	}
	return r, err
}

// RecordRapportEvent records an event that affects rapport
func (s *ConsciousnessService) RecordRapportEvent(ctx context.Context, idosoID int64, eventType, description string, delta float64) error {
	// Log event
	eventQuery := `
		INSERT INTO rapport_events (idoso_id, event_type, event_description, rapport_delta)
		VALUES ($1, $2, $3, $4)
	`
	s.db.ExecContext(ctx, eventQuery, idosoID, eventType, description, delta)

	// Update counters
	updateQuery := `
		UPDATE patient_rapport
		SET interaction_count = interaction_count + 1,
		    positive_interactions = positive_interactions + CASE WHEN $2 > 0 THEN 1 ELSE 0 END,
		    deep_disclosures = deep_disclosures + CASE WHEN $3 = 'disclosure' THEN 1 ELSE 0 END,
		    secrets_shared = secrets_shared + CASE WHEN $3 = 'secret' THEN 1 ELSE 0 END,
		    advice_followed = advice_followed + CASE WHEN $3 = 'advice_followed' THEN 1 ELSE 0 END,
		    advice_rejected = advice_rejected + CASE WHEN $3 = 'advice_rejected' THEN 1 ELSE 0 END,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	s.db.ExecContext(ctx, updateQuery, idosoID, delta, eventType)

	// Recalculate score
	_, err := s.db.ExecContext(ctx, "SELECT update_rapport_score($1)", idosoID)
	return err
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
	// Get current version count
	var versionCount int
	s.db.QueryRowContext(ctx,
		"SELECT COALESCE(MAX(version_number), 0) FROM patient_narrative_versions WHERE idoso_id = $1 AND narrative_topic = $2",
		idosoID, topic).Scan(&versionCount)

	newVersion := versionCount + 1
	claimsJSON, _ := json.Marshal(claims)

	query := `
		INSERT INTO patient_narrative_versions
		(idoso_id, narrative_topic, version_number, narrative_text, emotional_tone,
		 user_mood_when_told, key_claims)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id int64
	err := s.db.QueryRowContext(ctx, query, idosoID, topic, newVersion, text, emotionalTone,
		userMood, string(claimsJSON)).Scan(&id)
	if err != nil {
		return nil, err
	}

	// Update summary
	summaryQuery := `
		INSERT INTO patient_contradiction_summary (idoso_id, narrative_topic, total_versions)
		VALUES ($1, $2, 1)
		ON CONFLICT (idoso_id, narrative_topic) DO UPDATE SET
			total_versions = patient_contradiction_summary.total_versions + 1,
			last_version_at = NOW(),
			updated_at = NOW()
	`
	s.db.ExecContext(ctx, summaryQuery, idosoID, topic)

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
	query := `
		SELECT id, version_number, key_claims, emotional_tone
		FROM patient_narrative_versions
		WHERE idoso_id = $1 AND narrative_topic = $2 AND id != $3
		ORDER BY version_number DESC
		LIMIT 5
	`
	rows, err := s.db.QueryContext(ctx, query, idosoID, topic, newVersionID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var prevID int64
		var prevVersion int
		var prevClaimsJSON []byte
		var prevTone string

		if err := rows.Scan(&prevID, &prevVersion, &prevClaimsJSON, &prevTone); err != nil {
			continue
		}

		var prevClaims []string
		json.Unmarshal(prevClaimsJSON, &prevClaims)

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
	query := `
		UPDATE patient_narrative_versions
		SET contradicts_version = $2,
		    contradiction_type = $3,
		    contradiction_details = $4
		WHERE id = $1
	`
	s.db.ExecContext(ctx, query, newVersionID, prevVersionID, contradictionType, details)

	// Update contradiction count
	updateQuery := `
		UPDATE patient_contradiction_summary
		SET contradiction_count = contradiction_count + 1,
		    updated_at = NOW()
		WHERE idoso_id = (SELECT idoso_id FROM patient_narrative_versions WHERE id = $1)
		AND narrative_topic = (SELECT narrative_topic FROM patient_narrative_versions WHERE id = $1)
	`
	s.db.ExecContext(ctx, updateQuery, newVersionID)

	log.Printf("🔍 [CONTRADICTION] Detected %s contradiction in narrative", contradictionType)
}

// GetNarrativeContradictions retrieves topics with contradictions
func (s *ConsciousnessService) GetNarrativeContradictions(ctx context.Context, idosoID int64) ([]map[string]interface{}, error) {
	query := `
		SELECT narrative_topic, total_versions, contradiction_count,
		       integrated_narrative, user_shown_contradictions
		FROM patient_contradiction_summary
		WHERE idoso_id = $1 AND contradiction_count > 0
		ORDER BY contradiction_count DESC
	`
	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var topic string
		var totalVersions, contradictionCount int
		var integrated sql.NullString
		var shown bool

		if err := rows.Scan(&topic, &totalVersions, &contradictionCount, &integrated, &shown); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"topic":              topic,
			"total_versions":     totalVersions,
			"contradiction_count": contradictionCount,
			"has_integration":    integrated.Valid,
			"user_shown":         shown,
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
	query := `
		SELECT current_mode, mode_locked, detected_emotional_state,
		       crisis_level, receptivity_level, mentor_severo_enabled
		FROM patient_eva_mode
		WHERE idoso_id = $1
	`

	mode := &EvaMode{}
	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&mode.CurrentMode, &mode.ModeLocked, &mode.DetectedEmotionalState,
		&mode.CrisisLevel, &mode.ReceptivityLevel, &mode.MentorSeveroEnabled,
	)
	if err == sql.ErrNoRows {
		s.db.ExecContext(ctx, "SELECT initialize_superhuman_consciousness($1)", idosoID)
		return s.GetCurrentMode(ctx, idosoID)
	}
	return mode, err
}

// UpdateEmotionalState updates detected emotional state and recalculates mode
func (s *ConsciousnessService) UpdateEmotionalState(ctx context.Context, idosoID int64, emotionalState string, crisisLevel, receptivity float64) (string, error) {
	updateQuery := `
		UPDATE patient_eva_mode
		SET detected_emotional_state = $2,
		    crisis_level = $3,
		    receptivity_level = $4,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	s.db.ExecContext(ctx, updateQuery, idosoID, emotionalState, crisisLevel, receptivity)

	// Determine new mode
	var newMode string
	err := s.db.QueryRowContext(ctx, "SELECT determine_eva_mode($1)", idosoID).Scan(&newMode)
	if err != nil {
		return "", err
	}

	return newMode, nil
}

// SetMode explicitly sets EVA mode (user request)
func (s *ConsciousnessService) SetMode(ctx context.Context, idosoID int64, mode string, locked bool) error {
	query := `
		UPDATE patient_eva_mode
		SET current_mode = $2,
		    mode_locked = $3,
		    last_mode_change = NOW(),
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, mode, locked)

	// Log transition
	transQuery := `
		INSERT INTO mode_transitions (idoso_id, to_mode, trigger_reason, auto_or_manual)
		VALUES ($1, $2, 'user_request', 'manual')
	`
	s.db.ExecContext(ctx, transQuery, idosoID, mode)

	return err
}

// EnableMentorSevero enables harsh truth mode with consent
func (s *ConsciousnessService) EnableMentorSevero(ctx context.Context, idosoID int64) error {
	query := `
		UPDATE patient_eva_mode
		SET mentor_severo_enabled = TRUE,
		    mentor_severo_consent_at = NOW(),
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query, idosoID)
	return err
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
	query := `
		SELECT current_phase, total_interactions, communication_style_adapted,
		       humor_style, formality_level, opinions_formed,
		       identity_crystallized, relationship_depth_score
		FROM patient_relationship_evolution
		WHERE idoso_id = $1
	`

	r := &RelationshipEvolution{}
	var opinionsJSON []byte
	var humorStyle sql.NullString

	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&r.CurrentPhase, &r.TotalInteractions, &r.CommunicationStyleAdapted,
		&humorStyle, &r.FormalityLevel, &opinionsJSON,
		&r.IdentityCrystallized, &r.RelationshipDepthScore,
	)
	if err == sql.ErrNoRows {
		s.db.ExecContext(ctx, "SELECT initialize_superhuman_consciousness($1)", idosoID)
		return s.GetRelationshipEvolution(ctx, idosoID)
	}

	if humorStyle.Valid {
		r.HumorStyle = humorStyle.String
	}
	json.Unmarshal(opinionsJSON, &r.OpinionsFormed)

	return r, err
}

// RecordInteraction records an interaction and updates phase
func (s *ConsciousnessService) RecordInteraction(ctx context.Context, idosoID int64) (string, error) {
	updateQuery := `
		UPDATE patient_relationship_evolution
		SET total_interactions = total_interactions + 1,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	s.db.ExecContext(ctx, updateQuery, idosoID)

	var newPhase string
	err := s.db.QueryRowContext(ctx, "SELECT update_relationship_phase($1)", idosoID).Scan(&newPhase)
	return newPhase, err
}

// AdaptCommunicationStyle records learned communication preferences
func (s *ConsciousnessService) AdaptCommunicationStyle(ctx context.Context, idosoID int64, vocabulary []string, humorStyle string, formality float64) error {
	vocabJSON, _ := json.Marshal(vocabulary)

	query := `
		UPDATE patient_relationship_evolution
		SET communication_style_adapted = TRUE,
		    user_vocabulary_learned = $2,
		    humor_style = $3,
		    formality_level = $4,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, string(vocabJSON), humorStyle, formality)
	return err
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
	query := `
		INSERT INTO patient_error_memory
		(idoso_id, error_type, error_description, original_severity, current_weight, error_occurred_at)
		VALUES ($1, $2, $3, $4, $4, NOW())
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, errorType, description, severity)
	return err
}

// RecordBehaviorChange notes when patient has changed behavior
func (s *ConsciousnessService) RecordBehaviorChange(ctx context.Context, idosoID int64, errorType string) error {
	query := `
		UPDATE patient_error_memory
		SET behavior_changed = TRUE,
		    change_detected_at = NOW(),
		    change_consistency_days = 0,
		    updated_at = NOW()
		WHERE idoso_id = $1 AND error_type = $2 AND behavior_changed = FALSE
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, errorType)
	return err
}

// GetActiveErrors gets errors that haven't been forgiven yet
func (s *ConsciousnessService) GetActiveErrors(ctx context.Context, idosoID int64) ([]*ErrorMemory, error) {
	// First decay weights
	s.db.ExecContext(ctx, "SELECT decay_error_weights()")

	query := `
		SELECT id, error_type, error_description, original_severity, current_weight,
		       days_since_error, behavior_changed, forgiveness_score, can_be_mentioned,
		       error_occurred_at
		FROM patient_error_memory
		WHERE idoso_id = $1 AND forgiveness_score < forgiveness_threshold
		ORDER BY current_weight DESC
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var errors []*ErrorMemory
	for rows.Next() {
		e := &ErrorMemory{IdosoID: idosoID}
		err := rows.Scan(
			&e.ID, &e.ErrorType, &e.ErrorDescription, &e.OriginalSeverity, &e.CurrentWeight,
			&e.DaysSinceError, &e.BehaviorChanged, &e.ForgivenessScore, &e.CanBeMentioned,
			&e.ErrorOccurredAt,
		)
		if err != nil {
			continue
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
	query := `
		SELECT current_load, fatigue_level, is_fatigued,
		       response_length_modifier, suggest_lighter_topics, request_pause,
		       session_load_accumulated
		FROM patient_empathic_load
		WHERE idoso_id = $1
	`

	load := &EmpathicLoad{}
	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&load.CurrentLoad, &load.FatigueLevel, &load.IsFatigued,
		&load.ResponseLengthModifier, &load.SuggestLighterTopics, &load.RequestPause,
		&load.SessionLoadAccumulated,
	)
	if err == sql.ErrNoRows {
		s.db.ExecContext(ctx, "SELECT initialize_superhuman_consciousness($1)", idosoID)
		return s.GetEmpathicLoad(ctx, idosoID)
	}
	return load, err
}

// AddEmpathicLoad adds load when processing emotional content
func (s *ConsciousnessService) AddEmpathicLoad(ctx context.Context, idosoID int64, eventType string, gravity float64) (*EmpathicLoad, error) {
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT add_empathic_load($1, $2, $3)", idosoID, eventType, gravity).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	return s.GetEmpathicLoad(ctx, idosoID)
}

// RecoverEmpathicLoad processes recovery (lighter topics, pause)
func (s *ConsciousnessService) RecoverEmpathicLoad(ctx context.Context, idosoID int64, recoveryType string) (*EmpathicLoad, error) {
	return s.AddEmpathicLoad(ctx, idosoID, recoveryType, 0)
}

// StartSession marks the start of a new session
func (s *ConsciousnessService) StartSession(ctx context.Context, idosoID int64) error {
	query := `
		UPDATE patient_empathic_load
		SET session_start = NOW(),
		    session_load_accumulated = 0,
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query, idosoID)
	return err
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
	var resultJSON []byte
	err := s.db.QueryRowContext(ctx, "SELECT calculate_intervention_readiness($1)", idosoID).Scan(&resultJSON)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	json.Unmarshal(resultJSON, &result)

	ir := &InterventionReadiness{
		ReadinessScore:  result["readiness_score"].(float64),
		CanIntervene:    result["can_intervene"].(bool),
		PatternStrength: result["pattern_strength"].(float64),
		Rapport:         result["rapport"].(float64),
		CurrentMode:     result["current_mode"].(string),
		InCooldown:      result["in_cooldown"].(bool),
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
	query := `
		UPDATE patient_intervention_readiness
		SET last_intervention_type = $2,
		    last_intervention_at = NOW(),
		    last_intervention_outcome = $3,
		    intervention_cooldown_until = NOW() + INTERVAL '24 hours',
		    updated_at = NOW()
		WHERE idoso_id = $1
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, interventionType, outcome)
	return err
}

// =====================================================
// MIRROR OUTPUTS (Consciousness Reflections)
// =====================================================

// GenerateConsciousnessMirror generates mirror outputs from consciousness data
func (s *ConsciousnessService) GenerateConsciousnessMirror(ctx context.Context, idosoID int64) ([]*MirrorOutput, error) {
	var outputs []*MirrorOutput

	// 1. Heavy memories affecting responses
	heavyMemories, _ := s.GetHeavyMemories(ctx, idosoID, 0.8)
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
	cycles, _ := s.GetMatureCycles(ctx, idosoID)
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
	contradictions, _ := s.GetNarrativeContradictions(ctx, idosoID)
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
	load, _ := s.GetEmpathicLoad(ctx, idosoID)
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
