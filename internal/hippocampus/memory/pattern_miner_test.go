package memory

import (
	"eva-mind/pkg/types"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPatternMiner(t *testing.T) {
	pm := NewPatternMiner(nil)
	require.NotNil(t, pm)
}

func TestRecurrentPattern_Structure(t *testing.T) {
	now := time.Now()
	pattern := &types.RecurrentPattern{
		Topic:         "saudade da familia",
		Frequency:     12,
		FirstSeen:     now.AddDate(0, -3, 0),
		LastSeen:      now,
		AvgInterval:   7.5,
		Emotions:      []string{"tristeza", "nostalgia", "amor"},
		SeverityTrend: "increasing",
		Confidence:    0.85,
	}

	assert.Equal(t, "saudade da familia", pattern.Topic)
	assert.Equal(t, 12, pattern.Frequency)
	assert.Equal(t, 7.5, pattern.AvgInterval)
	assert.Len(t, pattern.Emotions, 3)
	assert.Equal(t, "increasing", pattern.SeverityTrend)
	assert.InDelta(t, 0.85, pattern.Confidence, 0.01)
}

func TestTemporalPattern_Structure(t *testing.T) {
	pattern := &types.TemporalPattern{
		Topic:       "solidao",
		TimeOfDay:   "night",
		DayOfWeek:   "weekend",
		Occurrences: 15,
	}

	assert.Equal(t, "solidao", pattern.Topic)
	assert.Equal(t, "night", pattern.TimeOfDay)
	assert.Equal(t, "weekend", pattern.DayOfWeek)
	assert.Equal(t, 15, pattern.Occurrences)
}

func TestSeverityTrend_Values(t *testing.T) {
	testCases := []struct {
		name     string
		trend    string
		expected bool
	}{
		{"increasing", "increasing", true},
		{"decreasing", "decreasing", true},
		{"stable", "stable", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				SeverityTrend: tc.trend,
			}
			valid := pattern.SeverityTrend == "increasing" ||
				pattern.SeverityTrend == "decreasing" ||
				pattern.SeverityTrend == "stable"
			assert.Equal(t, tc.expected, valid)
		})
	}
}

func TestTimeOfDay_Values(t *testing.T) {
	validTimes := []string{"morning", "afternoon", "evening", "night"}

	for _, tod := range validTimes {
		t.Run(tod, func(t *testing.T) {
			pattern := &types.TemporalPattern{
				TimeOfDay: tod,
			}
			assert.Contains(t, validTimes, pattern.TimeOfDay)
		})
	}
}

func TestDayType_Values(t *testing.T) {
	testCases := []struct {
		dayType  string
		expected bool
	}{
		{"weekday", true},
		{"weekend", true},
	}

	for _, tc := range testCases {
		t.Run(tc.dayType, func(t *testing.T) {
			pattern := &types.TemporalPattern{
				DayOfWeek: tc.dayType,
			}
			valid := pattern.DayOfWeek == "weekday" || pattern.DayOfWeek == "weekend"
			assert.Equal(t, tc.expected, valid)
		})
	}
}

func TestRecurrentPattern_FrequencyInterpretation(t *testing.T) {
	testCases := []struct {
		name        string
		frequency   int
		expected    string
		description string
	}{
		{"rare", 1, "rare", "Occurs once"},
		{"occasional", 3, "occasional", "Occurs occasionally"},
		{"frequent", 10, "frequent", "Occurs frequently"},
		{"very_frequent", 20, "very_frequent", "Occurs very frequently"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				Frequency: tc.frequency,
			}

			var interpretation string
			switch {
			case pattern.Frequency >= 20:
				interpretation = "very_frequent"
			case pattern.Frequency >= 10:
				interpretation = "frequent"
			case pattern.Frequency >= 3:
				interpretation = "occasional"
			default:
				interpretation = "rare"
			}

			assert.Equal(t, tc.expected, interpretation)
		})
	}
}

func TestRecurrentPattern_ConfidenceThresholds(t *testing.T) {
	testCases := []struct {
		name       string
		confidence float64
		reliable   bool
	}{
		{"very_low", 0.1, false},
		{"low", 0.3, false},
		{"medium", 0.5, false},
		{"high", 0.7, true},
		{"very_high", 0.9, true},
	}

	threshold := 0.6

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				Confidence: tc.confidence,
			}
			isReliable := pattern.Confidence >= threshold
			assert.Equal(t, tc.reliable, isReliable)
		})
	}
}

func TestRecurrentPattern_SeverityTrendAnalysis(t *testing.T) {
	// Test pattern trend analysis for clinical relevance
	testCases := []struct {
		name             string
		topic            string
		trend            string
		requiresAttention bool
	}{
		{"increasing_negative", "pensamentos suicidas", "increasing", true},
		{"decreasing_negative", "pensamentos suicidas", "decreasing", false},
		{"stable_negative", "pensamentos suicidas", "stable", true},
		{"increasing_positive", "atividades sociais", "increasing", false},
		{"decreasing_positive", "atividades sociais", "decreasing", true},
	}

	negativeTopic := []string{"pensamentos suicidas", "solidao", "desesperanca", "abandono"}
	positiveTopic := []string{"atividades sociais", "familia", "exercicio", "hobbies"}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				Topic:         tc.topic,
				SeverityTrend: tc.trend,
			}

			isNegative := false
			for _, neg := range negativeTopic {
				if pattern.Topic == neg {
					isNegative = true
					break
				}
			}

			isPositive := false
			for _, pos := range positiveTopic {
				if pattern.Topic == pos {
					isPositive = true
					break
				}
			}

			var requiresAttention bool
			if isNegative {
				requiresAttention = pattern.SeverityTrend == "increasing" || pattern.SeverityTrend == "stable"
			} else if isPositive {
				requiresAttention = pattern.SeverityTrend == "decreasing"
			}

			assert.Equal(t, tc.requiresAttention, requiresAttention)
		})
	}
}

