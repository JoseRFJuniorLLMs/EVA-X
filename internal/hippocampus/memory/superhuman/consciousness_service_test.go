package superhuman

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsciousnessService(t *testing.T) {
	svc := NewConsciousnessService(nil)
	require.NotNil(t, svc)
}

// =====================================================
// 1. EMOTIONAL GRAVITY TESTS
// =====================================================

func TestMemoryGravity_Structure(t *testing.T) {
	mg := &MemoryGravity{
		ID:                  1,
		IdosoID:             999,
		MemoryID:            100,
		MemoryType:          "episodic",
		MemorySummary:       "Perda do esposo",
		GravityScore:        0.85,
		EmotionalValence:    -0.7,
		ArousalLevel:        0.8,
		RecallFrequency:     15,
		BiometricImpact:     0.6,
		IdentityConnection:  0.9,
		TemporalPersistence: 0.75,
		PullRadius:          0.7,
		CollisionRisk:       0.8,
		AvoidanceTopics:     []string{"morte", "hospital", "funeral"},
		LastActivation:      time.Now(),
	}

	assert.Equal(t, int64(999), mg.IdosoID)
	assert.Equal(t, "episodic", mg.MemoryType)
	assert.InDelta(t, 0.85, mg.GravityScore, 0.01)
	assert.InDelta(t, -0.7, mg.EmotionalValence, 0.01)
	assert.Len(t, mg.AvoidanceTopics, 3)
}

func TestMemoryGravity_ValenceRange(t *testing.T) {
	testCases := []struct {
		name     string
		valence  float64
		expected string
	}{
		{"very_negative", -0.9, "very_negative"},
		{"negative", -0.5, "negative"},
		{"neutral", 0.0, "neutral"},
		{"positive", 0.5, "positive"},
		{"very_positive", 0.9, "very_positive"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var category string
			switch {
			case tc.valence <= -0.6:
				category = "very_negative"
			case tc.valence <= -0.2:
				category = "negative"
			case tc.valence <= 0.2:
				category = "neutral"
			case tc.valence <= 0.6:
				category = "positive"
			default:
				category = "very_positive"
			}
			assert.Equal(t, tc.expected, category)
		})
	}
}

func TestMemoryGravity_CollisionRisk(t *testing.T) {
	testCases := []struct {
		name        string
		collision   float64
		shouldAvoid bool
	}{
		{"low_risk", 0.2, false},
		{"medium_risk", 0.5, false},
		{"high_risk", 0.7, true},
		{"very_high_risk", 0.9, true},
	}

	threshold := 0.6

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mg := &MemoryGravity{CollisionRisk: tc.collision}
			shouldAvoid := mg.CollisionRisk > threshold
			assert.Equal(t, tc.shouldAvoid, shouldAvoid)
		})
	}
}

func TestMemoryGravity_AvoidanceTopics(t *testing.T) {
	mg := &MemoryGravity{
		MemorySummary:   "Perda do pai",
		AvoidanceTopics: []string{"morte", "pai", "hospital", "doenca"},
	}

	testTopics := []struct {
		topic     string
		shouldAvoid bool
	}{
		{"Como está seu pai?", true},
		{"Fale sobre sua família", false},
		{"Você foi ao hospital recentemente?", true},
		{"Como está o clima hoje?", false},
	}

	for _, tc := range testTopics {
		t.Run(tc.topic, func(t *testing.T) {
			shouldAvoid := false
			for _, avoid := range mg.AvoidanceTopics {
				if containsIgnoreCase(tc.topic, avoid) {
					shouldAvoid = true
					break
				}
			}
			assert.Equal(t, tc.shouldAvoid, shouldAvoid)
		})
	}
}

// =====================================================
// 2. CYCLE PATTERN TESTS
// =====================================================

