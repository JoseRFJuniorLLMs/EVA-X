package thinking

import (
	"testing"
)

func TestIsHealthConcern(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{
			name:     "Dor no peito - deve detectar",
			message:  "Estou com dor no peito",
			expected: true,
		},
		{
			name:     "Febre alta - deve detectar",
			message:  "Estou com febre alta e cansaço",
			expected: true,
		},
		{
			name:     "Pergunta sobre medicamento - deve detectar",
			message:  "Posso tomar esse remédio?",
			expected: true,
		},
		{
			name:     "Conversa normal - não deve detectar",
			message:  "Como está o tempo hoje?",
			expected: false,
		},
		{
			name:     "Saudação - não deve detectar",
			message:  "Bom dia, tudo bem?",
			expected: false,
		},
		{
			name:     "Tontura e náusea - deve detectar",
			message:  "Estou sentindo tontura e náusea",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHealthConcern(tt.message)
			if result != tt.expected {
				t.Errorf("IsHealthConcern(%q) = %v, esperado %v", tt.message, result, tt.expected)
			}
		})
	}
}

func TestIsCriticalConcern(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected bool
	}{
		{
			name:     "Dor no peito - crítico",
			message:  "Estou com dor no peito forte",
			expected: true,
		},
		{
			name:     "Falta de ar severa - crítico",
			message:  "Não consigo respirar direito, falta de ar severa",
			expected: true,
		},
		{
			name:     "Desmaio - crítico",
			message:  "Acabei de desmaiar",
			expected: true,
		},
		{
			name:     "Dor de cabeça leve - não crítico",
			message:  "Estou com dor de cabeça leve",
			expected: false,
		},
		{
			name:     "Cansaço - não crítico",
			message:  "Estou me sentindo cansado",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCriticalConcern(tt.message)
			if result != tt.expected {
				t.Errorf("IsCriticalConcern(%q) = %v, esperado %v", tt.message, result, tt.expected)
			}
		})
	}
}

func TestParseRiskLevel(t *testing.T) {
	tc := &ThinkingClient{}

	tests := []struct {
		name     string
		text     string
		expected RiskLevel
	}{
		{
			name:     "Crítico em português",
			text:     "risk_level: CRÍTICO",
			expected: RiskCritical,
		},
		{
			name:     "Alto em português",
			text:     "risk_level: ALTO",
			expected: RiskHigh,
		},
		{
			name:     "Médio em português",
			text:     "risk_level: MÉDIO",
			expected: RiskMedium,
		},
		{
			name:     "Baixo por padrão",
			text:     "risk_level: BAIXO",
			expected: RiskLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tc.parseRiskLevel(tt.text)
			if result != tt.expected {
				t.Errorf("parseRiskLevel(%q) = %v, esperado %v", tt.text, result, tt.expected)
			}
		})
	}
}
