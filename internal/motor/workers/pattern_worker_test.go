package workers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPatternWorker(t *testing.T) {
	pw := NewPatternWorker(nil)
	require.NotNil(t, pw)
}

func TestPatternWorker_Name(t *testing.T) {
	pw := NewPatternWorker(nil)
	assert.Equal(t, "Pattern Detector", pw.Name())
}

func TestPatternWorker_Interval(t *testing.T) {
	pw := NewPatternWorker(nil)
	assert.Equal(t, 6*time.Hour, pw.Interval())
}

func TestBehaviorPattern_Structure(t *testing.T) {
	pattern := &BehaviorPattern{
		IdosoID:    999,
		TipoPadrao: "horario_sono",
		Descricao:  "Padrão de sono detectado: 22:00 - 06:00",
		Frequencia: "diario",
		Confianca:  0.85,
		DadosEstatisticos: map[string]interface{}{
			"hora_inicio":           22,
			"hora_fim":              6,
			"horas_sono":            8,
			"total_dias_analisados": 30,
		},
	}

	assert.Equal(t, 999, pattern.IdosoID)
	assert.Equal(t, "horario_sono", pattern.TipoPadrao)
	assert.Contains(t, pattern.Descricao, "sono")
	assert.Equal(t, "diario", pattern.Frequencia)
	assert.InDelta(t, 0.85, pattern.Confianca, 0.01)
	assert.NotEmpty(t, pattern.DadosEstatisticos)
}

func TestPatternWorker_FindContinuousInterval(t *testing.T) {
	pw := NewPatternWorker(nil)

	testCases := []struct {
		name          string
		horas         []int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "normal_night",
			horas:         []int{22, 23, 0, 1, 2, 3, 4, 5, 6},
			expectedStart: 22,
			expectedEnd:   6,
		},
		{
			name:          "short_interval",
			horas:         []int{1, 2, 3},
			expectedStart: 1,
			expectedEnd:   3,
		},
		{
			name:          "single_hour",
			horas:         []int{3},
			expectedStart: 3,
			expectedEnd:   3,
		},
		{
			name:          "empty",
			horas:         []int{},
			expectedStart: 0,
			expectedEnd:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := pw.findContinuousInterval(tc.horas)
			assert.Equal(t, tc.expectedStart, start)
			assert.Equal(t, tc.expectedEnd, end)
		})
	}
}

func TestBehaviorPattern_SleepPattern(t *testing.T) {
	pattern := &BehaviorPattern{
		IdosoID:    1,
		TipoPadrao: "horario_sono",
		Descricao:  "Padrão de sono detectado: 23:00 - 07:00",
		Frequencia: "diario",
		Confianca:  0.85,
		DadosEstatisticos: map[string]interface{}{
			"hora_inicio":           23,
			"hora_fim":              7,
			"horas_sono":            8,
			"total_dias_analisados": 30,
			"total_ligacoes":        100,
		},
	}

	assert.Equal(t, "horario_sono", pattern.TipoPadrao)

	horaInicio := pattern.DadosEstatisticos["hora_inicio"].(int)
	horaFim := pattern.DadosEstatisticos["hora_fim"].(int)

	// Calculate sleep hours (crossing midnight)
	var horasSono int
	if horaFim < horaInicio {
		horasSono = (24 - horaInicio) + horaFim
	} else {
		horasSono = horaFim - horaInicio
	}

	assert.Equal(t, 8, horasSono)
}

func TestBehaviorPattern_MoodPattern(t *testing.T) {
	pattern := &BehaviorPattern{
		IdosoID:    1,
		TipoPadrao: "humor_recorrente",
		Descricao:  "Humor predominante: tristeza (15 ocorrências em 30 dias)",
		Frequencia: "semanal",
		Confianca:  0.5, // 15/30
		DadosEstatisticos: map[string]interface{}{
			"sentimento_predominante": "tristeza",
			"ocorrencias":             15,
			"intensidade_media":       0.7,
			"dias_analisados":         30,
		},
	}

	assert.Equal(t, "humor_recorrente", pattern.TipoPadrao)
	assert.Equal(t, "tristeza", pattern.DadosEstatisticos["sentimento_predominante"])

	// Check if mood requires attention
	sentimento := pattern.DadosEstatisticos["sentimento_predominante"].(string)
	intensidade := pattern.DadosEstatisticos["intensidade_media"].(float64)

	needsAttention := (sentimento == "tristeza" || sentimento == "ansiedade" ||
		sentimento == "medo" || sentimento == "raiva") && intensidade > 0.5

	assert.True(t, needsAttention)
}

func TestBehaviorPattern_MedicationAdherence(t *testing.T) {
	testCases := []struct {
		name          string
		total         int
		taken         int
		expectedLevel string
	}{
		{"excellent", 30, 28, "Excelente"},
		{"good", 30, 24, "Boa"},
		{"moderate", 30, 18, "Adesão moderada"},
		{"poor", 30, 12, "Baixa"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taxa := float64(tc.taken) / float64(tc.total)

			var descricao string
			if taxa >= 0.9 {
				descricao = "Excelente adesão à medicação"
			} else if taxa >= 0.7 {
				descricao = "Boa adesão à medicação"
			} else if taxa >= 0.5 {
				descricao = "Adesão moderada à medicação"
			} else {
				descricao = "Baixa adesão à medicação"
			}

			assert.Contains(t, descricao, tc.expectedLevel)
		})
	}
}

