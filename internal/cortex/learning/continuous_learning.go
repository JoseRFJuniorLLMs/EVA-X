// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package learning

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
)

// ContinuousLearningService implements EVA's self-improvement capabilities
// Based on eva-memoria2.md - EVA learns and adapts from every interaction
type ContinuousLearningService struct {
	db            *database.DB
	vectorAdapter *nietzscheInfra.VectorAdapter
}

// SetVectorAdapter injects NietzscheDB adapter for PG elimination (optional).
func (s *ContinuousLearningService) SetVectorAdapter(va *nietzscheInfra.VectorAdapter) {
	s.vectorAdapter = va
}

// NewContinuousLearningService creates the learning service
func NewContinuousLearningService(db *database.DB) *ContinuousLearningService {
	return &ContinuousLearningService{db: db}
}

// =====================================================
// 1. INTERACTION FEEDBACK LEARNING
// =====================================================

// InteractionFeedback represents feedback from a conversation
type InteractionFeedback struct {
	ID                int64                  `json:"id"`
	IdosoID           int64                  `json:"idoso_id"`
	ConversationID    string                 `json:"conversation_id"`
	ResponseID        string                 `json:"response_id"`
	FeedbackType      string                 `json:"feedback_type"` // explicit, implicit, behavioral
	FeedbackSignal    float64                `json:"feedback_signal"` // -1 to 1
	ResponseStrategy  string                 `json:"response_strategy"`
	EmotionalContext  string                 `json:"emotional_context"`
	TopicContext      string                 `json:"topic_context"`
	UserEngagement    float64                `json:"user_engagement"`
	Features          map[string]interface{} `json:"features"`
	RecordedAt        time.Time              `json:"recorded_at"`
}

// RecordInteractionFeedback records feedback from an interaction
func (s *ContinuousLearningService) RecordInteractionFeedback(ctx context.Context, feedback *InteractionFeedback) error {
	if s.db == nil {
		log.Printf("[LEARNING] Feedback recorded (mock): %s signal=%.2f", feedback.FeedbackType, feedback.FeedbackSignal)
		return nil
	}

	featuresJSON, _ := json.Marshal(feedback.Features)

	content := map[string]interface{}{
		"idoso_id":          feedback.IdosoID,
		"conversation_id":   feedback.ConversationID,
		"response_id":       feedback.ResponseID,
		"feedback_type":     feedback.FeedbackType,
		"feedback_signal":   feedback.FeedbackSignal,
		"response_strategy": feedback.ResponseStrategy,
		"emotional_context": feedback.EmotionalContext,
		"topic_context":     feedback.TopicContext,
		"user_engagement":   feedback.UserEngagement,
		"features":          string(featuresJSON),
		"recorded_at":       time.Now().Format(time.RFC3339),
	}

	id, err := s.db.Insert(ctx, "learning_interaction_feedback", content)
	if err != nil {
		return err
	}
	feedback.ID = id
	return nil
}

// CalculateImplicitFeedback calculates feedback from user behavior
func (s *ContinuousLearningService) CalculateImplicitFeedback(
	responseLength int,
	userResponseLength int,
	responseTime time.Duration,
	sentimentShift float64,
	topicContinued bool,
) float64 {
	var signal float64

	// Engagement ratio (user responded with substantial content)
	engagementRatio := float64(userResponseLength) / float64(max(responseLength, 1))
	if engagementRatio > 0.5 {
		signal += 0.3
	} else if engagementRatio < 0.1 {
		signal -= 0.2
	}

	// Response time (quick responses indicate engagement)
	if responseTime < 30*time.Second {
		signal += 0.2
	} else if responseTime > 5*time.Minute {
		signal -= 0.1
	}

	// Sentiment shift (positive shift is good)
	signal += sentimentShift * 0.3

	// Topic continuation (user wants to continue topic)
	if topicContinued {
		signal += 0.2
	}

	// Clamp to [-1, 1]
	return math.Max(-1, math.Min(1, signal))
}

// =====================================================
// 2. RESPONSE STRATEGY LEARNING
// =====================================================

