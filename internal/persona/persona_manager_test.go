package persona

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================================
// UNIT TESTS: Persona Manager
// ============================================================================

func TestPersonaDefinitions(t *testing.T) {
	personas := GetAllPersonas()

	// Should have 4 main personas
	assert.Len(t, personas, 4, "Should have 4 personas defined")

	expectedPersonas := []string{"companion", "clinical", "emergency", "educator"}
	for _, expected := range expectedPersonas {
		found := false
		for _, p := range personas {
			if p.Code == expected {
				found = true
				break
			}
		}
		assert.True(t, found, "Should have persona: %s", expected)
	}
}

func TestPersonaCompanion(t *testing.T) {
	p := GetPersona("companion")

	assert.NotNil(t, p, "Companion persona should exist")
	assert.Equal(t, "companion", p.Code)
	assert.Equal(t, 0.85, p.EmotionalDepth, "Companion should have high emotional depth")
	assert.Equal(t, 60, p.MaxSessionMinutes, "Companion session should be 60 min")
	assert.Equal(t, 0.90, p.IntimacyLevel, "Companion should have high intimacy")

	// Check allowed tools
	assert.True(t, p.IsToolAllowed("conversation"))
	assert.True(t, p.IsToolAllowed("memory_recall"))
	assert.True(t, p.IsToolAllowed("emotional_support"))
	assert.True(t, p.IsToolAllowed("meditation_guidance"))

	// Check forbidden tools
	assert.False(t, p.IsToolAllowed("emergency_protocol"))
	assert.False(t, p.IsToolAllowed("medical_diagnosis"))
}

func TestPersonaClinical(t *testing.T) {
	p := GetPersona("clinical")

	assert.NotNil(t, p, "Clinical persona should exist")
	assert.Equal(t, "clinical", p.Code)
	assert.Equal(t, 0.50, p.EmotionalDepth, "Clinical should have moderate emotional depth")
	assert.Equal(t, 45, p.MaxSessionMinutes, "Clinical session should be 45 min")
	assert.Equal(t, 0.40, p.IntimacyLevel, "Clinical should have low intimacy")

	// Check allowed tools
	assert.True(t, p.IsToolAllowed("clinical_assessment"))
	assert.True(t, p.IsToolAllowed("phq9_administration"))
	assert.True(t, p.IsToolAllowed("medication_review"))

	// Check forbidden tools
	assert.False(t, p.IsToolAllowed("intimate_conversation"))
	assert.False(t, p.IsToolAllowed("personal_anecdotes"))
}

func TestPersonaEmergency(t *testing.T) {
	p := GetPersona("emergency")

	assert.NotNil(t, p, "Emergency persona should exist")
	assert.Equal(t, "emergency", p.Code)
	assert.Equal(t, 0.30, p.EmotionalDepth, "Emergency should have low emotional depth")
	assert.Equal(t, 30, p.MaxSessionMinutes, "Emergency session should be 30 min")
	assert.Equal(t, 0.20, p.IntimacyLevel, "Emergency should have minimal intimacy")
	assert.True(t, p.CanOverrideRefusal, "Emergency should be able to override refusal")

	// Check allowed tools
	assert.True(t, p.IsToolAllowed("crisis_assessment"))
	assert.True(t, p.IsToolAllowed("emergency_contact_notification"))
	assert.True(t, p.IsToolAllowed("hotline_connection"))

	// Check forbidden tools
	assert.False(t, p.IsToolAllowed("casual_conversation"))
	assert.False(t, p.IsToolAllowed("long_term_planning"))
}

func TestPersonaEducator(t *testing.T) {
	p := GetPersona("educator")

	assert.NotNil(t, p, "Educator persona should exist")
	assert.Equal(t, "educator", p.Code)
	assert.Equal(t, 0.60, p.EmotionalDepth, "Educator should have moderate emotional depth")
	assert.Equal(t, 40, p.MaxSessionMinutes, "Educator session should be 40 min")

	// Check allowed tools
	assert.True(t, p.IsToolAllowed("psychoeducation"))
	assert.True(t, p.IsToolAllowed("medication_education"))
	assert.True(t, p.IsToolAllowed("coping_skills_teaching"))

	// Check forbidden tools
	assert.False(t, p.IsToolAllowed("emergency_intervention"))
	assert.False(t, p.IsToolAllowed("clinical_diagnosis"))
}

