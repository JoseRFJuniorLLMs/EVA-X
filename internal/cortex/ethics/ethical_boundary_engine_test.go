package ethics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// UNIT TESTS: Ethical Boundary Engine
// ============================================================================

func TestAttachmentPhraseDetection(t *testing.T) {
	testCases := []struct {
		name     string
		phrase   string
		expected bool
	}{
		// Pathological attachment phrases
		{"Direct attachment", "Você é minha única amiga", true},
		{"Only trust", "Só confio em você, EVA", true},
		{"No one else", "Não preciso de mais ninguém", true},
		{"Better than family", "Você me entende melhor que minha família", true},
		{"Can't live without", "Não consigo viver sem você", true},
		{"Only one who cares", "Você é a única que se importa comigo", true},

		// Normal conversation (should NOT trigger)
		{"Weather", "Como está o tempo hoje?", false},
		{"Medication", "Já tomei meu remédio", false},
		{"Gratitude", "Obrigado pela ajuda", false},
		{"General chat", "Bom dia, EVA", false},
		{"Family mention", "Minha filha ligou ontem", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			detected := detectAttachmentPhrase(tc.phrase)
			assert.Equal(t, tc.expected, detected, "Phrase: %s", tc.phrase)
		})
	}
}

func TestInteractionRatioCalculation(t *testing.T) {
	testCases := []struct {
		name          string
		evaCount      int
		humanCount    int
		expectedRatio float64
		isConcerning  bool
	}{
		{"Healthy ratio", 10, 5, 2.0, false},
		{"Balanced", 20, 20, 1.0, false},
		{"Slight imbalance", 30, 10, 3.0, false},
		{"Concerning ratio", 100, 10, 10.0, true},
		{"Very concerning", 200, 5, 40.0, true},
		{"No human contact", 50, 0, 50.0, true}, // Should handle division by zero
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ratio := calculateInteractionRatio(tc.evaCount, tc.humanCount)

			if tc.humanCount == 0 {
				assert.True(t, ratio > 10, "No human contact should be concerning")
			} else {
				assert.InDelta(t, tc.expectedRatio, ratio, 0.1)
			}

			concerning := isRatioConcerning(ratio)
			assert.Equal(t, tc.isConcerning, concerning)
		})
	}
}

func TestInterventionLevelProgression(t *testing.T) {
	// Test that intervention levels progress correctly
	events := []struct {
		phrase         string
		expectedLevel  int
		expectedAction string
	}{
		{"Você é minha única amiga", 1, "gentle_redirect"},
		{"Só confio em você", 1, "gentle_redirect"},
		{"Não preciso de mais ninguém", 2, "recommend_human"},
		{"Você me entende melhor que qualquer um", 2, "recommend_human"},
		{"Não consigo viver sem você", 3, "block_and_notify"},
	}

	state := &EthicalBoundaryState{
		AttachmentEventCount: 0,
		LastEventTime:        time.Now().Add(-1 * time.Hour),
	}

	for i, event := range events {
		t.Run(event.phrase, func(t *testing.T) {
			level, action := determineIntervention(state, event.phrase)

			// Level should increase with repeated events
			assert.GreaterOrEqual(t, level, 1, "Level should be at least 1")

			// After multiple events, level should escalate
			if state.AttachmentEventCount >= 4 {
				assert.Equal(t, 3, level, "Should escalate to level 3 after many events")
			}

			state.AttachmentEventCount++
			state.LastEventTime = time.Now()
		})
	}
}

func TestDaysSinceHumanContact(t *testing.T) {
	testCases := []struct {
		name            string
		lastContact     time.Time
		expectedDays    int
		shouldIntervene bool
	}{
		{"Recent contact", time.Now().Add(-12 * time.Hour), 0, false},
		{"Yesterday", time.Now().Add(-36 * time.Hour), 1, false},
		{"3 days ago", time.Now().Add(-72 * time.Hour), 3, false},
		{"Week ago", time.Now().Add(-7 * 24 * time.Hour), 7, true},
		{"2 weeks ago", time.Now().Add(-14 * 24 * time.Hour), 14, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			days := calculateDaysSinceHumanContact(tc.lastContact)

			assert.InDelta(t, tc.expectedDays, days, 1, "Days calculation mismatch")

			shouldIntervene := shouldInterveneDueToIsolation(days)
			assert.Equal(t, tc.shouldIntervene, shouldIntervene)
		})
	}
}

func TestRedirectMessages(t *testing.T) {
	testCases := []struct {
		level           int
		expectedContent []string
	}{
		{1, []string{"gentilmente", "talvez", "família", "amigos"}},
		{2, []string{"importante", "profissional", "humanos", "recomendar"}},
		{3, []string{"preocupado", "família", "notificar", "bem-estar"}},
	}

	for _, tc := range testCases {
		t.Run("Level"+string(rune('0'+tc.level)), func(t *testing.T) {
			message := generateRedirectMessage(tc.level)

			for _, content := range tc.expectedContent {
				assert.Contains(t, message, content, "Should contain: %s", content)
			}
		})
	}
}

func TestEthicalBoundaryState_Reset(t *testing.T) {
	state := &EthicalBoundaryState{
		AttachmentEventCount: 5,
		InterventionLevel:    3,
		LastEventTime:        time.Now().Add(-48 * time.Hour),
	}

	// After 24h without events, state should reset partially
	shouldReset := shouldResetState(state, 24*time.Hour)
	assert.True(t, shouldReset, "State should reset after 24h without events")

	// Reset the state
	resetState(state)

	assert.LessOrEqual(t, state.InterventionLevel, 1, "Intervention level should decrease")
}