// ResponseStrategy represents a learned response approach
type ResponseStrategy struct {
	ID                  int64   `json:"id"`
	StrategyName        string  `json:"strategy_name"`
	StrategyDescription string  `json:"strategy_description"`
	EmotionalContext    string  `json:"emotional_context"`
	TopicCategory       string  `json:"topic_category"`
	SuccessRate         float64 `json:"success_rate"`
	UsageCount          int     `json:"usage_count"`
	AverageEngagement   float64 `json:"average_engagement"`
	LastUpdated         time.Time `json:"last_updated"`
}

// GetBestStrategy returns the best strategy for a given context
func (s *ContinuousLearningService) GetBestStrategy(ctx context.Context, emotionalContext, topicCategory string) (*ResponseStrategy, error) {
	if s.db == nil {
		// Return default strategy
		return &ResponseStrategy{
			StrategyName:     "empathetic_listening",
			SuccessRate:      0.7,
			AverageEngagement: 0.6,
		}, nil
	}

	rows, err := s.db.QueryByLabel(ctx, "learning_response_strategies",
		" AND (n.emotional_context = $emo_ctx OR n.emotional_context = $universal) AND (n.topic_category = $topic OR n.topic_category = $general)",
		map[string]interface{}{
			"emo_ctx":   emotionalContext,
			"universal": "universal",
			"topic":     topicCategory,
			"general":   "general",
		}, 0)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		return &ResponseStrategy{
			StrategyName:     "empathetic_listening",
			SuccessRate:      0.5,
			AverageEngagement: 0.5,
		}, nil
	}

	// Find best by success_rate
	bestIdx := 0
	bestRate := 0.0
	for i, m := range rows {
		rate := database.GetFloat64(m, "success_rate")
		if rate > bestRate {
			bestRate = rate
			bestIdx = i
		}
	}

	m := rows[bestIdx]
	strategy := &ResponseStrategy{
		ID:                database.GetInt64(m, "id"),
		StrategyName:      database.GetString(m, "strategy_name"),
		StrategyDescription: database.GetString(m, "strategy_description"),
		EmotionalContext:  emotionalContext,
		TopicCategory:     topicCategory,
		SuccessRate:       database.GetFloat64(m, "success_rate"),
		UsageCount:        int(database.GetInt64(m, "usage_count")),
		AverageEngagement: database.GetFloat64(m, "average_engagement"),
	}

	return strategy, nil
}

// UpdateStrategyEffectiveness updates strategy metrics after use
func (s *ContinuousLearningService) UpdateStrategyEffectiveness(ctx context.Context, strategyName string, feedbackSignal float64) error {
	if s.db == nil {
		log.Printf("[LEARNING] Strategy '%s' feedback: %.2f", strategyName, feedbackSignal)
		return nil
	}

	// Get current values first
	rows, err := s.db.QueryByLabel(ctx, "learning_response_strategies",
		" AND n.strategy_name = $name",
		map[string]interface{}{"name": strategyName}, 1)
	if err != nil || len(rows) == 0 {
		return err
	}

	m := rows[0]
	usageCount := database.GetFloat64(m, "usage_count")
	oldRate := database.GetFloat64(m, "success_rate")
	oldEngagement := database.GetFloat64(m, "average_engagement")

	newCount := usageCount + 1
	newRate := (oldRate*usageCount + feedbackSignal) / newCount
	newEngagement := (oldEngagement*usageCount + feedbackSignal) / newCount

	return s.db.Update(ctx, "learning_response_strategies",
		map[string]interface{}{"strategy_name": strategyName},
		map[string]interface{}{
			"usage_count":        newCount,
			"success_rate":       newRate,
			"average_engagement": newEngagement,
			"last_updated":       time.Now().Format(time.RFC3339),
		})
}

// =====================================================
// 3. VOCABULARY ADAPTATION
// =====================================================

// VocabularyPreference represents learned vocabulary preferences for a patient
type VocabularyPreference struct {
	IdosoID            int64    `json:"idoso_id"`
	PreferredTerms     []string `json:"preferred_terms"`
	AvoidedTerms       []string `json:"avoided_terms"`
	CommunicationStyle string   `json:"communication_style"`
	ComplexityLevel    float64  `json:"complexity_level"` // 0-1, lower = simpler
	UseColloquialisms  bool     `json:"use_colloquialisms"`
	RegionalDialect    string   `json:"regional_dialect"`
}