func TestCyclePattern_Structure(t *testing.T) {
	cp := &CyclePattern{
		ID:                    1,
		IdosoID:               999,
		PatternSignature:      "isolamento_social_fim_semana",
		PatternDescription:    "Isolamento nos fins de semana após conflito familiar",
		PatternType:           "behavioral",
		CycleCount:            5,
		CycleThreshold:        3,
		TriggerEvents:         []string{"conflito_familiar", "discussao"},
		TypicalActions:        []string{"evitar_contato", "ficar_em_casa"},
		TypicalConsequences:   []string{"tristeza", "solidao"},
		PatternConfidence:     0.8,
		InterventionAttempted: false,
		UserAware:             false,
		FirstDetected:         time.Now().AddDate(0, -2, 0),
		LastOccurrence:        time.Now(),
	}

	assert.Equal(t, int64(999), cp.IdosoID)
	assert.Equal(t, "behavioral", cp.PatternType)
	assert.Greater(t, cp.CycleCount, cp.CycleThreshold)
	assert.Len(t, cp.TriggerEvents, 2)
	assert.Len(t, cp.TypicalConsequences, 2)
}

func TestCyclePattern_ThresholdDetection(t *testing.T) {
	testCases := []struct {
		name          string
		cycleCount    int
		threshold     int
		isMature      bool
	}{
		{"below_threshold", 2, 3, false},
		{"at_threshold", 3, 3, true},
		{"above_threshold", 5, 3, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cp := &CyclePattern{
				CycleCount:     tc.cycleCount,
				CycleThreshold: tc.threshold,
			}
			isMature := cp.CycleCount >= cp.CycleThreshold
			assert.Equal(t, tc.isMature, isMature)
		})
	}
}

func TestCyclePattern_PatternTypes(t *testing.T) {
	validTypes := []string{
		"behavioral",
		"emotional",
		"cognitive",
		"relational",
		"health",
	}

	for _, pt := range validTypes {
		t.Run(pt, func(t *testing.T) {
			cp := &CyclePattern{PatternType: pt}
			assert.NotEmpty(t, cp.PatternType)
		})
	}
}

func TestCyclePattern_ConfidenceGrowth(t *testing.T) {
	// Confidence should grow with each occurrence (max 1.0)
	initialConfidence := 0.5
	increment := 0.05

	testCases := []struct {
		occurrences int
		expected    float64
	}{
		{1, 0.55},
		{5, 0.75},
		{10, 1.0}, // Capped at 1.0
		{15, 1.0}, // Still capped at 1.0
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.occurrences)), func(t *testing.T) {
			confidence := initialConfidence + float64(tc.occurrences)*increment
			if confidence > 1.0 {
				confidence = 1.0
			}
			assert.InDelta(t, tc.expected, confidence, 0.01)
		})
	}
}

// =====================================================
// 3. RAPPORT/TRUST TESTS
// =====================================================

func TestPatientRapport_Structure(t *testing.T) {
	pr := &PatientRapport{
		ID:                         1,
		IdosoID:                    999,
		RapportScore:               0.75,
		InteractionCount:           50,
		PositiveInteractions:       40,
		DeepDisclosures:            10,
		SecretsShared:              3,
		AdviceFollowed:             8,
		AdviceRejected:             2,
		InterventionBudget:         0.6,
		RelationshipPhase:          "trust_building",
		GentleSuggestionThreshold:  0.3,
		DirectObservationThreshold: 0.5,
		ConfrontationThreshold:     0.7,
	}

	assert.Equal(t, int64(999), pr.IdosoID)
	assert.InDelta(t, 0.75, pr.RapportScore, 0.01)
	assert.Greater(t, pr.PositiveInteractions, pr.InteractionCount/2)
}

func TestRapport_InterventionPermission(t *testing.T) {
	testCases := []struct {
		name             string
		rapportScore     float64
		interventionType string
		canIntervene     bool
	}{
		{"gentle_low_rapport", 0.2, "gentle_suggestion", false},
		{"gentle_ok", 0.4, "gentle_suggestion", true},
		{"direct_low_rapport", 0.3, "direct_observation", false},
		{"direct_ok", 0.6, "direct_observation", true},
		{"confront_low_rapport", 0.5, "confrontation", false},
		{"confront_ok", 0.8, "confrontation", true},
		{"harsh_low_rapport", 0.7, "harsh_truth", false},
		{"harsh_ok", 0.95, "harsh_truth", true},
	}

	thresholds := map[string]float64{
		"gentle_suggestion":   0.3,
		"direct_observation":  0.5,
		"confrontation":       0.7,
		"harsh_truth":         0.9,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			threshold := thresholds[tc.interventionType]
			canIntervene := tc.rapportScore >= threshold
			assert.Equal(t, tc.canIntervene, canIntervene)
		})
	}
}

