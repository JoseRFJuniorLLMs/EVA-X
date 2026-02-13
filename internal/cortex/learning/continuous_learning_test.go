package learning

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContinuousLearningService(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	require.NotNil(t, svc)
}

// =====================================================
// INTERACTION FEEDBACK TESTS
// =====================================================

func TestInteractionFeedback_Structure(t *testing.T) {
	feedback := &InteractionFeedback{
		ID:               1,
		IdosoID:          999,
		ConversationID:   "conv-123",
		ResponseID:       "resp-456",
		FeedbackType:     "implicit",
		FeedbackSignal:   0.7,
		ResponseStrategy: "empathetic_listening",
		EmotionalContext: "tristeza",
		TopicContext:     "familia",
		UserEngagement:   0.8,
		Features: map[string]interface{}{
			"response_length": 150,
			"user_response":   120,
		},
		RecordedAt: time.Now(),
	}

	assert.Equal(t, int64(999), feedback.IdosoID)
	assert.Equal(t, "implicit", feedback.FeedbackType)
	assert.InDelta(t, 0.7, feedback.FeedbackSignal, 0.01)
	assert.NotEmpty(t, feedback.Features)
}

func TestFeedbackTypes(t *testing.T) {
	validTypes := []string{"explicit", "implicit", "behavioral"}

	for _, ft := range validTypes {
		t.Run(ft, func(t *testing.T) {
			feedback := &InteractionFeedback{FeedbackType: ft}
			assert.NotEmpty(t, feedback.FeedbackType)
		})
	}
}

func TestCalculateImplicitFeedback(t *testing.T) {
	svc := NewContinuousLearningService(nil)

	testCases := []struct {
		name               string
		responseLength     int
		userResponseLength int
		responseTime       time.Duration
		sentimentShift     float64
		topicContinued     bool
		minExpected        float64
		maxExpected        float64
	}{
		{
			name:               "high_engagement",
			responseLength:     100,
			userResponseLength: 80,
			responseTime:       20 * time.Second,
			sentimentShift:     0.3,
			topicContinued:     true,
			minExpected:        0.5,
			maxExpected:        1.0,
		},
		{
			name:               "low_engagement",
			responseLength:     200,
			userResponseLength: 10,
			responseTime:       10 * time.Minute,
			sentimentShift:     -0.2,
			topicContinued:     false,
			minExpected:        -0.5,
			maxExpected:        0.0,
		},
		{
			name:               "neutral_engagement",
			responseLength:     100,
			userResponseLength: 30,
			responseTime:       1 * time.Minute,
			sentimentShift:     0.0,
			topicContinued:     true,
			minExpected:        0.0,
			maxExpected:        0.5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signal := svc.CalculateImplicitFeedback(
				tc.responseLength,
				tc.userResponseLength,
				tc.responseTime,
				tc.sentimentShift,
				tc.topicContinued,
			)
			assert.GreaterOrEqual(t, signal, tc.minExpected)
			assert.LessOrEqual(t, signal, tc.maxExpected)
		})
	}
}

func TestFeedbackSignal_Range(t *testing.T) {
	svc := NewContinuousLearningService(nil)

	// Test extreme cases - signal should always be [-1, 1]
	extremeCases := []struct {
		responseLength     int
		userResponseLength int
		responseTime       time.Duration
		sentimentShift     float64
		topicContinued     bool
	}{
		{1, 1000, 1 * time.Second, 1.0, true},   // Very positive
		{1000, 1, 1 * time.Hour, -1.0, false},   // Very negative
		{100, 0, 30 * time.Second, 0.0, false},  // No response
	}

	for _, tc := range extremeCases {
		signal := svc.CalculateImplicitFeedback(
			tc.responseLength,
			tc.userResponseLength,
			tc.responseTime,
			tc.sentimentShift,
			tc.topicContinued,
		)
		assert.GreaterOrEqual(t, signal, -1.0)
		assert.LessOrEqual(t, signal, 1.0)
	}
}

func TestRecordInteractionFeedback_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	feedback := &InteractionFeedback{
		IdosoID:        999,
		FeedbackType:   "implicit",
		FeedbackSignal: 0.5,
	}

	err := svc.RecordInteractionFeedback(ctx, feedback)
	assert.NoError(t, err)
}

// =====================================================
// RESPONSE STRATEGY TESTS
// =====================================================