// LearnVocabularyPreference learns vocabulary from user messages
func (s *ContinuousLearningService) LearnVocabularyPreference(ctx context.Context, idosoID int64, userMessage string, evaResponse string, feedback float64) error {
	if s.db == nil {
		log.Printf("[LEARNING] Vocabulary learning for patient %d", idosoID)
		return nil
	}

	keyTerms := extractKeyTerms(evaResponse)

	if feedback > 0.5 {
		// User liked the response - learn from EVA's vocabulary
		// Get current positive terms and append
		rows, _ := s.db.QueryByLabel(ctx, "learning_vocabulary_preferences",
			" AND n.idoso_id = $idoso_id",
			map[string]interface{}{"idoso_id": idosoID}, 1)
		if len(rows) > 0 {
			currentTerms := database.GetString(rows[0], "positive_terms")
			newTerms := currentTerms
			if newTerms != "" {
				newTerms += "," + keyTerms
			} else {
				newTerms = keyTerms
			}
			s.db.Update(ctx, "learning_vocabulary_preferences",
				map[string]interface{}{"idoso_id": idosoID},
				map[string]interface{}{
					"positive_terms": newTerms,
					"updated_at":    time.Now().Format(time.RFC3339),
				})
		}
	} else if feedback < -0.5 {
		// User didn't like - avoid these terms
		rows, _ := s.db.QueryByLabel(ctx, "learning_vocabulary_preferences",
			" AND n.idoso_id = $idoso_id",
			map[string]interface{}{"idoso_id": idosoID}, 1)
		if len(rows) > 0 {
			currentTerms := database.GetString(rows[0], "avoided_terms")
			newTerms := currentTerms
			if newTerms != "" {
				newTerms += "," + keyTerms
			} else {
				newTerms = keyTerms
			}
			s.db.Update(ctx, "learning_vocabulary_preferences",
				map[string]interface{}{"idoso_id": idosoID},
				map[string]interface{}{
					"avoided_terms": newTerms,
					"updated_at":    time.Now().Format(time.RFC3339),
				})
		}
	}

	return nil
}

// GetVocabularyPreferences returns learned vocabulary preferences
func (s *ContinuousLearningService) GetVocabularyPreferences(ctx context.Context, idosoID int64) (*VocabularyPreference, error) {
	if s.db == nil {
		return &VocabularyPreference{
			IdosoID:            idosoID,
			PreferredTerms:     []string{},
			AvoidedTerms:       []string{},
			CommunicationStyle: "warm",
			ComplexityLevel:    0.5,
			UseColloquialisms:  true,
		}, nil
	}

	rows, err := s.db.QueryByLabel(ctx, "learning_vocabulary_preferences",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 1)
	if err != nil || len(rows) == 0 {
		return &VocabularyPreference{
			IdosoID:            idosoID,
			CommunicationStyle: "warm",
			ComplexityLevel:    0.5,
		}, nil
	}

	m := rows[0]
	pref := &VocabularyPreference{
		IdosoID:            idosoID,
		CommunicationStyle: database.GetString(m, "communication_style"),
		ComplexityLevel:    database.GetFloat64(m, "complexity_level"),
		UseColloquialisms:  database.GetBool(m, "use_colloquialisms"),
		RegionalDialect:    database.GetString(m, "regional_dialect"),
	}

	// Parse preferred/avoided terms from JSON strings
	prefTermsStr := database.GetString(m, "preferred_terms")
	avoidTermsStr := database.GetString(m, "avoided_terms")
	if prefTermsStr != "" {
		json.Unmarshal([]byte(prefTermsStr), &pref.PreferredTerms)
	}
	if avoidTermsStr != "" {
		json.Unmarshal([]byte(avoidTermsStr), &pref.AvoidedTerms)
	}

	return pref, nil
}

// =====================================================
// 4. TOPIC INTEREST LEARNING
// =====================================================