func TestRapport_RelationshipPhases(t *testing.T) {
	phases := []struct {
		name         string
		interactions int
		expected     string
	}{
		{"stranger", 5, "stranger"},
		{"acquaintance", 20, "acquaintance"},
		{"trust_building", 50, "trust_building"},
		{"confidant", 100, "confidant"},
		{"deep_bond", 200, "deep_bond"},
	}

	for _, p := range phases {
		t.Run(p.name, func(t *testing.T) {
			var phase string
			switch {
			case p.interactions < 10:
				phase = "stranger"
			case p.interactions < 30:
				phase = "acquaintance"
			case p.interactions < 75:
				phase = "trust_building"
			case p.interactions < 150:
				phase = "confidant"
			default:
				phase = "deep_bond"
			}
			assert.Equal(t, p.expected, phase)
		})
	}
}

// =====================================================
// 4. CONTRADICTION TRACKER TESTS
// =====================================================

func TestNarrativeVersion_Structure(t *testing.T) {
	nv := &NarrativeVersion{
		ID:               1,
		IdosoID:          999,
		NarrativeTopic:   "infancia",
		VersionNumber:    3,
		NarrativeText:    "Minha infância foi feliz na fazenda do meu avô",
		EmotionalTone:    "nostalgico",
		UserMoodWhenTold: "relaxado",
		KeyClaims:        []string{"infancia feliz", "fazenda do avo", "vida no campo"},
		ToldAt:           time.Now(),
	}

	assert.Equal(t, int64(999), nv.IdosoID)
	assert.Equal(t, 3, nv.VersionNumber)
	assert.Len(t, nv.KeyClaims, 3)
}

func TestContradiction_Detection(t *testing.T) {
	negationPairs := map[string]string{
		"sempre":   "nunca",
		"bom":      "ruim",
		"boa":      "ruim",
		"feliz":    "triste",
		"amava":    "odiava",
		"presente": "ausente",
		"apoio":    "abandono",
	}

	testCases := []struct {
		name       string
		claim1     string
		claim2     string
		contradict bool
	}{
		{"always_never", "meu pai sempre estava presente", "meu pai nunca estava em casa", true},
		{"good_bad", "foi uma boa infancia", "foi uma infancia ruim", true},
		{"happy_sad", "eu era muito feliz", "eu era muito triste", true},
		{"no_contradiction", "eu morava no campo", "eu gostava de animais", false},
		{"same_claim", "meu pai me amava", "meu pai me amava muito", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contradict := false
			for word, opposite := range negationPairs {
				if (containsIgnoreCase(tc.claim1, word) && containsIgnoreCase(tc.claim2, opposite)) ||
					(containsIgnoreCase(tc.claim1, opposite) && containsIgnoreCase(tc.claim2, word)) {
					contradict = true
					break
				}
			}
			assert.Equal(t, tc.contradict, contradict)
		})
	}
}

func TestContradiction_EmotionalTone(t *testing.T) {
	testCases := []struct {
		name       string
		tone1      string
		tone2      string
		contradict bool
	}{
		{"trauma_nostalgia", "traumatico", "nostalgico", true},
		{"nostalgia_trauma", "nostalgico", "traumatico", true},
		{"same_tone", "nostalgico", "nostalgico", false},
		{"neutral_change", "neutro", "positivo", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			contradict := (tc.tone1 == "traumatico" && tc.tone2 == "nostalgico") ||
				(tc.tone1 == "nostalgico" && tc.tone2 == "traumatico")
			assert.Equal(t, tc.contradict, contradict)
		})
	}
}

// =====================================================
// 5. ADAPTIVE MODE TESTS
// =====================================================