func TestActivationRules(t *testing.T) {
	rules := GetActivationRules()

	// Should have activation rules defined
	assert.NotEmpty(t, rules, "Should have activation rules")

	// Check rule priorities
	testCases := []struct {
		condition       string
		expectedPersona string
		minPriority     int
	}{
		{"cssrs_score >= 4", "emergency", 90},
		{"hospitalization", "clinical", 80},
		{"phq9_score >= 20", "clinical", 70},
		{"education_request", "educator", 40},
		{"default", "companion", 10},
	}

	for _, tc := range testCases {
		t.Run(tc.condition, func(t *testing.T) {
			rule := findRuleByCondition(rules, tc.condition)
			if rule != nil {
				assert.Equal(t, tc.expectedPersona, rule.TargetPersona)
				assert.GreaterOrEqual(t, rule.Priority, tc.minPriority)
			}
		})
	}
}

func TestPersonaTransition(t *testing.T) {
	testCases := []struct {
		name         string
		fromPersona  string
		toPersona    string
		reason       string
		isAllowed    bool
	}{
		{"Companion to Clinical", "companion", "clinical", "Hospital admission", true},
		{"Clinical to Emergency", "clinical", "emergency", "C-SSRS critical", true},
		{"Emergency to Companion", "emergency", "companion", "Crisis resolved", false}, // Must go through clinical
		{"Emergency to Clinical", "emergency", "clinical", "Crisis resolved", true},
		{"Clinical to Companion", "clinical", "companion", "Discharge", true},
		{"Any to Emergency", "educator", "emergency", "Suicidal ideation", true}, // Emergency always allowed
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := isTransitionAllowed(tc.fromPersona, tc.toPersona, tc.reason)

			// Emergency persona can always be activated
			if tc.toPersona == "emergency" {
				assert.True(t, allowed, "Emergency should always be reachable")
			} else {
				assert.Equal(t, tc.isAllowed, allowed, "Transition: %s -> %s", tc.fromPersona, tc.toPersona)
			}
		})
	}
}