func TestResponseStrategy_Structure(t *testing.T) {
	strategy := &ResponseStrategy{
		ID:                  1,
		StrategyName:        "empathetic_listening",
		StrategyDescription: "Listen with empathy and validate feelings",
		EmotionalContext:    "tristeza",
		TopicCategory:       "perda",
		SuccessRate:         0.85,
		UsageCount:          150,
		AverageEngagement:   0.78,
		LastUpdated:         time.Now(),
	}

	assert.Equal(t, "empathetic_listening", strategy.StrategyName)
	assert.InDelta(t, 0.85, strategy.SuccessRate, 0.01)
}

func TestGetBestStrategy_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	strategy, err := svc.GetBestStrategy(ctx, "tristeza", "familia")

	require.NoError(t, err)
	require.NotNil(t, strategy)
	assert.NotEmpty(t, strategy.StrategyName)
	assert.Greater(t, strategy.SuccessRate, 0.0)
}

func TestStrategyNames(t *testing.T) {
	strategies := []string{
		"empathetic_listening",
		"validation",
		"gentle_challenge",
		"reframing",
		"problem_solving",
		"psychoeducation",
		"distraction",
		"grounding",
	}

	for _, s := range strategies {
		t.Run(s, func(t *testing.T) {
			strategy := &ResponseStrategy{StrategyName: s}
			assert.NotEmpty(t, strategy.StrategyName)
		})
	}
}

func TestUpdateStrategyEffectiveness_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	err := svc.UpdateStrategyEffectiveness(ctx, "empathetic_listening", 0.8)
	assert.NoError(t, err)
}

// =====================================================
// VOCABULARY PREFERENCE TESTS
// =====================================================

func TestVocabularyPreference_Structure(t *testing.T) {
	pref := &VocabularyPreference{
		IdosoID:            999,
		PreferredTerms:     []string{"querido", "meu bem", "vamos lá"},
		AvoidedTerms:       []string{"senhor", "idoso", "velho"},
		CommunicationStyle: "warm",
		ComplexityLevel:    0.3,
		UseColloquialisms:  true,
		RegionalDialect:    "mineiro",
	}

	assert.Equal(t, int64(999), pref.IdosoID)
	assert.Len(t, pref.PreferredTerms, 3)
	assert.Len(t, pref.AvoidedTerms, 3)
	assert.InDelta(t, 0.3, pref.ComplexityLevel, 0.01)
}

func TestCommunicationStyles(t *testing.T) {
	styles := []string{
		"warm",
		"professional",
		"casual",
		"nurturing",
		"direct",
	}

	for _, style := range styles {
		t.Run(style, func(t *testing.T) {
			pref := &VocabularyPreference{CommunicationStyle: style}
			assert.NotEmpty(t, pref.CommunicationStyle)
		})
	}
}

func TestComplexityLevel(t *testing.T) {
	testCases := []struct {
		name       string
		level      float64
		description string
	}{
		{"very_simple", 0.1, "muito simples"},
		{"simple", 0.3, "simples"},
		{"moderate", 0.5, "moderado"},
		{"complex", 0.7, "complexo"},
		{"very_complex", 0.9, "muito complexo"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var desc string
			switch {
			case tc.level < 0.2:
				desc = "muito simples"
			case tc.level < 0.4:
				desc = "simples"
			case tc.level < 0.6:
				desc = "moderado"
			case tc.level < 0.8:
				desc = "complexo"
			default:
				desc = "muito complexo"
			}
			assert.Equal(t, tc.description, desc)
		})
	}
}

func TestGetVocabularyPreferences_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	pref, err := svc.GetVocabularyPreferences(ctx, 999)

	require.NoError(t, err)
	require.NotNil(t, pref)
	assert.Equal(t, int64(999), pref.IdosoID)
	assert.NotEmpty(t, pref.CommunicationStyle)
}

// =====================================================
// TOPIC INTEREST TESTS
// =====================================================

func TestTopicInterest_Structure(t *testing.T) {
	interest := &TopicInterest{
		IdosoID:       999,
		Topic:         "familia",
		InterestLevel: 0.9,
		EngagementAvg: 0.85,
		MentionCount:  25,
		LastMentioned: time.Now(),
	}

	assert.Equal(t, "familia", interest.Topic)
	assert.InDelta(t, 0.9, interest.InterestLevel, 0.01)
	assert.Equal(t, 25, interest.MentionCount)
}

func TestLearnTopicInterest_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	err := svc.LearnTopicInterest(ctx, 999, "familia", 0.8)
	assert.NoError(t, err)
}

func TestGetTopInterests_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	interests, err := svc.GetTopInterests(ctx, 999, 5)

	require.NoError(t, err)
	require.NotEmpty(t, interests)
	assert.LessOrEqual(t, len(interests), 5)

	// Verify ordering (should be by interest level descending)
	for i := 1; i < len(interests); i++ {
		assert.GreaterOrEqual(t, interests[i-1].InterestLevel, interests[i].InterestLevel)
	}
}