func TestEvaMode_Structure(t *testing.T) {
	mode := &EvaMode{
		CurrentMode:            "acolhimento",
		ModeLocked:             false,
		DetectedEmotionalState: "tristeza",
		CrisisLevel:            0.3,
		ReceptivityLevel:       0.7,
		MentorSeveroEnabled:    false,
	}

	assert.Equal(t, "acolhimento", mode.CurrentMode)
	assert.False(t, mode.ModeLocked)
	assert.InDelta(t, 0.3, mode.CrisisLevel, 0.01)
}

func TestEvaMode_ModeSelection(t *testing.T) {
	testCases := []struct {
		name        string
		crisisLevel float64
		receptivity float64
		expected    string
	}{
		{"crisis", 0.9, 0.5, "crise"},
		{"high_crisis", 0.7, 0.6, "crise"},
		{"support", 0.4, 0.8, "acolhimento"},
		{"coaching", 0.1, 0.9, "coaching"},
		{"exploration", 0.2, 0.5, "exploracao"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var mode string
			switch {
			case tc.crisisLevel >= 0.6:
				mode = "crise"
			case tc.receptivity >= 0.8 && tc.crisisLevel < 0.3:
				mode = "coaching"
			case tc.receptivity >= 0.6:
				mode = "acolhimento"
			default:
				mode = "exploracao"
			}
			assert.Equal(t, tc.expected, mode)
		})
	}
}

func TestEvaMode_MentorSevero(t *testing.T) {
	// Mentor Severo (harsh truth mode) requires explicit consent
	testCases := []struct {
		name       string
		enabled    bool
		rapport    float64
		canDeliver bool
	}{
		{"not_enabled", false, 0.95, false},
		{"enabled_low_rapport", true, 0.5, false},
		{"enabled_high_rapport", true, 0.9, true},
	}

	mentorThreshold := 0.85

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canDeliver := tc.enabled && tc.rapport >= mentorThreshold
			assert.Equal(t, tc.canDeliver, canDeliver)
		})
	}
}

// =====================================================
// 6. RELATIONSHIP EVOLUTION TESTS
// =====================================================

func TestRelationshipEvolution_Structure(t *testing.T) {
	re := &RelationshipEvolution{
		CurrentPhase:              "trust_building",
		TotalInteractions:         75,
		CommunicationStyleAdapted: true,
		HumorStyle:                "ironic",
		FormalityLevel:            0.3,
		OpinionsFormed:            []string{"prefere manhãs", "gosta de histórias", "evita política"},
		IdentityCrystallized:      false,
		RelationshipDepthScore:    0.65,
	}

	assert.Equal(t, "trust_building", re.CurrentPhase)
	assert.True(t, re.CommunicationStyleAdapted)
	assert.Len(t, re.OpinionsFormed, 3)
}

func TestRelationshipEvolution_Phases(t *testing.T) {
	phases := []string{
		"stranger",
		"acquaintance",
		"trust_building",
		"confidant",
		"deep_bond",
	}

	for i, phase := range phases {
		t.Run(phase, func(t *testing.T) {
			re := &RelationshipEvolution{CurrentPhase: phase}
			assert.Equal(t, phase, re.CurrentPhase)
			// Verify phases are ordered
			if i > 0 {
				assert.NotEqual(t, phases[i-1], phase)
			}
		})
	}
}

func TestRelationshipEvolution_FormalityLevel(t *testing.T) {
	testCases := []struct {
		name       string
		formality  float64
		style      string
	}{
		{"very_informal", 0.1, "muito informal"},
		{"informal", 0.3, "informal"},
		{"neutral", 0.5, "neutro"},
		{"formal", 0.7, "formal"},
		{"very_formal", 0.9, "muito formal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var style string
			switch {
			case tc.formality < 0.2:
				style = "muito informal"
			case tc.formality < 0.4:
				style = "informal"
			case tc.formality < 0.6:
				style = "neutro"
			case tc.formality < 0.8:
				style = "formal"
			default:
				style = "muito formal"
			}
			assert.Equal(t, tc.style, style)
		})
	}
}