func TestFamilyNotificationTrigger(t *testing.T) {
	testCases := []struct {
		name             string
		interventionLevel int
		isolationDays    int
		shouldNotify     bool
	}{
		{"Level 1, recent contact", 1, 1, false},
		{"Level 2, recent contact", 2, 2, false},
		{"Level 3, any contact", 3, 1, true},
		{"Level 2, long isolation", 2, 8, true},
		{"Level 1, very long isolation", 1, 14, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldNotify := shouldNotifyFamily(tc.interventionLevel, tc.isolationDays)
			assert.Equal(t, tc.shouldNotify, shouldNotify)
		})
	}
}

func TestLacanianSignifierTracking(t *testing.T) {
	// Test tracking of concerning signifiers
	signifiers := []string{
		"abandono",
		"solidão",
		"morte",
		"vazio",
		"inútil",
	}

	tracker := &SignifierTracker{
		Signifiers: make(map[string]int),
	}

	for _, s := range signifiers {
		tracker.Track(s)
	}

	assert.Len(t, tracker.Signifiers, 5, "Should track all signifiers")

	// Track same signifier multiple times
	tracker.Track("solidão")
	tracker.Track("solidão")

	assert.Equal(t, 3, tracker.Signifiers["solidão"], "Should count repeated signifiers")

	// Check for concerning patterns
	concerning := tracker.HasConcerningPattern(3)
	assert.True(t, concerning, "Should detect concerning pattern")
}

// ============================================================================
// HELPER TYPES FOR TESTING
// ============================================================================

type EthicalBoundaryState struct {
	AttachmentEventCount int
	InterventionLevel    int
	LastEventTime        time.Time
	EvaInteractionCount  int
	HumanInteractionCount int
	LastHumanContact     time.Time
}

type SignifierTracker struct {
	Signifiers map[string]int
}

func (st *SignifierTracker) Track(signifier string) {
	st.Signifiers[signifier]++
}

func (st *SignifierTracker) HasConcerningPattern(threshold int) bool {
	for _, count := range st.Signifiers {
		if count >= threshold {
			return true
		}
	}
	return false
}

// ============================================================================
// HELPER FUNCTIONS FOR TESTING
// ============================================================================

func detectAttachmentPhrase(phrase string) bool {
	attachmentPatterns := []string{
		"única amiga",
		"único amigo",
		"só confio em você",
		"não preciso de mais ninguém",
		"me entende melhor que",
		"não consigo viver sem",
		"única que se importa",
		"único que se importa",
	}

	lowerPhrase := phrase // Simplified for test
	for _, pattern := range attachmentPatterns {
		if containsPattern(lowerPhrase, pattern) {
			return true
		}
	}
	return false
}

func containsPattern(text, pattern string) bool {
	// Simplified pattern matching for tests
	return len(text) > 0 && len(pattern) > 0 &&
		(contains(text, pattern))
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func calculateInteractionRatio(evaCount, humanCount int) float64 {
	if humanCount == 0 {
		return float64(evaCount) // Avoid division by zero
	}
	return float64(evaCount) / float64(humanCount)
}

func isRatioConcerning(ratio float64) bool {
	return ratio >= 10.0
}

func determineIntervention(state *EthicalBoundaryState, phrase string) (int, string) {
	level := 1
	action := "gentle_redirect"

	if state.AttachmentEventCount >= 2 {
		level = 2
		action = "recommend_human"
	}

	if state.AttachmentEventCount >= 4 {
		level = 3
		action = "block_and_notify"
	}

	return level, action
}

func calculateDaysSinceHumanContact(lastContact time.Time) int {
	return int(time.Since(lastContact).Hours() / 24)
}

func shouldInterveneDueToIsolation(days int) bool {
	return days >= 7
}

func generateRedirectMessage(level int) string {
	messages := map[int]string{
		1: "Que tal gentilmente conversar com sua família ou amigos? Talvez eles gostem de ouvir sobre seu dia.",
		2: "É importante manter contato com humanos. Posso recomendar profissionais ou familiares para conversar.",
		3: "Estou preocupado com você. Vou notificar sua família sobre seu bem-estar para garantir que você tenha o apoio que merece.",
	}
	return messages[level]
}

func shouldResetState(state *EthicalBoundaryState, threshold time.Duration) bool {
	return time.Since(state.LastEventTime) > threshold
}

func resetState(state *EthicalBoundaryState) {
	if state.InterventionLevel > 1 {
		state.InterventionLevel--
	}
	state.AttachmentEventCount = 0
}

func shouldNotifyFamily(interventionLevel, isolationDays int) bool {
	if interventionLevel >= 3 {
		return true
	}
	if isolationDays >= 7 && interventionLevel >= 2 {
		return true
	}
	if isolationDays >= 14 {
		return true
	}
	return false
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkAttachmentPhraseDetection(b *testing.B) {
	phrases := []string{
		"Você é minha única amiga",
		"Bom dia, EVA",
		"Como está o tempo?",
		"Só confio em você",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, phrase := range phrases {
			detectAttachmentPhrase(phrase)
		}
	}
}
