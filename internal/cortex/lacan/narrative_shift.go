// Package lacan - Narrative Shift Detection
// Reference: "Detecting Narrative Shifts through Persistent Structures" (arXiv:2506.14836, 2025)
// Simplified for single-conversation density: uses embedding-based shift detection
// instead of full persistent homology (insufficient data density in individual sessions).
//
// Detects: abrupt topic changes, emotional flips, and topic circling (rumination).
// Integrates with Lacan signifier system to flag clinically relevant avoidance patterns.
package lacan

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"eva-mind/internal/brainstem/infrastructure/graph"
)

// ShiftType categorizes the type of narrative shift
type ShiftType string

const (
	ShiftAbruptChange  ShiftType = "abrupt_change"  // Sudden topic switch (low cosine sim)
	ShiftEmotionalFlip ShiftType = "emotional_flip"  // Positive ↔ Negative valence change
	ShiftTopicReturn   ShiftType = "topic_return"     // Circling back to earlier topic (rumination)
)

// SessionMessage represents a single message in a conversation session
type SessionMessage struct {
	Content   string    `json:"content"`
	Speaker   string    `json:"speaker"`
	Timestamp time.Time `json:"timestamp"`
	Embedding []float32 `json:"embedding,omitempty"`
}

// ShiftEvent records a detected narrative shift
type ShiftEvent struct {
	FromContent    string    `json:"from_content"`
	ToContent      string    `json:"to_content"`
	FromTopics     []string  `json:"from_topics"`
	ToTopics       []string  `json:"to_topics"`
	CosineDelta    float64   `json:"cosine_delta"`    // 1 - cosineSim (higher = more different)
	EmotionalShift float64   `json:"emotional_shift"` // magnitude of emotional change
	Timestamp      time.Time `json:"timestamp"`
	SessionID      string    `json:"session_id"`
	ShiftType      ShiftType `json:"shift_type"`
	MessageIndex   int       `json:"message_index"`
}

// AvoidanceProfile aggregates shift patterns across sessions
type AvoidanceProfile struct {
	PatientID        int64          `json:"patient_id"`
	AvoidedTopics    []AvoidedTopic `json:"avoided_topics"`
	CircularTopics   []string       `json:"circular_topics"`    // Topics with repeated return
	EmotionalFlipCount int          `json:"emotional_flip_count"`
	AnalyzedSessions int            `json:"analyzed_sessions"`
	LookbackDays     int            `json:"lookback_days"`
}

// AvoidedTopic represents a topic that the patient tends to avoid
type AvoidedTopic struct {
	Topic          string    `json:"topic"`
	AvoidanceCount int       `json:"avoidance_count"`
	LastAvoided    time.Time `json:"last_avoided"`
	AvgCosineDelta float64   `json:"avg_cosine_delta"`
}

// NarrativeShiftDetector detects conversation shifts and avoidance patterns
type NarrativeShiftDetector struct {
	neo4j      *graph.Neo4jClient
	signifiers *SignifierService

	// Thresholds
	abruptThreshold    float64 // cosine sim below this = abrupt change (default: 0.3)
	returnGapThreshold int     // messages gap for topic_return detection (default: 5)
}

// NewNarrativeShiftDetector creates a new detector
func NewNarrativeShiftDetector(
	neo4j *graph.Neo4jClient,
	signifiers *SignifierService,
) *NarrativeShiftDetector {
	return &NarrativeShiftDetector{
		neo4j:              neo4j,
		signifiers:         signifiers,
		abruptThreshold:    0.3,
		returnGapThreshold: 5,
	}
}