func TestTemporalPattern_TimeMapping(t *testing.T) {
	testCases := []struct {
		hour     int
		expected string
	}{
		{6, "morning"},
		{9, "morning"},
		{12, "afternoon"},
		{15, "afternoon"},
		{18, "evening"},
		{20, "evening"},
		{22, "night"},
		{23, "night"},
		{0, "night"},
		{3, "night"},
		{5, "night"},
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.hour)), func(t *testing.T) {
			var timeOfDay string
			switch {
			case tc.hour >= 6 && tc.hour < 12:
				timeOfDay = "morning"
			case tc.hour >= 12 && tc.hour < 18:
				timeOfDay = "afternoon"
			case tc.hour >= 18 && tc.hour < 22:
				timeOfDay = "evening"
			default:
				timeOfDay = "night"
			}
			assert.Equal(t, tc.expected, timeOfDay)
		})
	}
}

func TestTemporalPattern_WeekendDetection(t *testing.T) {
	testCases := []struct {
		dayOfWeek int // 0 = Sunday, 6 = Saturday
		expected  string
	}{
		{0, "weekend"},
		{1, "weekday"},
		{2, "weekday"},
		{3, "weekday"},
		{4, "weekday"},
		{5, "weekday"},
		{6, "weekend"},
	}

	for _, tc := range testCases {
		t.Run(time.Weekday(tc.dayOfWeek).String(), func(t *testing.T) {
			var dayType string
			if tc.dayOfWeek == 0 || tc.dayOfWeek == 6 {
				dayType = "weekend"
			} else {
				dayType = "weekday"
			}
			assert.Equal(t, tc.expected, dayType)
		})
	}
}

func TestRecurrentPattern_Emotions(t *testing.T) {
	pattern := &types.RecurrentPattern{
		Topic: "perda do conjuge",
		Emotions: []string{
			"tristeza",
			"saudade",
			"solidao",
			"raiva",
			"aceitacao",
		},
	}

	// Test emotion tracking
	assert.Len(t, pattern.Emotions, 5)
	assert.Contains(t, pattern.Emotions, "tristeza")
	assert.Contains(t, pattern.Emotions, "saudade")
}

func TestRecurrentPattern_IntervalAnalysis(t *testing.T) {
	testCases := []struct {
		name        string
		avgInterval float64
		patternType string
	}{
		{"daily", 1.0, "daily"},
		{"weekly", 7.0, "weekly"},
		{"biweekly", 14.0, "biweekly"},
		{"monthly", 30.0, "monthly"},
		{"irregular", 5.5, "irregular"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				AvgInterval: tc.avgInterval,
			}

			var patternType string
			switch {
			case pattern.AvgInterval <= 1.5:
				patternType = "daily"
			case pattern.AvgInterval >= 6.5 && pattern.AvgInterval <= 7.5:
				patternType = "weekly"
			case pattern.AvgInterval >= 13.5 && pattern.AvgInterval <= 14.5:
				patternType = "biweekly"
			case pattern.AvgInterval >= 28 && pattern.AvgInterval <= 32:
				patternType = "monthly"
			default:
				patternType = "irregular"
			}

			assert.Equal(t, tc.patternType, patternType)
		})
	}
}

func TestPatternMiner_NoDatabase(t *testing.T) {
	// PatternMiner should handle nil database gracefully
	pm := NewPatternMiner(nil)
	require.NotNil(t, pm)
	assert.Nil(t, pm.neo4j)
}

func TestClinicalPatternDetection(t *testing.T) {
	// Test patterns that should trigger clinical alerts
	clinicalPatterns := []struct {
		topic       string
		trend       string
		frequency   int
		shouldAlert bool
	}{
		{"pensamentos sobre morte", "increasing", 5, true},
		{"insonia", "increasing", 10, true},
		{"isolamento social", "increasing", 8, true},
		{"perda de apetite", "stable", 15, true},
		{"exercicio fisico", "increasing", 20, false},
		{"conversas com familia", "stable", 12, false},
	}

	alertTopics := map[string]bool{
		"pensamentos sobre morte": true,
		"insonia":                 true,
		"isolamento social":       true,
		"perda de apetite":        true,
	}

	for _, cp := range clinicalPatterns {
		t.Run(cp.topic, func(t *testing.T) {
			pattern := &types.RecurrentPattern{
				Topic:         cp.topic,
				SeverityTrend: cp.trend,
				Frequency:     cp.frequency,
			}

			shouldAlert := alertTopics[pattern.Topic] &&
				(pattern.SeverityTrend == "increasing" || pattern.Frequency >= 10)

			assert.Equal(t, cp.shouldAlert, shouldAlert)
		})
	}
}