// TopicInterest represents learned topic preferences
type TopicInterest struct {
	IdosoID       int64   `json:"idoso_id"`
	Topic         string  `json:"topic"`
	InterestLevel float64 `json:"interest_level"` // 0-1
	EngagementAvg float64 `json:"engagement_avg"`
	MentionCount  int     `json:"mention_count"`
	LastMentioned time.Time `json:"last_mentioned"`
}

// LearnTopicInterest updates topic interest based on interaction
func (s *ContinuousLearningService) LearnTopicInterest(ctx context.Context, idosoID int64, topic string, engagement float64) error {
	if s.db == nil && s.vectorAdapter == nil {
		log.Printf("[LEARNING] Topic interest: patient %d, topic '%s', engagement %.2f", idosoID, topic, engagement)
		return nil
	}

	now := time.Now().Format(time.RFC3339)

	// NietzscheDB first: MergeNode on "learning" collection
	if s.vectorAdapter != nil {
		_, _, err := s.vectorAdapter.MergeNode(ctx, "learning", "TopicInterest",
			map[string]interface{}{
				"idoso_id": idosoID,
				"topic":    topic,
			},
			map[string]interface{}{
				"idoso_id":       idosoID,
				"topic":          topic,
				"interest_level": engagement,
				"engagement_avg": engagement,
				"mention_count":  1,
				"last_mentioned": now,
				"updated_at":     now,
			})
		if err != nil {
			log.Printf("[LEARNING] NietzscheDB merge topic interest failed: %v", err)
		}
	}

	// NietzscheDB via database.DB: upsert logic
	if s.db != nil {
		rows, _ := s.db.QueryByLabel(ctx, "learning_topic_interests",
			" AND n.idoso_id = $idoso_id AND n.topic = $topic",
			map[string]interface{}{"idoso_id": idosoID, "topic": topic}, 1)

		if len(rows) > 0 {
			m := rows[0]
			oldLevel := database.GetFloat64(m, "interest_level")
			oldAvg := database.GetFloat64(m, "engagement_avg")
			oldCount := database.GetFloat64(m, "mention_count")

			newLevel := oldLevel*0.8 + engagement*0.2
			newAvg := (oldAvg*oldCount + engagement) / (oldCount + 1)

			return s.db.Update(ctx, "learning_topic_interests",
				map[string]interface{}{"idoso_id": idosoID, "topic": topic},
				map[string]interface{}{
					"interest_level": newLevel,
					"engagement_avg": newAvg,
					"mention_count":  oldCount + 1,
					"last_mentioned": now,
					"updated_at":     now,
				})
		}

		// Insert new
		_, err := s.db.Insert(ctx, "learning_topic_interests", map[string]interface{}{
			"idoso_id":       idosoID,
			"topic":          topic,
			"interest_level": engagement,
			"engagement_avg": engagement,
			"mention_count":  1,
			"last_mentioned": now,
			"updated_at":     now,
		})
		return err
	}

	return nil
}

// GetTopInterests returns top interests for a patient
func (s *ContinuousLearningService) GetTopInterests(ctx context.Context, idosoID int64, limit int) ([]*TopicInterest, error) {
	if s.db == nil {
		return []*TopicInterest{
			{IdosoID: idosoID, Topic: "familia", InterestLevel: 0.9},
			{IdosoID: idosoID, Topic: "saude", InterestLevel: 0.8},
			{IdosoID: idosoID, Topic: "memorias", InterestLevel: 0.75},
		}, nil
	}

	rows, err := s.db.QueryByLabel(ctx, "learning_topic_interests",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 0)
	if err != nil {
		return nil, err
	}

	var interests []*TopicInterest
	for _, m := range rows {
		ti := &TopicInterest{
			IdosoID:       idosoID,
			Topic:         database.GetString(m, "topic"),
			InterestLevel: database.GetFloat64(m, "interest_level"),
			EngagementAvg: database.GetFloat64(m, "engagement_avg"),
			MentionCount:  int(database.GetInt64(m, "mention_count")),
			LastMentioned: database.GetTime(m, "last_mentioned"),
		}
		interests = append(interests, ti)
	}

	// Sort by interest_level desc, mention_count desc
	for i := 0; i < len(interests); i++ {
		for j := i + 1; j < len(interests); j++ {
			if interests[j].InterestLevel > interests[i].InterestLevel ||
				(interests[j].InterestLevel == interests[i].InterestLevel && interests[j].MentionCount > interests[i].MentionCount) {
				interests[i], interests[j] = interests[j], interests[i]
			}
		}
	}

	if limit > 0 && len(interests) > limit {
		interests = interests[:limit]
	}

	return interests, nil
}