func TestTopicCategories(t *testing.T) {
	categories := []string{
		"familia",
		"saude",
		"memorias",
		"hobbies",
		"espiritualidade",
		"relacionamentos",
		"trabalho",
		"medos",
	}

	for _, cat := range categories {
		t.Run(cat, func(t *testing.T) {
			interest := &TopicInterest{Topic: cat}
			assert.NotEmpty(t, interest.Topic)
		})
	}
}

// =====================================================
// TIMING PREFERENCE TESTS
// =====================================================

func TestTimingPreference_Structure(t *testing.T) {
	pref := &TimingPreference{
		IdosoID:              999,
		PreferredTimeOfDay:   "morning",
		OptimalSessionLength: 20,
		BestDaysOfWeek:       []int{1, 2, 3, 4, 5},
		AvoidTimes:           []string{"late_night"},
		ResponsePacePrefer:   0.6,
	}

	assert.Equal(t, "morning", pref.PreferredTimeOfDay)
	assert.Equal(t, 20, pref.OptimalSessionLength)
	assert.Len(t, pref.BestDaysOfWeek, 5)
}

func TestTimeOfDayValues(t *testing.T) {
	times := []string{"morning", "afternoon", "evening", "night"}

	for _, tod := range times {
		t.Run(tod, func(t *testing.T) {
			pref := &TimingPreference{PreferredTimeOfDay: tod}
			assert.NotEmpty(t, pref.PreferredTimeOfDay)
		})
	}
}

func TestLearnTimingPreference_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	err := svc.LearnTimingPreference(ctx, 999, "morning", 1, 15, 0.8)
	assert.NoError(t, err)
}

func TestGetTimingPreferences_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	pref, err := svc.GetTimingPreferences(ctx, 999)

	require.NoError(t, err)
	require.NotNil(t, pref)
	assert.NotEmpty(t, pref.PreferredTimeOfDay)
	assert.Greater(t, pref.OptimalSessionLength, 0)
}

func TestOptimalSessionLength(t *testing.T) {
	testCases := []struct {
		name     string
		length   int
		category string
	}{
		{"very_short", 5, "muito curta"},
		{"short", 10, "curta"},
		{"optimal", 20, "ideal"},
		{"long", 30, "longa"},
		{"very_long", 45, "muito longa"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var category string
			switch {
			case tc.length < 8:
				category = "muito curta"
			case tc.length < 15:
				category = "curta"
			case tc.length < 25:
				category = "ideal"
			case tc.length < 40:
				category = "longa"
			default:
				category = "muito longa"
			}
			assert.Equal(t, tc.category, category)
		})
	}
}

// =====================================================
// PERSONA EFFECTIVENESS TESTS
// =====================================================

func TestPersonaEffectiveness_Structure(t *testing.T) {
	pe := &PersonaEffectiveness{
		IdosoID:            999,
		PersonaID:          "companion",
		EffectivenessScore: 0.85,
		UsageCount:         50,
		ContextMatch:       "general",
	}

	assert.Equal(t, "companion", pe.PersonaID)
	assert.InDelta(t, 0.85, pe.EffectivenessScore, 0.01)
}

func TestRecordPersonaUsage_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	err := svc.RecordPersonaUsage(ctx, 999, "companion", "general", 0.8)
	assert.NoError(t, err)
}

func TestGetBestPersona_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	personaID, effectiveness, err := svc.GetBestPersona(ctx, 999, "general")

	require.NoError(t, err)
	assert.NotEmpty(t, personaID)
	assert.Greater(t, effectiveness, 0.0)
}

func TestPersonaIDs(t *testing.T) {
	personas := []string{
		"companion",
		"therapist",
		"coach",
		"crisis_counselor",
		"psychoeducator",
		"guardian_angel",
		"caregiver_support",
	}

	for _, p := range personas {
		t.Run(p, func(t *testing.T) {
			pe := &PersonaEffectiveness{PersonaID: p}
			assert.NotEmpty(t, pe.PersonaID)
		})
	}
}

// =====================================================
// LEARNING SUMMARY TESTS
// =====================================================

