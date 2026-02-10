package learning

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"
)

// ContinuousLearningService implements EVA's self-improvement capabilities
// Based on eva-memoria2.md - EVA learns and adapts from every interaction
type ContinuousLearningService struct {
	db *sql.DB
}

// NewContinuousLearningService creates the learning service
func NewContinuousLearningService(db *sql.DB) *ContinuousLearningService {
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
		log.Printf("📚 [LEARNING] Feedback recorded (mock): %s signal=%.2f", feedback.FeedbackType, feedback.FeedbackSignal)
		return nil
	}

	featuresJSON, _ := json.Marshal(feedback.Features)

	query := `
		INSERT INTO learning_interaction_feedback
		(idoso_id, conversation_id, response_id, feedback_type, feedback_signal,
		 response_strategy, emotional_context, topic_context, user_engagement, features)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	return s.db.QueryRowContext(ctx, query,
		feedback.IdosoID, feedback.ConversationID, feedback.ResponseID,
		feedback.FeedbackType, feedback.FeedbackSignal, feedback.ResponseStrategy,
		feedback.EmotionalContext, feedback.TopicContext, feedback.UserEngagement,
		string(featuresJSON),
	).Scan(&feedback.ID)
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

	query := `
		SELECT id, strategy_name, strategy_description, success_rate,
		       usage_count, average_engagement
		FROM learning_response_strategies
		WHERE emotional_context = $1 OR emotional_context = 'universal'
		  AND (topic_category = $2 OR topic_category = 'general')
		ORDER BY success_rate DESC, usage_count DESC
		LIMIT 1
	`

	strategy := &ResponseStrategy{
		EmotionalContext: emotionalContext,
		TopicCategory:    topicCategory,
	}

	err := s.db.QueryRowContext(ctx, query, emotionalContext, topicCategory).Scan(
		&strategy.ID, &strategy.StrategyName, &strategy.StrategyDescription,
		&strategy.SuccessRate, &strategy.UsageCount, &strategy.AverageEngagement,
	)
	if err == sql.ErrNoRows {
		return &ResponseStrategy{
			StrategyName:     "empathetic_listening",
			SuccessRate:      0.5,
			AverageEngagement: 0.5,
		}, nil
	}

	return strategy, err
}

// UpdateStrategyEffectiveness updates strategy metrics after use
func (s *ContinuousLearningService) UpdateStrategyEffectiveness(ctx context.Context, strategyName string, feedbackSignal float64) error {
	if s.db == nil {
		log.Printf("📊 [LEARNING] Strategy '%s' feedback: %.2f", strategyName, feedbackSignal)
		return nil
	}

	query := `
		UPDATE learning_response_strategies
		SET usage_count = usage_count + 1,
		    success_rate = (success_rate * usage_count + $2) / (usage_count + 1),
		    average_engagement = (average_engagement * usage_count + $2) / (usage_count + 1),
		    last_updated = NOW()
		WHERE strategy_name = $1
	`
	_, err := s.db.ExecContext(ctx, query, strategyName, feedbackSignal)
	return err
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
		log.Printf("📝 [LEARNING] Vocabulary learning for patient %d", idosoID)
		return nil
	}

	// Extract terms from positive/negative feedback
	if feedback > 0.5 {
		// User liked the response - learn from EVA's vocabulary
		query := `
			UPDATE learning_vocabulary_preferences
			SET positive_terms = array_append(positive_terms, $2),
			    updated_at = NOW()
			WHERE idoso_id = $1
		`
		s.db.ExecContext(ctx, query, idosoID, extractKeyTerms(evaResponse))
	} else if feedback < -0.5 {
		// User didn't like - avoid these terms
		query := `
			UPDATE learning_vocabulary_preferences
			SET avoided_terms = array_append(avoided_terms, $2),
			    updated_at = NOW()
			WHERE idoso_id = $1
		`
		s.db.ExecContext(ctx, query, idosoID, extractKeyTerms(evaResponse))
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

	query := `
		SELECT preferred_terms, avoided_terms, communication_style,
		       complexity_level, use_colloquialisms, regional_dialect
		FROM learning_vocabulary_preferences
		WHERE idoso_id = $1
	`

	pref := &VocabularyPreference{IdosoID: idosoID}
	var prefTermsJSON, avoidTermsJSON []byte
	var dialect sql.NullString

	err := s.db.QueryRowContext(ctx, query, idosoID).Scan(
		&prefTermsJSON, &avoidTermsJSON, &pref.CommunicationStyle,
		&pref.ComplexityLevel, &pref.UseColloquialisms, &dialect,
	)
	if err == sql.ErrNoRows {
		return &VocabularyPreference{
			IdosoID:            idosoID,
			CommunicationStyle: "warm",
			ComplexityLevel:    0.5,
		}, nil
	}

	json.Unmarshal(prefTermsJSON, &pref.PreferredTerms)
	json.Unmarshal(avoidTermsJSON, &pref.AvoidedTerms)
	if dialect.Valid {
		pref.RegionalDialect = dialect.String
	}

	return pref, err
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
	if s.db == nil {
		log.Printf("🎯 [LEARNING] Topic interest: patient %d, topic '%s', engagement %.2f", idosoID, topic, engagement)
		return nil
	}

	query := `
		INSERT INTO learning_topic_interests (idoso_id, topic, interest_level, engagement_avg, mention_count)
		VALUES ($1, $2, $3, $3, 1)
		ON CONFLICT (idoso_id, topic) DO UPDATE SET
			interest_level = (learning_topic_interests.interest_level * 0.8) + ($3 * 0.2),
			engagement_avg = (learning_topic_interests.engagement_avg * learning_topic_interests.mention_count + $3) / (learning_topic_interests.mention_count + 1),
			mention_count = learning_topic_interests.mention_count + 1,
			last_mentioned = NOW(),
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, topic, engagement)
	return err
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

	query := `
		SELECT topic, interest_level, engagement_avg, mention_count, last_mentioned
		FROM learning_topic_interests
		WHERE idoso_id = $1
		ORDER BY interest_level DESC, mention_count DESC
		LIMIT $2
	`

	rows, err := s.db.QueryContext(ctx, query, idosoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var interests []*TopicInterest
	for rows.Next() {
		ti := &TopicInterest{IdosoID: idosoID}
		err := rows.Scan(&ti.Topic, &ti.InterestLevel, &ti.EngagementAvg,
			&ti.MentionCount, &ti.LastMentioned)
		if err != nil {
			continue
		}
		interests = append(interests, ti)
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
		log.Printf("⏰ [LEARNING] Timing: patient %d, %s, day %d, engagement %.2f", idosoID, timeOfDay, dayOfWeek, engagement)
		return nil
	}

	query := `
		INSERT INTO learning_timing_preferences
		(idoso_id, time_of_day, day_of_week, session_length_minutes, engagement_score)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, timeOfDay, dayOfWeek, sessionLength, engagement)
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

	// Analyze best time of day
	var bestTime string
	s.db.QueryRowContext(ctx, `
		SELECT time_of_day
		FROM learning_timing_preferences
		WHERE idoso_id = $1
		GROUP BY time_of_day
		ORDER BY AVG(engagement_score) DESC
		LIMIT 1
	`, idosoID).Scan(&bestTime)

	// Analyze optimal session length
	var avgLength float64
	s.db.QueryRowContext(ctx, `
		SELECT AVG(session_length_minutes)
		FROM learning_timing_preferences
		WHERE idoso_id = $1 AND engagement_score > 0.6
	`, idosoID).Scan(&avgLength)

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
func (s *ContinuousLearningService) RecordPersonaUsage(ctx context.Context, idosoID int64, personaID, context string, feedback float64) error {
	if s.db == nil {
		log.Printf("🎭 [LEARNING] Persona '%s' for patient %d: feedback %.2f", personaID, idosoID, feedback)
		return nil
	}

	query := `
		INSERT INTO learning_persona_effectiveness
		(idoso_id, persona_id, context_match, effectiveness_score, usage_count)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (idoso_id, persona_id, context_match) DO UPDATE SET
			effectiveness_score = (learning_persona_effectiveness.effectiveness_score * learning_persona_effectiveness.usage_count + $4) / (learning_persona_effectiveness.usage_count + 1),
			usage_count = learning_persona_effectiveness.usage_count + 1,
			updated_at = NOW()
	`
	_, err := s.db.ExecContext(ctx, query, idosoID, personaID, context, feedback)
	return err
}

// GetBestPersona returns the most effective persona for a context
func (s *ContinuousLearningService) GetBestPersona(ctx context.Context, idosoID int64, context string) (string, float64, error) {
	if s.db == nil {
		return "companion", 0.7, nil
	}

	var personaID string
	var effectiveness float64

	query := `
		SELECT persona_id, effectiveness_score
		FROM learning_persona_effectiveness
		WHERE idoso_id = $1 AND context_match = $2
		ORDER BY effectiveness_score DESC
		LIMIT 1
	`

	err := s.db.QueryRowContext(ctx, query, idosoID, context).Scan(&personaID, &effectiveness)
	if err == sql.ErrNoRows {
		return "companion", 0.5, nil
	}

	return personaID, effectiveness, err
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
	s.db.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(AVG(feedback_signal), 0)
		FROM learning_interaction_feedback
		WHERE idoso_id = $1
	`, idosoID).Scan(&summary.TotalInteractions, &summary.AverageFeedback)

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
		"Após %d interações, aprendi que este paciente prefere conversas no período da %s, "+
		"se interessa principalmente por %s, e responde melhor a um estilo de comunicação %s. "+
		"Minha confiança nesse aprendizado é de %.0f%%.",
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