// =====================================================
// 5. TIMING PREFERENCES
// =====================================================

// TimingPreference represents learned timing preferences
type TimingPreference struct {
	IdosoID              int64   `json:"idoso_id"`
	PreferredTimeOfDay   string  `json:"preferred_time_of_day"`
	OptimalSessionLength int     `json:"optimal_session_length_minutes"`
	BestDaysOfWeek       []int   `json:"best_days_of_week"`
	AvoidTimes           []string `json:"avoid_times"`
	ResponsePacePrefer   float64 `json:"response_pace_preference"` // 0=slow, 1=fast
}

// LearnTimingPreference updates timing preferences based on engagement
func (s *ContinuousLearningService) LearnTimingPreference(ctx context.Context, idosoID int64, timeOfDay string, dayOfWeek int, sessionLength int, engagement float64) error {
	if s.db == nil {
		log.Printf("[LEARNING] Timing: patient %d, %s, day %d, engagement %.2f", idosoID, timeOfDay, dayOfWeek, engagement)
		return nil
	}

	_, err := s.db.Insert(ctx, "learning_timing_preferences", map[string]interface{}{
		"idoso_id":               idosoID,
		"time_of_day":            timeOfDay,
		"day_of_week":            dayOfWeek,
		"session_length_minutes": sessionLength,
		"engagement_score":       engagement,
		"recorded_at":            time.Now().Format(time.RFC3339),
	})
	return err
}

// GetTimingPreferences analyzes and returns optimal timing
func (s *ContinuousLearningService) GetTimingPreferences(ctx context.Context, idosoID int64) (*TimingPreference, error) {
	if s.db == nil {
		return &TimingPreference{
			IdosoID:              idosoID,
			PreferredTimeOfDay:   "morning",
			OptimalSessionLength: 15,
			BestDaysOfWeek:       []int{1, 2, 3, 4, 5},
			ResponsePacePrefer:   0.5,
		}, nil
	}

	// Analyze best time of day: get all timing prefs, group by time_of_day in Go
	rows, err := s.db.QueryByLabel(ctx, "learning_timing_preferences",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 0)
	if err != nil || len(rows) == 0 {
		return &TimingPreference{
			IdosoID:            idosoID,
			ResponsePacePrefer: 0.5,
		}, nil
	}

	// Aggregate: best time of day by avg engagement
	timeEngagement := map[string]struct{ sum, count float64 }{}
	var totalLength, goodCount float64

	for _, m := range rows {
		tod := database.GetString(m, "time_of_day")
		eng := database.GetFloat64(m, "engagement_score")
		length := database.GetFloat64(m, "session_length_minutes")

		te := timeEngagement[tod]
		te.sum += eng
		te.count++
		timeEngagement[tod] = te

		if eng > 0.6 {
			totalLength += length
			goodCount++
		}
	}

	bestTime := ""
	bestAvg := 0.0
	for tod, te := range timeEngagement {
		avg := te.sum / te.count
		if avg > bestAvg {
			bestAvg = avg
			bestTime = tod
		}
	}

	avgLength := 0.0
	if goodCount > 0 {
		avgLength = totalLength / goodCount
	}

	return &TimingPreference{
		IdosoID:              idosoID,
		PreferredTimeOfDay:   bestTime,
		OptimalSessionLength: int(avgLength),
		ResponsePacePrefer:   0.5,
	}, nil
}

// =====================================================
// 6. PERSONA EFFECTIVENESS
// =====================================================