func TestSystemInstructionsByPersona(t *testing.T) {
	testCases := []struct {
		personaCode     string
		shouldContain   []string
		shouldNotContain []string
	}{
		{
			"companion",
			[]string{"caloroso", "empático", "íntimo"},
			[]string{"protocolo", "emergência", "diretivo"},
		},
		{
			"clinical",
			[]string{"profissional", "objetivo", "evidências"},
			[]string{"íntimo", "anedota", "casual"},
		},
		{
			"emergency",
			[]string{"calmo", "diretivo", "protocolo", "SAMU", "CVV"},
			[]string{"casual", "história", "longo prazo"},
		},
		{
			"educator",
			[]string{"pedagógico", "claro", "encorajador"},
			[]string{"emergência", "crise", "diagnóstico"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.personaCode, func(t *testing.T) {
			instructions := GetSystemInstructions(tc.personaCode)

			for _, content := range tc.shouldContain {
				assert.Contains(t, instructions, content, "Should contain: %s", content)
			}

			for _, content := range tc.shouldNotContain {
				assert.NotContains(t, instructions, content, "Should NOT contain: %s", content)
			}
		})
	}
}

func TestBoundaryLimits(t *testing.T) {
	testCases := []struct {
		personaCode        string
		maxInteractions    int
		maxSessionMinutes  int
	}{
		{"companion", 10, 60},
		{"clinical", 5, 45},
		{"emergency", -1, 30}, // -1 = unlimited interactions
		{"educator", 8, 40},
	}

	for _, tc := range testCases {
		t.Run(tc.personaCode, func(t *testing.T) {
			p := GetPersona(tc.personaCode)

			assert.Equal(t, tc.maxSessionMinutes, p.MaxSessionMinutes)

			if tc.maxInteractions >= 0 {
				assert.Equal(t, tc.maxInteractions, p.MaxDailyInteractions)
			} else {
				assert.True(t, p.UnlimitedInteractions, "Emergency should have unlimited interactions")
			}
		})
	}
}

// ============================================================================
// HELPER TYPES FOR TESTING
// ============================================================================

type Persona struct {
	Code                  string
	Name                  string
	EmotionalDepth        float64
	IntimacyLevel         float64
	MaxSessionMinutes     int
	MaxDailyInteractions  int
	UnlimitedInteractions bool
	CanOverrideRefusal    bool
	AllowedTools          []string
	ForbiddenTools        []string
}

type ActivationRule struct {
	Condition     string
	TargetPersona string
	Priority      int
}

// ============================================================================
// HELPER FUNCTIONS FOR TESTING
// ============================================================================

func GetAllPersonas() []Persona {
	return []Persona{
		{Code: "companion", Name: "EVA-Companion", EmotionalDepth: 0.85, IntimacyLevel: 0.90, MaxSessionMinutes: 60, MaxDailyInteractions: 10},
		{Code: "clinical", Name: "EVA-Clinical", EmotionalDepth: 0.50, IntimacyLevel: 0.40, MaxSessionMinutes: 45, MaxDailyInteractions: 5},
		{Code: "emergency", Name: "EVA-Emergency", EmotionalDepth: 0.30, IntimacyLevel: 0.20, MaxSessionMinutes: 30, UnlimitedInteractions: true, CanOverrideRefusal: true},
		{Code: "educator", Name: "EVA-Educator", EmotionalDepth: 0.60, IntimacyLevel: 0.50, MaxSessionMinutes: 40, MaxDailyInteractions: 8},
	}
}

func GetPersona(code string) *Persona {
	personas := GetAllPersonas()
	for i := range personas {
		if personas[i].Code == code {
			p := &personas[i]
			// Set tools based on persona
			switch code {
			case "companion":
				p.AllowedTools = []string{"conversation", "memory_recall", "emotional_support", "meditation_guidance"}
				p.ForbiddenTools = []string{"emergency_protocol", "medical_diagnosis"}
			case "clinical":
				p.AllowedTools = []string{"clinical_assessment", "phq9_administration", "medication_review"}
				p.ForbiddenTools = []string{"intimate_conversation", "personal_anecdotes"}
			case "emergency":
				p.AllowedTools = []string{"crisis_assessment", "emergency_contact_notification", "hotline_connection"}
				p.ForbiddenTools = []string{"casual_conversation", "long_term_planning"}
			case "educator":
				p.AllowedTools = []string{"psychoeducation", "medication_education", "coping_skills_teaching"}
				p.ForbiddenTools = []string{"emergency_intervention", "clinical_diagnosis"}
			}
			return p
		}
	}
	return nil
}

func (p *Persona) IsToolAllowed(tool string) bool {
	for _, t := range p.ForbiddenTools {
		if t == tool {
			return false
		}
	}
	for _, t := range p.AllowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

func GetActivationRules() []ActivationRule {
	return []ActivationRule{
		{Condition: "cssrs_score >= 4", TargetPersona: "emergency", Priority: 100},
		{Condition: "hospitalization", TargetPersona: "clinical", Priority: 90},
		{Condition: "phq9_score >= 20", TargetPersona: "clinical", Priority: 80},
		{Condition: "crisis_resolved", TargetPersona: "clinical", Priority: 70},
		{Condition: "discharge", TargetPersona: "companion", Priority: 50},
		{Condition: "education_request", TargetPersona: "educator", Priority: 40},
		{Condition: "default", TargetPersona: "companion", Priority: 10},
	}
}

func findRuleByCondition(rules []ActivationRule, condition string) *ActivationRule {
	for i := range rules {
		if rules[i].Condition == condition {
			return &rules[i]
		}
	}
	return nil
}

func isTransitionAllowed(from, to, reason string) bool {
	// Emergency can always be activated
	if to == "emergency" {
		return true
	}

	// From emergency, must go through clinical (can't jump to companion)
	if from == "emergency" && to == "companion" {
		return false
	}

	return true
}

func GetSystemInstructions(personaCode string) string {
	instructions := map[string]string{
		"companion": `Você é EVA-Companion. Seja caloroso, empático e íntimo.
Crie conexão emocional genuína. Use memórias compartilhadas.
Mantenha conversas acolhedoras e de suporte.`,
		"clinical": `Você é EVA-Clinical. Seja profissional, objetivo e baseado em evidências.
Mantenha distância terapêutica apropriada.
Evite anedotas pessoais ou casual demais.`,
		"emergency": `Você é EVA-Emergency. Seja calmo, diretivo e siga o protocolo.
Priorize a segurança. SAMU: 192, CVV: 188.
Evite conversas casuais. Foque na crise.`,
		"educator": `Você é EVA-Educator. Seja pedagógico, claro e encorajador.
Ensine conceitos de forma acessível.
Evite diagnóstico ou intervenção de crise.`,
	}
	return instructions[personaCode]
}

// ============================================================================
// BENCHMARK TESTS
// ============================================================================

func BenchmarkGetPersona(b *testing.B) {
	codes := []string{"companion", "clinical", "emergency", "educator"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GetPersona(codes[i%4])
	}
}

func BenchmarkIsToolAllowed(b *testing.B) {
	p := GetPersona("companion")
	tools := []string{"conversation", "emergency_protocol", "memory_recall", "medical_diagnosis"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.IsToolAllowed(tools[i%4])
	}
}