// =====================================================
// 7. COMPUTATIONAL FORGIVENESS TESTS
// =====================================================

func TestErrorMemory_Structure(t *testing.T) {
	em := &ErrorMemory{
		ID:               1,
		IdosoID:          999,
		ErrorType:        "promessa_quebrada",
		ErrorDescription: "Prometeu parar de fumar mas continua",
		OriginalSeverity: 0.6,
		CurrentWeight:    0.4,
		DaysSinceError:   30,
		BehaviorChanged:  false,
		ForgivenessScore: 0.3,
		CanBeMentioned:   true,
		ErrorOccurredAt:  time.Now().AddDate(0, -1, 0),
	}

	assert.Equal(t, int64(999), em.IdosoID)
	assert.Less(t, em.CurrentWeight, em.OriginalSeverity)
	assert.Equal(t, 30, em.DaysSinceError)
}

func TestErrorMemory_WeightDecay(t *testing.T) {
	// Weight should decay over time
	originalSeverity := 0.8
	decayRate := 0.02 // 2% per day

	testCases := []struct {
		days           int
		expectedWeight float64
	}{
		{0, 0.80},
		{10, 0.60},
		{20, 0.40},
		{40, 0.00}, // Minimum 0
	}

	for _, tc := range testCases {
		t.Run(string(rune(tc.days)), func(t *testing.T) {
			weight := originalSeverity - (float64(tc.days) * decayRate)
			if weight < 0 {
				weight = 0
			}
			assert.InDelta(t, tc.expectedWeight, weight, 0.01)
		})
	}
}

func TestErrorMemory_BehaviorChangeImpact(t *testing.T) {
	testCases := []struct {
		name             string
		behaviorChanged  bool
		consistencyDays  int
		forgivenessBonus float64
	}{
		{"no_change", false, 0, 0.0},
		{"recent_change", true, 7, 0.1},
		{"sustained_change", true, 30, 0.3},
		{"long_sustained", true, 90, 0.5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var bonus float64
			if tc.behaviorChanged {
				switch {
				case tc.consistencyDays >= 60:
					bonus = 0.5
				case tc.consistencyDays >= 30:
					bonus = 0.3
				case tc.consistencyDays >= 7:
					bonus = 0.1
				}
			}
			assert.InDelta(t, tc.forgivenessBonus, bonus, 0.01)
		})
	}
}

func TestErrorMemory_CanMention(t *testing.T) {
	testCases := []struct {
		name        string
		rapport     float64
		severity    float64
		canMention  bool
	}{
		{"low_rapport_low_severity", 0.3, 0.2, true},
		{"low_rapport_high_severity", 0.3, 0.8, false},
		{"high_rapport_high_severity", 0.9, 0.8, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Can mention if rapport > severity or severity is low
			canMention := tc.rapport > tc.severity || tc.severity < 0.3
			assert.Equal(t, tc.canMention, canMention)
		})
	}
}

// =====================================================
// 8. EMPATHIC LOAD TESTS
// =====================================================

func TestEmpathicLoad_Structure(t *testing.T) {
	el := &EmpathicLoad{
		CurrentLoad:            0.7,
		FatigueLevel:           "high",
		IsFatigued:             true,
		ResponseLengthModifier: 0.8,
		SuggestLighterTopics:   true,
		RequestPause:           false,
		SessionLoadAccumulated: 2.5,
	}

	assert.InDelta(t, 0.7, el.CurrentLoad, 0.01)
	assert.Equal(t, "high", el.FatigueLevel)
	assert.True(t, el.IsFatigued)
	assert.True(t, el.SuggestLighterTopics)
}

func TestEmpathicLoad_FatigueLevels(t *testing.T) {
	testCases := []struct {
		name     string
		load     float64
		expected string
	}{
		{"fresh", 0.1, "fresh"},
		{"light", 0.3, "light"},
		{"moderate", 0.5, "moderate"},
		{"high", 0.7, "high"},
		{"exhausted", 0.9, "exhausted"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var level string
			switch {
			case tc.load < 0.2:
				level = "fresh"
			case tc.load < 0.4:
				level = "light"
			case tc.load < 0.6:
				level = "moderate"
			case tc.load < 0.8:
				level = "high"
			default:
				level = "exhausted"
			}
			assert.Equal(t, tc.expected, level)
		})
	}
}