func TestBehaviorPattern_Confidence(t *testing.T) {
	testCases := []struct {
		name       string
		total      int
		days       int
		maxConf    float64
		isReliable bool
	}{
		{"high_confidence", 30, 30, 1.0, true},
		{"medium_confidence", 20, 30, 0.67, true},
		{"low_confidence", 10, 30, 0.33, false},
		{"very_low_confidence", 5, 30, 0.17, false},
	}

	reliabilityThreshold := 0.5

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			confianca := float64(tc.total) / float64(tc.days)
			if confianca > 1.0 {
				confianca = 1.0
			}

			assert.InDelta(t, tc.maxConf, confianca, 0.01)
			assert.Equal(t, tc.isReliable, confianca >= reliabilityThreshold)
		})
	}
}

func TestBehaviorPattern_FrequencyTypes(t *testing.T) {
	testCases := []struct {
		frequencia string
		valid      bool
	}{
		{"diario", true},
		{"semanal", true},
		{"mensal", true},
		{"irregular", true},
		{"invalid", false},
	}

	validFrequencies := map[string]bool{
		"diario":    true,
		"semanal":   true,
		"mensal":    true,
		"irregular": true,
	}

	for _, tc := range testCases {
		t.Run(tc.frequencia, func(t *testing.T) {
			isValid := validFrequencies[tc.frequencia]
			assert.Equal(t, tc.valid, isValid)
		})
	}
}

func TestBehaviorPattern_PatternTypes(t *testing.T) {
	patternTypes := []string{
		"horario_sono",
		"humor_recorrente",
		"medicacao_adesao",
	}

	for _, pt := range patternTypes {
		t.Run(pt, func(t *testing.T) {
			pattern := &BehaviorPattern{
				IdosoID:    1,
				TipoPadrao: pt,
			}
			assert.NotEmpty(t, pattern.TipoPadrao)
		})
	}
}

func TestSleepDetection_MinimumData(t *testing.T) {
	// Minimum 10 calls required for pattern detection
	testCases := []struct {
		name         string
		totalCalls   int
		shouldDetect bool
	}{
		{"insufficient", 5, false},
		{"minimum", 10, true},
		{"sufficient", 20, true},
	}

	minimumRequired := 10

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			canDetect := tc.totalCalls >= minimumRequired
			assert.Equal(t, tc.shouldDetect, canDetect)
		})
	}
}

func TestActivityThreshold_SleepHours(t *testing.T) {
	// Test that low activity hours are correctly identified as sleep
	testCases := []struct {
		name           string
		totalCalls     int
		hourCalls      int
		isSleepHour    bool
	}{
		{"active_hour", 100, 10, false},
		{"sleep_hour", 100, 1, true},
		{"borderline", 100, 3, false}, // 3% > 30% of average
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mediaAtividade := float64(tc.totalCalls) / 24.0
			threshold := mediaAtividade * 0.3
			isSleep := float64(tc.hourCalls) < threshold
			assert.Equal(t, tc.isSleepHour, isSleep)
		})
	}
}

func TestPatternWorker_NoDatabase(t *testing.T) {
	// Worker should handle nil database gracefully
	pw := NewPatternWorker(nil)
	require.NotNil(t, pw)
	assert.Nil(t, pw.db)
}

func TestBehaviorPattern_ClinicalRelevance(t *testing.T) {
	// Test patterns that should trigger clinical alerts
	testCases := []struct {
		name            string
		tipoPadrao      string
		dadosStats      map[string]interface{}
		requiresAlert   bool
	}{
		{
			name:       "poor_medication_adherence",
			tipoPadrao: "medicacao_adesao",
			dadosStats: map[string]interface{}{
				"taxa_adesao": 0.3,
			},
			requiresAlert: true,
		},
		{
			name:       "good_medication_adherence",
			tipoPadrao: "medicacao_adesao",
			dadosStats: map[string]interface{}{
				"taxa_adesao": 0.9,
			},
			requiresAlert: false,
		},
		{
			name:       "persistent_sadness",
			tipoPadrao: "humor_recorrente",
			dadosStats: map[string]interface{}{
				"sentimento_predominante": "tristeza",
				"intensidade_media":       0.8,
			},
			requiresAlert: true,
		},
		{
			name:       "irregular_sleep",
			tipoPadrao: "horario_sono",
			dadosStats: map[string]interface{}{
				"horas_sono": 4,
			},
			requiresAlert: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pattern := &BehaviorPattern{
				TipoPadrao:        tc.tipoPadrao,
				DadosEstatisticos: tc.dadosStats,
			}

			var requiresAlert bool
			switch pattern.TipoPadrao {
			case "medicacao_adesao":
				if taxa, ok := pattern.DadosEstatisticos["taxa_adesao"].(float64); ok {
					requiresAlert = taxa < 0.5
				}
			case "humor_recorrente":
				sent, _ := pattern.DadosEstatisticos["sentimento_predominante"].(string)
				intens, _ := pattern.DadosEstatisticos["intensidade_media"].(float64)
				requiresAlert = (sent == "tristeza" || sent == "ansiedade") && intens > 0.6
			case "horario_sono":
				if horas, ok := pattern.DadosEstatisticos["horas_sono"].(int); ok {
					requiresAlert = horas < 6 || horas > 10
				}
			}

			assert.Equal(t, tc.requiresAlert, requiresAlert)
		})
	}
}