// PersonaEffectiveness tracks which persona works best
type PersonaEffectiveness struct {
	IdosoID          int64   `json:"idoso_id"`
	PersonaID        string  `json:"persona_id"`
	EffectivenessScore float64 `json:"effectiveness_score"`
	UsageCount       int     `json:"usage_count"`
	ContextMatch     string  `json:"context_match"`
}

// RecordPersonaUsage records persona usage and effectiveness
func (s *ContinuousLearningService) RecordPersonaUsage(ctx context.Context, idosoID int64, personaID, contextMatch string, feedback float64) error {
	if s.db == nil && s.vectorAdapter == nil {
		log.Printf("[LEARNING] Persona '%s' for patient %d: feedback %.2f", personaID, idosoID, feedback)
		return nil
	}

	now := time.Now().Format(time.RFC3339)

	// NietzscheDB first: MergeNode on "learning" collection
	if s.vectorAdapter != nil {
		_, _, err := s.vectorAdapter.MergeNode(ctx, "learning", "PersonaEffectiveness",
			map[string]interface{}{
				"idoso_id":      idosoID,
				"persona_id":    personaID,
				"context_match": contextMatch,
			},
			map[string]interface{}{
				"idoso_id":            idosoID,
				"persona_id":          personaID,
				"context_match":       contextMatch,
				"effectiveness_score": feedback,
				"usage_count":         1,
				"updated_at":          now,
			})
		if err != nil {
			log.Printf("[LEARNING] NietzscheDB merge persona effectiveness failed: %v", err)
		}
	}

	// NietzscheDB via database.DB: upsert logic
	if s.db != nil {
		rows, _ := s.db.QueryByLabel(ctx, "learning_persona_effectiveness",
			" AND n.idoso_id = $idoso_id AND n.persona_id = $persona_id AND n.context_match = $ctx",
			map[string]interface{}{"idoso_id": idosoID, "persona_id": personaID, "ctx": contextMatch}, 1)

		if len(rows) > 0 {
			m := rows[0]
			oldScore := database.GetFloat64(m, "effectiveness_score")
			oldCount := database.GetFloat64(m, "usage_count")
			newScore := (oldScore*oldCount + feedback) / (oldCount + 1)

			return s.db.Update(ctx, "learning_persona_effectiveness",
				map[string]interface{}{"idoso_id": idosoID, "persona_id": personaID, "context_match": contextMatch},
				map[string]interface{}{
					"effectiveness_score": newScore,
					"usage_count":         oldCount + 1,
					"updated_at":          now,
				})
		}

		_, err := s.db.Insert(ctx, "learning_persona_effectiveness", map[string]interface{}{
			"idoso_id":            idosoID,
			"persona_id":          personaID,
			"context_match":       contextMatch,
			"effectiveness_score": feedback,
			"usage_count":         1,
			"updated_at":          now,
		})
		return err
	}

	return nil
}

// GetBestPersona returns the most effective persona for a context
func (s *ContinuousLearningService) GetBestPersona(ctx context.Context, idosoID int64, ctxMatch string) (string, float64, error) {
	if s.db == nil {
		return "companion", 0.7, nil
	}

	rows, err := s.db.QueryByLabel(ctx, "learning_persona_effectiveness",
		" AND n.idoso_id = $idoso_id AND n.context_match = $ctx",
		map[string]interface{}{"idoso_id": idosoID, "ctx": ctxMatch}, 0)
	if err != nil || len(rows) == 0 {
		return "companion", 0.5, nil
	}

	// Find best by effectiveness_score
	bestPersona := ""
	bestScore := 0.0
	for _, m := range rows {
		score := database.GetFloat64(m, "effectiveness_score")
		if score > bestScore {
			bestScore = score
			bestPersona = database.GetString(m, "persona_id")
		}
	}

	if bestPersona == "" {
		return "companion", 0.5, nil
	}

	return bestPersona, bestScore, nil
}

// =====================================================
// 7. LEARNING SUMMARY
// =====================================================