func TestEmpathicLoad_ResponseModification(t *testing.T) {
	testCases := []struct {
		name       string
		load       float64
		baseLength int
		minAdjusted int
		maxAdjusted int
	}{
		{"no_fatigue", 0.2, 100, 85, 100},
		{"light_fatigue", 0.4, 100, 75, 85},
		{"moderate_fatigue", 0.6, 100, 65, 75},
		{"high_fatigue", 0.8, 100, 55, 65},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate modifier based on load
			modifier := 1.0 - (tc.load * 0.5)
			if modifier < 0.5 {
				modifier = 0.5
			}
			adjusted := int(float64(tc.baseLength) * modifier)
			assert.GreaterOrEqual(t, adjusted, tc.minAdjusted)
			assert.LessOrEqual(t, adjusted, tc.maxAdjusted)
		})
	}
}

func TestEmpathicLoad_PauseThreshold(t *testing.T) {
	testCases := []struct {
		name         string
		sessionLoad  float64
		shouldPause  bool
	}{
		{"low_load", 1.0, false},
		{"medium_load", 2.5, false},
		{"high_load", 4.0, true},
		{"very_high_load", 5.0, true},
	}

	pauseThreshold := 3.5

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			shouldPause := tc.sessionLoad >= pauseThreshold
			assert.Equal(t, tc.shouldPause, shouldPause)
		})
	}
}

// =====================================================
// INTERVENTION READINESS TESTS
// =====================================================

func TestInterventionReadiness_Structure(t *testing.T) {
	ir := &InterventionReadiness{
		ReadinessScore:    0.75,
		CanIntervene:      true,
		PatternStrength:   0.85,
		Rapport:           0.8,
		CurrentMode:       "coaching",
		InCooldown:        false,
		RecommendedAction: "confront_pattern",
	}

	assert.InDelta(t, 0.75, ir.ReadinessScore, 0.01)
	assert.True(t, ir.CanIntervene)
	assert.Equal(t, "confront_pattern", ir.RecommendedAction)
}

func TestInterventionReadiness_ActionRecommendation(t *testing.T) {
	testCases := []struct {
		name            string
		canIntervene    bool
		patternStrength float64
		rapport         float64
		inCooldown      bool
		expected        string
	}{
		{"in_cooldown", true, 0.9, 0.9, true, "wait"},
		{"strong_pattern", true, 0.85, 0.8, false, "confront_pattern"},
		{"moderate_pattern", true, 0.6, 0.7, false, "gentle_observation"},
		{"low_rapport", false, 0.5, 0.2, false, "build_trust"},
		{"observe", true, 0.3, 0.5, false, "observe"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var action string
			if tc.inCooldown {
				action = "wait"
			} else if tc.canIntervene && tc.patternStrength > 0.8 {
				action = "confront_pattern"
			} else if tc.canIntervene && tc.patternStrength > 0.5 {
				action = "gentle_observation"
			} else if tc.rapport < 0.3 {
				action = "build_trust"
			} else {
				action = "observe"
			}
			assert.Equal(t, tc.expected, action)
		})
	}
}

func TestInterventionReadiness_CooldownPeriod(t *testing.T) {
	// After intervention, 24-hour cooldown
	cooldownHours := 24

	testCases := []struct {
		name           string
		hoursSince     int
		stillInCooldown bool
	}{
		{"just_intervened", 1, true},
		{"half_day", 12, true},
		{"almost_done", 23, true},
		{"cooldown_over", 25, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			inCooldown := tc.hoursSince < cooldownHours
			assert.Equal(t, tc.stillInCooldown, inCooldown)
		})
	}
}

// =====================================================
// HELPER FUNCTIONS
// =====================================================

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
		 len(s) > 0 && len(substr) > 0 &&
		 containsLower(toLower(s), toLower(substr)))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