// DetectShiftsInSession analyzes a conversation session for narrative shifts.
// Operates on pre-computed embeddings (no API calls).
func (d *NarrativeShiftDetector) DetectShiftsInSession(
	sessionID string,
	messages []SessionMessage,
) []ShiftEvent {
	if len(messages) < 2 {
		return nil
	}

	var shifts []ShiftEvent

	// Track topics for circling detection
	type topicOccurrence struct {
		topic    string
		msgIndex int
	}
	var topicHistory []topicOccurrence

	// Seed topic history with first message
	if messages[0].Speaker == "user" {
		for _, topic := range extractTopicKeywords(messages[0].Content) {
			topicHistory = append(topicHistory, topicOccurrence{topic, 0})
		}
	}

	for i := 1; i < len(messages); i++ {
		prev := messages[i-1]
		curr := messages[i]

		// Only analyze user messages (not assistant)
		if curr.Speaker != "user" {
			continue
		}

		// 1. Abrupt Change Detection (embedding-based)
		if len(prev.Embedding) > 0 && len(curr.Embedding) > 0 {
			sim := cosineSim32(prev.Embedding, curr.Embedding)
			if sim < d.abruptThreshold {
				shifts = append(shifts, ShiftEvent{
					FromContent:  truncate(prev.Content, 100),
					ToContent:    truncate(curr.Content, 100),
					FromTopics:   extractTopicKeywords(prev.Content),
					ToTopics:     extractTopicKeywords(curr.Content),
					CosineDelta:  1.0 - sim,
					Timestamp:    curr.Timestamp,
					SessionID:    sessionID,
					ShiftType:    ShiftAbruptChange,
					MessageIndex: i,
				})
			}
		}

		// 2. Emotional Flip Detection
		prevValence := computeEmotionalValence(prev.Content)
		currValence := computeEmotionalValence(curr.Content)
		emotionalDelta := math.Abs(currValence - prevValence)

		if emotionalDelta > 1.0 { // significant flip (e.g., +0.5 → -0.5)
			shifts = append(shifts, ShiftEvent{
				FromContent:    truncate(prev.Content, 100),
				ToContent:      truncate(curr.Content, 100),
				EmotionalShift: emotionalDelta,
				Timestamp:      curr.Timestamp,
				SessionID:      sessionID,
				ShiftType:      ShiftEmotionalFlip,
				MessageIndex:   i,
			})
		}

		// 3. Topic Return Detection (circling)
		currTopics := extractTopicKeywords(curr.Content)
		for _, topic := range currTopics {
			topicHistory = append(topicHistory, topicOccurrence{topic, i})
		}

		for _, topic := range currTopics {
			for _, prev := range topicHistory {
				if prev.topic == topic && (i-prev.msgIndex) >= d.returnGapThreshold {
					shifts = append(shifts, ShiftEvent{
						FromContent:  topic,
						ToContent:    truncate(curr.Content, 100),
						ToTopics:     []string{topic},
						Timestamp:    curr.Timestamp,
						SessionID:    sessionID,
						ShiftType:    ShiftTopicReturn,
						MessageIndex: i,
					})
					break // only flag once per topic per message
				}
			}
		}
	}

	return shifts
}

// BuildAvoidanceProfile aggregates shift events into a patient avoidance profile
func (d *NarrativeShiftDetector) BuildAvoidanceProfile(
	ctx context.Context,
	patientID int64,
	shifts []ShiftEvent,
	lookbackDays int,
) *AvoidanceProfile {
	profile := &AvoidanceProfile{
		PatientID:    patientID,
		LookbackDays: lookbackDays,
	}

	// Count avoidance by topic (topics in FROM of abrupt changes)
	topicAvoidance := make(map[string]*AvoidedTopic)
	circularCount := make(map[string]int)

	for _, shift := range shifts {
		switch shift.ShiftType {
		case ShiftAbruptChange:
			for _, topic := range shift.FromTopics {
				if at, ok := topicAvoidance[topic]; ok {
					at.AvoidanceCount++
					at.AvgCosineDelta = (at.AvgCosineDelta*float64(at.AvoidanceCount-1) + shift.CosineDelta) / float64(at.AvoidanceCount)
					if shift.Timestamp.After(at.LastAvoided) {
						at.LastAvoided = shift.Timestamp
					}
				} else {
					topicAvoidance[topic] = &AvoidedTopic{
						Topic:          topic,
						AvoidanceCount: 1,
						LastAvoided:    shift.Timestamp,
						AvgCosineDelta: shift.CosineDelta,
					}
				}
			}

		case ShiftTopicReturn:
			for _, topic := range shift.ToTopics {
				circularCount[topic]++
			}

		case ShiftEmotionalFlip:
			profile.EmotionalFlipCount++
		}
	}

	// Convert to sorted slices
	for _, at := range topicAvoidance {
		if at.AvoidanceCount >= 2 { // minimum 2 occurrences to be "avoided"
			profile.AvoidedTopics = append(profile.AvoidedTopics, *at)
		}
	}
	sort.Slice(profile.AvoidedTopics, func(i, j int) bool {
		return profile.AvoidedTopics[i].AvoidanceCount > profile.AvoidedTopics[j].AvoidanceCount
	})

	for topic, count := range circularCount {
		if count >= 2 { // minimum 2 returns
			profile.CircularTopics = append(profile.CircularTopics, topic)
		}
	}

	return profile
}

// GetAvoidedSignifiers cross-references avoidance profile with Lacan signifier system.
// Returns topics that are both avoided AND known signifiers (highest clinical relevance).
func (d *NarrativeShiftDetector) GetAvoidedSignifiers(
	ctx context.Context,
	patientID int64,
	profile *AvoidanceProfile,
) []string {
	if d.signifiers == nil || profile == nil {
		return nil
	}

	// Get known signifiers for this patient
	signifiers, err := d.signifiers.GetKeySignifiers(ctx, patientID, 50)
	if err != nil {
		log.Printf("⚠️ [SHIFT] Error getting signifiers: %v", err)
		return nil
	}

	sigMap := make(map[string]float64)
	for _, sig := range signifiers {
		sigMap[strings.ToLower(sig.Word)] = sig.EmotionalCharge
	}

	// Cross-reference: avoided topics that are also signifiers
	type scored struct {
		topic string
		score float64
	}
	var matches []scored

	for _, avoided := range profile.AvoidedTopics {
		if charge, ok := sigMap[strings.ToLower(avoided.Topic)]; ok {
			matches = append(matches, scored{
				topic: avoided.Topic,
				score: float64(avoided.AvoidanceCount) * charge,
			})
		}
	}

	// Sort by score DESC
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].score > matches[j].score
	})

	result := make([]string, len(matches))
	for i, m := range matches {
		result[i] = m.topic
	}
	return result
}