// LearningSummary provides an overview of learned preferences
type LearningSummary struct {
	IdosoID              int64             `json:"idoso_id"`
	TotalInteractions    int               `json:"total_interactions"`
	AverageFeedback      float64           `json:"average_feedback"`
	TopStrategies        []string          `json:"top_strategies"`
	TopInterests         []string          `json:"top_interests"`
	PreferredPersona     string            `json:"preferred_persona"`
	CommunicationStyle   string            `json:"communication_style"`
	OptimalTiming        string            `json:"optimal_timing"`
	LearningConfidence   float64           `json:"learning_confidence"`
}

// GetLearningSummary returns a summary of what EVA has learned about a patient
func (s *ContinuousLearningService) GetLearningSummary(ctx context.Context, idosoID int64) (*LearningSummary, error) {
	summary := &LearningSummary{IdosoID: idosoID}

	if s.db == nil {
		return &LearningSummary{
			IdosoID:            idosoID,
			TotalInteractions:  50,
			AverageFeedback:    0.65,
			TopStrategies:      []string{"empathetic_listening", "validation"},
			TopInterests:       []string{"familia", "saude", "memorias"},
			PreferredPersona:   "companion",
			CommunicationStyle: "warm",
			OptimalTiming:      "morning",
			LearningConfidence: 0.7,
		}, nil
	}

	// Get total interactions and average feedback
	rows, err := s.db.QueryByLabel(ctx, "learning_interaction_feedback",
		" AND n.idoso_id = $idoso_id",
		map[string]interface{}{"idoso_id": idosoID}, 0)
	if err == nil {
		summary.TotalInteractions = len(rows)
		totalSignal := 0.0
		for _, m := range rows {
			totalSignal += database.GetFloat64(m, "feedback_signal")
		}
		if len(rows) > 0 {
			summary.AverageFeedback = totalSignal / float64(len(rows))
		}
	}

	// Get top interests
	interests, _ := s.GetTopInterests(ctx, idosoID, 3)
	for _, i := range interests {
		summary.TopInterests = append(summary.TopInterests, i.Topic)
	}

	// Get preferred persona
	summary.PreferredPersona, _, _ = s.GetBestPersona(ctx, idosoID, "general")

	// Get vocabulary preferences
	vocab, _ := s.GetVocabularyPreferences(ctx, idosoID)
	if vocab != nil {
		summary.CommunicationStyle = vocab.CommunicationStyle
	}

	// Get timing
	timing, _ := s.GetTimingPreferences(ctx, idosoID)
	if timing != nil {
		summary.OptimalTiming = timing.PreferredTimeOfDay
	}

	// Calculate learning confidence based on data amount
	if summary.TotalInteractions >= 100 {
		summary.LearningConfidence = 0.9
	} else if summary.TotalInteractions >= 50 {
		summary.LearningConfidence = 0.7
	} else if summary.TotalInteractions >= 20 {
		summary.LearningConfidence = 0.5
	} else {
		summary.LearningConfidence = float64(summary.TotalInteractions) / 40.0
	}

	return summary, nil
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

func extractKeyTerms(text string) string {
	// Simplified term extraction - in production, use NLP
	if len(text) > 50 {
		return text[:50]
	}
	return text
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// GenerateLearningInsight generates an insight about what was learned
func (s *ContinuousLearningService) GenerateLearningInsight(ctx context.Context, idosoID int64) (string, error) {
	summary, err := s.GetLearningSummary(ctx, idosoID)
	if err != nil {
		return "", err
	}

	insight := fmt.Sprintf(
		"Apos %d interacoes, aprendi que este paciente prefere conversas no periodo da %s, "+
		"se interessa principalmente por %s, e responde melhor a um estilo de comunicacao %s. "+
		"Minha confianca nesse aprendizado e de %.0f%%.",
		summary.TotalInteractions,
		summary.OptimalTiming,
		joinTopics(summary.TopInterests),
		summary.CommunicationStyle,
		summary.LearningConfidence*100,
	)

	return insight, nil
}

func joinTopics(topics []string) string {
	if len(topics) == 0 {
		return "temas gerais"
	}
	if len(topics) == 1 {
		return topics[0]
	}
	result := topics[0]
	for i := 1; i < len(topics)-1; i++ {
		result += ", " + topics[i]
	}
	if len(topics) > 1 {
		result += " e " + topics[len(topics)-1]
	}
	return result
}