func TestLearningSummary_Structure(t *testing.T) {
	summary := &LearningSummary{
		IdosoID:            999,
		TotalInteractions:  100,
		AverageFeedback:    0.65,
		TopStrategies:      []string{"empathetic_listening", "validation"},
		TopInterests:       []string{"familia", "saude"},
		PreferredPersona:   "companion",
		CommunicationStyle: "warm",
		OptimalTiming:      "morning",
		LearningConfidence: 0.8,
	}

	assert.Equal(t, int64(999), summary.IdosoID)
	assert.Equal(t, 100, summary.TotalInteractions)
	assert.InDelta(t, 0.65, summary.AverageFeedback, 0.01)
	assert.InDelta(t, 0.8, summary.LearningConfidence, 0.01)
}

func TestGetLearningSummary_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	summary, err := svc.GetLearningSummary(ctx, 999)

	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, int64(999), summary.IdosoID)
	assert.NotEmpty(t, summary.TopInterests)
	assert.NotEmpty(t, summary.PreferredPersona)
}

func TestLearningConfidence_Calculation(t *testing.T) {
	testCases := []struct {
		name        string
		interactions int
		minConf     float64
		maxConf     float64
	}{
		{"very_few", 5, 0.0, 0.2},
		{"few", 15, 0.2, 0.5},
		{"moderate", 35, 0.5, 0.7},
		{"many", 75, 0.7, 0.9},
		{"extensive", 150, 0.9, 1.0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var confidence float64
			if tc.interactions >= 100 {
				confidence = 0.9
			} else if tc.interactions >= 50 {
				confidence = 0.7
			} else if tc.interactions >= 20 {
				confidence = 0.5
			} else {
				confidence = float64(tc.interactions) / 40.0
			}

			assert.GreaterOrEqual(t, confidence, tc.minConf)
			assert.LessOrEqual(t, confidence, tc.maxConf)
		})
	}
}

func TestGenerateLearningInsight_MockMode(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()

	insight, err := svc.GenerateLearningInsight(ctx, 999)

	require.NoError(t, err)
	assert.NotEmpty(t, insight)
	assert.Contains(t, insight, "interações")
}

// =====================================================
// HELPER FUNCTION TESTS
// =====================================================

func TestExtractKeyTerms(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		maxLen   int
	}{
		{"short", "Olá, como vai?", 14},
		{"long", "Esta é uma mensagem muito longa que excede o limite de cinquenta caracteres", 50},
		{"empty", "", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := extractKeyTerms(tc.input)
			assert.LessOrEqual(t, len(result), 50)
		})
	}
}

func TestMax(t *testing.T) {
	testCases := []struct {
		a, b     int
		expected int
	}{
		{1, 2, 2},
		{5, 3, 5},
		{0, 0, 0},
		{-1, 1, 1},
	}

	for _, tc := range testCases {
		result := max(tc.a, tc.b)
		assert.Equal(t, tc.expected, result)
	}
}

func TestJoinTopics(t *testing.T) {
	testCases := []struct {
		name     string
		topics   []string
		expected string
	}{
		{"empty", []string{}, "temas gerais"},
		{"single", []string{"familia"}, "familia"},
		{"double", []string{"familia", "saude"}, "familia e saude"},
		{"triple", []string{"familia", "saude", "memorias"}, "familia, saude e memorias"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := joinTopics(tc.topics)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// =====================================================
// INTEGRATION TESTS
// =====================================================

func TestLearningWorkflow(t *testing.T) {
	svc := NewContinuousLearningService(nil)
	ctx := context.Background()
	idosoID := int64(999)

	// 1. Record some interactions
	for i := 0; i < 5; i++ {
		feedback := &InteractionFeedback{
			IdosoID:          idosoID,
			FeedbackType:     "implicit",
			FeedbackSignal:   0.6 + float64(i)*0.05,
			ResponseStrategy: "empathetic_listening",
			TopicContext:     "familia",
		}
		err := svc.RecordInteractionFeedback(ctx, feedback)
		assert.NoError(t, err)
	}

	// 2. Learn topic interests
	topics := []string{"familia", "saude", "memorias"}
	for _, topic := range topics {
		err := svc.LearnTopicInterest(ctx, idosoID, topic, 0.7)
		assert.NoError(t, err)
	}

	// 3. Record persona usage
	err := svc.RecordPersonaUsage(ctx, idosoID, "companion", "general", 0.8)
	assert.NoError(t, err)

	// 4. Learn timing
	err = svc.LearnTimingPreference(ctx, idosoID, "morning", 1, 15, 0.9)
	assert.NoError(t, err)

	// 5. Get learning summary
	summary, err := svc.GetLearningSummary(ctx, idosoID)
	require.NoError(t, err)
	require.NotNil(t, summary)

	// 6. Generate insight
	insight, err := svc.GenerateLearningInsight(ctx, idosoID)
	require.NoError(t, err)
	assert.NotEmpty(t, insight)
}