// StoreShiftEvents persists detected shifts to Neo4j for historical analysis
func (d *NarrativeShiftDetector) StoreShiftEvents(ctx context.Context, patientID int64, shifts []ShiftEvent) error {
	if d.neo4j == nil || len(shifts) == 0 {
		return nil
	}

	query := `
		MATCH (p:Person {id: $patientId})
		WITH p
		UNWIND $shifts AS shift
		CREATE (s:NarrativeShift {
			shift_type: shift.type,
			cosine_delta: shift.delta,
			emotional_shift: shift.emotional,
			session_id: shift.session,
			detected_at: datetime(),
			from_content: shift.from_content,
			to_content: shift.to_content
		})
		CREATE (p)-[:HAS_SHIFT]->(s)
	`

	shiftParams := make([]map[string]interface{}, len(shifts))
	for i, s := range shifts {
		shiftParams[i] = map[string]interface{}{
			"type":         string(s.ShiftType),
			"delta":        s.CosineDelta,
			"emotional":    s.EmotionalShift,
			"session":      s.SessionID,
			"from_content": s.FromContent,
			"to_content":   s.ToContent,
		}
	}

	_, err := d.neo4j.ExecuteWrite(ctx, query, map[string]interface{}{
		"patientId": patientID,
		"shifts":    shiftParams,
	})
	if err != nil {
		return fmt.Errorf("failed to store shift events: %w", err)
	}

	log.Printf("📊 [SHIFT] Stored %d shift events for patient %d", len(shifts), patientID)
	return nil
}

// --- Helper functions ---

// computeEmotionalValence returns a valence score from -1 (negative) to +1 (positive)
func computeEmotionalValence(text string) float64 {
	positiveWords := map[string]float64{
		"alegria": 0.8, "feliz": 0.7, "amor": 0.9, "bem": 0.5,
		"ótimo": 0.7, "bom": 0.5, "felicidade": 0.8, "esperança": 0.6,
		"contente": 0.6, "maravilhoso": 0.8, "bonito": 0.4,
	}
	negativeWords := map[string]float64{
		"triste": -0.7, "medo": -0.8, "dor": -0.7, "solidão": -0.8,
		"raiva": -0.7, "ansiedade": -0.6, "morte": -0.9, "culpa": -0.7,
		"mal": -0.5, "ruim": -0.5, "péssimo": -0.8, "sofrimento": -0.8,
		"angústia": -0.7, "desespero": -0.9, "abandono": -0.8,
	}

	words := strings.Fields(strings.ToLower(text))
	var totalValence float64
	var count int

	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:")
		if v, ok := positiveWords[cleaned]; ok {
			totalValence += v
			count++
		}
		if v, ok := negativeWords[cleaned]; ok {
			totalValence += v
			count++
		}
	}

	if count == 0 {
		return 0.0 // neutral
	}
	return totalValence / float64(count)
}

// extractTopicKeywords extracts content words (non-stopwords, non-short)
func extractTopicKeywords(text string) []string {
	stopwords := map[string]bool{
		"o": true, "a": true, "os": true, "as": true,
		"um": true, "uma": true, "de": true, "do": true, "da": true,
		"em": true, "no": true, "na": true, "para": true, "com": true,
		"que": true, "e": true, "é": true, "ou": true, "se": true,
		"eu": true, "ele": true, "ela": true, "me": true, "meu": true,
		"minha": true, "não": true, "sim": true, "como": true,
		"muito": true, "mais": true, "por": true, "dos": true, "das": true,
	}

	words := strings.Fields(strings.ToLower(text))
	seen := make(map[string]bool)
	var keywords []string

	for _, w := range words {
		cleaned := strings.Trim(w, ".,!?;:\"'()")
		if len(cleaned) > 2 && !stopwords[cleaned] && !seen[cleaned] {
			keywords = append(keywords, cleaned)
			seen[cleaned] = true
		}
	}

	return keywords
}

// cosineSim32 computes cosine similarity between two float32 vectors
func cosineSim32(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0.0
	}

	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA < 1e-10 || normB < 1e-10 {
		return 0.0
	}

	return dot / (normA * normB)
}

// truncate limits string length
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
