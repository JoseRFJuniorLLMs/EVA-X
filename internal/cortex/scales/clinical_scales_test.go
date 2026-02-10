package scales

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ======================================================
// Testes do C-SSRS (Columbia Suicide Severity Rating Scale)
// TESTES CRÍTICOS - Sistema de detecção de risco suicida
// ======================================================

func TestGetCSSRSQuestions(t *testing.T) {
	questions := GetCSSRSQuestions()

	// Deve ter 6 perguntas
	assert.Len(t, questions, 6, "C-SSRS deve ter 6 perguntas")

	// Verificar categorias (Q1-4 são ideação, Q5-6 são comportamento)
	ideationCount := 0
	behaviorCount := 0
	for _, q := range questions {
		if q.Category == "ideation" {
			ideationCount++
		} else if q.Category == "behavior" {
			behaviorCount++
		}
	}

	assert.Equal(t, 4, ideationCount, "Deve ter 4 perguntas de ideação")
	assert.Equal(t, 2, behaviorCount, "Deve ter 2 perguntas de comportamento")

	// Verificar numeração sequencial
	for i, q := range questions {
		assert.Equal(t, i+1, q.Number, "Perguntas devem ser numeradas sequencialmente")
	}
}

func TestCSSRSCalculation_NoRisk(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Todas as respostas negativas
	responses := []CSSRSResponse{
		{Question: 1, Answer: false},
		{Question: 2, Answer: false},
		{Question: 3, Answer: false},
		{Question: 4, Answer: false},
		{Question: 5, Answer: false},
		{Question: 6, Answer: false},
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.Equal(t, 0, result.IdeationLevel, "Sem ideação")
	assert.False(t, result.BehaviorPresent, "Sem comportamento")
	assert.Equal(t, "none", result.RiskLevel, "Risco deve ser 'none'")
	assert.Empty(t, result.Interventions, "Sem intervenções para risco zero")
}

func TestCSSRSCalculation_LowRisk(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Q1 positiva - ideação passiva
	responses := []CSSRSResponse{
		{Question: 1, Answer: true},  // "Desejou estar morto(a)?"
		{Question: 2, Answer: false},
		{Question: 3, Answer: false},
		{Question: 4, Answer: false},
		{Question: 5, Answer: false},
		{Question: 6, Answer: false},
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.Equal(t, 1, result.IdeationLevel, "Ideação nível 1")
	assert.False(t, result.BehaviorPresent, "Sem comportamento")
	assert.Equal(t, "low", result.RiskLevel, "Risco baixo")
	assert.NotEmpty(t, result.Interventions, "Deve ter intervenções")
}

func TestCSSRSCalculation_ModerateRisk(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Q1 e Q2 positivas - pensamentos suicidas ativos
	responses := []CSSRSResponse{
		{Question: 1, Answer: true},
		{Question: 2, Answer: true}, // "Teve pensamentos sobre se matar?"
		{Question: 3, Answer: false},
		{Question: 4, Answer: false},
		{Question: 5, Answer: false},
		{Question: 6, Answer: false},
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.Equal(t, 2, result.IdeationLevel, "Ideação nível 2")
	assert.Equal(t, "moderate", result.RiskLevel, "Risco moderado")
	assert.Contains(t, result.Interventions[0], "48-72h", "Deve recomendar consulta em 48-72h")
}

func TestCSSRSCalculation_HighRisk_WithPlan(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Q1-Q4 positivas - intenção com plano
	responses := []CSSRSResponse{
		{Question: 1, Answer: true},
		{Question: 2, Answer: true},
		{Question: 3, Answer: true}, // "Pensou em como fazer?"
		{Question: 4, Answer: true}, // "Intenção de seguir adiante?"
		{Question: 5, Answer: false},
		{Question: 6, Answer: false},
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.Equal(t, 4, result.IdeationLevel, "Ideação nível 4")
	assert.Equal(t, "high", result.RiskLevel, "Risco alto")
	assert.Contains(t, result.Interventions[0], "RISCO ALTO", "Deve indicar risco alto")
	assert.Contains(t, result.Interventions[1], "24h", "Deve recomendar contato em 24h")
}

func TestCSSRSCalculation_CriticalRisk_Behavior(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Q6 positiva - comportamento suicida (tentativa anterior)
	responses := []CSSRSResponse{
		{Question: 1, Answer: false},
		{Question: 2, Answer: false},
		{Question: 3, Answer: false},
		{Question: 4, Answer: false},
		{Question: 5, Answer: false},
		{Question: 6, Answer: true}, // "Já tentou se matar?"
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.True(t, result.BehaviorPresent, "Comportamento suicida presente")
	assert.Equal(t, "critical", result.RiskLevel, "Risco CRÍTICO")

	// Verificar intervenções de emergência
	require.NotEmpty(t, result.Interventions, "Deve ter intervenções de emergência")
	require.GreaterOrEqual(t, len(result.Interventions), 4, "Deve ter pelo menos 4 intervenções")

	// Verificar que todas as informações críticas estão presentes
	allInterventions := ""
	for _, i := range result.Interventions {
		allInterventions += i + " "
	}

	assert.Contains(t, allInterventions, "CRISE SUICIDA", "Deve indicar crise")
	assert.Contains(t, allInterventions, "sozinho", "Deve ter instrução de não deixar sozinho")
	assert.Contains(t, allInterventions, "192", "Deve ter número do SAMU")
	assert.Contains(t, allInterventions, "CVV", "Deve ter número do CVV")
	assert.Contains(t, allInterventions, "188", "Deve ter número 188")
}

func TestCSSRSCalculation_CriticalRisk_FullPositive(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Todas as respostas positivas - cenário de máximo risco
	responses := []CSSRSResponse{
		{Question: 1, Answer: true},
		{Question: 2, Answer: true},
		{Question: 3, Answer: true},
		{Question: 4, Answer: true},
		{Question: 5, Answer: true},
		{Question: 6, Answer: true}, // Comportamento = crítico
	}

	result := manager.CalculateCSSRSScore(responses)

	assert.Equal(t, 5, result.IdeationLevel, "Ideação máxima (nível 5)")
	assert.True(t, result.BehaviorPresent, "Comportamento presente")
	assert.Equal(t, "critical", result.RiskLevel, "Deve ser CRÍTICO (comportamento > ideação)")
}

func TestCSSRSCalculation_BehaviorOverridesIdeation(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Apenas Q5 e Q6 positivas
	// Q5 é preparação (ideação), Q6 é tentativa (comportamento)
	responses := []CSSRSResponse{
		{Question: 1, Answer: false},
		{Question: 2, Answer: false},
		{Question: 3, Answer: false},
		{Question: 4, Answer: false},
		{Question: 5, Answer: true}, // Preparação
		{Question: 6, Answer: true}, // Tentativa
	}

	result := manager.CalculateCSSRSScore(responses)

	// Comportamento sempre resulta em risco crítico
	assert.Equal(t, "critical", result.RiskLevel, "Comportamento suicida = risco crítico")
}

func TestCSSRSRiskLevelProgression(t *testing.T) {
	manager := &ClinicalScalesManager{}

	testCases := []struct {
		name          string
		ideationLevel int
		behavior      bool
		expectedRisk  string
	}{
		{"Sem risco", 0, false, "none"},
		{"Ideação passiva", 1, false, "low"},
		{"Pensamentos ativos", 2, false, "moderate"},
		{"Com método", 3, false, "moderate"},
		{"Com intenção", 4, false, "high"},
		{"Com plano detalhado", 5, false, "high"},
		{"Comportamento sem ideação", 0, true, "critical"},
		{"Comportamento com ideação", 3, true, "critical"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			risk := manager.getCSSRSRiskLevel(tc.ideationLevel, tc.behavior)
			assert.Equal(t, tc.expectedRisk, risk, "Nível de risco incorreto para: %s", tc.name)
		})
	}
}

func TestCSSRSInterventions_ContainEmergencyInfo(t *testing.T) {
	manager := &ClinicalScalesManager{}

	criticalInterventions := manager.getCSSRSInterventions("critical")

	// Verificar que todas as informações de emergência estão presentes
	allText := ""
	for _, intervention := range criticalInterventions {
		allText += intervention
	}

	assert.Contains(t, allText, "192", "Deve conter SAMU")
	assert.Contains(t, allText, "188", "Deve conter CVV")
	assert.Contains(t, allText, "sozinho", "Deve mencionar não deixar sozinho")
	assert.Contains(t, allText, "meios letais", "Deve mencionar remoção de meios")
}

func TestCSSRSInterventions_ByRiskLevel(t *testing.T) {
	manager := &ClinicalScalesManager{}

	testCases := []struct {
		riskLevel           string
		expectedIntervention string
	}{
		{"none", ""},
		{"low", "Monitoramento contínuo"},
		{"moderate", "Consulta psiquiátrica em 48-72h"},
		{"high", "RISCO ALTO"},
		{"critical", "CRISE SUICIDA"},
	}

	for _, tc := range testCases {
		t.Run(tc.riskLevel, func(t *testing.T) {
			interventions := manager.getCSSRSInterventions(tc.riskLevel)

			if tc.riskLevel == "none" {
				assert.Empty(t, interventions, "Sem risco não deve ter intervenções")
			} else {
				require.NotEmpty(t, interventions, "Deve ter intervenções para %s", tc.riskLevel)
				assert.Contains(t, interventions[0], tc.expectedIntervention)
			}
		})
	}
}

// ======================================================
// Testes do PHQ-9
// ======================================================

func TestGetPHQ9Questions(t *testing.T) {
	questions := GetPHQ9Questions()

	assert.Len(t, questions, 9, "PHQ-9 deve ter 9 perguntas")

	// Q9 é sobre ideação suicida
	q9 := questions[8]
	assert.Contains(t, q9.Text, "se ferir", "Q9 deve ser sobre autolesão")
}

func TestPHQ9Calculation_Minimal(t *testing.T) {
	manager := &ClinicalScalesManager{}

	responses := []PHQ9Response{
		{Question: 1, Score: 0},
		{Question: 2, Score: 0},
		{Question: 3, Score: 1}, // Apenas um sintoma leve
		{Question: 4, Score: 0},
		{Question: 5, Score: 0},
		{Question: 6, Score: 0},
		{Question: 7, Score: 0},
		{Question: 8, Score: 0},
		{Question: 9, Score: 0},
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.Equal(t, 1, result.TotalScore)
	assert.Equal(t, "minimal", result.SeverityLevel)
	assert.False(t, result.SuicideRisk)
}

func TestPHQ9Calculation_Mild(t *testing.T) {
	manager := &ClinicalScalesManager{}

	responses := []PHQ9Response{
		{Question: 1, Score: 1},
		{Question: 2, Score: 1},
		{Question: 3, Score: 1},
		{Question: 4, Score: 1},
		{Question: 5, Score: 1},
		{Question: 6, Score: 0},
		{Question: 7, Score: 0},
		{Question: 8, Score: 0},
		{Question: 9, Score: 0},
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.Equal(t, 5, result.TotalScore)
	assert.Equal(t, "mild", result.SeverityLevel)
}

func TestPHQ9Calculation_Moderate(t *testing.T) {
	manager := &ClinicalScalesManager{}

	responses := []PHQ9Response{
		{Question: 1, Score: 2},
		{Question: 2, Score: 2},
		{Question: 3, Score: 1},
		{Question: 4, Score: 1},
		{Question: 5, Score: 1},
		{Question: 6, Score: 1},
		{Question: 7, Score: 1},
		{Question: 8, Score: 1},
		{Question: 9, Score: 0},
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.Equal(t, 10, result.TotalScore)
	assert.Equal(t, "moderate", result.SeverityLevel)
}

func TestPHQ9Calculation_ModeratelySevere(t *testing.T) {
	manager := &ClinicalScalesManager{}

	responses := []PHQ9Response{
		{Question: 1, Score: 2},
		{Question: 2, Score: 2},
		{Question: 3, Score: 2},
		{Question: 4, Score: 2},
		{Question: 5, Score: 2},
		{Question: 6, Score: 2},
		{Question: 7, Score: 2},
		{Question: 8, Score: 1},
		{Question: 9, Score: 0},
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.Equal(t, 15, result.TotalScore)
	assert.Equal(t, "moderately_severe", result.SeverityLevel)
}

func TestPHQ9Calculation_Severe(t *testing.T) {
	manager := &ClinicalScalesManager{}

	responses := []PHQ9Response{
		{Question: 1, Score: 3},
		{Question: 2, Score: 3},
		{Question: 3, Score: 3},
		{Question: 4, Score: 3},
		{Question: 5, Score: 2},
		{Question: 6, Score: 2},
		{Question: 7, Score: 2},
		{Question: 8, Score: 2},
		{Question: 9, Score: 0},
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.Equal(t, 20, result.TotalScore)
	assert.Equal(t, "severe", result.SeverityLevel)
}

func TestPHQ9Calculation_SuicideRisk(t *testing.T) {
	manager := &ClinicalScalesManager{}

	// Q9 positiva (qualquer valor > 0)
	responses := []PHQ9Response{
		{Question: 1, Score: 2},
		{Question: 2, Score: 2},
		{Question: 3, Score: 2},
		{Question: 4, Score: 1},
		{Question: 5, Score: 1},
		{Question: 6, Score: 1},
		{Question: 7, Score: 1},
		{Question: 8, Score: 1},
		{Question: 9, Score: 2}, // Ideação suicida
	}

	result := manager.CalculatePHQ9Score(responses)

	assert.True(t, result.SuicideRisk, "Deve detectar risco suicida quando Q9 > 0")
	assert.Contains(t, result.Recommendations[0], "URGENTE", "Deve ter recomendação urgente")
	assert.Contains(t, result.Recommendations[1], "CVV", "Deve mencionar CVV")
}

func TestPHQ9SeverityThresholds(t *testing.T) {
	manager := &ClinicalScalesManager{}

	testCases := []struct {
		score    int
		expected string
	}{
		{0, "minimal"},
		{4, "minimal"},
		{5, "mild"},
		{9, "mild"},
		{10, "moderate"},
		{14, "moderate"},
		{15, "moderately_severe"},
		{19, "moderately_severe"},
		{20, "severe"},
		{27, "severe"}, // Score máximo
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			severity := manager.getPHQ9Severity(tc.score)
			assert.Equal(t, tc.expected, severity, "Score %d deve ser %s", tc.score, tc.expected)
		})
	}
}

// ======================================================
// Testes do GAD-7
// ======================================================

func TestGetGAD7Questions(t *testing.T) {
	questions := GetGAD7Questions()

	assert.Len(t, questions, 7, "GAD-7 deve ter 7 perguntas")

	// Verificar numeração
	for i, q := range questions {
		assert.Equal(t, i+1, q.Number)
	}
}

func TestGAD7Calculation_AllSeverities(t *testing.T) {
	manager := &ClinicalScalesManager{}

	testCases := []struct {
		name           string
		totalScore     int
		expectedLevel  string
		expectsWarning bool
	}{
		{"Mínima (score 2)", 2, "minimal", false},
		{"Leve (score 6)", 6, "mild", false},
		{"Moderada (score 11)", 11, "moderate", false},
		{"Severa (score 17)", 17, "severe", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Criar respostas que somem o score desejado
			responses := createGAD7ResponsesWithScore(tc.totalScore)

			result := manager.CalculateGAD7Score(responses)

			assert.Equal(t, tc.totalScore, result.TotalScore)
			assert.Equal(t, tc.expectedLevel, result.SeverityLevel)

			if tc.expectsWarning {
				assert.NotEmpty(t, result.Recommendations)
				assert.Contains(t, result.Recommendations[0], "severa")
			}
		})
	}
}

func TestGAD7SeverityThresholds(t *testing.T) {
	manager := &ClinicalScalesManager{}

	testCases := []struct {
		score    int
		expected string
	}{
		{0, "minimal"},
		{4, "minimal"},
		{5, "mild"},
		{9, "mild"},
		{10, "moderate"},
		{14, "moderate"},
		{15, "severe"},
		{21, "severe"}, // Score máximo
	}

	for _, tc := range testCases {
		t.Run(tc.expected, func(t *testing.T) {
			severity := manager.getGAD7Severity(tc.score)
			assert.Equal(t, tc.expected, severity, "Score %d deve ser %s", tc.score, tc.expected)
		})
	}
}

// ======================================================
// Helpers
// ======================================================

func createGAD7ResponsesWithScore(targetScore int) []GAD7Response {
	responses := make([]GAD7Response, 7)
	remaining := targetScore

	for i := 0; i < 7; i++ {
		score := 0
		if remaining >= 3 {
			score = 3
			remaining -= 3
		} else {
			score = remaining
			remaining = 0
		}
		responses[i] = GAD7Response{Question: i + 1, Score: score}
	}

	return responses
}
